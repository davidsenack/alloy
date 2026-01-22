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
  alloy <command> [options]

Commands:
  install <package>   Install a package
  remove <package>    Remove an installed package
  list                List installed packages
  info <package>      Show information about a package
  doctor              Check system health and diagnose issues
  version             Show version information
  help                Show this help message

Options:
  --dry-run           Run without making any changes
  --verbose           Show detailed output`)
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

func cmdDoctor(_ []string) {
	fmt.Println("Running system health check...")
	fmt.Println()

	issues := 0

	// Check ledger directory
	ledgerDir, err := ledger.DefaultDir()
	if err != nil {
		fmt.Printf("✗ Cannot determine ledger directory: %v\n", err)
		issues++
	} else {
		if _, err := os.Stat(ledgerDir); os.IsNotExist(err) {
			fmt.Printf("✓ Ledger directory does not exist yet (will be created on first install)\n")
		} else if err != nil {
			fmt.Printf("✗ Cannot access ledger directory: %v\n", err)
			issues++
		} else {
			packages, err := ledger.List(ledgerDir)
			if err != nil {
				fmt.Printf("✗ Cannot list packages: %v\n", err)
				issues++
			} else {
				fmt.Printf("✓ Ledger directory: %s (%d packages)\n", ledgerDir, len(packages))
			}
		}
	}

	// Check backup directory
	backupDir, err := ledger.DefaultBackupDir()
	if err != nil {
		fmt.Printf("✗ Cannot determine backup directory: %v\n", err)
		issues++
	} else {
		if _, err := os.Stat(backupDir); os.IsNotExist(err) {
			fmt.Printf("✓ Backup directory does not exist yet (will be created when needed)\n")
		} else if err != nil {
			fmt.Printf("✗ Cannot access backup directory: %v\n", err)
			issues++
		} else {
			fmt.Printf("✓ Backup directory: %s\n", backupDir)
		}
	}

	// Check packages directory
	packagesDir := "packages"
	if _, err := os.Stat(packagesDir); os.IsNotExist(err) {
		fmt.Printf("✗ Packages directory not found: %s\n", packagesDir)
		issues++
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

	// Check write permissions to common install paths
	testPaths := []string{"/usr/local/bin", "/usr/local/lib", "/usr/local/share"}
	for _, path := range testPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("⚠ Path does not exist: %s\n", path)
		} else if err != nil {
			fmt.Printf("✗ Cannot access path: %s: %v\n", path, err)
			issues++
		} else {
			// Check if writable by attempting to create a temp file
			testFile := filepath.Join(path, ".alloy-test-"+fmt.Sprint(os.Getpid()))
			if f, err := os.Create(testFile); err != nil {
				fmt.Printf("⚠ Path not writable: %s (may need sudo)\n", path)
			} else {
				f.Close()
				os.Remove(testFile)
				fmt.Printf("✓ Path writable: %s\n", path)
			}
		}
	}

	// Check for required tools
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
	if issues > 0 {
		fmt.Printf("Found %d issue(s)\n", issues)
		os.Exit(1)
	}
	fmt.Println("All checks passed!")
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
