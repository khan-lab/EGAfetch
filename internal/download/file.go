package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/khan-lab/EGAfetch/internal/api"
	"github.com/khan-lab/EGAfetch/internal/state"
	"github.com/khan-lab/EGAfetch/internal/verify"
)

const (
	maxFileRetries = 3
	// EGA files in plain mode have 16 bytes of IV stripped.
	ivSize = 16

	// Adaptive chunk sizing constants.
	minAdaptiveChunkSize = 8 * 1024 * 1024   // 8 MB
	maxAdaptiveChunkSize = 256 * 1024 * 1024  // 256 MB
	adaptiveWindowSize   = 3                  // rolling window of throughput measurements
	highThroughputMBps   = 50.0               // above this: scale up
	lowThroughputMBps    = 10.0               // below this: scale down
	scaleUpFactor        = 1.5
	scaleDownFactor      = 0.5
)

// DownloadOptions holds configuration for a download session.
type DownloadOptions struct {
	ParallelFiles    int
	ParallelChunks   int
	ChunkSize        int64
	Limiter          *rate.Limiter // nil = no throttling; shared across all goroutines
	AdaptiveChunking bool          // auto-adjust chunk size based on throughput
}

// ProgressCallback is called to report download progress.
type ProgressCallback func(fileID string, bytesDownloaded int64, totalBytes int64)

// adaptiveState tracks throughput measurements for adaptive chunk sizing.
type adaptiveState struct {
	mu               sync.Mutex
	measurements     []float64 // last N throughputs in bytes/sec
	currentChunkSize int64
}

func newAdaptiveState(initialChunkSize int64) *adaptiveState {
	return &adaptiveState{currentChunkSize: initialChunkSize}
}

// recordAndAdjust records a chunk's throughput and adjusts the chunk size.
func (a *adaptiveState) recordAndAdjust(bytesDownloaded int64, duration time.Duration) int64 {
	a.mu.Lock()
	defer a.mu.Unlock()

	if duration <= 0 {
		return a.currentChunkSize
	}

	bps := float64(bytesDownloaded) / duration.Seconds()
	a.measurements = append(a.measurements, bps)
	if len(a.measurements) > adaptiveWindowSize {
		a.measurements = a.measurements[len(a.measurements)-adaptiveWindowSize:]
	}

	// Need a full window before adjusting.
	if len(a.measurements) < adaptiveWindowSize {
		return a.currentChunkSize
	}

	var total float64
	for _, m := range a.measurements {
		total += m
	}
	avgMBps := (total / float64(len(a.measurements))) / (1024 * 1024)

	newSize := a.currentChunkSize
	if avgMBps > highThroughputMBps {
		newSize = int64(float64(a.currentChunkSize) * scaleUpFactor)
	} else if avgMBps < lowThroughputMBps {
		newSize = int64(float64(a.currentChunkSize) * scaleDownFactor)
	}

	if newSize < minAdaptiveChunkSize {
		newSize = minAdaptiveChunkSize
	}
	if newSize > maxAdaptiveChunkSize {
		newSize = maxAdaptiveChunkSize
	}

	a.currentChunkSize = newSize
	return newSize
}

// FileDownload manages the download of a single file through the state machine.
type FileDownload struct {
	spec           state.FileSpec
	apiClient      *api.Client
	stateManager   *state.StateManager
	opts           DownloadOptions
	fstate         *state.FileState
	mu             sync.Mutex
	onProgress     ProgressCallback
	liveBytesSoFar int64          // running total for live progress, updated by chunk callbacks
	adaptive       *adaptiveState // nil if adaptive chunking disabled
}

// NewFileDownload creates a new file download task.
func NewFileDownload(
	spec state.FileSpec,
	apiClient *api.Client,
	stateManager *state.StateManager,
	opts DownloadOptions,
	onProgress ProgressCallback,
) *FileDownload {
	fd := &FileDownload{
		spec:         spec,
		apiClient:    apiClient,
		stateManager: stateManager,
		opts:         opts,
		onProgress:   onProgress,
	}
	if opts.AdaptiveChunking {
		fd.adaptive = newAdaptiveState(opts.ChunkSize)
	}
	return fd
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
			if err := fd.writeMD5File(); err != nil {
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

	if fd.adaptive == nil {
		// Non-adaptive: dispatch all pending chunks at once.
		return fd.downloadChunksBatch(ctx, chunksDir, pending)
	}

	// Adaptive: dispatch in waves, rechunk remaining after each wave.
	for len(pending) > 0 {
		batchSize := fd.opts.ParallelChunks
		if batchSize > len(pending) {
			batchSize = len(pending)
		}
		batch := pending[:batchSize]
		pending = pending[batchSize:]

		if err := fd.downloadChunksBatch(ctx, chunksDir, batch); err != nil {
			return err
		}

		// After each batch, check if adaptive sizing wants to rechunk.
		if len(pending) > 0 {
			fd.adaptive.mu.Lock()
			newSize := fd.adaptive.currentChunkSize
			fd.adaptive.mu.Unlock()

			if newSize != fd.fstate.ChunkSize {
				fd.rechunkRemaining(newSize)
				pending = fd.fstate.PendingChunks()
			}
		}
	}
	return nil
}

// downloadChunksBatch downloads a batch of chunks concurrently.
func (fd *FileDownload) downloadChunksBatch(ctx context.Context, chunksDir string, chunks []*state.ChunkState) error {
	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, fd.opts.ParallelChunks)

	for _, chunk := range chunks {
		chunk := chunk
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}

			startTime := time.Now()

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

			downloader := NewChunkDownloader(fd.apiClient, fd.fstate.DownloadURL, chunksDir, onBytes, fd.opts.Limiter)
			err := downloader.Download(ctx, chunk)

			// Record throughput for adaptive sizing.
			if err == nil && fd.adaptive != nil {
				elapsed := time.Since(startTime)
				chunkBytes := chunk.End - chunk.Start
				fd.adaptive.recordAndAdjust(chunkBytes, elapsed)
			}

			// Save state after each chunk completes (or fails).
			fd.mu.Lock()
			fd.saveState()
			fd.mu.Unlock()

			return err
		})
	}

	return g.Wait()
}

// rechunkRemaining re-splits all pending chunks using the new chunk size.
// Completed chunks are preserved; only not-yet-started chunks are resized.
func (fd *FileDownload) rechunkRemaining(newChunkSize int64) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	var completedChunks []state.ChunkState
	var pendingStart int64 = -1

	for _, c := range fd.fstate.Chunks {
		if c.Status == state.ChunkComplete {
			completedChunks = append(completedChunks, c)
		} else if pendingStart == -1 {
			pendingStart = c.Start
		}
	}

	if pendingStart == -1 {
		return // all complete
	}

	// Re-create chunks from pendingStart to file end using newChunkSize.
	var newChunks []state.ChunkState
	offset := pendingStart
	index := len(completedChunks)
	for offset < fd.fstate.Size {
		end := offset + newChunkSize
		if end > fd.fstate.Size {
			end = fd.fstate.Size
		}
		newChunks = append(newChunks, state.ChunkState{
			Index:  index,
			Start:  offset,
			End:    end,
			Status: state.ChunkPending,
		})
		offset = end
		index++
	}

	fd.fstate.Chunks = append(completedChunks, newChunks...)
	fd.fstate.ChunkSize = newChunkSize
	fd.saveState()
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

// writeMD5File computes the MD5 checksum of the downloaded file and writes it
// to a .md5 sidecar file in standard md5sum format.
func (fd *FileDownload) writeMD5File() error {
	outputPath := filepath.Join(fd.stateManager.BaseDir(), fd.fstate.FileName)
	md5sum, err := verify.ComputeChecksum(outputPath, "MD5")
	if err != nil {
		return fmt.Errorf("compute MD5: %w", err)
	}
	md5Path := outputPath + ".md5"
	content := fmt.Sprintf("%s  %s\n", md5sum, filepath.Base(fd.fstate.FileName))
	if err := os.WriteFile(md5Path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write MD5 file: %w", err)
	}
	return nil
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
