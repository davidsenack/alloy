package installer

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarGz(t *testing.T) {
	// Create a temp tar.gz file
	archiveDir := t.TempDir()
	archivePath := filepath.Join(archiveDir, "test.tar.gz")

	// Create archive with a nested structure
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a directory
	if err := tw.WriteHeader(&tar.Header{
		Name:     "pkg-1.0/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		t.Fatalf("write dir header: %v", err)
	}

	// Add a file
	content := []byte("file content")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "pkg-1.0/file.txt",
		Mode:     0644,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write file header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}

	// Add a subdirectory
	if err := tw.WriteHeader(&tar.Header{
		Name:     "pkg-1.0/subdir/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		t.Fatalf("write subdir header: %v", err)
	}

	// Add file in subdirectory
	subContent := []byte("sub content")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "pkg-1.0/subdir/sub.txt",
		Mode:     0600,
		Size:     int64(len(subContent)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write sub file header: %v", err)
	}
	if _, err := tw.Write(subContent); err != nil {
		t.Fatalf("write sub file content: %v", err)
	}

	tw.Close()
	gw.Close()
	f.Close()

	// Extract with strip=1
	destDir := t.TempDir()
	inst := &Installer{}

	if err := inst.extractTarGz(archivePath, 1, destDir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	// Verify files were extracted correctly (with strip)
	filePath := filepath.Join(destDir, "file.txt")
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(fileContent) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", fileContent, content)
	}

	// Verify subdirectory file
	subPath := filepath.Join(destDir, "subdir", "sub.txt")
	subFileContent, err := os.ReadFile(subPath)
	if err != nil {
		t.Fatalf("read extracted sub file: %v", err)
	}
	if string(subFileContent) != string(subContent) {
		t.Errorf("sub content mismatch")
	}

	// Verify permissions
	info, err := os.Stat(subPath)
	if err != nil {
		t.Fatalf("stat sub file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode mismatch: got %o, want %o", info.Mode().Perm(), 0600)
	}
}

func TestExtractTarGzNoStrip(t *testing.T) {
	// Create a temp tar.gz file
	archiveDir := t.TempDir()
	archivePath := filepath.Join(archiveDir, "test.tar.gz")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a file at root
	content := []byte("root file")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "file.txt",
		Mode:     0644,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write file header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}

	tw.Close()
	gw.Close()
	f.Close()

	// Extract with strip=0
	destDir := t.TempDir()
	inst := &Installer{}

	if err := inst.extractTarGz(archivePath, 0, destDir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	// Verify file was extracted at root
	filePath := filepath.Join(destDir, "file.txt")
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(fileContent) != string(content) {
		t.Errorf("content mismatch")
	}
}

func TestExtractTarSymlink(t *testing.T) {
	// Create a temp tar.gz file with a symlink
	archiveDir := t.TempDir()
	archivePath := filepath.Join(archiveDir, "test.tar.gz")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a file
	content := []byte("target content")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "target.txt",
		Mode:     0644,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write file header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}

	// Add a symlink
	if err := tw.WriteHeader(&tar.Header{
		Name:     "link.txt",
		Linkname: "target.txt",
		Typeflag: tar.TypeSymlink,
	}); err != nil {
		t.Fatalf("write symlink header: %v", err)
	}

	tw.Close()
	gw.Close()
	f.Close()

	// Extract
	destDir := t.TempDir()
	inst := &Installer{}

	if err := inst.extractTarGz(archivePath, 0, destDir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	// Verify symlink
	linkPath := filepath.Join(destDir, "link.txt")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "target.txt" {
		t.Errorf("target mismatch: got %q, want %q", target, "target.txt")
	}

	// Verify we can read through the symlink
	linkContent, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("read through link: %v", err)
	}
	if string(linkContent) != string(content) {
		t.Errorf("content through link mismatch")
	}
}

func TestPathTraversalPrevention(t *testing.T) {
	// Create a tar.gz with a path traversal attempt
	archiveDir := t.TempDir()
	archivePath := filepath.Join(archiveDir, "malicious.tar.gz")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a malicious file path
	content := []byte("malicious content")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "../../../tmp/evil.txt",
		Mode:     0644,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write file header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}

	tw.Close()
	gw.Close()
	f.Close()

	// Extract should fail
	destDir := t.TempDir()
	inst := &Installer{}

	err = inst.extractTarGz(archivePath, 0, destDir)
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
}
