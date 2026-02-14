package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/khan-lab/EGAfetch/internal/api"
	"github.com/khan-lab/EGAfetch/internal/state"
)

const (
	maxChunkRetries = 5
	baseDelay       = 1 * time.Second
	maxDelay        = 60 * time.Second
)

// BytesWrittenCallback is called during streaming with the number of new bytes written.
type BytesWrittenCallback func(n int64)

// ChunkDownloader downloads a single chunk of a file using HTTP Range requests.
type ChunkDownloader struct {
	apiClient      *api.Client
	downloadURL    string
	chunksDir      string
	onBytesWritten BytesWrittenCallback
}

// NewChunkDownloader creates a chunk downloader for the given file.
func NewChunkDownloader(apiClient *api.Client, downloadURL string, chunksDir string, onBytes BytesWrittenCallback) *ChunkDownloader {
	return &ChunkDownloader{
		apiClient:      apiClient,
		downloadURL:    downloadURL,
		chunksDir:      chunksDir,
		onBytesWritten: onBytes,
	}
}

// Download downloads the chunk with retry logic and exponential backoff.
func (d *ChunkDownloader) Download(ctx context.Context, chunk *state.ChunkState) error {
	var lastErr error

	for attempt := 0; attempt <= maxChunkRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1))
			if delay > maxDelay {
				delay = maxDelay
			}
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay + jitter):
			}
		}

		lastErr = d.attemptDownload(ctx, chunk)
		if lastErr == nil {
			return nil
		}

		if !isRetryableError(lastErr) {
			return fmt.Errorf("non-retryable error: %w", lastErr)
		}

		chunk.RetryCount++
		chunk.Status = state.ChunkFailed
	}

	return fmt.Errorf("chunk %d failed after %d retries: %w", chunk.Index, maxChunkRetries, lastErr)
}

// attemptDownload performs a single download attempt for a chunk.
func (d *ChunkDownloader) attemptDownload(ctx context.Context, chunk *state.ChunkState) error {
	chunkPath := d.chunkPath(chunk.Index)

	// Check existing progress for resume.
	var existingSize int64
	if info, err := os.Stat(chunkPath); err == nil {
		existingSize = info.Size()
	}

	expectedSize := chunk.End - chunk.Start
	if expectedSize == 0 {
		// Zero-size chunk (empty file), just create the file.
		f, err := os.Create(chunkPath)
		if err != nil {
			return err
		}
		f.Close()
		chunk.Status = state.ChunkComplete
		chunk.BytesDownloaded = 0
		return nil
	}

	if existingSize >= expectedSize {
		// Already complete from a previous run — report the bytes for progress display.
		if d.onBytesWritten != nil && chunk.BytesDownloaded < expectedSize {
			d.onBytesWritten(expectedSize - chunk.BytesDownloaded)
		}
		chunk.Status = state.ChunkComplete
		chunk.BytesDownloaded = expectedSize
		return nil
	}

	// Build request with Range header.
	req, err := d.apiClient.NewAuthenticatedRequest(ctx, "GET", d.downloadURL)
	if err != nil {
		return err
	}

	rangeStart := chunk.Start + existingSize
	rangeEnd := chunk.End - 1 // HTTP Range is inclusive
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))

	resp, err := d.apiClient.DoStreamRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Ensure chunks directory exists.
	if err := os.MkdirAll(filepath.Dir(chunkPath), 0755); err != nil {
		return fmt.Errorf("create chunk directory: %w", err)
	}

	// Open file in append mode for resume.
	f, err := os.OpenFile(chunkPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Use a progress-aware writer so the UI updates during streaming.
	var written int64
	buf := make([]byte, 32*1024)
	for {
		nr, readErr := resp.Body.Read(buf)
		if nr > 0 {
			nw, writeErr := f.Write(buf[:nr])
			if writeErr != nil {
				return writeErr
			}
			written += int64(nw)
			chunk.BytesDownloaded = existingSize + written
			if d.onBytesWritten != nil {
				d.onBytesWritten(int64(nw))
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	chunk.Status = state.ChunkComplete
	return nil
}

// chunkPath returns the path to the chunk file on disk.
func (d *ChunkDownloader) chunkPath(index int) string {
	return filepath.Join(d.chunksDir, fmt.Sprintf("%03d.part", index))
}

// ChunkPath returns the path to a chunk file (exported for merge).
func ChunkPath(chunksDir string, index int) string {
	return filepath.Join(chunksDir, fmt.Sprintf("%03d.part", index))
}

// isRetryableError checks whether an error is worth retrying.
// Note: net.Error must be checked BEFORE context errors because Go's net.Dialer
// wraps dial timeouts with context.DeadlineExceeded internally. Without this
// ordering, dial timeouts would be incorrectly classified as non-retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors and timeouts are retryable (check FIRST — dial timeouts
	// wrap context.DeadlineExceeded internally, so this must come before the
	// context check below).
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Context cancellation by the caller is not retryable.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// API errors: retry 5xx and 429, not 4xx.
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRetryable()
	}

	// Default: retry (network glitch, connection reset, etc.)
	return true
}
