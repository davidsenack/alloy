# Contributing to Alloy

Thank you for your interest in contributing to Alloy! This guide will help you add new package definitions.

## Adding a New Package

Package definitions live in the `packages/` directory as TOML files. Each package gets its own file named `<package-name>.toml`.

### Step 1: Create the Package File

Create a new file in `packages/` with the package name:

```bash
touch packages/mypackage.toml
```

### Step 2: Add Required Fields

Every package definition needs at minimum:

```toml
name = "mypackage"
version = "1.0.0"

[source]
url = "https://example.com/mypackage-{{version}}.tar.gz"
sha256 = "..."

[[install_steps]]
type = "copy"
src = "mypackage"
dest = "{{bindir}}/mypackage"
mode = "0755"
```

### Step 3: Add Metadata (Recommended)

Help users understand what they're installing:

```toml
name = "mypackage"
version = "1.0.0"
description = "Brief description of what this package does"
homepage = "https://github.com/author/mypackage"
license = "MIT"
```

### Step 4: Configure the Source

Choose the appropriate source type:

**For GitHub releases (most common):**
```toml
[source]
url = "https://github.com/author/repo/releases/download/v{{version}}/repo-{{version}}-{{arch}}-{{os}}.tar.gz"
sha256 = "abc123..."
strip = 1  # Number of leading path components to strip
```

**For direct binary downloads:**
```toml
[source]
binary = "https://github.com/author/repo/releases/download/v{{version}}/repo-{{arch}}-{{os}}"
sha256 = "abc123..."
```

**For git repositories:**
```toml
[source]
git = "https://github.com/author/repo.git"
ref = "v1.0.0"  # Tag, branch, or commit
```

### Step 5: Define Install Steps

Common install step patterns:

**Copy a binary:**
```toml
[[install_steps]]
type = "copy"
src = "mypackage"
dest = "{{bindir}}/mypackage"
mode = "0755"
```

**Copy a man page:**
```toml
[[install_steps]]
type = "copy"
src = "doc/mypackage.1"
dest = "{{mandir}}/man1/mypackage.1"
```

**Create a directory:**
```toml
[[install_steps]]
type = "mkdir"
path = "{{datadir}}/mypackage"
```

**Install shell completions:**
```toml
[[install_steps]]
type = "mkdir"
path = "{{datadir}}/bash-completion/completions"

[[install_steps]]
type = "copy"
src = "completions/mypackage.bash"
dest = "{{datadir}}/bash-completion/completions/mypackage"
```

**Run a build command:**
```toml
[[install_steps]]
type = "run"
command = "make install PREFIX={{prefix}}"
```

**Create a symlink:**
```toml
[[install_steps]]
type = "symlink"
src = "{{bindir}}/mypackage-1.0"
dest = "{{bindir}}/mypackage"
```

### Step 6: Handle Platform-Specific Sources

If the package has different URLs per platform:

```toml
[[install_steps]]
type = "copy"
src = "mypackage-darwin"
dest = "{{bindir}}/mypackage"
mode = "0755"
platforms = ["darwin-arm64", "darwin-amd64"]

[[install_steps]]
type = "copy"
src = "mypackage-linux"
dest = "{{bindir}}/mypackage"
mode = "0755"
platforms = ["linux-arm64", "linux-amd64"]
```

## Template Variables

Use these variables in your package definition:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{name}}` | Package name | `ripgrep` |
| `{{version}}` | Package version | `14.1.0` |
| `{{prefix}}` | Install prefix | `/usr/local` |
| `{{bindir}}` | Binary directory | `/usr/local/bin` |
| `{{libdir}}` | Library directory | `/usr/local/lib` |
| `{{datadir}}` | Data directory | `/usr/local/share` |
| `{{mandir}}` | Man page directory | `/usr/local/share/man` |
| `{{docdir}}` | Documentation directory | `/usr/local/share/doc/ripgrep` |
| `{{arch}}` | CPU architecture | `amd64` or `arm64` |
| `{{os}}` | Operating system | `darwin` or `linux` |

## Getting the SHA256 Checksum

Download the archive and compute its checksum:

```bash
# macOS
curl -L "https://example.com/package.tar.gz" | shasum -a 256

# Linux
curl -L "https://example.com/package.tar.gz" | sha256sum
```

## Testing Your Package

1. **Dry run the installation:**
   ```bash
   alloy install --dry-run mypackage
   ```

2. **Install with verbose output:**
   ```bash
   alloy install --verbose mypackage
   ```

3. **Verify the installation:**
   ```bash
   alloy info mypackage
   mypackage --version
   ```

4. **Test removal:**
   ```bash
   alloy remove --dry-run mypackage
   alloy remove mypackage
   ```

5. **Run doctor to check for issues:**
   ```bash
   alloy doctor --verbose
   ```

## Example: Complete Package Definition

Here's a complete example for a typical CLI tool:

```toml
name = "fd"
version = "10.2.0"
description = "A simple, fast and user-friendly alternative to find"
homepage = "https://github.com/sharkdp/fd"
license = "MIT"

[source]
url = "https://github.com/sharkdp/fd/releases/download/v{{version}}/fd-v{{version}}-{{arch}}-unknown-{{os}}-gnu.tar.gz"
sha256 = "a1b2c3d4e5f6..."
strip = 1

[install_paths]
prefix = "/usr/local"

[[install_steps]]
type = "copy"
src = "fd"
dest = "{{bindir}}/fd"
mode = "0755"

[[install_steps]]
type = "copy"
src = "fd.1"
dest = "{{mandir}}/man1/fd.1"

[[install_steps]]
type = "mkdir"
path = "{{datadir}}/bash-completion/completions"

[[install_steps]]
type = "copy"
src = "autocomplete/fd.bash"
dest = "{{datadir}}/bash-completion/completions/fd"

[[install_steps]]
type = "mkdir"
path = "{{datadir}}/zsh/site-functions"

[[install_steps]]
type = "copy"
src = "autocomplete/_fd"
dest = "{{datadir}}/zsh/site-functions/_fd"

[[install_steps]]
type = "mkdir"
path = "{{datadir}}/fish/vendor_completions.d"

[[install_steps]]
type = "copy"
src = "autocomplete/fd.fish"
dest = "{{datadir}}/fish/vendor_completions.d/fd.fish"
```

## Checklist Before Submitting

- [ ] Package name is lowercase with hyphens (no underscores or spaces)
- [ ] Version is specified and matches the source URL
- [ ] SHA256 checksum is correct
- [ ] Description, homepage, and license are provided
- [ ] Binary has executable mode (`mode = "0755"`)
- [ ] Man pages are installed if available
- [ ] Shell completions are installed if available
- [ ] Dry-run install succeeds
- [ ] Actual install and remove work correctly
- [ ] `alloy doctor` reports no issues after install

## Schema Reference

For the complete schema documentation, see [`packages/SCHEMA.md`](packages/SCHEMA.md).

## Questions?

If you're unsure about something, look at existing package definitions in `packages/` for examples.
