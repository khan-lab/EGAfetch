# Management Commands

## Status

```bash
egafetch status [directory]
```

Shows the download progress for all files tracked in the given directory.

```bash
egafetch status ./data
```

```
File ID              Status          Size         Progress  File Name
--------------------------------------------------------------------------------
EGAF00001104661     complete        500.0 MB     100%      SLX-9630.A006.bwa.bam
EGAF00001104662     downloading     320.0 MB     45.3%     SLX-9630.A007.bwa.bam
EGAF00001104480     pending         450.0 MB     -         SLX-9630.A005.bwa.bam
```

Default directory is `.` (current directory).

## Verify

```bash
egafetch verify [directory]
```

Re-verifies checksums (MD5 or SHA256) of all completed files.

```bash
egafetch verify ./data
```

```
  OK    SLX-9630.A006.bwa.bam
  OK    SLX-9630.A007.bwa.bam
  SKIP  SLX-9630.A005.bwa.bam (status: downloading)

2 passed, 0 failed, 1 skipped
```

| Status | Meaning |
|--------|---------|
| `OK` | Checksum matches expected value |
| `FAIL` | Checksum mismatch -- file may be corrupted |
| `SKIP` | File not yet complete, or no checksum available |

If any files fail verification, the command exits with a non-zero status code.

## Clean

```bash
egafetch clean [directory]
```

Removes temporary files while keeping completed downloads:

- Deletes all chunk files (`.egafetch/chunks/`)
- Removes state files for completed downloads
- Keeps state files for incomplete downloads (so they can still resume)

```bash
egafetch clean ./data
```

```
Removing chunk files from ./data/.egafetch/chunks/...
Cleaned 45 completed state file(s).
```

!!! note
    `clean` does not remove the final downloaded files -- only the temporary chunks and state tracking files.
