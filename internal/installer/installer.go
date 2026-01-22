// Package installer provides the core installation engine for Alloy packages.
package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/alloy/internal/ledger"
	"github.com/anthropics/alloy/internal/pkg"
)

// Installer handles package installation.
type Installer struct {
	// PackagesDir is the directory containing package definitions.
	PackagesDir string

	// LedgerDir is the directory for storing ledgers.
	LedgerDir string

	// BackupDir is the directory for storing backups.
	BackupDir string

	// CacheDir is the directory for downloaded sources.
	CacheDir string

	// DryRun if true, doesn't actually make changes.
	DryRun bool

	// Verbose enables detailed output.
	Verbose bool

	// OnProgress is called with progress updates.
	OnProgress func(msg string)
}

// New creates a new Installer with default directories.
func New() (*Installer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	alloyDir := filepath.Join(home, ".alloy")

	return &Installer{
		PackagesDir: "packages",
		LedgerDir:   filepath.Join(alloyDir, "ledgers"),
		BackupDir:   filepath.Join(alloyDir, "backups"),
		CacheDir:    filepath.Join(alloyDir, "cache"),
	}, nil
}

// Install installs a package by name.
func (i *Installer) Install(name string) error {
	i.progress("Loading package definition for %s", name)

	// Find and parse package definition
	pkgDef, err := i.loadPackage(name)
	if err != nil {
		return fmt.Errorf("load package: %w", err)
	}

	// Check if already installed
	if ledger.Exists(i.LedgerDir, name) {
		return fmt.Errorf("package %q is already installed", name)
	}

	// In dry-run mode, only validate and show what would happen
	if i.DryRun {
		return i.dryRunInstall(pkgDef)
	}

	// Fetch source
	i.progress("Fetching source from %s", pkgDef.Source.Location())
	srcDir, err := i.fetchSource(pkgDef)
	if err != nil {
		return fmt.Errorf("fetch source: %w", err)
	}
	defer os.RemoveAll(srcDir)

	// Create ledger
	source := pkgDef.ExpandedSource()
	ledg, err := ledger.Create(i.LedgerDir, name, source.Location())
	if err != nil {
		return fmt.Errorf("create ledger: %w", err)
	}
	defer ledg.Close()

	// Create recorder
	recorder := ledger.NewRecorder(ledg, i.BackupDir)

	// Execute install steps
	steps := pkgDef.ExpandedSteps(srcDir)
	i.progress("Executing %d install steps", len(steps))

	for idx, step := range steps {
		i.progress("Step %d/%d: %s", idx+1, len(steps), describeStep(step))

		if err := i.executeStep(step, srcDir, recorder); err != nil {
			// Try to rollback
			i.progress("Error during installation, rolling back...")
			i.rollback(ledg)
			ledg.Delete()
			return fmt.Errorf("step %d (%s): %w", idx+1, step.Type, err)
		}
	}

	i.progress("Successfully installed %s@%s", pkgDef.Name, pkgDef.Version)
	return nil
}

// dryRunInstall simulates an installation without making any changes.
func (i *Installer) dryRunInstall(pkgDef *pkg.Package) error {
	source := pkgDef.ExpandedSource()
	i.progress("[dry-run] Would fetch source from %s", source.Location())

	// Show what steps would be executed
	steps := pkgDef.ExpandedSteps("/tmp/source")
	i.progress("[dry-run] Would execute %d install steps:", len(steps))

	for idx, step := range steps {
		i.progress("[dry-run]   Step %d: %s", idx+1, describeStep(step))
	}

	i.progress("[dry-run] Dry run complete, no changes made")
	return nil
}

// loadPackage finds and parses a package definition.
func (i *Installer) loadPackage(name string) (*pkg.Package, error) {
	path := filepath.Join(i.PackagesDir, name+".toml")
	return pkg.ParseFile(path)
}

// rollback attempts to undo a partial installation.
func (i *Installer) rollback(ledg *ledger.Ledger) {
	result, err := ledger.ReverseReplay(ledg, ledger.ReplayOptions{
		Force: true,
		OnEntry: func(entry ledger.Entry, action string) {
			if i.Verbose {
				i.progress("  Rollback: %s %s -> %s", entry.Op, entry.Path, action)
			}
		},
	})
	if err != nil {
		i.progress("Rollback error: %v", err)
	}
	if result != nil && result.HasErrors() {
		for _, e := range result.Errors {
			i.progress("  Rollback failed for %s: %v", e.Entry.Path, e.Err)
		}
	}
}

// progress reports progress if a handler is set.
func (i *Installer) progress(format string, args ...any) {
	if i.OnProgress != nil {
		i.OnProgress(fmt.Sprintf(format, args...))
	}
}

func describeStep(step pkg.InstallStep) string {
	switch step.Type {
	case pkg.StepRun:
		return fmt.Sprintf("run: %s", step.Command)
	case pkg.StepCopy:
		return fmt.Sprintf("copy: %s -> %s", step.Src, step.Dest)
	case pkg.StepMkdir:
		return fmt.Sprintf("mkdir: %s", step.Path)
	case pkg.StepSymlink:
		return fmt.Sprintf("symlink: %s -> %s", step.Src, step.Dest)
	default:
		return fmt.Sprintf("%s", step.Type)
	}
}
