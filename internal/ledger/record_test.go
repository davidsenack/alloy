package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecorderFileCreate(t *testing.T) {
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	// Create ledger
	l, err := Create(ledgerDir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r := NewRecorder(l, backupDir)

	// Create a test file
	testFile := filepath.Join(targetDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Record the creation
	if err := r.RecordFileCreate(testFile); err != nil {
		t.Fatalf("RecordFileCreate: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify ledger contents
	l2, err := Open(ledgerDir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if len(l2.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(l2.Entries))
	}

	e := l2.Entries[0]
	if e.Op != OpFileCreate {
		t.Errorf("Op = %s, want %s", e.Op, OpFileCreate)
	}
	if e.Path != testFile {
		t.Errorf("Path = %s, want %s", e.Path, testFile)
	}
	if e.Checksum == "" {
		t.Error("Checksum should be set")
	}
	if e.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", e.Size, len(content))
	}
}

func TestRecorderDirCreate(t *testing.T) {
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	l, err := Create(ledgerDir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r := NewRecorder(l, backupDir)

	// Create a test directory
	testDir := filepath.Join(targetDir, "subdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	if err := r.RecordDirCreate(testDir); err != nil {
		t.Fatalf("RecordDirCreate: %v", err)
	}

	r.Close()

	l2, err := Open(ledgerDir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if len(l2.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(l2.Entries))
	}

	e := l2.Entries[0]
	if e.Op != OpDirCreate {
		t.Errorf("Op = %s, want %s", e.Op, OpDirCreate)
	}
	if e.Mode == 0 {
		t.Error("Mode should be set")
	}
}

func TestRecorderSymlink(t *testing.T) {
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	l, err := Create(ledgerDir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r := NewRecorder(l, backupDir)

	// Create target and symlink
	target := filepath.Join(targetDir, "target")
	link := filepath.Join(targetDir, "link")

	if err := os.WriteFile(target, []byte("target"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if err := r.RecordSymlinkCreate(link, target); err != nil {
		t.Fatalf("RecordSymlinkCreate: %v", err)
	}

	r.Close()

	l2, err := Open(ledgerDir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	e := l2.Entries[0]
	if e.Op != OpSymlinkCreate {
		t.Errorf("Op = %s, want %s", e.Op, OpSymlinkCreate)
	}
	if e.Target != target {
		t.Errorf("Target = %s, want %s", e.Target, target)
	}
}

func TestRecorderHardlink(t *testing.T) {
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	l, err := Create(ledgerDir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r := NewRecorder(l, backupDir)

	// Create target and hardlink
	target := filepath.Join(targetDir, "target")
	link := filepath.Join(targetDir, "link")

	if err := os.WriteFile(target, []byte("target content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Link(target, link); err != nil {
		t.Fatalf("Link: %v", err)
	}

	if err := r.RecordHardlinkCreate(link, target); err != nil {
		t.Fatalf("RecordHardlinkCreate: %v", err)
	}

	r.Close()

	l2, err := Open(ledgerDir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	e := l2.Entries[0]
	if e.Op != OpHardlinkCreate {
		t.Errorf("Op = %s, want %s", e.Op, OpHardlinkCreate)
	}
	if e.Target != target {
		t.Errorf("Target = %s, want %s", e.Target, target)
	}
	if e.Checksum == "" {
		t.Error("Checksum should be set for hardlink")
	}
}

func TestRecorderFileDelete(t *testing.T) {
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	l, err := Create(ledgerDir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r := NewRecorder(l, backupDir)

	// Create a test file to "delete"
	testFile := filepath.Join(targetDir, "test.txt")
	content := []byte("file to delete")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Record the deletion (this creates a backup)
	if err := r.RecordFileDelete(testFile); err != nil {
		t.Fatalf("RecordFileDelete: %v", err)
	}

	r.Close()

	l2, err := Open(ledgerDir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	e := l2.Entries[0]
	if e.Op != OpFileDelete {
		t.Errorf("Op = %s, want %s", e.Op, OpFileDelete)
	}
	if e.Original == nil {
		t.Fatal("Original should be set")
	}
	if e.Original.BackupPath == "" {
		t.Error("Original.BackupPath should be set")
	}
	if e.Original.Checksum == "" {
		t.Error("Original.Checksum should be set")
	}

	// Verify backup was created
	if _, err := os.Stat(e.Original.BackupPath); os.IsNotExist(err) {
		t.Error("backup file should exist")
	}
}

func TestRecorderPrepareOverwrite(t *testing.T) {
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	l, err := Create(ledgerDir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r := NewRecorder(l, backupDir)

	// Create original file
	testFile := filepath.Join(targetDir, "test.txt")
	originalContent := []byte("original content")
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Prepare overwrite (creates backup)
	orig, err := r.PrepareOverwrite(testFile)
	if err != nil {
		t.Fatalf("PrepareOverwrite: %v", err)
	}

	if orig == nil {
		t.Fatal("Original should not be nil")
	}
	if orig.BackupPath == "" {
		t.Error("BackupPath should be set")
	}

	// Verify backup exists
	backupContent, err := os.ReadFile(orig.BackupPath)
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}
	if string(backupContent) != string(originalContent) {
		t.Errorf("backup content = %q, want %q", backupContent, originalContent)
	}

	// Now overwrite the file
	newContent := []byte("new content")
	if err := os.WriteFile(testFile, newContent, 0644); err != nil {
		t.Fatalf("WriteFile new: %v", err)
	}

	newChecksum := ChecksumBytes(newContent)

	// Record the overwrite
	if err := r.RecordFileOverwriteWithBackup(testFile, orig, newChecksum, int64(len(newContent)), 0644); err != nil {
		t.Fatalf("RecordFileOverwriteWithBackup: %v", err)
	}

	r.Close()

	l2, err := Open(ledgerDir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	e := l2.Entries[0]
	if e.Op != OpFileOverwrite {
		t.Errorf("Op = %s, want %s", e.Op, OpFileOverwrite)
	}
	if e.Checksum != newChecksum {
		t.Errorf("Checksum = %s, want %s", e.Checksum, newChecksum)
	}
	if e.Original == nil {
		t.Fatal("Original should be set")
	}
}

func TestRecorderPrepareOverwriteNonexistent(t *testing.T) {
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()

	l, err := Create(ledgerDir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r := NewRecorder(l, backupDir)

	// Prepare overwrite of nonexistent file
	orig, err := r.PrepareOverwrite("/nonexistent/file.txt")
	if err != nil {
		t.Fatalf("PrepareOverwrite: %v", err)
	}

	// Should return nil (no original to backup)
	if orig != nil {
		t.Error("Original should be nil for nonexistent file")
	}

	r.Close()
}

func TestRecorderDeleteSymlink(t *testing.T) {
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	l, err := Create(ledgerDir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r := NewRecorder(l, backupDir)

	// Create symlink
	target := filepath.Join(targetDir, "target")
	link := filepath.Join(targetDir, "link")

	if err := os.WriteFile(target, []byte("target"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	// Record deletion of symlink
	if err := r.RecordFileDelete(link); err != nil {
		t.Fatalf("RecordFileDelete: %v", err)
	}

	r.Close()

	l2, err := Open(ledgerDir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	e := l2.Entries[0]
	if e.Original == nil {
		t.Fatal("Original should be set")
	}
	if e.Original.Target != target {
		t.Errorf("Original.Target = %s, want %s", e.Original.Target, target)
	}
	// Symlinks don't have backups, only target info
	if e.Original.BackupPath != "" {
		t.Error("BackupPath should be empty for symlink")
	}
}
