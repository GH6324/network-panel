package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func withMockGostAPIServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock gost api: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	t.Setenv("GOST_API_PORT", strconv.Itoa(port))
	oldClient := gostAPIHTTPClient
	gostAPIHTTPClient = srv.Client()
	t.Cleanup(func() { gostAPIHTTPClient = oldClient })
}

func TestAPIGetServicesListRejectsTooManyItems(t *testing.T) {
	t.Setenv("AGENT_GOST_API_SERVICES_MAX_ITEMS", "2")
	withMockGostAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/config/services" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"svc-a"},{"name":"svc-b"},{"name":"svc-c"}]`)
	})

	list, err := apiGetServicesList()
	if err == nil {
		t.Fatalf("expected max services error, got nil and %d services", len(list))
	}
	if !strings.Contains(err.Error(), "too many services") {
		t.Fatalf("expected too many services error, got %v", err)
	}
}

func TestAPIGetServicesListAllowsBoundedItems(t *testing.T) {
	t.Setenv("AGENT_GOST_API_SERVICES_MAX_ITEMS", "2")
	withMockGostAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"svc-a"},{"name":"svc-b"}]`)
	})

	list, err := apiGetServicesList()
	if err != nil {
		t.Fatalf("apiGetServicesList returned error: %v", err)
	}
	if len(list) != 2 || list[0]["name"] != "svc-a" || list[1]["name"] != "svc-b" {
		t.Fatalf("unexpected list: %#v", list)
	}
}

func TestAPIGetServicesListUsesBoundedStreaming(t *testing.T) {
	t.Setenv("AGENT_GOST_API_SERVICES_MAX_ITEMS", "1")
	writeDone := make(chan struct{})
	withMockGostAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, `[{"name":"svc-a"},{"name":"svc-b"}`)
		if flusher != nil {
			flusher.Flush()
		}
		close(writeDone)
		// Keep the response open unless the client closes first. The client should
		// reject after item 2 instead of waiting for the full array to finish.
		select {
		case <-r.Context().Done():
			return
		case <-time.After(3 * time.Second):
			fmt.Fprint(w, `]`)
		}
	})

	start := time.Now()
	_, err := apiGetServicesList()
	if err == nil || !strings.Contains(err.Error(), "too many services") {
		t.Fatalf("expected too many services error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected streaming limit to fail fast, took %s", elapsed)
	}
	<-writeDone
}

func TestFilterAnyTLSGostLoopServicesSkipsOnlyPanelManagedRuntimePortForwards(t *testing.T) {
	services := []map[string]any{
		{
			"name":     "116_1_0",
			"addr":     ":10087",
			"listener": map[string]any{"type": "tcp"},
			"handler":  map[string]any{"type": "forward"},
			"metadata": map[string]any{"managedBy": "network-panel"},
		},
		{
			"name":     "116_1_0_rudp",
			"addr":     ":10087",
			"listener": map[string]any{"type": "rudp"},
			"handler":  map[string]any{"type": "forward"},
			"metadata": map[string]any{"managedBy": "network-panel"},
		},
		{
			"name":     "normal_rudp",
			"addr":     ":1000",
			"listener": map[string]any{"type": "rudp"},
			"handler":  map[string]any{"type": "forward"},
			"metadata": map[string]any{"managedBy": "network-panel"},
		},
		{
			"name":     "manual_10087",
			"addr":     ":10087",
			"listener": map[string]any{"type": "rudp"},
			"handler":  map[string]any{"type": "forward"},
		},
	}

	filtered, skipped := filterAnyTLSGostLoopServices(services, map[int]struct{}{10087: {}})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 safe services, got %#v", filtered)
	}
	if filtered[0]["name"] != "normal_rudp" || filtered[1]["name"] != "manual_10087" {
		t.Fatalf("unexpected filtered services: %#v", filtered)
	}
	if len(skipped) != 2 || skipped[0] != "116_1_0" || skipped[1] != "116_1_0_rudp" {
		t.Fatalf("unexpected skipped names: %#v", skipped)
	}
}
