# Authentication

Manage your EGA session with the `auth` subcommands.

## Login

```bash
egafetch auth login [--cf FILE]
```

Authenticates with EGA and stores OAuth2 tokens locally.

**Interactive mode** (default):

```bash
egafetch auth login
# EGA Username (email): user@example.com
# EGA Password: ********
# Authenticating...
# Login successful!
```

**Config file mode:**

```bash
egafetch auth login --cf credentials.json
# Authenticating...
# Login successful!
```

### Flags

| Flag | Description |
|------|-------------|
| `--cf` | Path to JSON config file (`{"username":"...","password":"..."}`) |
| `--config-file` | Alias for `--cf` |

## Status

```bash
egafetch auth status
```

Shows the current authentication state without refreshing the token.

**Output when logged in:**

```
Logged in as: user@example.com
Token expires: 58m30s
```

**Output when not logged in:**

```
Not logged in. Run 'egafetch auth login' to authenticate.
```

## Logout

```bash
egafetch auth logout
```

Clears stored credentials from both memory and disk (`~/.egafetch/credentials.json`).

```
Logged out.
```
