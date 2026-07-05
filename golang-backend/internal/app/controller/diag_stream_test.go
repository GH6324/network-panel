package controller

import (
	"strings"
	"testing"
)

func TestAppendBoundedTextKeepsTail(t *testing.T) {
	base := strings.Repeat("a", 90*1024)
	chunk := strings.Repeat("b", 90*1024)
	out := appendBoundedText(base, chunk, 64*1024)
	if len(out) > 80*1024 {
		t.Fatalf("bounded diagnostic text too large: %d", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected truncation marker")
	}
	if !strings.HasSuffix(out, strings.Repeat("b", 1024)) {
		t.Fatalf("expected newest tail to be retained")
	}
}
