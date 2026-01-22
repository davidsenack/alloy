package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/alloy/internal/ledger"
	"github.com/anthropics/alloy/internal/pkg"
)

func TestExecuteCopy(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	destDir := t.TempDir()
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()

	// Create source file
	srcContent := []byte("hello world")
	srcPath := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(srcPath, srcContent, 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	// Create ledger
	ledg, err := ledger.Create(ledgerDir, "test-pkg", "test://source")
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}
	defer ledg.Close()

	recorder := ledger.NewRecorder(ledg, backupDir)

	// Create installer
	inst := &Installer{
		Verbose: true,
	}

	// Execute copy step
	destPath := filepath.Join(destDir, "output.txt")
	step := pkg.InstallStep{
		Type: pkg.StepCopy,
		Src:  "test.txt",
		Dest: destPath,
		Mode: "0755",
	}

	if err := inst.executeCopy(step, srcDir, recorder); err != nil {
		t.Fatalf("executeCopy: %v", err)
	}

	// Verify file was copied
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest file: %v", err)
	}
	if string(content) != string(srcContent) {
		t.Errorf("content mismatch: got %q, want %q", content, srcContent)
	}

	// Verify permissions
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat dest file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode mismatch: got %o, want %o", info.Mode().Perm(), 0755)
	}

	// Verify ledger entry
	if len(ledg.Entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(ledg.Entries))
	}
	if ledg.Entries[0].Op != ledger.OpFileCreate {
		t.Errorf("expected OpFileCreate, got %s", ledg.Entries[0].Op)
	}
	if ledg.Entries[0].Path != destPath {
		t.Errorf("path mismatch: got %q, want %q", ledg.Entries[0].Path, destPath)
	}
}

func TestExecuteMkdir(t *testing.T) {
	destDir := t.TempDir()
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()

	// Create ledger
	ledg, err := ledger.Create(ledgerDir, "test-pkg", "test://source")
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}
	defer ledg.Close()

	recorder := ledger.NewRecorder(ledg, backupDir)

	// Create installer
	inst := &Installer{}

	// Execute mkdir step for nested path
	newDir := filepath.Join(destDir, "a", "b", "c")
	step := pkg.InstallStep{
		Type: pkg.StepMkdir,
		Path: newDir,
		Mode: "0700",
	}

	if err := inst.executeMkdir(step, recorder); err != nil {
		t.Fatalf("executeMkdir: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("mode mismatch: got %o, want %o", info.Mode().Perm(), 0700)
	}

	// Verify ledger entries for all created directories
	if len(ledg.Entries) != 3 {
		t.Fatalf("expected 3 ledger entries (a, b, c), got %d", len(ledg.Entries))
	}
	for _, entry := range ledg.Entries {
		if entry.Op != ledger.OpDirCreate {
			t.Errorf("expected OpDirCreate, got %s", entry.Op)
		}
	}
}

func TestExecuteSymlink(t *testing.T) {
	destDir := t.TempDir()
	ledgerDir := t.TempDir()
	backupDir := t.TempDir()

	// Create target file
	targetPath := filepath.Join(destDir, "target.txt")
	if err := os.WriteFile(targetPath, []byte("target"), 0644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	// Create ledger
	ledg, err := ledger.Create(ledgerDir, "test-pkg", "test://source")
	if err != nil {
		t.Fatalf("create ledger: %v", err)
	}
	defer ledg.Close()

	recorder := ledger.NewRecorder(ledg, backupDir)

	// Create installer
	inst := &Installer{}

	// Execute symlink step
	linkPath := filepath.Join(destDir, "link.txt")
	step := pkg.InstallStep{
		Type: pkg.StepSymlink,
		Src:  targetPath,
		Dest: linkPath,
	}

	if err := inst.executeSymlink(step, recorder); err != nil {
		t.Fatalf("executeSymlink: %v", err)
	}

	// Verify symlink was created
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}

	// Verify target
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != targetPath {
		t.Errorf("target mismatch: got %q, want %q", target, targetPath)
	}

	// Verify ledger entry
	if len(ledg.Entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(ledg.Entries))
	}
	if ledg.Entries[0].Op != ledger.OpSymlinkCreate {
		t.Errorf("expected OpSymlinkCreate, got %s", ledg.Entries[0].Op)
	}
}

func TestMkdirAllRecording(t *testing.T) {
	destDir := t.TempDir()

	// Create nested path
	path := filepath.Join(destDir, "a", "b", "c")
	created, err := mkdirAllRecording(path, 0755)
	if err != nil {
		t.Fatalf("mkdirAllRecording: %v", err)
	}

	// Should have created 3 directories
	if len(created) != 3 {
		t.Errorf("expected 3 created dirs, got %d: %v", len(created), created)
	}

	// Verify all exist
	for _, dir := range created {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("dir %s does not exist: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestCopyFile(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(srcDir, "source.txt")
	srcContent := []byte("test content")
	if err := os.WriteFile(srcPath, srcContent, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	// Copy file
	destPath := filepath.Join(destDir, "dest.txt")
	if err := copyFile(srcPath, destPath, 0600); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(content) != string(srcContent) {
		t.Errorf("content mismatch")
	}

	// Verify permissions
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode mismatch: got %o, want %o", info.Mode().Perm(), 0600)
	}
}
