# Migrating from pyEGA3

EGAfetch is a drop-in replacement for pyEGA3 with a similar interface. This guide maps pyEGA3 commands to their EGAfetch equivalents.

## Command Mapping

### Authentication

=== "pyEGA3"

    ```bash
    # pyEGA3 reads credentials from -cf on every command
    pyega3 -cf credentials.json fetch EGAD00001001938
    ```

=== "EGAfetch"

    ```bash
    # Option 1: Login once, then run commands
    egafetch auth login --cf credentials.json
    egafetch download EGAD00001001938

    # Option 2: Pass --cf to each command (same as pyEGA3)
    egafetch download EGAD00001001938 --cf credentials.json
    ```

### Download a Dataset

=== "pyEGA3"

    ```bash
    pyega3 -cf creds.json fetch EGAD00001001938 --output-dir ./data
    ```

=== "EGAfetch"

    ```bash
    egafetch download EGAD00001001938 -o ./data --cf creds.json
    ```

### Download Specific Files

=== "pyEGA3"

    ```bash
    pyega3 -cf creds.json fetch EGAF00001104661
    ```

=== "EGAfetch"

    ```bash
    egafetch download EGAF00001104661 --cf creds.json
    ```

### List Dataset Files

=== "pyEGA3"

    ```bash
    pyega3 -cf creds.json files EGAD00001001938
    ```

=== "EGAfetch"

    ```bash
    egafetch list EGAD00001001938 --cf creds.json
    ```

## Config File

The credentials file format is identical:

```json title="credentials.json"
{
  "username": "your.email@example.com",
  "password": "your_password"
}
```

The only difference is the flag name: pyEGA3 uses `-cf`, EGAfetch uses `--cf` (double dash).

## Key Differences

| Feature | pyEGA3 | EGAfetch |
|---------|--------|----------|
| **Parallelism** | Sequential (1 file, 1 stream) | Configurable (4 files x 8 chunks default) |
| **Resume** | Re-downloads from scratch | Byte-precise resume with HTTP Range |
| **Token refresh** | Fails when token expires | Automatic refresh before expiry |
| **Progress** | Basic text output | Live progress bars per file |
| **Interruption** | May corrupt state | Safe at any point (atomic state writes) |
| **Metadata** | Not available | `egafetch metadata` exports TSV/CSV/JSON |
| **Metadata auto-download** | Not available | Auto-fetched after dataset download (with `--cf`) |
| **Bandwidth throttling** | Not available | `--max-bandwidth` global limit |
| **File filtering** | Not available | `--include` / `--exclude` glob patterns |
| **Adaptive chunk sizing** | Not available | `--adaptive-chunks` auto-tunes based on throughput |
| **Persistent config** | Not available | `~/.egafetch/config.yaml` for default settings |
| **Installation** | Python + pip | Single binary, zero dependencies |
| **Batch file input** | Not available | Text file with identifiers (one per line) |
| **MD5 sidecar files** | Not available | `.md5` file written alongside each download |
| **`.cip` stripping** | Strips `.cip` extension | Strips `.cip` extension (same behavior) |
| **Checksum** | After download | After download (same, but automatic) |

## New Features in EGAfetch

Features not available in pyEGA3:

- **`egafetch metadata`** -- Export dataset metadata as TSV, CSV, or JSON with a merged master file
- **`egafetch status`** -- Check download progress without re-running the download
- **`egafetch verify`** -- Re-verify checksums at any time
- **`egafetch clean`** -- Remove temporary files while keeping completed downloads
- **`--restart`** -- Force a fresh download, discarding all progress
- **`--parallel-files` / `--parallel-chunks` / `--chunk-size`** -- Fine-grained control over download parallelism
- **`--max-bandwidth`** -- Global bandwidth throttling (e.g., `100M`, `1G`)
- **`--include` / `--exclude`** -- Glob-based file filtering (e.g., `--include "*.bam"`)
- **`--adaptive-chunks`** -- Auto-adjust chunk sizes based on measured throughput
- **`--no-metadata` / `--metadata-format`** -- Control automatic metadata download during dataset downloads
- **`~/.egafetch/config.yaml`** -- Persistent config file for default settings (chunk size, parallelism, bandwidth, output dir)
- **Identifier files** -- Pass a text file with one EGAD/EGAF per line instead of listing IDs on the command line
- **MD5 sidecar files** -- `.md5` checksum file written alongside each downloaded file for easy verification
