# Dataset & File Info

## List Authorized Datasets

```bash
egafetch list [--cf FILE]
```

When run without arguments, lists all datasets the authenticated user has access to.

```bash
egafetch list --cf credentials.json
```

```
Fetching authorized datasets...

Authorized datasets (3):

  EGAD00001001938
  EGAD00001003245
  EGAD00001005678
```

## List Files in a Dataset

```bash
egafetch list EGAD... [--cf FILE]
```

When a dataset ID is provided, lists all files in that dataset with their IDs, sizes, and checksums.

```bash
egafetch list EGAD00001001938
```

```
Fetching files for dataset EGAD00001001938...
File ID              Size         Check  File Name
---------------------------------------------------------------------------
EGAF00001104661     500.0 MB     MD5    SLX-9630.A006.bwa.bam
EGAF00001104662     320.0 MB     MD5    SLX-9630.A007.bwa.bam
EGAF00001104480     450.0 MB     MD5    SLX-9630.A005.bwa.bam
...

60 files, 25.3 GB total
```

### Flags

| Flag | Description |
|------|-------------|
| `--cf, --config-file` | JSON config file with credentials |

## Show File Metadata

```bash
egafetch info EGAF... [--cf FILE]
```

Displays detailed metadata for a single file.

```bash
egafetch info EGAF00001104661
```

```
File ID:       EGAF00001104661
File Name:     SLX-9630.A006.bwa.bam
File Size:     500.0 MB (524288000 bytes)
Checksum:      d41d8cd98f00b204e9800998ecf8427e
Checksum Type: MD5
Status:        available
```

### Flags

| Flag | Description |
|------|-------------|
| `--cf, --config-file` | JSON config file with credentials |
