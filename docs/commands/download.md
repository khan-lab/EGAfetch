# Download

Download datasets or individual files from EGA with parallel chunked downloads and automatic resume.

## Usage

```bash
egafetch download [EGAD.../EGAF.../file.txt] [flags]
```

Arguments can be dataset IDs (`EGAD...`), file IDs (`EGAF...`), or text files containing one identifier per line. See [Identifier Files](#identifier-files) below.

## Examples

```bash
# Download all files in a dataset
egafetch download EGAD00001001938 -o ./data

# Download specific files
egafetch download EGAF00001104661 EGAF00001104662 -o ./data

# Download from a list of identifiers in a text file
egafetch download identifiers.txt -o ./data --cf credentials.json

# Mix direct IDs and identifier files
egafetch download EGAF00000009999 identifiers.txt -o ./data

# Tune parallelism for fast networks
egafetch download EGAD00001001938 -o ./data \
    --parallel-files 8 \
    --parallel-chunks 16 \
    --chunk-size 128M

# Limit bandwidth on shared HPC networks
egafetch download EGAD00001001938 -o ./data --max-bandwidth 100M

# Download only BAM files, excluding unmapped
egafetch download EGAD00001001938 -o ./data \
    --include "*.bam" \
    --exclude "*_unmapped*"

# Auto-tune chunk sizes based on connection speed
egafetch download EGAD00001001938 -o ./data --adaptive-chunks

# Force a completely fresh download
egafetch download EGAD00001001938 -o ./data --restart

# Non-interactive (for scripts / HPC jobs)
egafetch download EGAD00001001938 -o ./data --cf credentials.json
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | `.` | Output directory for downloaded files |
| `--parallel-files` | `4` | Number of files downloaded simultaneously |
| `--parallel-chunks` | `8` | Number of chunks per file downloaded simultaneously |
| `--chunk-size` | `64M` | Size of each chunk (supports `K`, `M`, `G` suffixes) |
| `--max-bandwidth` | | Global bandwidth limit (e.g., `100M`, `1G`) |
| `--include` | | Glob patterns to include (matched against file name) |
| `--exclude` | | Glob patterns to exclude (matched against file name) |
| `--adaptive-chunks` | `false` | Auto-adjust chunk size based on throughput |
| `--no-metadata` | `false` | Skip downloading dataset metadata |
| `--metadata-format` | `tsv` | Metadata output format (`tsv`, `csv`, `json`) |
| `--restart` | `false` | Wipe all existing progress and start fresh |
| `--cf, --config-file` | | JSON config file with credentials |

## Automatic Resume

Re-running the same download command automatically resumes where it left off:

- **Completed files** are skipped instantly (no network call)
- **Partial files** resume from the last downloaded byte using HTTP Range requests
- **Failed files** are retried (up to 3 attempts per file)

```bash
# First run -- downloads 30 of 60 files, then interrupted
egafetch download EGAD00001001938 -o ./data
# ^C

# Second run -- skips the 30 completed files, resumes the rest
egafetch download EGAD00001001938 -o ./data
```

No separate `resume` command is needed.

## Fresh Start

If you want to discard all progress and re-download everything:

```bash
egafetch download EGAD00001001938 -o ./data --restart
```

This removes the `.egafetch/` state directory before proceeding.

## Tuning Performance

### Parallel Files

Controls how many files are downloaded at the same time. Increase this if you have many small files:

```bash
--parallel-files 8
```

### Parallel Chunks

Controls how many chunks of a single file are downloaded simultaneously. Increase for large files on fast networks:

```bash
--parallel-chunks 16
```

### Chunk Size

Controls the size of each chunk. Larger chunks mean fewer HTTP requests but coarser resume granularity:

```bash
--chunk-size 128M   # Good for fast, stable connections
--chunk-size 32M    # Good for unstable connections (finer resume)
```

### Bandwidth Throttling

Cap the total download bandwidth across all parallel connections. Useful on shared HPC networks where you should not saturate the link:

```bash
--max-bandwidth 100M   # Limit to 100 MB/s total
--max-bandwidth 1G     # Limit to 1 GB/s total
```

The limit is enforced globally -- all files and chunks share the same bandwidth pool.

### Adaptive Chunk Sizing

When enabled, EGAfetch monitors download throughput and automatically adjusts chunk sizes:

```bash
egafetch download EGAD00001001938 -o ./data --adaptive-chunks
```

- Chunks start at the default size (64 MB or `--chunk-size` value)
- Throughput is measured over a rolling window of 3 chunks
- **Fast connections** (> 50 MB/s avg): chunk size scaled up by 1.5x (max 256 MB)
- **Slow connections** (< 10 MB/s avg): chunk size scaled down by 0.5x (min 8 MB)

This is useful when you don't know the network speed in advance. Larger chunks reduce HTTP overhead on fast links, while smaller chunks reduce wasted work on failures with slow links.

### File Filtering

Selectively download files from a dataset using glob patterns:

```bash
# Only BAM files
egafetch download EGAD00001001938 -o ./data --include "*.bam"

# Everything except encrypted files
egafetch download EGAD00001001938 -o ./data --exclude "*.cip"

# Combine: only BAM files, but not unmapped ones
egafetch download EGAD00001001938 -o ./data \
    --include "*.bam" \
    --exclude "*_unmapped*"
```

- Patterns are matched against the **file name** only (not the full path)
- Uses Go's `filepath.Match` syntax (`*`, `?`, `[...]`)
- `--include`: file must match **at least one** include pattern
- `--exclude`: file is skipped if it matches **any** exclude pattern
- Explicitly named EGAF file IDs are never filtered out
- Multiple patterns can be specified by repeating the flag

### Metadata During Download

When downloading a dataset (EGAD) with `--cf`, metadata is fetched automatically after the data download completes. Use `--no-metadata` to skip, or `--metadata-format` to choose the format:

```bash
# Download data + metadata as TSV (default)
egafetch download EGAD00001001938 -o ./data --cf creds.json

# Download data + metadata as JSON
egafetch download EGAD00001001938 -o ./data --cf creds.json --metadata-format json

# Download data only, skip metadata
egafetch download EGAD00001001938 -o ./data --cf creds.json --no-metadata
```

Metadata files are saved to `{output}/{datasetID}-metadata/`. If metadata auth fails, the data download still succeeds with a warning.

### Identifier Files

Any argument that does not start with `EGAD` or `EGAF` is treated as a text file containing identifiers, one per line. This is useful for batch downloads from curated lists:

```text title="identifiers.txt"
# WGS samples from project X
EGAD00001002071

# Additional individual files
EGAF00000001234
EGAF00000005678
```

```bash
egafetch download identifiers.txt -o ./data --cf credentials.json
```

- Blank lines and lines starting with `#` are ignored
- Mixed `EGAD` and `EGAF` identifiers are allowed in the same file
- You can combine identifier files with direct IDs on the command line
- Errors include the filename and line number for easy debugging

### Output File Names

EGA stores files in encrypted `.cip` format. When downloading in plain (decrypted) mode (the default), EGAfetch automatically strips the `.cip` extension from output file names. For example, `sample.bam.cip` on the EGA server becomes `sample.bam` in your output directory.

### MD5 Checksum Files

After each file is downloaded and verified, EGAfetch writes an MD5 checksum sidecar file alongside the downloaded file. For example:

```
output-dir/
    EGAF00001104661/
        SLX-9630.A006.bwa.bam       # Downloaded file
        SLX-9630.A006.bwa.bam.md5   # MD5 checksum file
```

The `.md5` file uses standard `md5sum` format:

```
a1b2c3d4e5f6...  SLX-9630.A006.bwa.bam
```

This allows verification with standard tools: `cd output-dir/EGAF00001104661 && md5sum -c SLX-9630.A006.bwa.bam.md5`.

### Recommended Settings

| Scenario | Flags |
|----------|-------|
| HPC with fast network | `--parallel-files 8 --parallel-chunks 16 --chunk-size 128M` |
| HPC with shared link | `--parallel-files 4 --max-bandwidth 500M` |
| Laptop on WiFi | `--parallel-files 2 --parallel-chunks 4 --chunk-size 32M` |
| Unknown network | `--adaptive-chunks` |
| Many small files | `--parallel-files 16 --parallel-chunks 4` |
| Few large files | `--parallel-files 2 --parallel-chunks 16 --chunk-size 128M` |
| Only BAM files | `--include "*.bam"` |

## Progress Output

During download, a live progress display shows the state of each file:

```
Downloading 60 file(s) to ./data
  SLX-9630.A006.bwa.bam  [========>         ] 45%  225.0 MB / 500.0 MB
  SLX-9630.A007.bwa.bam  [==============>   ] 72%  230.4 MB / 320.0 MB
  SLX-9631.A001.bwa.bam  [=>                ]  8%   12.0 MB / 150.0 MB
  SLX-9631.A002.bwa.bam  [waiting...]
```

Status indicators:

| Display | Meaning |
|---------|---------|
| `[========>    ] 45%` | Actively downloading |
| `[========================] 100%` | Complete |
| `[---- skipped ----]` | Already complete from a previous run |
| `[---- FAILED  ----]` | Download failed after retries |
| `[waiting...]` | Queued, waiting for a parallel slot |

## Retry Behavior

EGAfetch automatically retries on transient errors:

- **Per chunk:** Up to 5 retries with exponential backoff (1s, 2s, 4s, 8s, 16s) plus random jitter, capped at 60 seconds
- **Per file:** Up to 3 retries of the entire file state machine
- **Retryable errors:** Network timeouts, connection resets, HTTP 5xx, HTTP 429 (rate limited)
- **Non-retryable errors:** HTTP 4xx (except 429), authentication failures

## Graceful Interruption

Pressing `Ctrl+C` triggers a graceful shutdown:

1. In-flight HTTP requests are cancelled
2. Current state is saved to disk
3. Partial chunk files are preserved for resume

You can safely interrupt at any time without data loss.
