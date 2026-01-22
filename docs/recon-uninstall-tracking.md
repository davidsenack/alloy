# Recon: Uninstall Tracking Approaches

**Issue:** al-wr5
**Date:** 2026-01-22
**Author:** furiosa (polecat)

## Executive Summary

This document evaluates uninstall tracking strategies for Alloy: ledger-based tracking, filesystem tracing, and snapshot-based approaches. After analyzing trade-offs for speed, reliability, and system compatibility, **ledger-based tracking with symlink farms** is the recommended approach for Alloy.

---

## 1. Ledger-Based Tracking

### How It Works

A ledger records every file installed by a package. During uninstall, the ledger is consulted to determine which files to remove.

### Existing Implementations

#### dpkg/apt (Debian)
- **Location:** `/var/lib/dpkg/`
- **Format:** Plain text files
  - `status` - Package metadata and state
  - `info/<package>.list` - Files owned by each package
- **Pros:** Simple, human-readable, proven at scale
- **Cons:** No transactional guarantees, can become corrupted

#### RPM
- **Location:** `/var/lib/rpm/rpmdb.sqlite` (RHEL 9+)
- **Format:** SQLite database (previously Berkeley DB)
- **Pros:** Transactional, queryable, well-tested
- **Cons:** Requires librpm API for proper access, format is internal implementation detail

#### Nix Store
- **Location:** `/nix/store/`
- **Format:** Content-addressed paths with hash prefixes (e.g., `/nix/store/nawl092prjblbhvv16kxxbk6j9gkgcqm-git-2.14.1`)
- **Key Innovation:** Automatic reference tracking via hash scanning
- **Pros:**
  - Immutable store prevents corruption
  - Garbage collection handles orphaned packages
  - Multiple versions coexist safely
  - Rollback via profile symlink switching
- **Cons:** Disk space overhead, learning curve

#### GNU Stow
- **Location:** User-defined stow directory (e.g., `/usr/local/stow/`)
- **Format:** Directory hierarchy with symlink farms
- **Key Innovation:** "Tree folding" minimizes symlinks
- **Pros:**
  - Stateless - no database to corrupt
  - Clean uninstall by removing symlinks
  - Original files remain in stow directory
- **Cons:** Requires disciplined directory structure

### Ledger Persistence Formats

| Format | Pros | Cons |
|--------|------|------|
| **Plain Text** | Human-readable, easy debugging, git-friendly | No transactions, slow queries at scale |
| **JSONL** | Structured, appendable, streamable | Larger than binary, still slow for lookups |
| **SQLite** | Fast queries, ACID transactions, single file | Binary format, requires library |
| **Custom Binary** | Optimized for use case | Maintenance burden, tooling needed |

**Recommendation:** SQLite for the ledger, with JSONL export for debugging/backup.

---

## 2. Filesystem Tracing

### strace/ptrace Approach

Intercept all filesystem syscalls during installation to record what files are created.

#### How It Works
- `ptrace` pauses the traced process on every syscall entry/exit
- `strace` decodes syscall arguments and logs file operations
- Post-process logs to extract created/modified files

#### Performance Overhead

**Critical Finding:** strace can slow applications by **100-173x**.

From benchmarks:
- Process traced with strace: 173x slower (Arnaldo Carvalho de Melo, Red Hat)
- strace pauses application twice per syscall with context switches each time
- Overhead scales with syscall frequency

#### Recent Improvements
- Seccomp BPF filtering reduces unnecessary ptrace stops
- Caching of frequently-used data
- Still unsuitable for production or large installations

#### Verdict: NOT RECOMMENDED for Alloy
The overhead is unacceptable for package installation which involves many file operations.

### eBPF Alternative

Modern alternative to ptrace-based tracing.

#### How It Works
- Programs run in kernel space, not userspace
- No context switch per syscall
- Filter and aggregate in-kernel before copying to userspace

#### Performance
- **~100x faster than strace** for equivalent tracing
- Benchmark: 1.0 GB/s throughput vs 11.3 MB/s with strace

#### Tools
- **bpftrace** - High-level tracing language
- **BCC** - Collection of 70+ ready-to-use tools
- **perf trace** - Low-overhead syscall tracing

#### Limitations
- Requires recent kernel (4.x+)
- Root/CAP_BPF required
- Adds deployment complexity
- Still adds some overhead vs no tracing

#### Verdict: POSSIBLE but adds complexity
Could be useful for debugging or optional "discovery mode" but not as primary tracking mechanism.

---

## 3. inotify-Based Tracking

### How It Works
- Register watches on directories
- Kernel notifies on file events (create, modify, delete)
- Event-driven rather than polling

### Key Limitations

1. **No recursive watching** - Must create separate watch for every subdirectory
2. **Watch limits** - Default max_user_watches is 8192 (tunable to ~500K)
3. **Race conditions** - Events can be missed during rapid operations
4. **Queue overflow** - Events dropped if queue fills faster than consumption
5. **No file content** - Only notifies of changes, not what changed

### Scalability Issues

From research:
- Watching a large directory tree requires thousands of watches
- Each watch consumes ~1KB kernel memory
- Creating watches is O(n) for n directories

### Verdict: NOT RECOMMENDED for Alloy
The lack of recursive watching and reliability issues make this unsuitable for tracking package installations that may write to arbitrary locations.

---

## 4. Snapshot-Based Approaches

### How It Works
- Create filesystem snapshot before installation
- Install package
- Diff snapshot vs current state to identify changes
- Store diff as the "manifest" for uninstall

### Implementations

#### Btrfs Snapshots
- **Copy-on-write** - Snapshots are nearly instant
- **Tools:** Snapper, Timeshift, apt-btrfs-snapshot
- **Package manager hooks:** snap-pac (Pacman), DNF snapshot plugin

#### Rollback Process
1. Create snapshot before operation
2. Perform installation
3. If needed, rollback by replacing subvolume with snapshot

### Requirements
- Btrfs filesystem (or ZFS, but less common on Linux)
- Root on single device/partition
- Minimum 50-100GB for root partition
- Separate /boot handling needed

### Pros
- Complete system state capture
- Atomic rollback
- No need to track individual files

### Cons
- **Filesystem dependency** - Btrfs/ZFS only
- **Storage overhead** - Changed blocks accumulate
- **Coarse granularity** - Whole system, not per-package
- **Complexity** - /boot, /home often excluded
- **Not portable** - Can't use on ext4, XFS, etc.

### Verdict: NOT RECOMMENDED as primary approach
Filesystem dependency eliminates most potential users. Could be optional feature for Btrfs users.

---

## 5. Symlink and Hardlink Handling

### Symlinks

**Challenges:**
- Target may not exist yet during installation
- Can create circular references
- TOCTOU (Time Of Check To Time Of Use) race conditions
- Backup software may not handle correctly

**Tracking approach:**
- Record symlink path AND target path in ledger
- On uninstall, remove symlink only (not target)
- Validate target exists on install

### Hardlinks

**Challenges:**
- Multiple paths to same inode
- Reference counting needed
- Cannot span filesystems

**Tracking approach:**
- Track by inode number + device ID, not just path
- Decrement reference count on uninstall
- Only remove file when reference count reaches zero

### Recommendation

Store in ledger:
```
{
  "path": "/usr/bin/foo",
  "type": "symlink",
  "target": "../lib/foo/bin/foo",
  "inode": null
}
{
  "path": "/usr/lib/libfoo.so.1",
  "type": "hardlink",
  "inode": 12345678,
  "device": 2049,
  "refcount": 2
}
```

---

## 6. Trade-off Summary

| Approach | Speed | Reliability | Compatibility | Complexity |
|----------|-------|-------------|---------------|------------|
| **Ledger (SQLite)** | Fast | High | Universal | Low |
| **Ledger (Stow-style)** | Fast | High | Universal | Low |
| **strace/ptrace** | Very Slow | Medium | Universal | Medium |
| **eBPF tracing** | Medium | Medium | Kernel 4.x+ | High |
| **inotify** | N/A | Low | Universal | Medium |
| **Btrfs snapshots** | Fast | High | Btrfs only | High |

---

## 7. Recommendation for Alloy

### Primary Approach: Ledger-Based with Symlink Farms

Combine the best aspects of Nix and GNU Stow:

#### Architecture

```
/opt/alloy/store/
  <hash>-<package>-<version>/
    bin/
    lib/
    share/

/opt/alloy/profiles/
  default -> /opt/alloy/profiles/profile-42
  profile-42/
    bin/ -> symlinks to store
    lib/ -> symlinks to store

/opt/alloy/ledger/
  packages.db          # SQLite: package metadata
  manifests/           # Per-package file lists (JSONL backup)
```

#### Key Design Decisions

1. **SQLite ledger** for package metadata and file ownership
2. **Content-addressed store** (optional, like Nix) or version-addressed
3. **Symlink profiles** for activation (like Nix/Stow)
4. **JSONL manifests** as human-readable backup of file lists

#### Uninstall Process

1. Query ledger for package's file list
2. Remove symlinks from active profile
3. Check if store path has other references
4. If no references, remove from store
5. Update ledger atomically

#### Benefits

- **Fast:** No tracing overhead, direct file operations
- **Reliable:** SQLite transactions, symlinks prevent partial states
- **Compatible:** Works on any POSIX filesystem
- **Simple:** No kernel features required
- **Debuggable:** JSONL manifests for inspection

#### Optional Enhancements

- **eBPF discovery mode:** For packages that install outside expected paths
- **Btrfs integration:** Optional snapshot-based rollback for supported systems
- **Hardlink deduplication:** For store efficiency (like Nix)

---

## 8. Next Steps

This research unblocks:
- **al-05g:** CLI skeleton and command routing
- **al-bt4:** Package definition format
- **al-zb6:** Install ledger design and persistence

Specific follow-up tasks:
1. Design SQLite schema for ledger
2. Define package manifest JSONL format
3. Implement symlink farm creation/removal
4. Design store path naming convention

---

## References

- [Nix Package Manager Wiki](https://nixos.wiki/wiki/Nix_package_manager)
- [GNU Stow Manual](https://www.gnu.org/software/stow/manual/stow.html)
- [dpkg Manual](https://man7.org/linux/man-pages/man1/dpkg.1.html)
- [Fedora SQLite Rpmdb Change](https://fedoraproject.org/wiki/Changes/Sqlite_Rpmdb)
- [Brendan Gregg: strace Performance](https://www.brendangregg.com/blog/2014-05-11/strace-wow-much-syscall.html)
- [Brendan Gregg: eBPF Tracing](https://www.brendangregg.com/blog/2019-01-01/learn-ebpf-tracing.html)
- [inotify Manual](https://man7.org/linux/man-pages/man7/inotify.7.html)
- [Btrfs Snapshots Guide](https://www.dwarmstrong.org/btrfs-snapshots-rollbacks/)
