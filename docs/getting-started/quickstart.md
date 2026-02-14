# Quick Start

This guide walks you through your first download with EGAfetch.

## Prerequisites

- An EGA account with access to at least one dataset
- EGAfetch binary installed (see [Installation](installation.md))

## Step 1: Log In

=== "Interactive"

    ```bash
    egafetch auth login
    ```

    You will be prompted for your EGA email and password (password is hidden).

=== "Config File"

    Create a JSON file with your credentials:

    ```json title="credentials.json"
    {
      "username": "your.email@example.com",
      "password": "your_password"
    }
    ```

    Then log in:

    ```bash
    egafetch auth login --cf credentials.json
    ```

Verify your session:

```bash
egafetch auth status
# Logged in as: your.email@example.com
# Token expires: 58m30s
```

## Step 2: Explore a Dataset

List the files in a dataset:

```bash
egafetch list EGAD00001001938
```

Output:

```
File ID              Size         Check  File Name
---------------------------------------------------------------------------
EGAF00001104661     500.0 MB     MD5    SLX-9630.A006.bwa.bam
EGAF00001104662     320.0 MB     MD5    SLX-9630.A007.bwa.bam
...

60 files, 25.3 GB total
```

## Step 3: Download

Download the entire dataset:

```bash
egafetch download EGAD00001001938 -o ./my-data
```

Or download specific files:

```bash
egafetch download EGAF00001104661 EGAF00001104662 -o ./my-data
```

You will see live progress for each file:

```
Downloading 60 file(s) to ./my-data
  SLX-9630.A006.bwa.bam  [========>         ] 45%  225.0 MB / 500.0 MB
  SLX-9630.A007.bwa.bam  [====>             ] 22%   70.4 MB / 320.0 MB
  SLX-9631.A001.bwa.bam  [waiting...]
```

## Step 4: Resume (If Interrupted)

If the download is interrupted (Ctrl+C, network failure, etc.), simply re-run the same command:

```bash
egafetch download EGAD00001001938 -o ./my-data
```

Completed files are skipped, partial files resume from the last byte.

## Step 5: Verify

Re-verify checksums of all completed files:

```bash
egafetch verify ./my-data
```

## Step 6: Clean Up

Remove temporary chunk files (keeps your completed downloads):

```bash
egafetch clean ./my-data
```

## Using a Config File Everywhere

Pass `--cf` to any command to skip interactive login:

```bash
egafetch download EGAD00001001938 -o ./data --cf credentials.json
egafetch list EGAD00001001938 --cf credentials.json
egafetch metadata EGAD00001001938 --cf credentials.json
```

This is especially useful for scripts and batch jobs on HPC clusters.

## Next Steps

- [Download command reference](../commands/download.md) -- all flags and tuning options
- [Metadata export](../commands/metadata.md) -- export dataset metadata as TSV/CSV/JSON
- [Configuration](configuration.md) -- credentials and config file details
