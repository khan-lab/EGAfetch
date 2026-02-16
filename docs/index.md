# EGAfetch

**Fast, parallel, resumable download of data and metadata from the [European Genome-phenome Archive (EGA)](https://ega-archive.org/).**

EGAfetch is a GO-based command-line tool an alternative to [pyEGA3](https://github.com/EGA-archive/ega-download-client) with significantly faster downloads, automatic resume, and robust error handling.

---

## Why EGAfetch?

Downloading large genomic datasets from EGA with pyEGA3 is slow, fragile, and requires manual inspection. EGAfetch solves this:

- **Parallel downloads** -- multiple files and multiple chunks per file downloaded simultaneously
- **Automatic resume** -- interrupted downloads pick up exactly where they stopped
- **Checksum verification** -- MD5/SHA256 verified after every file
- **Token auto-refresh** -- OAuth2 tokens refreshed transparently before expiry
- **Retry with backoff** -- exponential backoff with jitter on transient failures
- **Metadata export** -- download dataset metadata as TSV, CSV, or JSON (auto-fetched during download)
- **Bandwidth throttling** -- cap total bandwidth with `--max-bandwidth` for shared HPC networks
- **Config file** -- persist defaults in `~/.egafetch/config.yaml`
- **File filtering** -- selectively download with `--include`/`--exclude` glob patterns
- **Adaptive chunk sizing** -- auto-tune chunk size based on throughput with `--adaptive-chunks`
- **Batch file input** -- pass a text file with identifiers (one per line, `#` comments supported)
- **MD5 sidecar files** -- `.md5` checksum file written alongside each downloaded file
- **Single binary** -- no Python, no pip, no dependencies; works on HPC clusters

## Quick Example

```bash
# Log in
egafetch auth login --cf credentials.json

# Download an entire dataset with parallel chunked downloads
egafetch download EGAD00001001938 -o ./data

# Interrupted? Just re-run -- it resumes automatically
egafetch download EGAD00001001938 -o ./data

# Metadata is auto-fetched during download (when using --cf)
# Or export independently:
egafetch metadata EGAD00001001938 --cf credentials.json
```

## At a Glance

| | pyEGA3 | EGAfetch |
|---|--------|----------|
| Parallel files | 1 | Configurable (default 4) |
| Parallel chunks | 1 | Configurable (default 8) |
| Resume | Limited | Full (chunk-level, byte-precise) |
| Token refresh | Manual | Automatic |
| Bandwidth throttling | No | `--max-bandwidth` |
| File filtering | No | `--include` / `--exclude` globs |
| Adaptive chunks | No | `--adaptive-chunks` |
| Persistent config | No | `~/.egafetch/config.yaml` |
| Installation | `pip install` | Single binary |
| Batch file input | No | Text file with identifiers |
| MD5 sidecar files | No | `.md5` per file |
| Metadata export | No | TSV / CSV / JSON (auto during download) |

Ready to get started? Head to the [Installation](getting-started/installation.md) guide.
