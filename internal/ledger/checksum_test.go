package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChecksum(t *testing.T) {
	dir := t.TempDir()

	// Create a test file
	path := filepath.Join(dir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Compute checksum
	checksum, err := Checksum(path)
	if err != nil {
		t.Fatalf("Checksum: %v", err)
	}

	// SHA-256 of "Hello, World!" is known
	// echo -n "Hello, World!" | sha256sum
	expected := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	if checksum != expected {
		t.Errorf("Checksum = %s, want %s", checksum, expected)
	}
}

func TestChecksumBytes(t *testing.T) {
	content := []byte("Hello, World!")
	checksum := ChecksumBytes(content)

	expected := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	if checksum != expected {
		t.Errorf("ChecksumBytes = %s, want %s", checksum, expected)
	}
}

func TestChecksumReader(t *testing.T) {
	r := strings.NewReader("Hello, World!")
	checksum, err := ChecksumReader(r)
	if err != nil {
		t.Fatalf("ChecksumReader: %v", err)
	}

	expected := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	if checksum != expected {
		t.Errorf("ChecksumReader = %s, want %s", checksum, expected)
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := []byte("Hello, World!")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"

	// Should match
	match, err := VerifyChecksum(path, expected)
	if err != nil {
		t.Fatalf("VerifyChecksum: %v", err)
	}
	if !match {
		t.Error("VerifyChecksum returned false for matching checksum")
	}

	// Should not match
	match, err = VerifyChecksum(path, "wrong-checksum")
	if err != nil {
		t.Fatalf("VerifyChecksum: %v", err)
	}
	if match {
		t.Error("VerifyChecksum returned true for wrong checksum")
	}
}

func TestChecksumNonexistent(t *testing.T) {
	_, err := Checksum("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
