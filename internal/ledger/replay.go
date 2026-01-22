package ledger

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

// ReplayError records an error that occurred while replaying an entry.
type ReplayError struct {
	Entry Entry
	Err   error
}

func (e *ReplayError) Error() string {
	return fmt.Sprintf("replay %s %s: %v", e.Entry.Op, e.Entry.Path, e.Err)
}

func (e *ReplayError) Unwrap() error {
	return e.Err
}

// ReplayResult contains the result of a reverse replay operation.
type ReplayResult struct {
	// Processed is the number of entries successfully processed.
	Processed int

	// Skipped is the number of entries skipped (e.g., already undone).
	Skipped int

	// Errors contains any errors encountered during replay.
	// Replay continues after errors to undo as much as possible.
	Errors []ReplayError

	// ModifiedFiles lists files that were modified externally
	// (checksum mismatch) but were still processed.
	ModifiedFiles []string
}

// HasErrors returns true if any errors occurred during replay.
func (r *ReplayResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// ReplayOptions configures the reverse replay behavior.
type ReplayOptions struct {
	// DryRun if true, doesn't actually perform operations.
	DryRun bool

	// Force if true, proceeds even if checksums don't match.
	Force bool

	// Verbose if true, enables detailed logging via OnEntry.
	Verbose bool

	// OnEntry is called for each entry being processed.
	// If nil, no callback is made.
	OnEntry func(entry Entry, action string)

	// KeepBackups if true, doesn't delete backup files after restore.
	KeepBackups bool
}

// ReverseReplay undoes all operations in the ledger in reverse order.
// This is the core uninstall mechanism.
func ReverseReplay(l *Ledger, opts ReplayOptions) (*ReplayResult, error) {
	result := &ReplayResult{}

	// Process entries in reverse order
	for i := len(l.Entries) - 1; i >= 0; i-- {
		entry := l.Entries[i]

		action, err := replayEntry(entry, opts)
		if opts.OnEntry != nil {
			opts.OnEntry(entry, action)
		}

		if err != nil {
			if errors.Is(err, errSkipped) {
				result.Skipped++
				continue
			}
			if errors.Is(err, errModified) {
				result.ModifiedFiles = append(result.ModifiedFiles, entry.Path)
				// Continue processing if Force is set
				if !opts.Force {
					result.Errors = append(result.Errors, ReplayError{Entry: entry, Err: err})
					continue
				}
			} else {
				result.Errors = append(result.Errors, ReplayError{Entry: entry, Err: err})
				continue
			}
		}

		result.Processed++
	}

	return result, nil
}

var (
	errSkipped  = errors.New("skipped")
	errModified = errors.New("file was modified externally")
)

// replayEntry undoes a single ledger entry.
func replayEntry(entry Entry, opts ReplayOptions) (string, error) {
	switch entry.Op {
	case OpFileCreate:
		return replayFileCreate(entry, opts)
	case OpFileDelete:
		return replayFileDelete(entry, opts)
	case OpFileOverwrite:
		return replayFileOverwrite(entry, opts)
	case OpDirCreate:
		return replayDirCreate(entry, opts)
	case OpSymlinkCreate:
		return replaySymlinkCreate(entry, opts)
	case OpHardlinkCreate:
		return replayHardlinkCreate(entry, opts)
	default:
		return "unknown", fmt.Errorf("unknown operation: %s", entry.Op)
	}
}

// replayFileCreate undoes a file creation by deleting the file.
func replayFileCreate(entry Entry, opts ReplayOptions) (string, error) {
	// Check if file exists
	info, err := os.Lstat(entry.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "skip (not found)", errSkipped
		}
		return "error", fmt.Errorf("stat file: %w", err)
	}

	// Verify it's a regular file
	if !info.Mode().IsRegular() {
		return "skip (not a file)", errSkipped
	}

	// Verify checksum if available
	if entry.Checksum != "" {
		match, err := VerifyChecksum(entry.Path, entry.Checksum)
		if err != nil {
			return "error", fmt.Errorf("verify checksum: %w", err)
		}
		if !match {
			return "modified", errModified
		}
	}

	if opts.DryRun {
		return "would delete", nil
	}

	if err := os.Remove(entry.Path); err != nil {
		return "error", fmt.Errorf("remove file: %w", err)
	}

	return "deleted", nil
}

// replayFileDelete restores a deleted file from backup.
func replayFileDelete(entry Entry, opts ReplayOptions) (string, error) {
	if entry.Original == nil {
		return "error", errors.New("no original file information")
	}

	// Check if file already exists (maybe restored manually)
	if _, err := os.Lstat(entry.Path); err == nil {
		return "skip (exists)", errSkipped
	}

	if entry.Original.BackupPath == "" {
		return "error", errors.New("no backup path")
	}

	if opts.DryRun {
		return "would restore", nil
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(entry.Path), 0755); err != nil {
		return "error", fmt.Errorf("create parent directory: %w", err)
	}

	// Copy backup to original location
	if err := copyFile(entry.Original.BackupPath, entry.Path); err != nil {
		return "error", fmt.Errorf("restore from backup: %w", err)
	}

	// Restore permissions and ownership
	if err := os.Chmod(entry.Path, os.FileMode(entry.Original.Mode)); err != nil {
		// Non-fatal: log but continue
	}

	// Restore modification time
	if !entry.Original.ModTime.IsZero() {
		os.Chtimes(entry.Path, entry.Original.ModTime, entry.Original.ModTime)
	}

	// Clean up backup if requested
	if !opts.KeepBackups {
		os.Remove(entry.Original.BackupPath)
	}

	return "restored", nil
}

// replayFileOverwrite restores the original file from backup.
func replayFileOverwrite(entry Entry, opts ReplayOptions) (string, error) {
	if entry.Original == nil {
		return "error", errors.New("no original file information")
	}

	if entry.Original.BackupPath == "" {
		return "error", errors.New("no backup path")
	}

	// Check if current file matches what we installed
	if entry.Checksum != "" {
		match, err := VerifyChecksum(entry.Path, entry.Checksum)
		if err != nil && !os.IsNotExist(err) {
			return "error", fmt.Errorf("verify checksum: %w", err)
		}
		if !match && !os.IsNotExist(err) {
			return "modified", errModified
		}
	}

	if opts.DryRun {
		return "would restore", nil
	}

	// Remove current file
	os.Remove(entry.Path)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(entry.Path), 0755); err != nil {
		return "error", fmt.Errorf("create parent directory: %w", err)
	}

	// Copy backup to original location
	if err := copyFile(entry.Original.BackupPath, entry.Path); err != nil {
		return "error", fmt.Errorf("restore from backup: %w", err)
	}

	// Restore permissions
	if err := os.Chmod(entry.Path, os.FileMode(entry.Original.Mode)); err != nil {
		// Non-fatal
	}

	// Restore modification time
	if !entry.Original.ModTime.IsZero() {
		os.Chtimes(entry.Path, entry.Original.ModTime, entry.Original.ModTime)
	}

	// Clean up backup
	if !opts.KeepBackups {
		os.Remove(entry.Original.BackupPath)
	}

	return "restored", nil
}

// replayDirCreate removes an empty directory.
func replayDirCreate(entry Entry, opts ReplayOptions) (string, error) {
	info, err := os.Lstat(entry.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "skip (not found)", errSkipped
		}
		return "error", fmt.Errorf("stat directory: %w", err)
	}

	if !info.IsDir() {
		return "skip (not a directory)", errSkipped
	}

	if opts.DryRun {
		return "would remove", nil
	}

	// Only remove if empty
	if err := os.Remove(entry.Path); err != nil {
		if os.IsExist(err) || isNotEmpty(err) {
			return "skip (not empty)", errSkipped
		}
		return "error", fmt.Errorf("remove directory: %w", err)
	}

	return "removed", nil
}

// replaySymlinkCreate removes a symbolic link.
func replaySymlinkCreate(entry Entry, opts ReplayOptions) (string, error) {
	info, err := os.Lstat(entry.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "skip (not found)", errSkipped
		}
		return "error", fmt.Errorf("stat symlink: %w", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return "skip (not a symlink)", errSkipped
	}

	// Verify target if we recorded it
	if entry.Target != "" {
		target, err := os.Readlink(entry.Path)
		if err != nil {
			return "error", fmt.Errorf("read symlink: %w", err)
		}
		if target != entry.Target {
			return "modified", errModified
		}
	}

	if opts.DryRun {
		return "would remove", nil
	}

	if err := os.Remove(entry.Path); err != nil {
		return "error", fmt.Errorf("remove symlink: %w", err)
	}

	return "removed", nil
}

// replayHardlinkCreate removes a hard link.
func replayHardlinkCreate(entry Entry, opts ReplayOptions) (string, error) {
	info, err := os.Lstat(entry.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "skip (not found)", errSkipped
		}
		return "error", fmt.Errorf("stat hardlink: %w", err)
	}

	if !info.Mode().IsRegular() {
		return "skip (not a file)", errSkipped
	}

	if opts.DryRun {
		return "would remove", nil
	}

	if err := os.Remove(entry.Path); err != nil {
		return "error", fmt.Errorf("remove hardlink: %w", err)
	}

	return "removed", nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}

// isNotEmpty checks if an error indicates a directory is not empty.
func isNotEmpty(err error) bool {
	// Different OSes return different errors for non-empty directories
	return err != nil && (os.IsExist(err) ||
		errors.Is(err, os.ErrExist) ||
		// Check for common "directory not empty" error patterns
		(err.Error() != "" && (slices.Contains([]string{
			"directory not empty",
			"not empty",
		}, err.Error()))))
}

// ReverseEntries returns the ledger entries in reverse order.
// Useful for custom replay implementations.
func (l *Ledger) ReverseEntries() []Entry {
	reversed := make([]Entry, len(l.Entries))
	for i, entry := range l.Entries {
		reversed[len(l.Entries)-1-i] = entry
	}
	return reversed
}

// FilterByOp returns entries matching the given operation type.
func (l *Ledger) FilterByOp(op Op) []Entry {
	var filtered []Entry
	for _, entry := range l.Entries {
		if entry.Op == op {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// FilterByPath returns entries for a specific path.
func (l *Ledger) FilterByPath(path string) []Entry {
	var filtered []Entry
	for _, entry := range l.Entries {
		if entry.Path == path {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
