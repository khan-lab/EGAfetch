package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds persistent user defaults from ~/.egafetch/config.yaml.
// Zero values mean "not set" — the caller should fall back to hardcoded defaults.
type Config struct {
	ChunkSize      string `yaml:"chunk_size"`
	ParallelFiles  int    `yaml:"parallel_files"`
	ParallelChunks int    `yaml:"parallel_chunks"`
	MaxBandwidth   string `yaml:"max_bandwidth"`
	OutputDir      string `yaml:"output_dir"`
	MetadataFormat string `yaml:"metadata_format"`
}

const configFileName = "config.yaml"

// Load reads ~/.egafetch/config.yaml and returns the parsed config.
// Returns a zero-valued Config (not an error) if the file does not exist.
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Config{}, nil
	}

	path := filepath.Join(home, ".egafetch", configFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	return &cfg, nil
}
