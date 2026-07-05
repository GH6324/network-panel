package controller

import (
	"strings"
	"testing"
)

func TestBuildGostConfigReadScriptIsBounded(t *testing.T) {
	script := buildGostConfigReadScript(65536)
	if strings.Contains(script, `cat "$p"`) {
		t.Fatalf("gost config script must not cat the full file: %s", script)
	}
	if !strings.Contains(script, "head -c 65536") {
		t.Fatalf("script should read only a bounded prefix: %s", script)
	}
	if !strings.Contains(script, "TRUNCATED") {
		t.Fatalf("script should emit an explicit truncation marker: %s", script)
	}
}

func TestBuildGostConfigReadScriptUsesSafeDefault(t *testing.T) {
	script := buildGostConfigReadScript(0)
	if !strings.Contains(script, "head -c 65536") {
		t.Fatalf("script should use 64KiB default for non-positive limit: %s", script)
	}
}
