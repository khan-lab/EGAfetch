package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	egafetchDir  = ".egafetch"
	stateDir     = "state"
	chunksDir    = "chunks"
	manifestFile = "manifest.json"
	dirPerm      = 0755
	filePerm     = 0644
)

// FileSpec describes a file to be downloaded.
type FileSpec struct {
	FileID       string `json:"file_id"`
	FileName     string `json:"file_name"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
	ChecksumType string `json:"checksum_type"`
}

// Manifest tracks the overall download job.
type Manifest struct {
	DatasetID string     `json:"dataset_id,omitempty"`
	Files     []FileSpec `json:"files"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// StateManager handles persistence of manifests and file states.
type StateManager struct {
	baseDir string
}

// NewStateManager creates a state manager rooted at the given output directory.
func NewStateManager(baseDir string) *StateManager {
	return &StateManager{baseDir: baseDir}
}

// BaseDir returns the output directory this state manager is rooted at.
func (sm *StateManager) BaseDir() string {
	return sm.baseDir
}

// EgafetchPath returns the path to .egafetch/ under the base directory.
func (sm *StateManager) EgafetchPath() string {
	return filepath.Join(sm.baseDir, egafetchDir)
}

// StatePath returns the path to .egafetch/state/ under the base directory.
func (sm *StateManager) StatePath() string {
	return filepath.Join(sm.EgafetchPath(), stateDir)
}

// ChunksPath returns the path to .egafetch/chunks/ under the base directory.
func (sm *StateManager) ChunksPath() string {
	return filepath.Join(sm.EgafetchPath(), chunksDir)
}

// ChunksPathForFile returns the path to .egafetch/chunks/<fileID>/.
func (sm *StateManager) ChunksPathForFile(fileID string) string {
	return filepath.Join(sm.ChunksPath(), fileID)
}

// EnsureDirs creates the .egafetch directory structure if it does not exist.
func (sm *StateManager) EnsureDirs() error {
	dirs := []string{
		sm.EgafetchPath(),
		sm.StatePath(),
		sm.ChunksPath(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, dirPerm); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}
	return nil
}

func (sm *StateManager) manifestPath() string {
	return filepath.Join(sm.EgafetchPath(), manifestFile)
}

// LoadManifest reads the manifest from disk.
// Returns (nil, nil) if the file does not exist.
func (sm *StateManager) LoadManifest() (*Manifest, error) {
	data, err := os.ReadFile(sm.manifestPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// Reset removes all EGAfetch state (manifest, file states, chunks) from the
// output directory. This is used by --restart to force a fresh download.
func (sm *StateManager) Reset() error {
	return os.RemoveAll(sm.EgafetchPath())
}

// SaveManifest writes the manifest to disk atomically.
func (sm *StateManager) SaveManifest(m *Manifest) error {
	if err := sm.EnsureDirs(); err != nil {
		return err
	}
	m.UpdatedAt = time.Now()
	return atomicWriteJSON(sm.manifestPath(), m)
}
