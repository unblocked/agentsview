package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSyncIfModified_CacheClearing(t *testing.T) {
	// Setup a minimal server instance (engine not needed for these failure cases)
	srv := &Server{}

	t.Run("NotExist_ClearsCache", func(t *testing.T) {
		path := "/non/existent/path/99999"
		sourcePath := path
		var lastMtime int64 = 12345

		srv.syncIfModified("s1", &sourcePath, &lastMtime)

		if sourcePath != "" {
			t.Errorf("expected sourcePath to be empty, got %q", sourcePath)
		}
		if lastMtime != 0 {
			t.Errorf("expected lastMtime to be 0, got %d", lastMtime)
		}
	})

	t.Run("NotDir_ClearsCache", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file")
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Use a path where a component is a file, causing ENOTDIR
		badPath := filepath.Join(filePath, "child")
		sourcePath := badPath
		var lastMtime int64 = 12345

		srv.syncIfModified("s1", &sourcePath, &lastMtime)

		if sourcePath != "" {
			t.Errorf("expected sourcePath to be empty, got %q", sourcePath)
		}
		if lastMtime != 0 {
			t.Errorf("expected lastMtime to be 0, got %d", lastMtime)
		}
	})

	t.Run("PermissionDenied_KeepsCache", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping Unix permission test on Windows")
		}
		if os.Getuid() == 0 {
			t.Skip("skipping permission test as root")
		}
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		targetPath := filepath.Join(subDir, "target")
		if err := os.WriteFile(targetPath, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		// Make subdir unreadable
		if err := os.Chmod(subDir, 0000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(subDir, 0755) // cleanup

		sourcePath := targetPath
		var lastMtime int64 = 12345

		// This should fail stat with permission denied, but keep the cache
		srv.syncIfModified("s1", &sourcePath, &lastMtime)

		if sourcePath != targetPath {
			t.Errorf("expected sourcePath to be preserved, got %q", sourcePath)
		}
		if lastMtime != 12345 {
			t.Errorf("expected lastMtime to be preserved, got %d", lastMtime)
		}
	})
}
