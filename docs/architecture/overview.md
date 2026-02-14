# Architecture

## Overview

```
CLI (cobra)
    |
Orchestrator              Manages file-level parallelism
    |
FileDownload              Per-file state machine
    |
ChunkDownloader           HTTP Range requests + retries
    |
Auth Manager              Automatic token refresh
```

## Package Structure

```
egafetch/
  cmd/egafetch/main.go       CLI entry point and command wiring
  internal/
    auth/
      auth.go                 OAuth2 token management + refresh
      credentials.go          Credential storage (~/.egafetch/)
    api/
      client.go               EGA API client (metadata + download)
      types.go                API response types
    download/
      orchestrator.go         Parallel file coordination
      file.go                 Single file state machine
      chunk.go                Chunk downloader with retries
      merge.go                Chunk merging into final file
    state/
      manifest.go             Download manifest management
      state.go                Per-file state persistence
    verify/
      checksum.go             MD5/SHA256 verification
    ui/
      progress.go             Terminal progress bars
      status.go               Status display formatting
```

## Orchestrator

The orchestrator manages file-level parallelism using a semaphore pattern:

1. Receives a manifest (list of files to download)
2. Launches one goroutine per file
3. Each goroutine checks if the file is already complete **before** acquiring a semaphore slot
4. Up to `--parallel-files` (default 4) files download simultaneously
5. Uses `errgroup` for cancellation propagation -- if one file fails fatally, all are cancelled

## File State Machine

Each file progresses through a deterministic state machine:

```
pending --> chunking --> downloading --> merging --> verifying --> complete
                            |              |
                            v              v
                          failed <---------+
```

| State | Description |
|-------|-------------|
| `pending` | Initial state, chunks not yet created |
| `chunking` | Splitting file into chunk ranges |
| `downloading` | Actively downloading chunks in parallel |
| `merging` | Concatenating chunk files into the final output |
| `verifying` | Validating checksum (MD5/SHA256) |
| `complete` | Download successful, chunks cleaned up |
| `failed` | Failed after retries; may be retried at file level |

State is **persisted to disk after every transition**. This means you can interrupt at any point and resume cleanly.

## Chunk Downloader

Files are split into chunks (default 64 MB) and downloaded in parallel:

1. Each chunk is assigned a byte range (`start` to `end`)
2. An HTTP Range request fetches exactly those bytes
3. The response is streamed to a `.part` file in append mode
4. If a `.part` file already has bytes on disk, the Range header starts from the existing size
5. On completion, the chunk state is marked `complete`

**Retry logic:** Up to 5 retries per chunk with exponential backoff (1s base, 60s max) plus random jitter (0-1000ms).

## Disk Layout

```
./output-dir/
    .egafetch/
        manifest.json              File list and dataset info
        state/
            EGAF00001104661.json   Per-file state (status, chunks, progress)
        chunks/
            EGAF00001104661/
                000.part           Temporary chunk files
                001.part
                002.part
    SLX-9630.A006.bwa.bam         Completed files (after merge + verify)
```

All JSON state files are written atomically (temp file + fsync + rename) to prevent corruption on crashes.

## Authentication Flow

EGAfetch uses two separate OAuth2 Identity Providers:

**Download API** (`ega.ebi.ac.uk:8443`):

- `grant_type=password` with EGA OIDC client credentials
- Tokens last ~1 hour
- Auto-refreshed 5 minutes before expiry using the refresh token
- Stored in `~/.egafetch/credentials.json`

**Metadata API** (`idp.ega-archive.org`):

- Separate IdP with `client_id=metadata-api`
- Tokens last 300 seconds
- Not persisted (fetched on-demand for metadata commands)

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `golang.org/x/sync` | `errgroup` for goroutine coordination |
| `golang.org/x/term` | Hidden password input |
