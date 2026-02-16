# EGAfetch

> **Fast, parallel, resumable data/metadata downloads from the [European Genome-phenome Archive (EGA)](https://ega-archive.org/)**.

EGAfetch is a single-binary CLI tool alternative to [pyEGA3](https://github.com/EGA-archive/ega-download-client) with ~8x faster downloads, automatic resume, and robust error handling.

## Features

- **Parallel downloads** -- multiple files and multiple chunks per file downloaded simultaneously
- **Automatic resume** -- interrupted downloads pick up exactly where they stopped, no re-downloading
- **Checksum verification** -- MD5/SHA256 verified after every file before marking complete
- **Token auto-refresh** -- OAuth2 tokens refreshed transparently before expiry
- **Retry with backoff** -- exponential backoff with jitter on transient failures (network errors, 5xx, 429)
- **Metadata export** -- download dataset metadata as TSV, CSV, or JSON with a merged master file
- **Bandwidth throttling** -- cap total bandwidth with `--max-bandwidth` to avoid saturating shared network links
- **Config file** -- persist defaults in `~/.egafetch/config.yaml` so you don't repeat flags every time
- **File filtering** -- selectively download files with `--include`/`--exclude` glob patterns
- **Adaptive chunk sizing** -- auto-tune chunk size based on observed throughput with `--adaptive-chunks`
- **pyEGA3-compatible config** -- same `{"username":"...","password":"..."}` JSON config file format
- **Single binary** -- no Python, no pip, no dependencies; works on HPC clusters

## Installation

### From source

Requires [Go 1.22+](https://go.dev/dl/).

```bash
git clone https://github.com/khan-lab/EGAfetch.git
cd EGAfetch
make build
# Binary is at ./bin/egafetch
```

Or install directly to your `$GOPATH/bin`:

```bash
make install
```

### Cross-compile

```bash
make release
# Produces binaries for:
#   linux/amd64, linux/arm64
#   darwin/amd64, darwin/arm64
#   windows/amd64
```

## Quick Start

```bash
# 1. Log in to EGA
egafetch auth login
# Or with a config file (pyEGA3 compatible):
egafetch auth login --cf credentials.json

# 2. Download an entire dataset
egafetch download EGAD00001001938 -o ./data

# 3. Check progress
egafetch status ./data

# 4. If interrupted, just re-run the same command -- it resumes automatically
egafetch download EGAD00001001938 -o ./data
```

## Usage

### Help

```bash
egafetch --help

Usage:
  egafetch [command]

Available Commands:
  auth        Manage EGA authentication
  clean       Remove temp files, keep completed downloads
  completion  Generate the autocompletion script for the specified shell
  download    Download datasets or files from EGA
  help        Help about any command
  info        Show file metadata
  list        List authorized datasets, or files in a dataset
  metadata    Download dataset metadata (TSV, CSV, or JSON)
  status      Show download progress
  verify      Re-verify checksums of downloaded files

Flags:
  -h, --help      help for egafetch
  -v, --version   version for egafetch

Use "egafetch [command] --help" for more information about a command.
```

### Authentication

```bash
# Interactive login (password hidden)
egafetch auth login

# Login using a JSON config file
egafetch auth login --cf credentials.json

# Check current session
egafetch auth status

# Log out (clear stored credentials)
egafetch auth logout
```

Credentials are stored in `~/.egafetch/credentials.json` with `0600` permissions. Tokens auto-refresh before expiry.

**Credentials file format** (same as pyEGA3):

```json
{
  "username": "your.email@example.com",
  "password": "your_password"
}
```

### Config File

Persist default settings in `~/.egafetch/config.yaml` to avoid repeating flags. CLI flags always override config values.

```yaml
chunk_size: 128M
parallel_files: 4
parallel_chunks: 8
max_bandwidth: 500M
output_dir: /data/ega
metadata_format: tsv
```

All fields are optional. If the file doesn't exist, hardcoded defaults are used.

### Downloading

```bash
# Download all files in a dataset
egafetch download EGAD00001001938 -o ./data

# Download specific files
egafetch download EGAF00001104661 EGAF00001104662 -o ./data

# Tune parallelism
egafetch download EGAD00001001938 -o ./data \
    --parallel-files 8 \
    --parallel-chunks 16 \
    --chunk-size 128M

# Limit bandwidth (useful on shared HPC networks)
egafetch download EGAD00001001938 -o ./data --max-bandwidth 100M

# Download only BAM files, excluding unmapped
egafetch download EGAD00001001938 -o ./data \
    --include "*.bam" \
    --exclude "*_unmapped*"

# Auto-tune chunk sizes based on connection speed
egafetch download EGAD00001001938 -o ./data --adaptive-chunks

# Force a fresh start (discard all progress)
egafetch download EGAD00001001938 -o ./data --restart

# Use config file for non-interactive auth
egafetch download EGAD00001001938 -o ./data --cf credentials.json
```

**Download flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | `.` | Output directory |
| `--parallel-files` | `4` | Files downloaded simultaneously |
| `--parallel-chunks` | `8` | Chunks per file downloaded simultaneously |
| `--chunk-size` | `64M` | Chunk size (supports K, M, G suffixes) |
| `--max-bandwidth` | | Global bandwidth limit (e.g., `100M`, `1G`) |
| `--include` | | Glob patterns to include (matched against file name) |
| `--exclude` | | Glob patterns to exclude (matched against file name) |
| `--adaptive-chunks` | `false` | Auto-adjust chunk size based on throughput |
| `--no-metadata` | `false` | Skip downloading dataset metadata |
| `--metadata-format` | `tsv` | Metadata output format (tsv, csv, json) |
| `--restart` | `false` | Wipe existing progress and start fresh |
| `--cf, --config-file` | | JSON config file with credentials |

**Resume behavior:** Re-running the same download command automatically skips completed files and resumes partial ones. No separate resume command needed.

### Dataset Info

```bash
# List all authorized datasets
egafetch list

# List all files in a dataset
egafetch list EGAD00001001938

# Show metadata for a specific file
egafetch info EGAF00001104661
```

### Metadata Export

Dataset metadata is **downloaded automatically** when using `egafetch download` with a `--cf` config file. Use `--no-metadata` to skip, or `--metadata-format` to choose the format.

You can also download metadata independently:

```bash
# Export as TSV (default)
egafetch metadata EGAD00001001938

# Export as CSV
egafetch metadata EGAD00001001938 --format csv

# Export as JSON to a custom directory
egafetch metadata EGAD00001001938 --format json -o ./my-metadata

# Non-interactive with config file
egafetch metadata EGAD00001001938 --cf credentials.json
```

This creates individual mapping files plus a merged master metadata file:

```
EGAD00001001938-metadata/
  study_experiment_run_sample.tsv
  run_sample.tsv
  study_analysis_sample.tsv
  analysis_sample.tsv
  sample_file.tsv
  EGAD00001001938_merged_metadata.tsv
```

The `_merged_metadata.tsv` file merges individual metadata to create one main metadata file.

### Management

```bash
# Show download progress for a directory
egafetch status ./data

# Re-verify checksums of completed files
egafetch verify ./data

# Remove temporary chunk files (keeps completed downloads)
egafetch clean ./data
```

## How It Works

### File State Machine

Each file progresses through these states:

```
pending --> chunking --> downloading --> merging --> verifying --> complete
                            |              |
                            v              v
                          failed <---------+
```

State is persisted to disk after every transition, so downloads survive interruptions at any point.

### Chunk Downloads

Files are split into chunks (default 64 MB) and downloaded in parallel using HTTP Range requests. Each chunk:

- Writes to `.egafetch/chunks/{fileID}/{index}.part`
- Resumes from existing bytes on disk (append mode)
- Retries up to 5 times with exponential backoff (1s base, 60s max, plus jitter)

After all chunks complete, they are merged into the final file and verified against the expected checksum.

### Directory Layout

```
./data/
  .egafetch/
    manifest.json              # File list and dataset info
    state/
      EGAF00001104661.json     # Per-file download state
      EGAF00001104662.json
    chunks/
      EGAF00001104661/
        000.part               # Temporary chunk files
        001.part
      EGAF00001104662/
        ...
  SLX-9630.A006.bwa.bam       # Completed files
  SLX-9630.A007.bwa.bam
```

The `.egafetch/` directory is removed by `egafetch clean` after downloads complete.

## Comparison with pyEGA3

| | pyEGA3 | EGAfetch |
|---|--------|----------|
| Language | Python | Go (single binary) |
| Parallel files | 1 | Configurable (default 4) |
| Parallel chunks | 1 | Configurable (default 8) |
| Resume | Limited | Full (chunk-level, byte-precise) |
| Token refresh | Manual | Automatic |
| Bandwidth throttling | No | `--max-bandwidth` flag |
| File filtering | No | `--include`/`--exclude` glob patterns |
| Adaptive chunk sizing | No | `--adaptive-chunks` auto-tunes |
| Persistent config | No | `~/.egafetch/config.yaml` |
| Config file | `-cf credentials.json` | `--cf credentials.json` (compatible format) |
| Installation | pip install | Single binary, zero dependencies |
| Metadata export | No | TSV/CSV/JSON with master file (auto during download) |


