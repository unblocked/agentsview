package sync

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeHash(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
	}{
		{
			name:  "hello world",
			input: "hello world\n",
			want:  helloWorldHash,
		},
		{
			name:  "empty input",
			input: "",
			want:  emptyInputHash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeHash(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("ComputeHash: %v", err)
			}
			if got != tt.want {
				t.Errorf("ComputeHash() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComputeFileHash(t *testing.T) {
	tests := []struct {
		name    string
		content []byte // nil means file does not exist
		want    string
		wantErr bool
	}{
		{
			name:    "hello world",
			content: []byte("hello world\n"),
			want:    helloWorldHash,
		},
		{
			name:    "empty file",
			content: []byte(""),
			want:    emptyInputHash,
		},
		{
			name:    "missing file",
			content: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.content != nil {
				path = createTempFile(t, tt.content)
			} else {
				path = filepath.Join(t.TempDir(), "nonexistent.txt")
			}

			got, err := ComputeFileHash(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				var pathErr *fs.PathError
				if !errors.As(err, &pathErr) {
					t.Errorf("expected *fs.PathError, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ComputeFileHash: %v", err)
			}
			if got != tt.want {
				t.Errorf("ComputeFileHash() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComputeHash_ReaderError(t *testing.T) {
	errInjected := errors.New("injected error")
	reader := &failingReader{err: errInjected}
	_, err := ComputeHash(reader)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("expected error wrapping 'injected error', got %v", err)
	}
}

func TestComputeFileHash_ReadError(t *testing.T) {
	// Use a directory to simulate a read error after open
	dir := t.TempDir()
	_, err := ComputeFileHash(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// On most systems, reading a directory fails.
	// We expect the error to be wrapped with "hashing <path>"
	if !strings.Contains(err.Error(), "hashing "+dir) {
		t.Errorf("expected error to contain 'hashing %s', got %v", dir, err)
	}

	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("expected error to wrap *fs.PathError, got %T: %v", err, err)
	}
}

type failingReader struct {
	err error
}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, f.err
}
