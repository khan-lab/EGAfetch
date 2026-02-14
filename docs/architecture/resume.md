# Resume & Recovery

EGAfetch is designed to be safe to interrupt at any point. This page explains how resume works at each level.

## How Resume Works

When you re-run a download command, EGAfetch checks existing state at three levels:

### 1. File Level

The orchestrator loads each file's state from `.egafetch/state/{fileID}.json` **before** acquiring a download slot:

- **`complete`** -- file is skipped instantly (no network call)
- **`downloading`** / `merging` / `verifying` -- file is resumed from its current state
- **`failed`** -- file is retried (up to 3 times)
- **`pending`** -- file starts fresh

### 2. Chunk Level

Within a file, only incomplete chunks are processed. The file state machine calls `PendingChunks()` which returns chunks not in the `complete` state.

For each pending chunk, the downloader checks the `.part` file on disk:

- If the `.part` file has `N` bytes already, the HTTP Range request starts at byte `chunk.Start + N`
- If the `.part` file is complete (size matches expected), the chunk is marked complete without a network call

### 3. Byte Level

HTTP Range requests resume from the exact byte where the previous download stopped:

```
Range: bytes=1048576-2097151
```

The `.part` file is opened in append mode (`O_APPEND`), so new bytes are added after existing content.

## State Persistence

State is saved to disk at critical points:

| Event | What's Saved |
|-------|-------------|
| File state transition | `{fileID}.json` updated with new status |
| Chunk completion | `{fileID}.json` updated with chunk marked `complete` |
| Download start | Manifest saved to `manifest.json` |
| Graceful shutdown (Ctrl+C) | In-progress state preserved as-is |

All writes are **atomic** (write to temp file, fsync, rename). This prevents corruption if the process is killed during a write.

## Scenarios

### Network Failure Mid-Download

```bash
egafetch download EGAD00001001938 -o ./data
# Network goes down after 30 files complete, 2 files partially downloaded

egafetch download EGAD00001001938 -o ./data
# 30 files skipped, 2 files resume from partial state, remaining files download
```

### Ctrl+C During Download

```bash
egafetch download EGAD00001001938 -o ./data
# ^C pressed -- "Interrupted. Saving state..."

egafetch download EGAD00001001938 -o ./data
# Resumes from exactly where it stopped
```

### Process Kill (SIGKILL / OOM)

Even without graceful shutdown, resume works because:

- Chunk `.part` files are written incrementally
- State files are written atomically (no partial JSON)
- The chunk downloader detects existing bytes on disk via file size

The worst case is re-downloading the bytes written since the last state save (at most one chunk's worth of data).

### Force Fresh Start

```bash
egafetch download EGAD00001001938 -o ./data --restart
# Removes .egafetch/ directory and starts from scratch
```

!!! warning
    `--restart` deletes all download state including partial chunks. Completed files in the output directory are **not** deleted, but they will be re-downloaded and overwritten.

## Idempotency

Running the same download command multiple times is idempotent:

- Complete files are skipped
- The manifest is overwritten with the same content
- No data is duplicated or corrupted
- Checksums are verified before marking any file complete
