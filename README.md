# Alloy

**Alloy** is a fast, opinionated package manager that installs software directly onto your system, prioritizing speed, breadth, and correctness over isolation.

It aims to answer a simple question:

> Why can't one tool install *everything* quickly — and remove it cleanly?

Alloy installs packages natively, tracks every file it touches, and guarantees complete removal on uninstall. No sandboxes, no virtual filesystems, no long-lived background services — just fast installs and a clean system.

---

## Installation

### Build from Source

Alloy is written in Go. To build from source:

```bash
# Clone the repository
git clone https://github.com/anthropics/alloy.git
cd alloy

# Build the binary
go build -o alloy ./cmd/alloy

# Install to your PATH (optional)
sudo mv alloy /usr/local/bin/
```

**Requirements:**
- Go 1.24.2 or later
- git (for cloning repositories)
- tar (for extracting archives)

To verify your installation:

```bash
alloy version
alloy doctor
```

---

## Quick Start

```bash
# Install a package
alloy install ripgrep

# List installed packages
alloy list

# Get information about a package
alloy info ripgrep

# Remove a package (all files tracked and cleaned up)
alloy remove ripgrep

# Check system health
alloy doctor
```

---

## Commands

### `alloy install <package>`

Install a package to your system.

```bash
# Install latest version
alloy install ripgrep

# Install with verbose output
alloy install --verbose ripgrep

# Preview what would happen without making changes
alloy install --dry-run ripgrep

# Install a specific version
alloy install --version 14.0.0 ripgrep
```

**Options:**
| Option | Description |
|--------|-------------|
| `--dry-run` | Show what would happen without making changes |
| `--verbose` | Show detailed output |
| `--version <ver>` | Install a specific version |

### `alloy remove <package>`

Remove an installed package. Alloy tracks every file created during installation and removes them cleanly.

```bash
# Remove a package
alloy remove ripgrep

# Preview removal without making changes
alloy remove --dry-run ripgrep

# Force removal even if files were modified externally
alloy remove --force ripgrep

# Show detailed removal output
alloy remove --verbose ripgrep
```

**Options:**
| Option | Description |
|--------|-------------|
| `--dry-run` | Show what would happen without making changes |
| `--verbose` | Show detailed output |
| `--force` | Force removal even if files were modified |

### `alloy list`

List all installed packages.

```bash
# List installed packages
alloy list

# List with detailed information (install time, file counts)
alloy list --verbose
```

**Options:**
| Option | Description |
|--------|-------------|
| `--verbose` | Show detailed information for each package |

### `alloy info <package>`

Show information about a package, whether installed or available.

```bash
alloy info ripgrep
```

Output includes:
- Package version, description, homepage, and license
- Source information (URL, git repo, or binary)
- Installation status and file counts (if installed)

### `alloy doctor`

Check system health and diagnose issues.

```bash
# Basic health check
alloy doctor

# Detailed output
alloy doctor --verbose

# Also verify installed files exist with correct checksums
alloy doctor --check-files
```

**Options:**
| Option | Description |
|--------|-------------|
| `--verbose` | Show detailed output |
| `--check-files` | Verify installed files exist and have correct checksums |

The doctor command checks:
- Directory permissions (~/.alloy)
- Package definitions directory
- Write permissions to install paths (/usr/local/bin, etc.)
- Required tools (git, tar)
- Ledger integrity for installed packages
- Orphaned backup files

---

## Design Principles

- **Speed first**
  Installs should be limited by network and disk, not dependency solvers or abstraction layers.

- **Broad coverage**
  System packages, CLI tools, language runtimes, and single binaries should all be installable through one interface.

- **Native by default**
  Packages are installed directly onto the host system, using conventional locations.

- **Strict uninstall hygiene**
  Every file created, modified, or removed during install is recorded. Uninstall means *nothing left behind*.

- **Opinionated, minimal UX**
  Few commands. Predictable behavior. No endless configuration.

---

## What Alloy Is (and Is Not)

**Alloy is:**
- A general-purpose package manager
- System-native and fast
- Focused on install → use → remove lifecycle correctness
- Designed to work well for individuals and small teams

**Alloy is not:**
- A container runtime
- A source-based distribution
- A full replacement for every ecosystem's build tools
- A sandboxing or isolation system

---

## Package Definitions

Packages are defined in TOML files in the `packages/` directory. Each package definition specifies where to download the software and how to install it.

### Example: Simple Binary Package

```toml
name = "ripgrep"
version = "14.1.0"
description = "Fast line-oriented search tool that recursively searches directories"
homepage = "https://github.com/BurntSushi/ripgrep"
license = "MIT"

[source]
url = "https://github.com/BurntSushi/ripgrep/releases/download/{{version}}/ripgrep-{{version}}-{{arch}}-unknown-{{os}}-gnu.tar.gz"
sha256 = "1a4f0a15e249f7e1c7e8c8b9de7c5ea5e6c2e8a7d6f3c4b5a6e7f8d9c0b1a2e3"
strip = 1

[install_paths]
prefix = "/usr/local"

[[install_steps]]
type = "copy"
src = "rg"
dest = "{{bindir}}/rg"
mode = "0755"

[[install_steps]]
type = "copy"
src = "doc/rg.1"
dest = "{{mandir}}/man1/rg.1"
```

### Example: Package with Shell Completions

```toml
name = "zoxide"
version = "0.9.6"
description = "A smarter cd command that learns your habits"
homepage = "https://github.com/ajeetdsouza/zoxide"
license = "MIT"

[source]
url = "https://github.com/ajeetdsouza/zoxide/releases/download/v{{version}}/zoxide-{{version}}-{{arch}}-{{os}}.tar.gz"
sha256 = "f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2"
strip = 0

[install_paths]
prefix = "/usr/local"

[[install_steps]]
type = "copy"
src = "zoxide"
dest = "{{bindir}}/zoxide"
mode = "0755"

[[install_steps]]
type = "mkdir"
path = "{{datadir}}/bash-completion/completions"

[[install_steps]]
type = "copy"
src = "completions/zoxide.bash"
dest = "{{datadir}}/bash-completion/completions/zoxide"
```

### Schema Overview

For the complete package definition schema, see [`packages/SCHEMA.md`](packages/SCHEMA.md).

**Source Types:**
- `url` - Download from archive (tar.gz, tar.xz, zip)
- `git` - Clone from git repository
- `binary` - Direct binary download

**Install Step Types:**
- `copy` - Copy files to destination
- `run` - Execute shell commands
- `mkdir` - Create directories
- `symlink` - Create symbolic links

**Template Variables:**
- `{{name}}`, `{{version}}` - Package metadata
- `{{prefix}}`, `{{bindir}}`, `{{libdir}}` - Install paths
- `{{arch}}`, `{{os}}` - System architecture (amd64/arm64) and OS (darwin/linux)

---

## Available Packages

Alloy currently includes definitions for popular CLI tools:

| Package | Description |
|---------|-------------|
| `bat` | A cat clone with syntax highlighting |
| `delta` | A syntax-highlighting pager for git |
| `eza` | A modern replacement for ls |
| `fd` | A simple, fast alternative to find |
| `fzf` | A command-line fuzzy finder |
| `jq` | A lightweight JSON processor |
| `lazygit` | A simple terminal UI for git |
| `ripgrep` | Fast line-oriented search tool |
| `starship` | A minimal, fast shell prompt |
| `zoxide` | A smarter cd command |

---

## How It Works

1. **Install**: Alloy downloads the package source, extracts it, and executes the install steps. Every file operation is recorded in a ledger (`~/.alloy/ledger/<package>.ledger`).

2. **Track**: The ledger stores checksums of created files and backups of any overwritten files.

3. **Remove**: On uninstall, Alloy replays the ledger in reverse, removing created files and restoring any backups.

This ensures complete removal with no orphaned files.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
