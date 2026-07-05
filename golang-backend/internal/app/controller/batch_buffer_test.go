package controller

import (
	"strings"
	"testing"
	"time"

	"network-panel/golang-backend/internal/app/model"
)

func TestEnqueueOpLogTruncatesLargePayloads(t *testing.T) {
	old := bufOp
	bufOp = nil
	t.Cleanup(func() { bufOp = old })

	large := strings.Repeat("x", 2*1024*1024)
	enqueueOpLog(model.NodeOpLog{
		TimeMs:    time.Now().UnixMilli(),
		NodeID:    1,
		Cmd:       "RunScriptResult",
		RequestID: "req-large",
		Success:   1,
		Message:   large,
		Stdout:    &large,
		Stderr:    &large,
	})

	rows := readBufferedOpLogsByReq("req-large")
	if len(rows) != 1 {
		t.Fatalf("expected one buffered op log, got %d", len(rows))
	}
	row := rows[0]
	if len(row.Message) > 80*1024 {
		t.Fatalf("message should be truncated, got %d bytes", len(row.Message))
	}
	if row.Stdout == nil || len(*row.Stdout) > 80*1024 || !strings.Contains(*row.Stdout, "truncated") {
		t.Fatalf("stdout should be bounded with marker, len=%d", len(testDerefString(row.Stdout)))
	}
	if row.Stderr == nil || len(*row.Stderr) > 80*1024 || !strings.Contains(*row.Stderr, "truncated") {
		t.Fatalf("stderr should be bounded with marker, len=%d", len(testDerefString(row.Stderr)))
	}
}

func testDerefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
