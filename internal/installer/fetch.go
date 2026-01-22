package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anthropics/alloy/internal/pkg"
)

// fetchSource downloads and extracts the package source.
// Returns the path to the extracted source directory.
func (i *Installer) fetchSource(p *pkg.Package) (string, error) {
	source := p.ExpandedSource()

	// Create temp directory for extraction
	srcDir, err := os.MkdirTemp("", "alloy-"+p.Name+"-")
	if err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}

	switch source.SourceType() {
	case "url":
		if err := i.fetchURL(source.URL, source.SHA256, source.Strip, srcDir); err != nil {
			os.RemoveAll(srcDir)
			return "", err
		}
	case "binary":
		if err := i.fetchBinary(source.Binary, source.SHA256, p.Name, srcDir); err != nil {
			os.RemoveAll(srcDir)
			return "", err
		}
	case "git":
		if err := i.fetchGit(source.Git, source.Ref, srcDir); err != nil {
			os.RemoveAll(srcDir)
			return "", err
		}
	default:
		os.RemoveAll(srcDir)
		return "", fmt.Errorf("unknown source type: %s", source.SourceType())
	}

	return srcDir, nil
}

// fetchURL downloads and extracts an archive.
func (i *Installer) fetchURL(url, expectedChecksum string, strip int, destDir string) error {
	i.progress("Downloading %s", url)

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "alloy-download-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Download
	resp, err := http.Get(url)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Hash while downloading
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	size, err := io.Copy(writer, resp.Body)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download: %w", err)
	}
	tmpFile.Close()

	// Verify checksum
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	i.progress("Downloaded %d bytes, checksum verified", size)

	// Extract archive
	return i.extractArchive(tmpPath, url, strip, destDir)
}

// fetchBinary downloads a standalone binary.
func (i *Installer) fetchBinary(url, expectedChecksum, name, destDir string) error {
	i.progress("Downloading binary %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Create binary file
	binPath := filepath.Join(destDir, name)
	f, err := os.Create(binPath)
	if err != nil {
		return fmt.Errorf("create binary file: %w", err)
	}

	// Hash while downloading
	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)

	size, err := io.Copy(writer, resp.Body)
	if err != nil {
		f.Close()
		return fmt.Errorf("download: %w", err)
	}
	f.Close()

	// Make executable
	if err := os.Chmod(binPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Verify checksum
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	i.progress("Downloaded %d bytes, checksum verified", size)
	return nil
}

// fetchGit clones a git repository.
func (i *Installer) fetchGit(repoURL, ref, destDir string) error {
	i.progress("Cloning %s", repoURL)

	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repoURL, destDir)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	return nil
}

// extractArchive extracts an archive to the destination directory.
func (i *Installer) extractArchive(archivePath, url string, strip int, destDir string) error {
	// Determine archive type from URL
	lowerURL := strings.ToLower(url)

	switch {
	case strings.HasSuffix(lowerURL, ".tar.gz") || strings.HasSuffix(lowerURL, ".tgz"):
		return i.extractTarGz(archivePath, strip, destDir)
	case strings.HasSuffix(lowerURL, ".tar.xz") || strings.HasSuffix(lowerURL, ".txz"):
		return i.extractTarXz(archivePath, strip, destDir)
	case strings.HasSuffix(lowerURL, ".tar.bz2") || strings.HasSuffix(lowerURL, ".tbz2"):
		return i.extractTarBz2(archivePath, strip, destDir)
	case strings.HasSuffix(lowerURL, ".zip"):
		return i.extractZip(archivePath, strip, destDir)
	case strings.HasSuffix(lowerURL, ".tar"):
		return i.extractTar(archivePath, strip, destDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", url)
	}
}

// extractTarGz extracts a .tar.gz archive.
func (i *Installer) extractTarGz(archivePath string, strip int, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzr.Close()

	return i.extractTarReader(tar.NewReader(gzr), strip, destDir)
}

// extractTarXz extracts a .tar.xz archive using external xz command.
func (i *Installer) extractTarXz(archivePath string, strip int, destDir string) error {
	// Use external tar command for xz support
	args := []string{"-xJf", archivePath, "-C", destDir}
	if strip > 0 {
		args = append(args, fmt.Sprintf("--strip-components=%d", strip))
	}

	cmd := exec.Command("tar", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar xz: %w: %s", err, output)
	}
	return nil
}

// extractTarBz2 extracts a .tar.bz2 archive.
func (i *Installer) extractTarBz2(archivePath string, strip int, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	bzr := bzip2.NewReader(f)
	return i.extractTarReader(tar.NewReader(bzr), strip, destDir)
}

// extractTar extracts a plain .tar archive.
func (i *Installer) extractTar(archivePath string, strip int, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	return i.extractTarReader(tar.NewReader(f), strip, destDir)
}

// extractTarReader extracts from a tar.Reader.
func (i *Installer) extractTarReader(tr *tar.Reader, strip int, destDir string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		// Strip leading path components
		name := header.Name
		if strip > 0 {
			parts := strings.Split(name, "/")
			if len(parts) <= strip {
				continue
			}
			name = strings.Join(parts[strip:], "/")
		}

		if name == "" || name == "." {
			continue
		}

		target := filepath.Join(destDir, name)

		// Security: prevent path traversal
		if !strings.HasPrefix(target, destDir+string(filepath.Separator)) && target != destDir {
			return fmt.Errorf("invalid path in archive: %s", name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			if err := extractFile(tr, target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("extract %s: %w", target, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}
		case tar.TypeLink:
			// Hard link - resolve the link target
			linkTarget := header.Linkname
			if strip > 0 {
				parts := strings.Split(linkTarget, "/")
				if len(parts) > strip {
					linkTarget = strings.Join(parts[strip:], "/")
				}
			}
			linkTarget = filepath.Join(destDir, linkTarget)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("hardlink %s: %w", target, err)
			}
		}
	}

	return nil
}

// extractZip extracts a .zip archive.
func (i *Installer) extractZip(archivePath string, strip int, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.Name
		if strip > 0 {
			parts := strings.Split(name, "/")
			if len(parts) <= strip {
				continue
			}
			name = strings.Join(parts[strip:], "/")
		}

		if name == "" || name == "." {
			continue
		}

		target := filepath.Join(destDir, name)

		// Security: prevent path traversal
		if !strings.HasPrefix(target, destDir+string(filepath.Separator)) && target != destDir {
			return fmt.Errorf("invalid path in archive: %s", name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("mkdir parent: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}

		if err := extractFileFromReader(rc, target, f.Mode()); err != nil {
			rc.Close()
			return fmt.Errorf("extract %s: %w", target, err)
		}
		rc.Close()
	}

	return nil
}

// extractFile extracts a file from a reader to the target path.
func extractFile(r io.Reader, target string, mode os.FileMode) error {
	return extractFileFromReader(r, target, mode)
}

// extractFileFromReader extracts a file from a reader.
func extractFileFromReader(r io.Reader, target string, mode os.FileMode) error {
	// Ensure mode is valid
	if mode == 0 {
		mode = 0644
	}

	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

// verifyChecksum verifies a file's SHA256 checksum.
func verifyChecksum(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return errors.New("checksum mismatch")
	}
	return nil
}
