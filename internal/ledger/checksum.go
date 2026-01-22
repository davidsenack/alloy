package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// Checksum computes the SHA-256 checksum of a file and returns it as a
// hex-encoded string. Returns an error if the file cannot be read.
func Checksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return ChecksumReader(f)
}

// ChecksumReader computes the SHA-256 checksum from a reader.
func ChecksumReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ChecksumBytes computes the SHA-256 checksum of a byte slice.
func ChecksumBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// VerifyChecksum checks if a file's current checksum matches the expected value.
// Returns true if they match, false if they differ or if the file cannot be read.
func VerifyChecksum(path, expected string) (bool, error) {
	actual, err := Checksum(path)
	if err != nil {
		return false, err
	}
	return actual == expected, nil
}
