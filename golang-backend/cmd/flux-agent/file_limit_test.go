package main

import (
	"os"
	"strings"
	"testing"
)

func TestReadFileLimitedRejectsOversizedFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "large-gost-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	if _, err := f.WriteString(strings.Repeat("x", 32)); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := readFileLimited(path, 16)
	if err == nil {
		t.Fatalf("expected oversized file error, got nil and %d bytes", len(b))
	}
	if len(b) != 0 {
		t.Fatalf("oversized read should not return partial content, got %d bytes", len(b))
	}
}

func TestReadFileLimitedAllowsBoundedFile(t *testing.T) {
	path := t.TempDir() + "/gost.json"
	if err := os.WriteFile(path, []byte(`{"services":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := readFileLimited(path, 1024)
	if err != nil {
		t.Fatalf("readFileLimited returned error: %v", err)
	}
	if string(b) != `{"services":[]}` {
		t.Fatalf("unexpected content: %q", string(b))
	}
}
