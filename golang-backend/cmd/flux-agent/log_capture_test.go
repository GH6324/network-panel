package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestReadFileFromOffsetIsBounded(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "large-log-*.log")
	if err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", 2*1024*1024)
	if _, err := f.WriteString(large); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := readFileFromOffset(f.Name(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > 80*1024 {
		t.Fatalf("log capture should be bounded, got %d bytes", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("bounded log should contain truncation marker")
	}
}

func TestJournalctlArgsAreBounded(t *testing.T) {
	args := journalctlArgs("gost", 1700000000)
	joined := strings.Join(args, " ")
	for _, want := range []string{"-u", "gost", "--since", "@1700000000", "--no-pager", "--lines", "--output", "short-iso"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("journalctl args missing %q in %q", want, joined)
		}
	}
}

func TestFetchURLRejectsOversizedScript(t *testing.T) {
	t.Setenv("AGENT_RUN_SCRIPT_URL_MAX_BYTES", "32")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 64)))
	}))
	defer srv.Close()

	_, err := fetchURL(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "response too large") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestAppendBoundedBuilderTailKeepsRecentOutput(t *testing.T) {
	var b strings.Builder
	appendBoundedBuilderTail(&b, strings.Repeat("a", 40), 64)
	appendBoundedBuilderTail(&b, strings.Repeat("b", 80), 64)
	out := b.String()
	if len(out) > 64 {
		t.Fatalf("stream chunk should be bounded, got %d", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("bounded stream chunk should include truncation marker: %q", out)
	}
	if !strings.HasSuffix(out, strings.Repeat("b", len(out)-strings.LastIndex(out, "\n")-1)) && !strings.Contains(out, "bbbb") {
		t.Fatalf("bounded stream chunk should retain recent output: %q", out)
	}
}

func TestStreamScriptChunkMaxBytesEnv(t *testing.T) {
	t.Setenv("AGENT_STREAM_SCRIPT_CHUNK_MAX_BYTES", "123")
	if got := streamScriptChunkMaxBytes(); got != 123 {
		t.Fatalf("streamScriptChunkMaxBytes()=%d", got)
	}
	if got := fmt.Sprint(runScriptURLMaxBytes()); got == "" {
		t.Fatalf("runScriptURLMaxBytes should have a default")
	}
}
