package sync

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// ComputeHash returns the SHA-256 hex digest of data from r.
func ComputeHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ComputeFileHash returns the SHA-256 hex digest of the file at path.
func ComputeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	hash, err := ComputeHash(f)
	if err != nil {
		return "", fmt.Errorf("hashing %s: %w", path, err)
	}
	return hash, nil
}
