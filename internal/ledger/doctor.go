package ledger

import (
	"fmt"
	"os"
	"path/filepath"
)

// DiagnosticResult represents the result of a diagnostic check.
type DiagnosticResult struct {
	// Name is a short description of the check.
	Name string

	// Status is the result: "ok", "warning", or "error".
	Status string

	// Message provides details about the check result.
	Message string
}

// LedgerIntegrityResult contains the results of checking a single ledger.
type LedgerIntegrityResult struct {
	// Package is the name of the package.
	Package string

	// ParseError is set if the ledger couldn't be parsed.
	ParseError error

	// MissingBackups lists backup files referenced but not found.
	MissingBackups []string

	// OrphanedFiles lists files that should exist but don't.
	OrphanedFiles []string

	// ModifiedFiles lists files with checksum mismatches.
	ModifiedFiles []string

	// EntryCount is the total number of ledger entries.
	EntryCount int
}

// HasIssues returns true if any issues were found.
func (r *LedgerIntegrityResult) HasIssues() bool {
	return r.ParseError != nil ||
		len(r.MissingBackups) > 0 ||
		len(r.OrphanedFiles) > 0 ||
		len(r.ModifiedFiles) > 0
}

// DoctorOptions configures the diagnostic checks.
type DoctorOptions struct {
	// Verbose enables detailed output.
	Verbose bool

	// CheckFiles enables checking installed files exist and have correct checksums.
	// This can be slow for packages with many files.
	CheckFiles bool
}

// CheckDirectoryPermissions checks read/write permissions on the alloy directory.
func CheckDirectoryPermissions(alloyDir string) []DiagnosticResult {
	var results []DiagnosticResult

	// Check if directory exists
	info, err := os.Stat(alloyDir)
	if os.IsNotExist(err) {
		results = append(results, DiagnosticResult{
			Name:    "Alloy directory",
			Status:  "ok",
			Message: fmt.Sprintf("%s does not exist yet (will be created on first install)", alloyDir),
		})
		return results
	}
	if err != nil {
		results = append(results, DiagnosticResult{
			Name:    "Alloy directory",
			Status:  "error",
			Message: fmt.Sprintf("cannot access %s: %v", alloyDir, err),
		})
		return results
	}

	if !info.IsDir() {
		results = append(results, DiagnosticResult{
			Name:    "Alloy directory",
			Status:  "error",
			Message: fmt.Sprintf("%s exists but is not a directory", alloyDir),
		})
		return results
	}

	// Check read permission
	entries, err := os.ReadDir(alloyDir)
	if err != nil {
		results = append(results, DiagnosticResult{
			Name:    "Alloy directory readable",
			Status:  "error",
			Message: fmt.Sprintf("cannot read %s: %v", alloyDir, err),
		})
	} else {
		results = append(results, DiagnosticResult{
			Name:    "Alloy directory readable",
			Status:  "ok",
			Message: fmt.Sprintf("%s is readable (%d entries)", alloyDir, len(entries)),
		})
	}

	// Check write permission by creating a test file
	testFile := filepath.Join(alloyDir, ".alloy-doctor-test")
	f, err := os.Create(testFile)
	if err != nil {
		results = append(results, DiagnosticResult{
			Name:    "Alloy directory writable",
			Status:  "error",
			Message: fmt.Sprintf("cannot write to %s: %v", alloyDir, err),
		})
	} else {
		f.Close()
		os.Remove(testFile)
		results = append(results, DiagnosticResult{
			Name:    "Alloy directory writable",
			Status:  "ok",
			Message: fmt.Sprintf("%s is writable", alloyDir),
		})
	}

	// Check subdirectories
	subdirs := []string{"ledgers", "backups", "cache"}
	for _, subdir := range subdirs {
		path := filepath.Join(alloyDir, subdir)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			results = append(results, DiagnosticResult{
				Name:    subdir + " directory",
				Status:  "ok",
				Message: fmt.Sprintf("%s does not exist yet (will be created when needed)", path),
			})
			continue
		}
		if err != nil {
			results = append(results, DiagnosticResult{
				Name:    subdir + " directory",
				Status:  "error",
				Message: fmt.Sprintf("cannot access %s: %v", path, err),
			})
			continue
		}
		if !info.IsDir() {
			results = append(results, DiagnosticResult{
				Name:    subdir + " directory",
				Status:  "error",
				Message: fmt.Sprintf("%s exists but is not a directory", path),
			})
			continue
		}

		// Test write permission
		testFile := filepath.Join(path, ".alloy-doctor-test")
		f, err := os.Create(testFile)
		if err != nil {
			results = append(results, DiagnosticResult{
				Name:    subdir + " directory",
				Status:  "error",
				Message: fmt.Sprintf("cannot write to %s: %v", path, err),
			})
		} else {
			f.Close()
			os.Remove(testFile)
			results = append(results, DiagnosticResult{
				Name:    subdir + " directory",
				Status:  "ok",
				Message: fmt.Sprintf("%s is writable", path),
			})
		}
	}

	return results
}

// CheckLedgerIntegrity checks the integrity of a single package ledger.
func CheckLedgerIntegrity(ledgerDir, backupDir, pkg string, opts DoctorOptions) *LedgerIntegrityResult {
	result := &LedgerIntegrityResult{Package: pkg}

	// Try to open and parse the ledger
	ledg, err := Open(ledgerDir, pkg)
	if err != nil {
		result.ParseError = err
		return result
	}

	result.EntryCount = len(ledg.Entries)

	// Check for missing backup files and orphaned installed files
	for _, entry := range ledg.Entries {
		// Check backup references
		if entry.Original != nil && entry.Original.BackupPath != "" {
			if _, err := os.Stat(entry.Original.BackupPath); os.IsNotExist(err) {
				result.MissingBackups = append(result.MissingBackups, entry.Original.BackupPath)
			}
		}

		// Check installed files if requested
		if opts.CheckFiles {
			switch entry.Op {
			case OpFileCreate, OpFileOverwrite:
				info, err := os.Lstat(entry.Path)
				if os.IsNotExist(err) {
					result.OrphanedFiles = append(result.OrphanedFiles, entry.Path)
				} else if err == nil && info.Mode().IsRegular() && entry.Checksum != "" {
					// Verify checksum
					match, err := VerifyChecksum(entry.Path, entry.Checksum)
					if err == nil && !match {
						result.ModifiedFiles = append(result.ModifiedFiles, entry.Path)
					}
				}
			case OpSymlinkCreate:
				info, err := os.Lstat(entry.Path)
				if os.IsNotExist(err) {
					result.OrphanedFiles = append(result.OrphanedFiles, entry.Path)
				} else if err == nil {
					if info.Mode()&os.ModeSymlink == 0 {
						result.ModifiedFiles = append(result.ModifiedFiles, entry.Path+" (not a symlink)")
					} else if entry.Target != "" {
						target, err := os.Readlink(entry.Path)
						if err == nil && target != entry.Target {
							result.ModifiedFiles = append(result.ModifiedFiles, entry.Path)
						}
					}
				}
			case OpDirCreate:
				info, err := os.Stat(entry.Path)
				if os.IsNotExist(err) {
					result.OrphanedFiles = append(result.OrphanedFiles, entry.Path)
				} else if err == nil && !info.IsDir() {
					result.ModifiedFiles = append(result.ModifiedFiles, entry.Path+" (not a directory)")
				}
			}
		}
	}

	return result
}

// CheckAllLedgers checks integrity of all package ledgers.
func CheckAllLedgers(ledgerDir, backupDir string, opts DoctorOptions) ([]*LedgerIntegrityResult, error) {
	packages, err := List(ledgerDir)
	if err != nil {
		return nil, err
	}

	var results []*LedgerIntegrityResult
	for _, pkg := range packages {
		result := CheckLedgerIntegrity(ledgerDir, backupDir, pkg, opts)
		results = append(results, result)
	}

	return results, nil
}

// FindOrphanedBackups finds backup files not referenced by any ledger.
func FindOrphanedBackups(ledgerDir, backupDir string) ([]string, error) {
	// First, collect all backup paths referenced by ledgers
	referenced := make(map[string]bool)

	packages, err := List(ledgerDir)
	if err != nil {
		return nil, err
	}

	for _, pkg := range packages {
		ledg, err := Open(ledgerDir, pkg)
		if err != nil {
			continue // Skip problematic ledgers
		}

		for _, entry := range ledg.Entries {
			if entry.Original != nil && entry.Original.BackupPath != "" {
				referenced[entry.Original.BackupPath] = true
			}
		}
	}

	// Now scan backup directory for orphans
	var orphans []string

	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return nil, nil // No backup directory, no orphans
	}

	err = filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible files
		}
		if info.IsDir() {
			return nil // Skip directories
		}
		if !referenced[path] {
			orphans = append(orphans, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return orphans, nil
}
