# Download

Download datasets or individual files from EGA with parallel chunked downloads and automatic resume.

## Usage

```bash
egafetch download [EGAD.../EGAF...] [flags]
```

## Examples

```bash
# Download all files in a dataset
egafetch download EGAD00001001938 -o ./data

# Download specific files
egafetch download EGAF00001104661 EGAF00001104662 -o ./data

# Tune parallelism for fast networks
egafetch download EGAD00001001938 -o ./data \
    --parallel-files 8 \
    --parallel-chunks 16 \
    --chunk-size 128M

# Download only BAM files from a dataset
egafetch download EGAD00001001938 -o ./data -f BAM

# Force a completely fresh download
egafetch download EGAD00001001938 -o ./data --restart

# Non-interactive (for scripts / HPC jobs)
egafetch download EGAD00001001938 -o ./data --cf credentials.json
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | `.` | Output directory for downloaded files |
| `-f, --format` | | Download only files of this type (e.g., `BAM`, `CRAM`, `VCF`, `BCF`) |
| `--parallel-files` | `4` | Number of files downloaded simultaneously |
| `--parallel-chunks` | `8` | Number of chunks per file downloaded simultaneously |
| `--chunk-size` | `64M` | Size of each chunk (supports `K`, `M`, `G` suffixes) |
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

### Recommended Settings

| Scenario | Flags |
|----------|-------|
| HPC with fast network | `--parallel-files 8 --parallel-chunks 16 --chunk-size 128M` |
| Laptop on WiFi | `--parallel-files 2 --parallel-chunks 4 --chunk-size 32M` |
| Many small files | `--parallel-files 16 --parallel-chunks 4` |
| Few large files | `--parallel-files 2 --parallel-chunks 16 --chunk-size 128M` |

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
