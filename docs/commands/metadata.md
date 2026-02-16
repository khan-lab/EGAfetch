# Metadata Export

Download dataset metadata from the EGA Private Metadata API and export as TSV, CSV, or JSON.

!!! tip "Auto-download during `egafetch download`"
    When downloading a dataset with `--cf`, metadata is fetched automatically after the data download completes. You only need the standalone `metadata` command if you want metadata without downloading files, or need to re-fetch metadata separately.

## Usage

```bash
egafetch metadata EGAD... [flags]
```

## Examples

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

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-f, --format` | `tsv` | Output format: `tsv`, `csv`, or `json` |
| `-o, --output` | `{datasetID}-metadata` | Output directory |
| `--cf, --config-file` | | JSON config file with credentials |

## Output Files

The command creates a directory with individual mapping files plus a merged file:

```
EGAD00001001938-metadata/
    study_experiment_run_sample.tsv    # Study/experiment/run/sample mappings
    run_sample.tsv                     # Run-to-sample mappings
    study_analysis_sample.tsv          # Study/analysis/sample mappings
    analysis_sample.tsv                # Analysis-to-sample mappings
    sample_file.tsv                    # Sample-to-file mappings
    EGAD00001001938_merged_metadata.tsv # Merged main file
```

### Merged Metadata File

This file merges `study_experiment_run_sample` with `sample_file` on the `sample_accession_id` column. This gives you a single wide table linking studies, experiments, runs, samples, and files.

If a column name exists in both tables, the `sample_file` column is prefixed with `file_` to avoid collisions.

## Authentication

The metadata API uses a **separate Identity Provider** from the download API. This means:

- **With `--cf`:** Credentials are read from the file -- no prompts
- **Without `--cf`:** You must be logged in (`egafetch auth login` first), and you will be prompted for your password

The metadata token is short-lived (300 seconds) but the entire metadata fetch completes well within that window.

## EGA Mapping Endpoints

EGAfetch fetches from these EGA Private Metadata API endpoints:

| Mapping | Description |
|---------|-------------|
| `study_experiment_run_sample` | Links studies to experiments, runs, and samples |
| `run_sample` | Maps sequencing runs to samples |
| `study_analysis_sample` | Links studies to analyses and samples |
| `analysis_sample` | Maps analyses to samples |
| `sample_file` | Maps samples to their EGA file accessions and filenames |
