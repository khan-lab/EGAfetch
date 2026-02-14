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
- **Metadata export** -- download dataset metadata as TSV, CSV, or JSON
- **Single binary** -- no Python, no pip, no dependencies; works on HPC clusters

## Quick Example

```bash
# Log in
egafetch auth login --cf credentials.json

# Download an entire dataset with parallel chunked downloads
egafetch download EGAD00001001938 -o ./data

# Interrupted? Just re-run -- it resumes automatically
egafetch download EGAD00001001938 -o ./data

# Export metadata as TSV
egafetch metadata EGAD00001001938 --cf credentials.json
```

## At a Glance

| | pyEGA3 | EGAfetch |
|---|--------|----------|
| Parallel files | 1 | Configurable (default 4) |
| Parallel chunks | 1 | Configurable (default 8) |
| Resume | Limited | Full (chunk-level, byte-precise) |
| Token refresh | Manual | Automatic |
| Installation | `pip install` | Single binary |
| Metadata export | No | TSV / CSV / JSON |

Ready to get started? Head to the [Installation](getting-started/installation.md) guide.
