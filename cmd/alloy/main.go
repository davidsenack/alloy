// Command alloy is a fast, opinionated package manager.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/alloy/internal/installer"
	"github.com/anthropics/alloy/internal/ledger"
	"github.com/anthropics/alloy/internal/pkg"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "install":
		cmdInstall(os.Args[2:])
	case "remove":
		cmdRemove(os.Args[2:])
	case "list":
		cmdList(os.Args[2:])
	case "info":
		cmdInfo(os.Args[2:])
	case "doctor":
		cmdDoctor(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("alloy version %s\n", version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`alloy - A fast, opinionated package manager

Usage:
  alloy <command> [options] [arguments]

Note: Options must come before arguments (e.g., 'alloy install --dry-run ripgrep')

Commands:
  install <package>   Install a package
  remove <package>    Remove an installed package
  list                List installed packages
  info <package>      Show information about a package
  doctor              Check system health and diagnose issues
  version             Show version information
  help                Show this help message

Install Options:
  --dry-run           Show what would happen without making changes
  --verbose           Show detailed output
  --version <ver>     Install a specific version

Remove Options:
  --dry-run           Show what would happen without making changes
  --verbose           Show detailed output
  --force             Force removal even if files were modified

Doctor Options:
  --verbose           Show detailed output
  --check-files       Verify installed files exist and have correct checksums`)
}

func cmdInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Run without making any changes")
	verbose := fs.Bool("verbose", false, "Show detailed output")
	versionFlag := fs.String("version", "", "Specific version to install")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: alloy install <package> [--version <version>]")
		os.Exit(1)
	}

	packageName := fs.Arg(0)

	inst, err := installer.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	inst.DryRun = *dryRun
	inst.Verbose = *verbose
	inst.OnProgress = func(msg string) {
		fmt.Println(msg)
	}

	if *versionFlag != "" {
		fmt.Printf("Installing %s@%s\n", packageName, *versionFlag)
	} else {
		fmt.Printf("Installing %s (latest)\n", packageName)
	}

	if *dryRun {
		fmt.Println("[dry-run] No changes will be made to the system")
	}

	if err := inst.Install(packageName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdRemove(args []string) {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Run without making any changes")
	force := fs.Bool("force", false, "Force removal even if files were modified")
	verbose := fs.Bool("verbose", false, "Show detailed output")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: alloy remove <package>")
		os.Exit(1)
	}

	packageName := fs.Arg(0)

	ledgerDir, err := ledger.DefaultDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !ledger.Exists(ledgerDir, packageName) {
		fmt.Fprintf(os.Stderr, "Package %q is not installed\n", packageName)
		os.Exit(1)
	}

	fmt.Printf("Removing %s\n", packageName)
	if *dryRun {
		fmt.Println("[dry-run] No changes will be made to the system")
	}

	ledg, err := ledger.Open(ledgerDir, packageName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening ledger: %v\n", err)
		os.Exit(1)
	}

	result, err := ledger.ReverseReplay(ledg, ledger.ReplayOptions{
		DryRun:  *dryRun,
		Force:   *force,
		Verbose: *verbose,
		OnEntry: func(entry ledger.Entry, action string) {
			if *verbose {
				fmt.Printf("  %s %s -> %s\n", entry.Op, entry.Path, action)
			}
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during removal: %v\n", err)
		os.Exit(1)
	}

	if len(result.ModifiedFiles) > 0 {
		fmt.Println("\nWarning: The following files were modified externally:")
		for _, f := range result.ModifiedFiles {
			fmt.Printf("  %s\n", f)
		}
		if !*force {
			fmt.Println("Use --force to remove anyway")
		}
	}

	if result.HasErrors() {
		fmt.Println("\nErrors occurred during removal:")
		for _, e := range result.Errors {
			fmt.Printf("  %s: %v\n", e.Entry.Path, e.Err)
		}
		os.Exit(1)
	}

	// Delete the ledger file
	if !*dryRun {
		ledgerPath := ledger.Path(ledgerDir, packageName)
		os.Remove(ledgerPath)
	}

	fmt.Printf("Successfully removed %s (%d files processed, %d skipped)\n",
		packageName, result.Processed, result.Skipped)
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show detailed information")
	fs.Parse(args)

	ledgerDir, err := ledger.DefaultDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	packages, err := ledger.List(ledgerDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(packages) == 0 {
		fmt.Println("No packages installed")
		return
	}

	fmt.Printf("Installed packages (%d):\n", len(packages))
	for _, name := range packages {
		if *verbose {
			ledg, err := ledger.Open(ledgerDir, name)
			if err != nil {
				fmt.Printf("  %s (error reading ledger)\n", name)
				continue
			}
			fileCount := len(ledg.FilterByOp(ledger.OpFileCreate)) + len(ledg.FilterByOp(ledger.OpFileOverwrite))
			fmt.Printf("  %s\n", name)
			fmt.Printf("    Installed: %s\n", ledg.Header.InstalledAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("    Source: %s\n", ledg.Header.Source)
			fmt.Printf("    Files: %d\n", fileCount)
		} else {
			fmt.Printf("  %s\n", name)
		}
	}
}

func cmdInfo(args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: alloy info <package>")
		os.Exit(1)
	}

	packageName := fs.Arg(0)

	// First try to read the package definition
	pkgPath := filepath.Join("packages", packageName+".toml")
	pkgDef, defErr := pkg.ParseFile(pkgPath)

	// Then check if it's installed
	ledgerDir, err := ledger.DefaultDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var ledg *ledger.Ledger
	if ledger.Exists(ledgerDir, packageName) {
		ledg, err = ledger.Open(ledgerDir, packageName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading ledger: %v\n", err)
		}
	}

	if defErr != nil && ledg == nil {
		fmt.Fprintf(os.Stderr, "Package %q not found\n", packageName)
		os.Exit(1)
	}

	fmt.Printf("Package: %s\n", packageName)

	if pkgDef != nil {
		fmt.Printf("Version: %s\n", pkgDef.Version)
		if pkgDef.Description != "" {
			fmt.Printf("Description: %s\n", pkgDef.Description)
		}
		if pkgDef.Homepage != "" {
			fmt.Printf("Homepage: %s\n", pkgDef.Homepage)
		}
		if pkgDef.License != "" {
			fmt.Printf("License: %s\n", pkgDef.License)
		}
		fmt.Printf("Source: %s (%s)\n", pkgDef.Source.Location(), pkgDef.Source.SourceType())
	}

	if ledg != nil {
		fmt.Println("\nInstallation:")
		fmt.Printf("  Status: installed\n")
		fmt.Printf("  Installed at: %s\n", ledg.Header.InstalledAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Source: %s\n", ledg.Header.Source)

		fileCreates := ledg.FilterByOp(ledger.OpFileCreate)
		fileOverwrites := ledg.FilterByOp(ledger.OpFileOverwrite)
		dirCreates := ledg.FilterByOp(ledger.OpDirCreate)
		symlinkCreates := ledg.FilterByOp(ledger.OpSymlinkCreate)

		fmt.Printf("  Files created: %d\n", len(fileCreates))
		fmt.Printf("  Files overwritten: %d\n", len(fileOverwrites))
		fmt.Printf("  Directories created: %d\n", len(dirCreates))
		fmt.Printf("  Symlinks created: %d\n", len(symlinkCreates))
	} else {
		fmt.Println("\nStatus: not installed")
	}
}

func cmdDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show detailed output")
	checkFiles := fs.Bool("check-files", false, "Verify installed files exist and have correct checksums")
	fs.Parse(args)

	fmt.Println("Running system health check...")
	fmt.Println()

	issues := 0
	warnings := 0

	// Get alloy directory
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("✗ Cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	alloyDir := filepath.Join(home, ".alloy")

	// Check alloy directory permissions
	fmt.Println("=== Directory Permissions ===")
	dirResults := ledger.CheckDirectoryPermissions(alloyDir)
	for _, r := range dirResults {
		switch r.Status {
		case "ok":
			fmt.Printf("✓ %s: %s\n", r.Name, r.Message)
		case "warning":
			fmt.Printf("⚠ %s: %s\n", r.Name, r.Message)
			warnings++
		case "error":
			fmt.Printf("✗ %s: %s\n", r.Name, r.Message)
			issues++
		}
	}
	fmt.Println()

	// Check ledger directory
	ledgerDir, err := ledger.DefaultDir()
	if err != nil {
		fmt.Printf("✗ Cannot determine ledger directory: %v\n", err)
		issues++
	}

	backupDir, err := ledger.DefaultBackupDir()
	if err != nil {
		fmt.Printf("✗ Cannot determine backup directory: %v\n", err)
		issues++
	}

	// Check packages directory
	fmt.Println("=== Package Definitions ===")
	packagesDir := "packages"
	if _, err := os.Stat(packagesDir); os.IsNotExist(err) {
		fmt.Printf("⚠ Packages directory not found: %s\n", packagesDir)
		warnings++
	} else if err != nil {
		fmt.Printf("✗ Cannot access packages directory: %v\n", err)
		issues++
	} else {
		entries, _ := os.ReadDir(packagesDir)
		count := 0
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".toml" {
				count++
			}
		}
		fmt.Printf("✓ Packages directory: %s (%d definitions)\n", packagesDir, count)
	}
	fmt.Println()

	// Check write permissions to common install paths
	fmt.Println("=== Install Paths ===")
	testPaths := []string{"/usr/local/bin", "/usr/local/lib", "/usr/local/share"}
	for _, path := range testPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("⚠ Path does not exist: %s\n", path)
			warnings++
		} else if err != nil {
			fmt.Printf("✗ Cannot access path: %s: %v\n", path, err)
			issues++
		} else {
			// Check if writable by attempting to create a temp file
			testFile := filepath.Join(path, ".alloy-test-"+fmt.Sprint(os.Getpid()))
			if f, err := os.Create(testFile); err != nil {
				fmt.Printf("⚠ Path not writable: %s (may need sudo)\n", path)
				warnings++
			} else {
				f.Close()
				os.Remove(testFile)
				fmt.Printf("✓ Path writable: %s\n", path)
			}
		}
	}
	fmt.Println()

	// Check for required tools
	fmt.Println("=== Required Tools ===")
	requiredTools := []string{"git", "tar"}
	for _, tool := range requiredTools {
		if _, err := findExecutable(tool); err != nil {
			fmt.Printf("✗ Required tool not found: %s\n", tool)
			issues++
		} else {
			fmt.Printf("✓ Tool available: %s\n", tool)
		}
	}
	fmt.Println()

	// Check ledger integrity
	fmt.Println("=== Ledger Integrity ===")
	if ledgerDir != "" {
		packages, _ := ledger.List(ledgerDir)
		if len(packages) == 0 {
			fmt.Println("✓ No packages installed (nothing to check)")
		} else {
			opts := ledger.DoctorOptions{
				Verbose:    *verbose,
				CheckFiles: *checkFiles,
			}

			results, err := ledger.CheckAllLedgers(ledgerDir, backupDir, opts)
			if err != nil {
				fmt.Printf("✗ Error checking ledgers: %v\n", err)
				issues++
			} else {
				for _, r := range results {
					if r.ParseError != nil {
						fmt.Printf("✗ %s: ledger parse error: %v\n", r.Package, r.ParseError)
						issues++
						continue
					}

					if !r.HasIssues() {
						if *verbose {
							fmt.Printf("✓ %s: OK (%d entries)\n", r.Package, r.EntryCount)
						}
						continue
					}

					// Report issues
					if len(r.MissingBackups) > 0 {
						fmt.Printf("✗ %s: %d missing backup file(s)\n", r.Package, len(r.MissingBackups))
						issues++
						if *verbose {
							for _, b := range r.MissingBackups {
								fmt.Printf("    - %s\n", b)
							}
						}
					}

					if len(r.OrphanedFiles) > 0 {
						fmt.Printf("⚠ %s: %d installed file(s) not found\n", r.Package, len(r.OrphanedFiles))
						warnings++
						if *verbose {
							for _, f := range r.OrphanedFiles {
								fmt.Printf("    - %s\n", f)
							}
						}
					}

					if len(r.ModifiedFiles) > 0 {
						fmt.Printf("⚠ %s: %d installed file(s) modified externally\n", r.Package, len(r.ModifiedFiles))
						warnings++
						if *verbose {
							for _, f := range r.ModifiedFiles {
								fmt.Printf("    - %s\n", f)
							}
						}
					}
				}

				if !*verbose && len(results) > 0 {
					okCount := 0
					for _, r := range results {
						if !r.HasIssues() {
							okCount++
						}
					}
					if okCount > 0 {
						fmt.Printf("✓ %d package(s) OK\n", okCount)
					}
				}
			}

			// Check for orphaned backups
			orphanedBackups, err := ledger.FindOrphanedBackups(ledgerDir, backupDir)
			if err != nil {
				if *verbose {
					fmt.Printf("⚠ Could not check for orphaned backups: %v\n", err)
				}
			} else if len(orphanedBackups) > 0 {
				fmt.Printf("⚠ %d orphaned backup file(s) found\n", len(orphanedBackups))
				warnings++
				if *verbose {
					for _, b := range orphanedBackups {
						fmt.Printf("    - %s\n", b)
					}
				}
			}
		}
	}
	fmt.Println()

	// Summary
	fmt.Println("=== Summary ===")
	if issues > 0 {
		fmt.Printf("Found %d error(s)", issues)
		if warnings > 0 {
			fmt.Printf(" and %d warning(s)", warnings)
		}
		fmt.Println()
		os.Exit(1)
	} else if warnings > 0 {
		fmt.Printf("Found %d warning(s), no errors\n", warnings)
	} else {
		fmt.Println("All checks passed!")
	}
}

// findExecutable looks for an executable in PATH.
func findExecutable(name string) (string, error) {
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		fullPath := filepath.Join(dir, name)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return fullPath, nil
		}
	}
	return "", fmt.Errorf("executable not found: %s", name)
}
