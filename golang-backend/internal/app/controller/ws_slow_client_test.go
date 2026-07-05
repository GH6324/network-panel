package controller

import (
	"errors"
	"testing"
	"time"
)

func TestWriteAdminClientsBoundedParallelDoesNotSerializeSlowClients(t *testing.T) {
	clients := []*adminClient{{}, {}}
	start := time.Now()
	drops := writeAdminClientsBoundedParallel(clients, []byte(`{"type":"status"}`), 2, func(*adminClient, []byte) error {
		time.Sleep(80 * time.Millisecond)
		return nil
	})
	elapsed := time.Since(start)
	if len(drops) != 0 {
		t.Fatalf("expected no dropped clients, got %d", len(drops))
	}
	if elapsed > 140*time.Millisecond {
		t.Fatalf("broadcast writes appear serialized, elapsed=%s", elapsed)
	}
}

func TestWriteAdminClientsBoundedParallelCollectsFailedClients(t *testing.T) {
	clients := []*adminClient{{}, {}, {}}
	failed := clients[1]
	drops := writeAdminClientsBoundedParallel(clients, []byte(`{"type":"status"}`), 2, func(ac *adminClient, _ []byte) error {
		if ac == failed {
			return errors.New("write failed")
		}
		return nil
	})
	if len(drops) != 1 || drops[0] != failed {
		t.Fatalf("expected failed client to be collected, got %#v", drops)
	}
}
