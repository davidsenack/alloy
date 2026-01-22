package ledger

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCreateAndOpen(t *testing.T) {
	dir := t.TempDir()

	// Create a new ledger
	l, err := Create(dir, "test-pkg", "https://example.com/pkg.tar.gz")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify header
	if l.Header.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", l.Header.Version, CurrentVersion)
	}
	if l.Header.Package != "test-pkg" {
		t.Errorf("Package = %q, want %q", l.Header.Package, "test-pkg")
	}
	if l.Header.Source != "https://example.com/pkg.tar.gz" {
		t.Errorf("Source = %q, want %q", l.Header.Source, "https://example.com/pkg.tar.gz")
	}

	// Record some entries
	entries := []Entry{
		{Op: OpDirCreate, Path: "/opt/test-pkg"},
		{Op: OpFileCreate, Path: "/opt/test-pkg/bin/test", Checksum: "abc123", Size: 1024},
		{Op: OpSymlinkCreate, Path: "/usr/local/bin/test", Target: "/opt/test-pkg/bin/test"},
	}

	for _, e := range entries {
		if err := l.Record(e); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Re-open and verify
	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if len(l2.Entries) != len(entries) {
		t.Fatalf("len(Entries) = %d, want %d", len(l2.Entries), len(entries))
	}

	for i, e := range l2.Entries {
		if e.Op != entries[i].Op {
			t.Errorf("Entries[%d].Op = %s, want %s", i, e.Op, entries[i].Op)
		}
		if e.Path != entries[i].Path {
			t.Errorf("Entries[%d].Path = %s, want %s", i, e.Path, entries[i].Path)
		}
		if e.Timestamp.IsZero() {
			t.Errorf("Entries[%d].Timestamp is zero", i)
		}
	}
}

func TestDuplicateCreate(t *testing.T) {
	dir := t.TempDir()

	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	l.Close()

	// Second create should fail
	_, err = Create(dir, "test-pkg", "")
	if err == nil {
		t.Error("expected error for duplicate create")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()

	// Create some ledgers
	pkgs := []string{"alpha", "beta", "gamma"}
	for _, pkg := range pkgs {
		l, err := Create(dir, pkg, "")
		if err != nil {
			t.Fatalf("Create %s: %v", pkg, err)
		}
		l.Close()
	}

	// List them
	found, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(found) != len(pkgs) {
		t.Fatalf("len(found) = %d, want %d", len(found), len(pkgs))
	}

	// Note: order may differ
	for _, pkg := range pkgs {
		var exists bool
		for _, f := range found {
			if f == pkg {
				exists = true
				break
			}
		}
		if !exists {
			t.Errorf("package %q not found in list", pkg)
		}
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()

	if Exists(dir, "nonexistent") {
		t.Error("Exists returned true for nonexistent package")
	}

	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	l.Close()

	if !Exists(dir, "test-pkg") {
		t.Error("Exists returned false for existing package")
	}
}

func TestStream(t *testing.T) {
	dir := t.TempDir()

	// Create ledger with entries
	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	numEntries := 100
	for i := 0; i < numEntries; i++ {
		l.Record(Entry{Op: OpFileCreate, Path: filepath.Join("/opt", "file"+string(rune('0'+i%10)))})
	}
	l.Close()

	// Stream entries
	s, err := OpenStream(dir, "test-pkg")
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	defer s.Close()

	if s.Header().Package != "test-pkg" {
		t.Errorf("Header().Package = %q, want %q", s.Header().Package, "test-pkg")
	}

	count := 0
	for {
		_, err := s.Next()
		if err != nil {
			break
		}
		count++
	}

	if count != numEntries {
		t.Errorf("streamed %d entries, want %d", count, numEntries)
	}
}

func TestAppend(t *testing.T) {
	dir := t.TempDir()

	// Create initial ledger
	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	l.Record(Entry{Op: OpFileCreate, Path: "/opt/file1"})
	l.Close()

	// Append more entries
	l2, err := Append(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	l2.Record(Entry{Op: OpFileCreate, Path: "/opt/file2"})
	l2.Close()

	// Verify
	l3, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if len(l3.Entries) != 2 {
		t.Errorf("len(Entries) = %d, want 2", len(l3.Entries))
	}
}

func TestOriginalFileTracking(t *testing.T) {
	dir := t.TempDir()

	l, err := Create(dir, "test-pkg", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Record an overwrite with original info
	orig := &OriginalFile{
		Mode:       0644,
		UID:        1000,
		GID:        1000,
		Size:       512,
		Checksum:   "original-checksum",
		BackupPath: "/backup/original",
		ModTime:    time.Now().Add(-time.Hour),
	}

	entry := Entry{
		Op:       OpFileOverwrite,
		Path:     "/etc/config",
		Checksum: "new-checksum",
		Size:     1024,
		Original: orig,
	}

	if err := l.Record(entry); err != nil {
		t.Fatalf("Record: %v", err)
	}
	l.Close()

	// Re-open and verify
	l2, err := Open(dir, "test-pkg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if len(l2.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(l2.Entries))
	}

	e := l2.Entries[0]
	if e.Original == nil {
		t.Fatal("Original is nil")
	}
	if e.Original.Checksum != "original-checksum" {
		t.Errorf("Original.Checksum = %q, want %q", e.Original.Checksum, "original-checksum")
	}
	if e.Original.BackupPath != "/backup/original" {
		t.Errorf("Original.BackupPath = %q, want %q", e.Original.BackupPath, "/backup/original")
	}
}
