package verify

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// Verify computes the checksum of the file at filePath and compares it
// against the expected value. Returns nil on match, error on mismatch
// or if the file cannot be read.
func Verify(filePath string, expected string, checksumType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file for checksum: %w", err)
	}
	defer f.Close()

	h, err := newHash(checksumType)
	if err != nil {
		return err
	}

	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("read file for checksum: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

// ComputeChecksum returns the hex-encoded checksum of the file.
func ComputeChecksum(filePath string, checksumType string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file for checksum: %w", err)
	}
	defer f.Close()

	h, err := newHash(checksumType)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read file for checksum: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func newHash(checksumType string) (hash.Hash, error) {
	switch strings.ToUpper(checksumType) {
	case "MD5":
		return md5.New(), nil
	case "SHA256":
		return sha256.New(), nil
	default:
		return nil, fmt.Errorf("unsupported checksum type: %s", checksumType)
	}
}
