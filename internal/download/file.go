package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/khan-lab/EGAfetch/internal/api"
	"github.com/khan-lab/EGAfetch/internal/state"
	"github.com/khan-lab/EGAfetch/internal/verify"
)

const (
	maxFileRetries = 3
	// EGA files in plain mode have 16 bytes of IV stripped.
	ivSize = 16
)

// DownloadOptions holds configuration for a download session.
type DownloadOptions struct {
	ParallelFiles  int
	ParallelChunks int
	ChunkSize      int64
}

// ProgressCallback is called to report download progress.
type ProgressCallback func(fileID string, bytesDownloaded int64, totalBytes int64)

// FileDownload manages the download of a single file through the state machine.
type FileDownload struct {
	spec            state.FileSpec
	apiClient       *api.Client
	stateManager    *state.StateManager
	opts            DownloadOptions
	fstate          *state.FileState
	mu              sync.Mutex
	onProgress    ProgressCallback
	liveBytesSoFar int64 // running total for live progress, updated by chunk callbacks
}

// NewFileDownload creates a new file download task.
func NewFileDownload(
	spec state.FileSpec,
	apiClient *api.Client,
	stateManager *state.StateManager,
	opts DownloadOptions,
	onProgress ProgressCallback,
) *FileDownload {
	return &FileDownload{
		spec:         spec,
		apiClient:    apiClient,
		stateManager: stateManager,
		opts:         opts,
		onProgress:   onProgress,
	}
}

// Run executes the file download state machine.
func (fd *FileDownload) Run(ctx context.Context) error {
	// Load or create state.
	existing, err := fd.stateManager.LoadFileState(fd.spec.FileID)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	if existing != nil {
		fd.fstate = existing
	} else {
		fd.fstate = state.NewFileState(fd.spec, fd.opts.ChunkSize)
	}

	for {
		select {
		case <-ctx.Done():
			fd.saveState()
			return ctx.Err()
		default:
		}

		if err := fd.saveState(); err != nil {
			return fmt.Errorf("save state: %w", err)
		}

		switch fd.fstate.Status {
		case state.StatusPending, state.StatusChunking:
			fd.fstate.InitChunks()
			fd.fstate.Status = state.StatusDownloading

		case state.StatusDownloading:
			downloadURL := fd.apiClient.FileDownloadURL(fd.fstate.FileID)
			fd.fstate.DownloadURL = downloadURL

			if err := fd.downloadChunks(ctx); err != nil {
				return fd.fail(err)
			}
			fd.fstate.Status = state.StatusMerging

		case state.StatusMerging:
			if err := fd.mergeChunks(); err != nil {
				return fd.fail(err)
			}
			fd.fstate.Status = state.StatusVerifying

		case state.StatusVerifying:
			if err := fd.verifyChecksum(); err != nil {
				return fd.fail(err)
			}
			fd.fstate.Status = state.StatusComplete
			now := time.Now()
			fd.fstate.CompletedAt = &now
			fd.saveState()
			fd.cleanup()
			return nil

		case state.StatusComplete:
			return nil

		case state.StatusFailed:
			if fd.fstate.RetryCount < maxFileRetries {
				fd.fstate.RetryCount++
				fd.fstate.Status = state.StatusDownloading
				fd.fstate.Error = ""
				continue
			}
			return fmt.Errorf("download failed after %d retries: %s", fd.fstate.RetryCount, fd.fstate.Error)
		}
	}
}

// downloadChunks downloads all pending chunks in parallel.
func (fd *FileDownload) downloadChunks(ctx context.Context) error {
	chunksDir := fd.stateManager.ChunksPathForFile(fd.fstate.FileID)
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		return fmt.Errorf("create chunks directory: %w", err)
	}

	// Seed liveBytesSoFar with bytes already downloaded (resume case).
	fd.liveBytesSoFar = fd.bytesDownloaded()

	pending := fd.fstate.PendingChunks()
	if len(pending) == 0 {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, fd.opts.ParallelChunks)

	for _, chunk := range pending {
		chunk := chunk
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}

			// Per-byte callback: atomically update running total and notify UI.
			onBytes := func(n int64) {
				fd.mu.Lock()
				fd.liveBytesSoFar += n
				current := fd.liveBytesSoFar
				fd.mu.Unlock()
				if fd.onProgress != nil {
					fd.onProgress(fd.fstate.FileID, current, fd.fstate.Size)
				}
			}

			downloader := NewChunkDownloader(fd.apiClient, fd.fstate.DownloadURL, chunksDir, onBytes)
			err := downloader.Download(ctx, chunk)

			// Save state after each chunk completes (or fails).
			fd.mu.Lock()
			fd.saveState()
			fd.mu.Unlock()

			return err
		})
	}

	return g.Wait()
}

// bytesDownloaded returns the total bytes downloaded across all chunks.
func (fd *FileDownload) bytesDownloaded() int64 {
	var total int64
	for _, c := range fd.fstate.Chunks {
		total += c.BytesDownloaded
	}
	return total
}

// mergeChunks concatenates all chunk files into the final output file.
func (fd *FileDownload) mergeChunks() error {
	chunksDir := fd.stateManager.ChunksPathForFile(fd.fstate.FileID)
	outputPath := filepath.Join(fd.stateManager.BaseDir(), fd.fstate.FileName)

	return MergeChunks(chunksDir, outputPath, fd.fstate.Chunks)
}

// verifyChecksum verifies the downloaded file against the expected checksum.
func (fd *FileDownload) verifyChecksum() error {
	outputPath := filepath.Join(fd.stateManager.BaseDir(), fd.fstate.FileName)

	if fd.fstate.ChecksumExpected == "" {
		return nil // No checksum to verify.
	}

	return verify.Verify(outputPath, fd.fstate.ChecksumExpected, fd.fstate.ChecksumType)
}

// cleanup removes chunk files after successful verification.
func (fd *FileDownload) cleanup() {
	chunksDir := fd.stateManager.ChunksPathForFile(fd.fstate.FileID)
	os.RemoveAll(chunksDir)
}

// fail transitions the file to failed state.
func (fd *FileDownload) fail(err error) error {
	fd.fstate.Status = state.StatusFailed
	fd.fstate.Error = err.Error()
	fd.saveState()
	return err
}

// saveState persists the current file state to disk.
func (fd *FileDownload) saveState() error {
	return fd.stateManager.SaveFileState(fd.fstate)
}
