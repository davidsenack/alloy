// Package ledger provides the install ledger for tracking file operations.
//
// The ledger records every file system operation performed during package
// installation, enabling clean uninstalls by replaying operations in reverse.
// Each package has its own ledger stored as JSONL in ~/.alloy/ledgers/<pkg>.jsonl.
package ledger

import (
	"time"
)

// Op represents the type of file system operation recorded in the ledger.
type Op string

const (
	// OpFileCreate records creation of a new file.
	OpFileCreate Op = "file_create"

	// OpFileDelete records deletion of an existing file.
	// The original file's checksum and backup path are stored for restoration.
	OpFileDelete Op = "file_delete"

	// OpFileOverwrite records replacement of an existing file.
	// The original file's checksum and backup path are stored for restoration.
	OpFileOverwrite Op = "file_overwrite"

	// OpDirCreate records creation of a new directory.
	OpDirCreate Op = "dir_create"

	// OpSymlinkCreate records creation of a symbolic link.
	OpSymlinkCreate Op = "symlink_create"

	// OpHardlinkCreate records creation of a hard link.
	OpHardlinkCreate Op = "hardlink_create"
)

// Entry represents a single ledger entry recording one file system operation.
// Entries are serialized as JSON, one per line, in the package ledger file.
type Entry struct {
	// Op is the operation type (required).
	Op Op `json:"op"`

	// Path is the absolute path of the file/directory affected (required).
	Path string `json:"path"`

	// Timestamp records when the operation occurred (required).
	Timestamp time.Time `json:"ts"`

	// Mode is the file permission bits (e.g., 0644).
	// Stored for file_create, file_overwrite, dir_create, symlink_create.
	Mode uint32 `json:"mode,omitempty"`

	// UID is the owner user ID.
	UID uint32 `json:"uid,omitempty"`

	// GID is the owner group ID.
	GID uint32 `json:"gid,omitempty"`

	// Size is the file size in bytes.
	// Stored for file_create, file_overwrite.
	Size int64 `json:"size,omitempty"`

	// Checksum is the SHA-256 hash of the file contents (hex-encoded).
	// Stored for file_create, file_overwrite to detect external modifications.
	Checksum string `json:"checksum,omitempty"`

	// Target is the link target path.
	// Stored for symlink_create and hardlink_create.
	Target string `json:"target,omitempty"`

	// Original holds information about the pre-existing file/link that was
	// replaced or deleted. Used for file_overwrite and file_delete operations.
	Original *OriginalFile `json:"original,omitempty"`
}

// OriginalFile stores information about a file that existed before an
// overwrite or delete operation, enabling restoration during uninstall.
type OriginalFile struct {
	// Mode is the original file permission bits.
	Mode uint32 `json:"mode"`

	// UID is the original owner user ID.
	UID uint32 `json:"uid"`

	// GID is the original owner group ID.
	GID uint32 `json:"gid"`

	// Size is the original file size in bytes.
	Size int64 `json:"size"`

	// Checksum is the SHA-256 hash of the original file contents (hex-encoded).
	Checksum string `json:"checksum"`

	// BackupPath is the path where the original file was backed up.
	// Backups are stored in ~/.alloy/backups/<pkg>/<hash>.
	BackupPath string `json:"backup_path"`

	// Target is set if the original was a symlink.
	Target string `json:"target,omitempty"`

	// ModTime is the original modification time.
	ModTime time.Time `json:"mtime"`
}

// Header contains metadata about the ledger file itself.
// This is always the first line of a ledger file.
type Header struct {
	// Version is the ledger format version.
	Version int `json:"version"`

	// Package is the name of the installed package.
	Package string `json:"package"`

	// InstalledAt is when the installation started.
	InstalledAt time.Time `json:"installed_at"`

	// Source describes where the package came from (URL, local path, etc.).
	Source string `json:"source,omitempty"`

	// SourceChecksum is the checksum of the source archive/binary if applicable.
	SourceChecksum string `json:"source_checksum,omitempty"`
}

// CurrentVersion is the current ledger format version.
const CurrentVersion = 1
