package installer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/anthropics/alloy/internal/ledger"
	"github.com/anthropics/alloy/internal/pkg"
)

// executeStep executes a single install step and records it to the ledger.
func (i *Installer) executeStep(step pkg.InstallStep, srcDir string, recorder *ledger.Recorder) error {
	switch step.Type {
	case pkg.StepRun:
		return i.executeRun(step, srcDir)
	case pkg.StepCopy:
		return i.executeCopy(step, srcDir, recorder)
	case pkg.StepMkdir:
		return i.executeMkdir(step, recorder)
	case pkg.StepSymlink:
		return i.executeSymlink(step, recorder)
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// executeRun executes a shell command.
func (i *Installer) executeRun(step pkg.InstallStep, srcDir string) error {
	workDir := srcDir
	if step.WorkDir != "" {
		workDir = filepath.Join(srcDir, step.WorkDir)
	}

	cmd := exec.Command("sh", "-c", step.Command)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// executeCopy copies a file from source to destination.
func (i *Installer) executeCopy(step pkg.InstallStep, srcDir string, recorder *ledger.Recorder) error {
	src := filepath.Join(srcDir, step.Src)
	dest := step.Dest

	// Determine file mode
	mode := os.FileMode(0644)
	if step.Mode != "" {
		parsed, err := strconv.ParseUint(step.Mode, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode %q: %w", step.Mode, err)
		}
		mode = os.FileMode(parsed)
	} else {
		// Preserve source mode
		if info, err := os.Stat(src); err == nil {
			mode = info.Mode().Perm()
		}
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", destDir, err)
	}

	// Check if destination already exists
	orig, err := recorder.PrepareOverwrite(dest)
	if err != nil {
		return fmt.Errorf("prepare overwrite: %w", err)
	}

	// Copy the file
	if err := copyFile(src, dest, mode); err != nil {
		return err
	}

	// Compute checksum of new file
	checksum, err := ledger.Checksum(dest)
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	// Get file size
	info, err := os.Stat(dest)
	if err != nil {
		return fmt.Errorf("stat destination: %w", err)
	}

	// Record the operation
	if orig != nil {
		// We overwrote an existing file
		return recorder.RecordFileOverwriteWithBackup(dest, orig, checksum, info.Size(), mode)
	}
	// Created a new file
	return recorder.RecordFileCreate(dest)
}

// executeMkdir creates a directory.
func (i *Installer) executeMkdir(step pkg.InstallStep, recorder *ledger.Recorder) error {
	path := step.Path

	// Determine mode
	mode := os.FileMode(0755)
	if step.Mode != "" {
		parsed, err := strconv.ParseUint(step.Mode, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode %q: %w", step.Mode, err)
		}
		mode = os.FileMode(parsed)
	}

	// Check if directory already exists
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		// Directory already exists, nothing to do
		return nil
	}

	// Create the directory and any parents
	// We need to track which directories we actually created
	created, err := mkdirAllRecording(path, mode)
	if err != nil {
		return err
	}

	// Record each created directory
	for _, dir := range created {
		if err := recorder.RecordDirCreate(dir); err != nil {
			return fmt.Errorf("record dir create: %w", err)
		}
	}

	return nil
}

// executeSymlink creates a symbolic link.
func (i *Installer) executeSymlink(step pkg.InstallStep, recorder *ledger.Recorder) error {
	// In symlink steps, src is the target and dest is the link path
	target := step.Src
	linkPath := step.Dest

	// Ensure parent directory exists
	linkDir := filepath.Dir(linkPath)
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", linkDir, err)
	}

	// Check if symlink already exists
	if info, err := os.Lstat(linkPath); err == nil {
		// Something exists at the link path
		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink - check if it points to the same target
			existing, err := os.Readlink(linkPath)
			if err == nil && existing == target {
				// Already correct, nothing to do
				return nil
			}
		}
		// Remove existing file/symlink
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("remove existing: %w", err)
		}
	}

	// Create the symlink
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	// Record the operation
	return recorder.RecordSymlinkCreate(linkPath, target)
}

// copyFile copies a file from src to dest with the given mode.
func copyFile(src, dest string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	if _, err := io.Copy(destFile, srcFile); err != nil {
		destFile.Close()
		return fmt.Errorf("copy: %w", err)
	}

	if err := destFile.Sync(); err != nil {
		destFile.Close()
		return fmt.Errorf("sync: %w", err)
	}

	if err := destFile.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	// Ensure mode is set (may need to be done after close on some systems)
	if err := os.Chmod(dest, mode); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	return nil
}

// mkdirAllRecording creates a directory and all parents, returning the list
// of directories that were actually created (in order from parent to child).
func mkdirAllRecording(path string, mode os.FileMode) ([]string, error) {
	// Find the first ancestor that exists
	var toCreate []string
	current := path

	for {
		if _, err := os.Stat(current); err == nil {
			// This directory exists
			break
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat %s: %w", current, err)
		}
		toCreate = append([]string{current}, toCreate...)
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root
			break
		}
		current = parent
	}

	// Create directories in order
	for _, dir := range toCreate {
		if err := os.Mkdir(dir, mode); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	return toCreate, nil
}
