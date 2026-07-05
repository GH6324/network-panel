package main

import (
	"crypto/tls"
	"testing"
)

func TestAnyTLSServerTLSConfigIsHardened(t *testing.T) {
	cfg := newAnyTLSServerTLSConfig(tls.Certificate{})
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %#x, want TLS1.2", cfg.MinVersion)
	}
	if !cfg.SessionTicketsDisabled {
		t.Fatalf("server TLS session tickets should be disabled by default")
	}
	if len(cfg.CipherSuites) == 0 {
		t.Fatalf("expected explicit TLS1.2 cipher allowlist")
	}
	if len(cfg.CurvePreferences) == 0 || cfg.CurvePreferences[0] != tls.X25519 {
		t.Fatalf("expected X25519-first curve preferences, got %#v", cfg.CurvePreferences)
	}
	if len(cfg.NextProtos) != 2 || cfg.NextProtos[0] != "h2" || cfg.NextProtos[1] != "http/1.1" {
		t.Fatalf("unexpected ALPN list: %#v", cfg.NextProtos)
	}
}
