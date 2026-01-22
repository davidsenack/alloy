package ledger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Recorder provides high-level methods for recording file operations.
// It wraps a Ledger and handles checksum computation and backup creation.
type Recorder struct {
	ledger    *Ledger
	backupDir string
	pkg       string
}

// NewRecorder creates a new Recorder wrapping the given ledger.
// Backups of overwritten/deleted files are stored in backupDir/<pkg>/.
func NewRecorder(l *Ledger, backupDir string) *Recorder {
	return &Recorder{
		ledger:    l,
		backupDir: backupDir,
		pkg:       l.Header.Package,
	}
}

// RecordFileCreate records creation of a new file.
// Computes the file's checksum automatically.
func (r *Recorder) RecordFileCreate(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	checksum, err := Checksum(path)
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	entry := Entry{
		Op:        OpFileCreate,
		Path:      path,
		Timestamp: time.Now().UTC(),
		Mode:      uint32(info.Mode().Perm()),
		Size:      info.Size(),
		Checksum:  checksum,
	}

	// Get ownership info (Unix-specific, handled in stat helper)
	entry.UID, entry.GID = getOwnership(info)

	return r.ledger.Record(entry)
}

// RecordFileDelete records deletion of a file.
// Creates a backup of the file before deletion.
func (r *Recorder) RecordFileDelete(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	// Handle symlinks specially
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("read symlink: %w", err)
		}

		entry := Entry{
			Op:        OpFileDelete,
			Path:      path,
			Timestamp: time.Now().UTC(),
			Original: &OriginalFile{
				Mode:    uint32(info.Mode().Perm()),
				Target:  target,
				ModTime: info.ModTime(),
			},
		}
		entry.Original.UID, entry.Original.GID = getOwnership(info)

		return r.ledger.Record(entry)
	}

	// Regular file: compute checksum and create backup
	checksum, err := Checksum(path)
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	backupPath, err := r.createBackup(path, checksum)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	uid, gid := getOwnership(info)

	entry := Entry{
		Op:        OpFileDelete,
		Path:      path,
		Timestamp: time.Now().UTC(),
		Original: &OriginalFile{
			Mode:       uint32(info.Mode().Perm()),
			UID:        uid,
			GID:        gid,
			Size:       info.Size(),
			Checksum:   checksum,
			BackupPath: backupPath,
			ModTime:    info.ModTime(),
		},
	}

	return r.ledger.Record(entry)
}

// RecordFileOverwrite records replacement of an existing file.
// Creates a backup of the original file.
func (r *Recorder) RecordFileOverwrite(path string, newChecksum string, newSize int64) error {
	// The file has already been overwritten, so we need the original info
	// passed in. This method assumes the backup was already created.
	return fmt.Errorf("use RecordFileOverwriteWithBackup instead")
}

// RecordFileOverwriteWithBackup records an overwrite with pre-computed backup info.
func (r *Recorder) RecordFileOverwriteWithBackup(path string, orig *OriginalFile, newChecksum string, newSize int64, newMode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	uid, gid := getOwnership(info)

	entry := Entry{
		Op:        OpFileOverwrite,
		Path:      path,
		Timestamp: time.Now().UTC(),
		Mode:      uint32(newMode.Perm()),
		UID:       uid,
		GID:       gid,
		Size:      newSize,
		Checksum:  newChecksum,
		Original:  orig,
	}

	return r.ledger.Record(entry)
}

// PrepareOverwrite prepares to overwrite a file by backing it up.
// Call this BEFORE overwriting, then call RecordFileOverwriteWithBackup after.
func (r *Recorder) PrepareOverwrite(path string) (*OriginalFile, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No existing file to backup
		}
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return nil, fmt.Errorf("read symlink: %w", err)
		}
		uid, gid := getOwnership(info)
		return &OriginalFile{
			Mode:    uint32(info.Mode().Perm()),
			UID:     uid,
			GID:     gid,
			Target:  target,
			ModTime: info.ModTime(),
		}, nil
	}

	// Regular file
	checksum, err := Checksum(path)
	if err != nil {
		return nil, fmt.Errorf("compute checksum: %w", err)
	}

	backupPath, err := r.createBackup(path, checksum)
	if err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
	}

	uid, gid := getOwnership(info)

	return &OriginalFile{
		Mode:       uint32(info.Mode().Perm()),
		UID:        uid,
		GID:        gid,
		Size:       info.Size(),
		Checksum:   checksum,
		BackupPath: backupPath,
		ModTime:    info.ModTime(),
	}, nil
}

// RecordDirCreate records creation of a directory.
func (r *Recorder) RecordDirCreate(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat directory: %w", err)
	}

	uid, gid := getOwnership(info)

	entry := Entry{
		Op:        OpDirCreate,
		Path:      path,
		Timestamp: time.Now().UTC(),
		Mode:      uint32(info.Mode().Perm()),
		UID:       uid,
		GID:       gid,
	}

	return r.ledger.Record(entry)
}

// RecordSymlinkCreate records creation of a symbolic link.
func (r *Recorder) RecordSymlinkCreate(path, target string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat symlink: %w", err)
	}

	uid, gid := getOwnership(info)

	entry := Entry{
		Op:        OpSymlinkCreate,
		Path:      path,
		Timestamp: time.Now().UTC(),
		Mode:      uint32(info.Mode().Perm()),
		UID:       uid,
		GID:       gid,
		Target:    target,
	}

	return r.ledger.Record(entry)
}

// RecordHardlinkCreate records creation of a hard link.
func (r *Recorder) RecordHardlinkCreate(path, target string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat hardlink: %w", err)
	}

	checksum, err := Checksum(path)
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	uid, gid := getOwnership(info)

	entry := Entry{
		Op:        OpHardlinkCreate,
		Path:      path,
		Timestamp: time.Now().UTC(),
		Mode:      uint32(info.Mode().Perm()),
		UID:       uid,
		GID:       gid,
		Size:      info.Size(),
		Checksum:  checksum,
		Target:    target,
	}

	return r.ledger.Record(entry)
}

// Close closes the underlying ledger.
func (r *Recorder) Close() error {
	return r.ledger.Close()
}

// createBackup copies a file to the backup directory.
// Returns the backup path.
func (r *Recorder) createBackup(path, checksum string) (string, error) {
	pkgBackupDir := filepath.Join(r.backupDir, r.pkg)
	if err := os.MkdirAll(pkgBackupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	// Use checksum as filename to deduplicate identical files
	backupPath := filepath.Join(pkgBackupDir, checksum)

	// Skip if backup already exists (same content)
	if _, err := os.Stat(backupPath); err == nil {
		return backupPath, nil
	}

	// Copy file to backup
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(backupPath)
		return "", err
	}

	if err := dst.Sync(); err != nil {
		os.Remove(backupPath)
		return "", err
	}

	return backupPath, nil
}
