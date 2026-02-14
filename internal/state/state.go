package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileStatus represents the download state of a single file.
type FileStatus string

const (
	StatusPending     FileStatus = "pending"
	StatusChunking    FileStatus = "chunking"
	StatusDownloading FileStatus = "downloading"
	StatusMerging     FileStatus = "merging"
	StatusVerifying   FileStatus = "verifying"
	StatusComplete    FileStatus = "complete"
	StatusFailed      FileStatus = "failed"
)

// ChunkStatus represents the download state of a single chunk.
type ChunkStatus string

const (
	ChunkPending     ChunkStatus = "pending"
	ChunkDownloading ChunkStatus = "downloading"
	ChunkComplete    ChunkStatus = "complete"
	ChunkFailed      ChunkStatus = "failed"
)

// ChunkState tracks the state of a single chunk within a file download.
type ChunkState struct {
	Index           int         `json:"index"`
	Start           int64       `json:"start"`            // Byte offset (inclusive)
	End             int64       `json:"end"`              // Byte offset (exclusive)
	Status          ChunkStatus `json:"status"`
	BytesDownloaded int64       `json:"bytes_downloaded"`
	RetryCount      int         `json:"retry_count"`
}

// FileState tracks the complete state of a single file download.
type FileState struct {
	FileID           string       `json:"file_id"`
	FileName         string       `json:"file_name"`
	Status           FileStatus   `json:"status"`
	Size             int64        `json:"size"`
	ChecksumExpected string       `json:"checksum_expected"`
	ChecksumType     string       `json:"checksum_type"`
	ChunkSize        int64        `json:"chunk_size"`
	Chunks           []ChunkState `json:"chunks"`
	DownloadURL      string       `json:"download_url,omitempty"`
	URLExpiresAt     *time.Time   `json:"url_expires_at,omitempty"`
	Error            string       `json:"error,omitempty"`
	RetryCount       int          `json:"retry_count"`
	StartedAt        *time.Time   `json:"started_at,omitempty"`
	CompletedAt      *time.Time   `json:"completed_at,omitempty"`
}

// NewFileState creates a new FileState in pending status from a FileSpec.
func NewFileState(spec FileSpec, chunkSize int64) *FileState {
	now := time.Now()
	return &FileState{
		FileID:           spec.FileID,
		FileName:         spec.FileName,
		Status:           StatusPending,
		Size:             spec.Size,
		ChecksumExpected: spec.Checksum,
		ChecksumType:     spec.ChecksumType,
		ChunkSize:        chunkSize,
		StartedAt:        &now,
	}
}

// InitChunks divides the file into chunks based on ChunkSize.
// This is idempotent — if chunks already exist (resume case), it does nothing.
func (fs *FileState) InitChunks() {
	if len(fs.Chunks) > 0 {
		return
	}

	var chunks []ChunkState
	var offset int64
	index := 0

	for offset < fs.Size {
		end := offset + fs.ChunkSize
		if end > fs.Size {
			end = fs.Size
		}
		chunks = append(chunks, ChunkState{
			Index:  index,
			Start:  offset,
			End:    end,
			Status: ChunkPending,
		})
		offset = end
		index++
	}

	// Handle zero-size files: create a single empty chunk.
	if len(chunks) == 0 {
		chunks = append(chunks, ChunkState{
			Index:  0,
			Start:  0,
			End:    0,
			Status: ChunkPending,
		})
	}

	fs.Chunks = chunks
}

// IsComplete returns true if the file download is fully complete.
func (fs *FileState) IsComplete() bool {
	return fs.Status == StatusComplete
}

// PendingChunks returns pointers to chunks that are not yet complete.
func (fs *FileState) PendingChunks() []*ChunkState {
	var pending []*ChunkState
	for i := range fs.Chunks {
		if fs.Chunks[i].Status != ChunkComplete {
			pending = append(pending, &fs.Chunks[i])
		}
	}
	return pending
}

// AllChunksComplete returns true if every chunk is in the Complete state.
func (fs *FileState) AllChunksComplete() bool {
	for _, c := range fs.Chunks {
		if c.Status != ChunkComplete {
			return false
		}
	}
	return true
}

// fileStatePath returns the path to the state file for a given file ID.
func (sm *StateManager) fileStatePath(fileID string) string {
	return filepath.Join(sm.StatePath(), fileID+".json")
}

// LoadFileState reads a file's state from disk.
// Returns (nil, nil) if the file does not exist.
func (sm *StateManager) LoadFileState(fileID string) (*FileState, error) {
	data, err := os.ReadFile(sm.fileStatePath(fileID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read file state for %s: %w", fileID, err)
	}
	var fs FileState
	if err := json.Unmarshal(data, &fs); err != nil {
		return nil, fmt.Errorf("parse file state for %s: %w", fileID, err)
	}
	return &fs, nil
}

// SaveFileState writes a file's state to disk atomically.
func (sm *StateManager) SaveFileState(fs *FileState) error {
	if err := sm.EnsureDirs(); err != nil {
		return err
	}
	return atomicWriteJSON(sm.fileStatePath(fs.FileID), fs)
}

// DeleteFileState removes the state file for a given file ID.
func (sm *StateManager) DeleteFileState(fileID string) error {
	err := os.Remove(sm.fileStatePath(fileID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ListFileStates returns all file states found on disk.
func (sm *StateManager) ListFileStates() ([]*FileState, error) {
	pattern := filepath.Join(sm.StatePath(), "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob file states: %w", err)
	}

	var states []*FileState
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var fs FileState
		if err := json.Unmarshal(data, &fs); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		states = append(states, &fs)
	}
	return states, nil
}

// atomicWriteJSON marshals v to JSON and writes it atomically to path.
// It writes to a temp file in the same directory, syncs, then renames.
func atomicWriteJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, filePerm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file to %s: %w", path, err)
	}

	success = true
	return nil
}
