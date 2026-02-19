package sync

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	helloWorldHash = "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"
	emptyInputHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func createTempFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test-file")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}
