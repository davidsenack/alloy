package ledger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReplayFileCreate(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()

	// Create a file that we want to "uninstall"
	testFile := filepath.Join(targetDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	checksum := ChecksumBytes(content)

	// Create ledger with file_create entry
	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	l.Record(Entry{
		Op:        OpFileCreate,
		Path:      testFile,
		Timestamp: time.Now(),
		Checksum:  checksum,
		Size:      int64(len(content)),
	})
	l.Close()

	// Re-open and replay
	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	result, err := ReverseReplay(l2, ReplayOptions{})
	if err != nil {
		t.Fatalf("ReverseReplay: %v", err)
	}

	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1", result.Processed)
	}
	if result.HasErrors() {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	// Verify file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestReplayDirCreate(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()

	// Create nested directories
	testDir := filepath.Join(targetDir, "a", "b", "c")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create ledger with dir_create entries (in order of creation)
	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	l.Record(Entry{Op: OpDirCreate, Path: filepath.Join(targetDir, "a")})
	l.Record(Entry{Op: OpDirCreate, Path: filepath.Join(targetDir, "a", "b")})
	l.Record(Entry{Op: OpDirCreate, Path: testDir})
	l.Close()

	// Re-open and replay (should remove in reverse order)
	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	result, err := ReverseReplay(l2, ReplayOptions{})
	if err != nil {
		t.Fatalf("ReverseReplay: %v", err)
	}

	if result.Processed != 3 {
		t.Errorf("Processed = %d, want 3", result.Processed)
	}

	// Verify directories were removed
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("directory c should have been removed")
	}
}

func TestReplayDryRun(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()

	testFile := filepath.Join(targetDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	checksum := ChecksumBytes(content)

	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	l.Record(Entry{
		Op:       OpFileCreate,
		Path:     testFile,
		Checksum: checksum,
	})
	l.Close()

	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	result, err := ReverseReplay(l2, ReplayOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ReverseReplay: %v", err)
	}

	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1", result.Processed)
	}

	// File should still exist
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("file should still exist in dry run mode")
	}
}

func TestReplaySymlinkCreate(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()

	// Create a symlink
	target := filepath.Join(targetDir, "target")
	link := filepath.Join(targetDir, "link")

	if err := os.WriteFile(target, []byte("target"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	l.Record(Entry{
		Op:     OpSymlinkCreate,
		Path:   link,
		Target: target,
	})
	l.Close()

	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	result, err := ReverseReplay(l2, ReplayOptions{})
	if err != nil {
		t.Fatalf("ReverseReplay: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	// Symlink should be removed
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Error("symlink should have been removed")
	}

	// Target should still exist
	if _, err := os.Stat(target); os.IsNotExist(err) {
		t.Error("target should still exist")
	}
}

func TestReplayModifiedFile(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()

	testFile := filepath.Join(targetDir, "test.txt")
	originalContent := []byte("original content")
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	originalChecksum := ChecksumBytes(originalContent)

	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	l.Record(Entry{
		Op:       OpFileCreate,
		Path:     testFile,
		Checksum: originalChecksum,
	})
	l.Close()

	// Modify the file externally
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Without Force, should report as modified
	result, err := ReverseReplay(l2, ReplayOptions{})
	if err != nil {
		t.Fatalf("ReverseReplay: %v", err)
	}

	if len(result.ModifiedFiles) != 1 {
		t.Errorf("ModifiedFiles = %d, want 1", len(result.ModifiedFiles))
	}

	// File should still exist (not deleted because modified)
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("modified file should not have been deleted without Force")
	}
}

func TestReplaySkipNonexistent(t *testing.T) {
	dir := t.TempDir()

	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Entry for a file that doesn't exist
	l.Record(Entry{
		Op:       OpFileCreate,
		Path:     "/nonexistent/file.txt",
		Checksum: "abc123",
	})
	l.Close()

	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	result, err := ReverseReplay(l2, ReplayOptions{})
	if err != nil {
		t.Fatalf("ReverseReplay: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if result.HasErrors() {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestReplayRestoreDeleted(t *testing.T) {
	dir := t.TempDir()
	backupDir := t.TempDir()
	targetDir := t.TempDir()

	testFile := filepath.Join(targetDir, "test.txt")
	content := []byte("original content")
	checksum := ChecksumBytes(content)

	// Create backup manually
	backupPath := filepath.Join(backupDir, "test-pkg", checksum)
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		t.Fatalf("WriteFile backup: %v", err)
	}

	// Create ledger with file_delete entry
	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	l.Record(Entry{
		Op:   OpFileDelete,
		Path: testFile,
		Original: &OriginalFile{
			Mode:       0644,
			Size:       int64(len(content)),
			Checksum:   checksum,
			BackupPath: backupPath,
			ModTime:    time.Now(),
		},
	})
	l.Close()

	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	result, err := ReverseReplay(l2, ReplayOptions{KeepBackups: true})
	if err != nil {
		t.Fatalf("ReverseReplay: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	// File should be restored
	restored, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(restored) != string(content) {
		t.Errorf("restored content = %q, want %q", restored, content)
	}
}

func TestReverseEntries(t *testing.T) {
	l := &Ledger{
		Entries: []Entry{
			{Path: "/a"},
			{Path: "/b"},
			{Path: "/c"},
		},
	}

	reversed := l.ReverseEntries()
	if len(reversed) != 3 {
		t.Fatalf("len(reversed) = %d, want 3", len(reversed))
	}
	if reversed[0].Path != "/c" {
		t.Errorf("reversed[0].Path = %s, want /c", reversed[0].Path)
	}
	if reversed[1].Path != "/b" {
		t.Errorf("reversed[1].Path = %s, want /b", reversed[1].Path)
	}
	if reversed[2].Path != "/a" {
		t.Errorf("reversed[2].Path = %s, want /a", reversed[2].Path)
	}
}

func TestFilterByOp(t *testing.T) {
	l := &Ledger{
		Entries: []Entry{
			{Op: OpFileCreate, Path: "/a"},
			{Op: OpDirCreate, Path: "/b"},
			{Op: OpFileCreate, Path: "/c"},
			{Op: OpSymlinkCreate, Path: "/d"},
		},
	}

	files := l.FilterByOp(OpFileCreate)
	if len(files) != 2 {
		t.Errorf("len(files) = %d, want 2", len(files))
	}

	dirs := l.FilterByOp(OpDirCreate)
	if len(dirs) != 1 {
		t.Errorf("len(dirs) = %d, want 1", len(dirs))
	}
}
