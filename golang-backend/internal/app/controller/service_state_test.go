package controller

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestUpdateNodeServicesBoundsSnapshot(t *testing.T) {
	old := nodeSvcs
	nodeSvcs = map[int64]*nodeSvcSnapshot{}
	t.Cleanup(func() { nodeSvcs = old })

	names := make([]string, 0, 1200)
	hashes := map[string]string{}
	for i := 0; i < 1200; i++ {
		name := fmt.Sprintf("svc-%04d-%s", i, strings.Repeat("n", 200))
		names = append(names, name)
		hashes[name] = strings.Repeat("h", 512)
	}
	updateNodeServices(7, names, hashes, time.Now().UnixMilli())

	svcNames, svcHashes, _, ok := getNodeServiceSnapshot(7)
	if !ok {
		t.Fatalf("snapshot missing")
	}
	if len(svcNames) > 512 {
		t.Fatalf("service names should be capped, got %d", len(svcNames))
	}
	if len(svcHashes) > 512 {
		t.Fatalf("hashes should be capped, got %d", len(svcHashes))
	}
	for name := range svcNames {
		if len(name) > 128 {
			t.Fatalf("service name should be truncated/sanitized, len=%d", len(name))
		}
	}
	for _, h := range svcHashes {
		if len(h) > 128 {
			t.Fatalf("hash value should be truncated, len=%d", len(h))
		}
	}
}
