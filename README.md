# basar (بصر)

[![CI](https://github.com/calilkhalil/basar/workflows/CI/badge.svg)](https://github.com/calilkhalil/basar/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![Go Report Card](https://goreportcard.com/badge/github.com/calilkhalil/basar)](https://goreportcard.com/report/github.com/calilkhalil/basar)

> Volatility3 ISF symbol cache manager. Fetches, merges, and caches ISF banner files from multiple upstream sources.

*basar* (بصر) means "vision" or "insight" in Arabic — what symbols provide to Volatility3.

## Why?

[Volatility3](https://github.com/volatilityfoundation/volatility3) is a memory forensics framework. To analyze a memory dump, it needs **symbol files (ISF)** that match the target system's kernel version.

The problem: finding the right ISF file manually is tedious. You need to:
1. Extract the kernel banner from the memory dump
2. Search community repositories for a matching symbol file
3. Download and configure it

**basar automates this.** It maintains a local cache of kernel banners → symbol URL mappings from multiple community sources ([Abyss-W4tcher](https://github.com/Abyss-W4tcher/volatility3-symbols), [leludo84](https://github.com/leludo84/vol3-linux-profiles), etc.). When you run Volatility3 with `$(basar)`, it automatically finds the right symbols.

```
┌─────────────┐     ┌─────────────┐     ┌──────────────────┐
│ memory.dmp  │────▶│ volatility3 │────▶│ basar cache      │
│             │     │ -u $(basar) │     │ (banners.json)   │
└─────────────┘     └─────────────┘     └────────┬─────────┘
                                                 │
                           ┌─────────────────────┼─────────────────────┐
                           ▼                     ▼                     ▼
                    ┌─────────────┐       ┌─────────────┐       ┌─────────────┐
                    │ Abyss repo  │       │ leludo repo │       │ local ISFs  │
                    └─────────────┘       └─────────────┘       └─────────────┘
```

## Features

- 🔄 **Automatic Updates**: Fetches and merges ISF banners from multiple sources
- ⚡ **Fast Caching**: Local cache with configurable TTL to minimize network requests
- 🔒 **Safe Concurrency**: File locking prevents race conditions during updates
- 📦 **XDG Compliant**: Follows XDG Base Directory Specification
- 🛠️ **Easy Integration**: Simple CLI that works seamlessly with Volatility3
- ⚙️ **Configurable**: Custom sources via simple config file

## Quick Start

```sh
# Install and setup (does everything)
git clone https://github.com/calilkhalil/basar
cd basar
./install.sh
```

That's it. Now just use Volatility3 normally:

```sh
volatility3 -f memory.dmp linux.pslist
```

The installer:
1. Builds and installs basar
2. Creates config with default sources
3. Downloads ISF banner index
4. Configures Volatility3 to use basar automatically
5. Sets up systemd timer (auto-updates every 2 weeks)

## Installation

### From Source

```sh
git clone https://github.com/calilkhalil/basar
cd basar
./install.sh                    # Install to ~/.local/bin
./install.sh /usr/local         # Install to /usr/local/bin (requires sudo)
```

### Using Make

```sh
make && sudo make install       # System-wide installation
make && make install-user       # User installation (~/.local/bin)
```

## Usage

basar maintains the ISF symbol cache service in `~/.cache/basar/` (or `$XDG_CACHE_HOME/basar/`).

### Basic Usage

```sh
# Use basar to provide the cache URI to volatility3
volatility3 -u $(basar) -f memory.dmp linux.pslist

# Check cache status
basar -c

# View cache statistics
basar -s

# Force update cache
basar --update
```

### Examples

```sh
# Ensure cache is up-to-date and use with volatility3
volatility3 -u $(basar) -f dump.raw linux.pslist

# Get cache file path for direct use
CACHE_PATH=$(basar -p)
volatility3 -u file://$CACHE_PATH -f dump.raw linux.bash

# Check if cache needs updating
if ! basar -c; then
    echo "Cache expired, updating..."
    basar --update
fi
```

## Commands

```
basar                  # ensure cache & print URI
basar -p               # print cache path
basar -s               # print stats as JSON
basar -c               # check validity (exit 0/2)
basar --update         # force update (re-download all)
basar --smart-update   # update only if sources changed
basar --clear          # remove cache
basar --init           # create config file
basar --setup          # complete setup (config + update + vol3 + systemd)
basar --install-service    # install systemd timer only
basar --configure-vol3     # configure volatility3 only
```

## Configuration

Sources are configured in `~/.config/basar/sources.conf`:

```
# One URL or local path per line
https://raw.githubusercontent.com/Abyss-W4tcher/volatility3-symbols/master/banners/banners.json
https://raw.githubusercontent.com/leludo84/vol3-linux-profiles/main/banners-isf.json
/path/to/local/banners.json
```

Create default config:

```sh
basar --init
```

## Environment

| Variable | Description | Default |
|----------|-------------|---------|
| `BASAR_TTL` | Cache TTL in seconds | 86400 |
| `BASAR_VERBOSE` | Enable verbose output | (unset) |
| `XDG_CACHE_HOME` | Cache directory | ~/.cache |
| `XDG_CONFIG_HOME` | Config directory | ~/.config |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success / cache valid |
| 1 | Error |
| 2 | Cache invalid (with `-c`) |

## How It Works

1. **On first run** (or when cache expires): basar fetches banner files from all configured sources concurrently
2. **Merges** all banners into a single JSON file, deduplicating URLs per kernel version
3. **Caches** the result in `~/.cache/basar/banners.json`
4. **Prints** the `file://` URI that Volatility3's `-u` flag expects

When the same kernel banner exists in multiple sources, basar keeps all symbol URLs as fallbacks:

```json
{
  "linux": {
    "Linux version 5.15.0-91-generic ...": [
      "https://github.com/Abyss-W4tcher/.../5.15.0-91-generic.json.xz",
      "https://github.com/leludo84/.../5.15.0-91-generic.json.xz"
    ]
  }
}
```

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) file for details.
