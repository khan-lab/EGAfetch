# Changelog

All notable changes to EGAfetch are documented here.

---

## v1.1.0 (2026-02-16)

### New Features

- **Bandwidth throttling** -- Global rate limit across all connections with `--max-bandwidth` (e.g., `100M`, `1G`). Uses `golang.org/x/time/rate.Limiter` shared across all goroutines.
- **Adaptive chunk sizing** -- `--adaptive-chunks` monitors throughput over a rolling window and auto-adjusts chunk sizes (8 MB -- 256 MB) based on connection speed.
- **File filtering** -- `--include` / `--exclude` glob patterns to selectively download files from a dataset (e.g., `--include "*.bam"`).
- **Persistent configuration** -- `~/.egafetch/config.yaml` for default settings (chunk size, parallelism, bandwidth, output dir, metadata format). CLI flags override config values.
- **Identifier file input** -- Pass a text file with one EGAD/EGAF identifier per line instead of listing IDs on the command line. Blank lines and `#` comments are supported.
- **MD5 sidecar files** -- `.md5` checksum file in standard `md5sum` format written alongside each downloaded file after verification.
- **Automatic metadata download** -- Dataset metadata (TSV/CSV/JSON + PEP) is fetched automatically after data download when using `--cf`. Control with `--no-metadata` and `--metadata-format`.
- **Dataset details in `list`** -- `egafetch list` now shows dataset title, description, and number of samples from the public metadata API.

### Bug Fixes

- **Checksum verification** -- `GetChecksum()` no longer falls back to the encrypted file's checksum (`Checksum` field). Only `PlainChecksum` and `UnencryptedChecksum` are used, preventing false mismatches when downloading in plain mode.
- **`.cip` extension stripping** -- Output filenames now have the `.cip` extension stripped (e.g., `sample.bam.cip` becomes `sample.bam`), matching pyEGA3 behavior.
- **Resume safety (HTTP 200 vs 206)** -- When resuming a partial chunk, if the server returns HTTP 200 (ignoring the Range header) instead of 206 Partial Content, the existing `.part` file is now truncated instead of appended to, preventing data corruption.

### Other Changes

- Added `golang.org/x/time` and `gopkg.in/yaml.v3` dependencies.
- New `internal/config` package for YAML configuration.
- Adaptive chunk sizing logic in `internal/download/file.go` with batch dispatch and rechunking.
- Updated documentation across all pages.

---

## v1.0.0

### Initial Release

- **Two-level parallelism** -- Configurable parallel files (default 4) and parallel chunks per file (default 8) for maximum throughput.
- **Chunked downloads** -- Files split into configurable chunks (default 64 MB) downloaded via HTTP Range requests.
- **Byte-precise resume** -- Interrupted downloads resume from the exact byte using persisted chunk state and append-mode writes.
- **Atomic state persistence** -- All state files written via temp file + fsync + rename to prevent corruption on crash.
- **Automatic token refresh** -- OAuth2 tokens refreshed 5 minutes before expiry using the refresh token.
- **Checksum verification** -- MD5/SHA256 verification after merge, with standalone `verify` command.
- **Exponential backoff with jitter** -- Up to 5 retries per chunk (1s base, 60s max) and 3 retries per file.
- **Graceful shutdown** -- SIGINT/SIGTERM saves state and preserves partial chunks for resume.
- **Subcommands** -- `auth login/status/logout`, `download`, `list`, `info`, `metadata`, `status`, `verify`, `clean`.
- **pyEGA3-compatible credentials** -- Same JSON format (`{"username": "...", "password": "..."}`).
- **Single static binary** -- No runtime dependencies; cross-compiled for Linux, macOS, and Windows.
