# Installation

## Homebrew (macOS / Linux)

```bash
brew install khan-lab/tap/egafetch
```

Or add the tap first:

```bash
brew tap khan-lab/tap
brew install egafetch
```

## From Source

Requires [Go 1.22+](https://go.dev/dl/).

```bash
git clone https://github.com/khan-lab/EGAfetch.git
cd EGAfetch
make build
```

The binary is built to `./bin/egafetch`. Copy it to a directory in your `$PATH`:

```bash
cp ./bin/egafetch /usr/local/bin/
```

Or install directly to `$GOPATH/bin`:

```bash
make install
```

## Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/khan-lab/EGAfetch/releases) page.

| Platform | Binary |
|----------|--------|
| Linux (x86_64) | `egafetch-linux-amd64` |
| Linux (ARM64) | `egafetch-linux-arm64` |
| macOS (Intel) | `egafetch-darwin-amd64` |
| macOS (Apple Silicon) | `egafetch-darwin-arm64` |
| Windows (x86_64) | `egafetch-windows-amd64.exe` |

After downloading:

```bash
chmod +x egafetch-linux-amd64
mv egafetch-linux-amd64 /usr/local/bin/egafetch
```

## Cross-Compile All Platforms

```bash
make release
ls bin/
# egafetch-linux-amd64
# egafetch-linux-arm64
# egafetch-darwin-amd64
# egafetch-darwin-arm64
# egafetch-windows-amd64.exe
```

## Verify Installation

```bash
egafetch --version
```

## HPC Clusters

EGAfetch is a statically-linked Go binary with zero runtime dependencies. Copy the single binary to your cluster -- no modules, conda environments, or pip installs needed.

```bash
scp ./bin/egafetch user@cluster:/home/user/bin/
```
