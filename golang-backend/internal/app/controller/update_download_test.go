package controller

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadToTmpRejectsOversizedBody(t *testing.T) {
	t.Setenv("UPDATE_DOWNLOAD_MAX_BYTES", "10")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("01234567890"))
	}))
	defer srv.Close()

	path, err := downloadToTmp(srv.URL)
	if path != "" {
		_ = os.Remove(path)
	}
	if err == nil || !strings.Contains(err.Error(), "exceeds max download size") {
		t.Fatalf("expected oversized download error, got path=%q err=%v", path, err)
	}
}

func TestDownloadToPathRejectsOversizedBodyAndRemovesTmp(t *testing.T) {
	t.Setenv("UPDATE_DOWNLOAD_MAX_BYTES", "10")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("01234567890"))
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "server")
	err := downloadToPath(srv.URL, dst, 0o755)
	if err == nil || !strings.Contains(err.Error(), "exceeds max download size") {
		t.Fatalf("expected oversized download error, got %v", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("destination should not exist after oversized download, stat err=%v", err)
	}
	if _, err := os.Stat(dst + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp file should be removed after oversized download, stat err=%v", err)
	}
}

func TestUnzipToRejectsOversizedEntry(t *testing.T) {
	t.Setenv("UPDATE_UNZIP_MAX_ENTRY_BYTES", "10")
	zipPath := filepath.Join(t.TempDir(), "web.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("index.html")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("01234567890"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	err = unzipTo(zipPath, dest)
	if err == nil || !strings.Contains(err.Error(), "exceeds max unzip entry size") {
		t.Fatalf("expected oversized unzip entry error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "index.html")); !os.IsNotExist(err) {
		t.Fatalf("oversized zip entry should not be installed, stat err=%v", err)
	}
}
