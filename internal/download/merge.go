package download

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/khan-lab/EGAfetch/internal/state"
)

// MergeChunks concatenates chunk files into a single output file.
// It writes to a temp file first, then renames for atomicity.
func MergeChunks(chunksDir string, outputPath string, chunks []state.ChunkState) error {
	// Ensure output directory exists.
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	tmpPath := outputPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp output file: %w", err)
	}

	success := false
	defer func() {
		out.Close()
		if !success {
			os.Remove(tmpPath)
		}
	}()

	for _, chunk := range chunks {
		chunkPath := ChunkPath(chunksDir, chunk.Index)
		if err := appendFile(out, chunkPath); err != nil {
			return fmt.Errorf("merge chunk %d: %w", chunk.Index, err)
		}
	}

	if err := out.Sync(); err != nil {
		return fmt.Errorf("sync output file: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close output file: %w", err)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		return fmt.Errorf("rename output file: %w", err)
	}

	success = true
	return nil
}

// appendFile appends the contents of src to dst.
func appendFile(dst *os.File, srcPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open chunk %s: %w", srcPath, err)
	}
	defer src.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy chunk %s: %w", srcPath, err)
	}

	return nil
}
