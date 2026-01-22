package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDirectoryPermissions(t *testing.T) {
	// Create a temp directory to test
	tmpDir := t.TempDir()

	results := CheckDirectoryPermissions(tmpDir)

	// Should have results for directory readable and writable
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// All checks should pass for a temp directory
	for _, r := range results {
		if r.Status == "error" {
			t.Errorf("unexpected error for %s: %s", r.Name, r.Message)
		}
	}
}

func TestCheckDirectoryPermissions_NonExistent(t *testing.T) {
	results := CheckDirectoryPermissions("/nonexistent/path/that/does/not/exist")

	if len(results) != 1 {
		t.Fatalf("expected 1 result for nonexistent path, got %d", len(results))
	}

	if results[0].Status != "ok" {
		t.Errorf("expected ok status for nonexistent dir (will be created), got %s", results[0].Status)
	}
}

func TestCheckLedgerIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	ledgerDir := filepath.Join(tmpDir, "ledgers")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create a valid ledger
	ledg, err := Create(ledgerDir, "test-pkg", "test-source")
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test-file")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Record it with a WRONG checksum (so it appears modified)
	wrongChecksum := ChecksumBytes([]byte("different content"))
	if err := ledg.Record(Entry{
		Op:       OpFileCreate,
		Path:     testFile,
		Checksum: wrongChecksum,
	}); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}
	ledg.Close()

	// Check integrity with CheckFiles enabled
	opts := DoctorOptions{CheckFiles: true}
	result := CheckLedgerIntegrity(ledgerDir, backupDir, "test-pkg", opts)

	if result.ParseError != nil {
		t.Errorf("unexpected parse error: %v", result.ParseError)
	}

	if result.EntryCount != 1 {
		t.Errorf("expected 1 entry, got %d", result.EntryCount)
	}

	// File exists but has wrong checksum, should be in ModifiedFiles
	if len(result.ModifiedFiles) != 1 {
		t.Errorf("expected 1 modified file, got %d", len(result.ModifiedFiles))
	}
}

func TestCheckLedgerIntegrity_CorrectChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	ledgerDir := filepath.Join(tmpDir, "ledgers")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create a valid ledger
	ledg, err := Create(ledgerDir, "test-pkg", "test-source")
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test-file")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Record it with the CORRECT checksum
	correctChecksum := ChecksumBytes(content)
	if err := ledg.Record(Entry{
		Op:       OpFileCreate,
		Path:     testFile,
		Checksum: correctChecksum,
	}); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}
	ledg.Close()

	// Check integrity with CheckFiles enabled
	opts := DoctorOptions{CheckFiles: true}
	result := CheckLedgerIntegrity(ledgerDir, backupDir, "test-pkg", opts)

	if result.ParseError != nil {
		t.Errorf("unexpected parse error: %v", result.ParseError)
	}

	// File has correct checksum, should NOT be in ModifiedFiles
	if len(result.ModifiedFiles) != 0 {
		t.Errorf("expected 0 modified files for correct checksum, got %d", len(result.ModifiedFiles))
	}

	// Should have no issues
	if result.HasIssues() {
		t.Errorf("expected no issues, but found some")
	}
}

func TestCheckLedgerIntegrity_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	ledgerDir := filepath.Join(tmpDir, "ledgers")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create a ledger referencing a file that doesn't exist
	ledg, err := Create(ledgerDir, "test-pkg", "test-source")
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	// Record an entry for a nonexistent file
	if err := ledg.Record(Entry{
		Op:   OpFileCreate,
		Path: "/nonexistent/file/path",
	}); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}
	ledg.Close()

	// Check integrity with CheckFiles enabled
	opts := DoctorOptions{CheckFiles: true}
	result := CheckLedgerIntegrity(ledgerDir, backupDir, "test-pkg", opts)

	if result.ParseError != nil {
		t.Errorf("unexpected parse error: %v", result.ParseError)
	}

	// File doesn't exist, should be in OrphanedFiles
	if len(result.OrphanedFiles) != 1 {
		t.Errorf("expected 1 orphaned file, got %d", len(result.OrphanedFiles))
	}
}

func TestCheckAllLedgers(t *testing.T) {
	tmpDir := t.TempDir()
	ledgerDir := filepath.Join(tmpDir, "ledgers")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create two ledgers
	ledg1, err := Create(ledgerDir, "pkg1", "source1")
	if err != nil {
		t.Fatalf("failed to create ledger 1: %v", err)
	}
	ledg1.Close()

	ledg2, err := Create(ledgerDir, "pkg2", "source2")
	if err != nil {
		t.Fatalf("failed to create ledger 2: %v", err)
	}
	ledg2.Close()

	results, err := CheckAllLedgers(ledgerDir, backupDir, DoctorOptions{})
	if err != nil {
		t.Fatalf("CheckAllLedgers failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestFindOrphanedBackups(t *testing.T) {
	tmpDir := t.TempDir()
	ledgerDir := filepath.Join(tmpDir, "ledgers")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create backup directory with orphan file
	pkgBackupDir := filepath.Join(backupDir, "test-pkg")
	if err := os.MkdirAll(pkgBackupDir, 0755); err != nil {
		t.Fatalf("failed to create backup dir: %v", err)
	}

	orphanBackup := filepath.Join(pkgBackupDir, "orphan-checksum")
	if err := os.WriteFile(orphanBackup, []byte("orphan"), 0644); err != nil {
		t.Fatalf("failed to write orphan backup: %v", err)
	}

	// Create a ledger that references a different backup
	ledg, err := Create(ledgerDir, "test-pkg", "test-source")
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	referencedBackup := filepath.Join(pkgBackupDir, "referenced-checksum")
	if err := os.WriteFile(referencedBackup, []byte("referenced"), 0644); err != nil {
		t.Fatalf("failed to write referenced backup: %v", err)
	}

	if err := ledg.Record(Entry{
		Op:   OpFileOverwrite,
		Path: "/some/path",
		Original: &OriginalFile{
			BackupPath: referencedBackup,
		},
	}); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}
	ledg.Close()

	// Find orphans
	orphans, err := FindOrphanedBackups(ledgerDir, backupDir)
	if err != nil {
		t.Fatalf("FindOrphanedBackups failed: %v", err)
	}

	if len(orphans) != 1 {
		t.Errorf("expected 1 orphan, got %d", len(orphans))
	}

	if len(orphans) > 0 && orphans[0] != orphanBackup {
		t.Errorf("expected orphan %s, got %s", orphanBackup, orphans[0])
	}
}

func TestLedgerIntegrityResult_HasIssues(t *testing.T) {
	tests := []struct {
		name     string
		result   LedgerIntegrityResult
		expected bool
	}{
		{
			name:     "no issues",
			result:   LedgerIntegrityResult{Package: "test"},
			expected: false,
		},
		{
			name:     "parse error",
			result:   LedgerIntegrityResult{Package: "test", ParseError: os.ErrNotExist},
			expected: true,
		},
		{
			name:     "missing backups",
			result:   LedgerIntegrityResult{Package: "test", MissingBackups: []string{"/backup"}},
			expected: true,
		},
		{
			name:     "orphaned files",
			result:   LedgerIntegrityResult{Package: "test", OrphanedFiles: []string{"/file"}},
			expected: true,
		},
		{
			name:     "modified files",
			result:   LedgerIntegrityResult{Package: "test", ModifiedFiles: []string{"/file"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasIssues(); got != tt.expected {
				t.Errorf("HasIssues() = %v, want %v", got, tt.expected)
			}
		})
	}
}
