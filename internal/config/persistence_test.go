package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCursorSecret_GeneratedAndPersisted(t *testing.T) {
	// 1. Setup a clean temp directory
	tmp := t.TempDir()
	t.Setenv("AGENT_VIEWER_DATA_DIR", tmp)

	// 2. First load: should generate a secret
	cfg1, err := LoadMinimal()
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}
	if cfg1.CursorSecret == "" {
		t.Fatal("cursor secret was not generated")
	}

	// Verify file existence and content
	configPath := filepath.Join(tmp, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	var fileCfg struct {
		CursorSecret string `json:"cursor_secret"`
	}
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		t.Fatalf("parsing config file: %v", err)
	}
	if fileCfg.CursorSecret != cfg1.CursorSecret {
		t.Errorf("file secret = %q, want %q", fileCfg.CursorSecret, cfg1.CursorSecret)
	}

	// 3. Second load: should read the same secret
	cfg2, err := LoadMinimal()
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}
	if cfg2.CursorSecret != cfg1.CursorSecret {
		t.Errorf("second load got %q, want %q", cfg2.CursorSecret, cfg1.CursorSecret)
	}
}

func TestCursorSecret_RegeneratedIfMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGENT_VIEWER_DATA_DIR", tmp)

	// Create config with empty secret
	configPath := filepath.Join(tmp, "config.json")
	initialContent := `{"cursor_secret": ""}`
	if err := os.WriteFile(configPath, []byte(initialContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMinimal()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.CursorSecret == "" {
		t.Fatal("cursor secret should have been regenerated")
	}

	// Verify it was updated in the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == initialContent {
		t.Error("config file was not updated")
	}
}

func TestCursorSecret_LoadErrorOnInvalidConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGENT_VIEWER_DATA_DIR", tmp)

	// Create invalid config
	configPath := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(configPath, []byte("{invalid-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMinimal()
	if err == nil {
		t.Fatal("expected error loading invalid config")
	}
}

func TestCursorSecret_PreservesOtherFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGENT_VIEWER_DATA_DIR", tmp)

	// Create config with other fields but no secret
	configPath := filepath.Join(tmp, "config.json")
	initialContent := `{"github_token": "my-token"}`
	if err := os.WriteFile(configPath, []byte(initialContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMinimal()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.CursorSecret == "" {
		t.Error("cursor secret not generated")
	}
	if cfg.GithubToken != "my-token" {
		t.Errorf("github_token = %q, want %q", cfg.GithubToken, "my-token")
	}

	// Verify file content has both
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var fileCfg struct {
		CursorSecret string `json:"cursor_secret"`
		GithubToken  string `json:"github_token"`
	}
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		t.Fatal(err)
	}
	if fileCfg.CursorSecret == "" {
		t.Error("cursor_secret missing in file")
	}
	if fileCfg.GithubToken != "my-token" {
		t.Error("github_token lost/changed in file")
	}
}
