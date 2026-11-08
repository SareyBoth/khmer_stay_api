package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageUpload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "khmerstay-upload-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	uploader := NewLocalStorage(tempDir, "/uploads")
	content := "fake-image-bytes"
	reader := strings.NewReader(content)

	url, err := uploader.Upload(context.Background(), reader, "stay.jpg", int64(len(content)), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/uploads/") || !strings.HasSuffix(url, ".jpg") {
		t.Fatalf("unexpected uploaded URL: %s", url)
	}

	savedFile := filepath.Join(tempDir, filepath.Base(url))
	data, err := os.ReadFile(savedFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("content mismatch: got %q, want %q", string(data), content)
	}
}
