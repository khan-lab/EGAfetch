# Configuration

## Credentials File

EGAfetch supports a JSON config file compatible with pyEGA3's `-cf` format:

```json title="credentials.json"
{
  "username": "your.email@example.com",
  "password": "your_password"
}
```

Use it with any command via `--cf` or `--config-file`:

```bash
egafetch auth login --cf credentials.json
egafetch download EGAD00001001938 --cf credentials.json
egafetch metadata EGAD00001001938 --cf credentials.json
```

!!! warning "Keep your credentials safe"
    Add `credentials.json` (or whatever you name it) to your `.gitignore`. The file contains your password in plain text.

## Stored Session

After logging in, EGAfetch stores your OAuth2 tokens at:

```
~/.egafetch/credentials.json
```

This file has `0600` permissions (owner read/write only) and contains:

```json
{
  "username": "your.email@example.com",
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "expires_at": "2025-02-10T14:30:00Z"
}
```

Tokens are automatically refreshed 5 minutes before expiry. You do not need to re-login between downloads unless the refresh token itself has expired.

## Commands That Accept `--cf`

| Command | Effect |
|---------|--------|
| `auth login` | Read credentials from file instead of prompting |
| `download` | Auto-login before downloading |
| `list` | Auto-login before listing dataset files |
| `info` | Auto-login before fetching file metadata |
| `metadata` | Auto-login + use password for metadata API |

When `--cf` is passed to a non-auth command, EGAfetch performs a fresh login before executing the command. This is useful for long-running jobs where a previous session may have expired.

## Token Lifetimes

Token lifetimes are set by EGA's servers and cannot be changed client-side:

| Token | Lifetime | Refresh |
|-------|----------|---------|
| Download API | ~1 hour | Automatic via refresh token |
| Metadata API | 300 seconds | Not needed (quick operation) |

EGAfetch handles refresh transparently. For the download API, tokens are refreshed 5 minutes before expiry using the refresh token. The metadata API token is short-lived but the metadata fetch completes well within 5 minutes.
