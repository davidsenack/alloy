# Alloy Package Definition Schema

Package definitions use TOML format. Each package is defined in its own `.toml` file.

## Schema Reference

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Package identifier (lowercase, alphanumeric, hyphens allowed) |
| `version` | string | Package version (semver preferred) |

### Source Definition (required)

Exactly one source type must be specified in the `[source]` table:

| Field | Type | Description |
|-------|------|-------------|
| `url` | string | URL to downloadable archive (tar.gz, tar.xz, tar.bz2, zip) |
| `git` | string | Git repository URL |
| `binary` | string | URL to standalone binary |

Additional source options:

| Field | Type | Description |
|-------|------|-------------|
| `sha256` | string | SHA256 checksum for verification (required for url/binary) |
| `ref` | string | Git ref (tag, branch, commit) for git sources |
| `strip` | integer | Number of leading path components to strip from archive (default: 1) |

### Install Steps (required)

`[[install_steps]]` is an ordered array of installation actions. Each step has a `type` and type-specific fields.

#### Step Types

**`run`** - Execute a shell command
```toml
[[install_steps]]
type = "run"
command = "make install PREFIX={{prefix}}"
workdir = "src"  # optional, relative to source root
```

**`copy`** - Copy files to destination
```toml
[[install_steps]]
type = "copy"
src = "bin/ripgrep"
dest = "{{bindir}}/rg"
mode = "0755"  # optional, defaults to source mode
```

**`mkdir`** - Create directory
```toml
[[install_steps]]
type = "mkdir"
path = "{{datadir}}/myapp"
mode = "0755"  # optional, defaults to 0755
```

**`symlink`** - Create symbolic link
```toml
[[install_steps]]
type = "symlink"
src = "{{bindir}}/node-20"
dest = "{{bindir}}/node"
```

### Install Paths

The `[install_paths]` table defines where package files are installed. All paths are recorded for clean uninstall.

| Field | Type | Description |
|-------|------|-------------|
| `prefix` | string | Base installation prefix (default: `/usr/local`) |
| `bindir` | string | Executable directory (default: `{{prefix}}/bin`) |
| `libdir` | string | Library directory (default: `{{prefix}}/lib`) |
| `datadir` | string | Data directory (default: `{{prefix}}/share`) |
| `mandir` | string | Man page directory (default: `{{datadir}}/man`) |
| `docdir` | string | Documentation directory (default: `{{datadir}}/doc/{{name}}`) |

### Template Variables

These variables can be used in `install_steps` and are expanded at install time:

| Variable | Description |
|----------|-------------|
| `{{name}}` | Package name |
| `{{version}}` | Package version |
| `{{prefix}}` | Install prefix |
| `{{bindir}}` | Binary directory |
| `{{libdir}}` | Library directory |
| `{{datadir}}` | Data directory |
| `{{mandir}}` | Man page directory |
| `{{docdir}}` | Documentation directory |
| `{{srcdir}}` | Source directory (extracted/cloned) |
| `{{arch}}` | System architecture (amd64, arm64) |
| `{{os}}` | Operating system (darwin, linux) |

### Optional Metadata

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Short description of the package |
| `homepage` | string | Project homepage URL |
| `license` | string | SPDX license identifier |
| `provides` | array | Virtual packages this provides |

### Platform Filtering

Steps and sources can be filtered by platform:

```toml
[[install_steps]]
type = "copy"
src = "bin/tool-darwin-arm64"
dest = "{{bindir}}/tool"
platforms = ["darwin-arm64", "darwin-amd64"]
```

Valid platform values: `darwin-arm64`, `darwin-amd64`, `linux-arm64`, `linux-amd64`

## Complete Example

```toml
name = "ripgrep"
version = "14.1.0"
description = "Fast line-oriented search tool"
homepage = "https://github.com/BurntSushi/ripgrep"
license = "MIT"

[source]
url = "https://github.com/BurntSushi/ripgrep/releases/download/{{version}}/ripgrep-{{version}}-{{arch}}-{{os}}.tar.gz"
sha256 = "abc123..."
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

[[install_steps]]
type = "mkdir"
path = "{{docdir}}"

[[install_steps]]
type = "copy"
src = "README.md"
dest = "{{docdir}}/README.md"
```
