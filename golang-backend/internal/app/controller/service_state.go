package controller

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// In-memory node services snapshot reported by agents.
type nodeSvcSnapshot struct {
	names  map[string]struct{}
	timeMs int64
	hashes map[string]string // name -> hash(subset)
}

var (
	nodeSvcMu sync.RWMutex
	nodeSvcs  = map[int64]*nodeSvcSnapshot{} // nodeId -> latest bounded snapshot

	nodeSvcMaxEntries = getEnvInt("NODE_SERVICE_SNAPSHOT_MAX_ENTRIES", 512)
	nodeSvcMaxText    = getEnvInt("NODE_SERVICE_SNAPSHOT_TEXT_MAX_BYTES", 128)
	nodeSvcTTL        = time.Duration(getEnvInt("NODE_SERVICE_SNAPSHOT_TTL_SEC", 6*3600)) * time.Second
)

func updateNodeServices(nodeID int64, names []string, hashes map[string]string, timeMs int64) {
	if timeMs <= 0 {
		timeMs = time.Now().UnixMilli()
	}
	nowMs := time.Now().UnixMilli()
	m := make(map[string]struct{}, minInt(len(names), nodeSvcMaxEntries))
	outHashes := make(map[string]string, minInt(len(hashes), nodeSvcMaxEntries))

	ordered := append([]string(nil), names...)
	sort.Strings(ordered)
	for _, raw := range ordered {
		if nodeSvcMaxEntries > 0 && len(m) >= nodeSvcMaxEntries {
			break
		}
		name := normalizeServiceSnapshotText(raw)
		if name == "" {
			continue
		}
		m[name] = struct{}{}
		if h, ok := hashes[raw]; ok {
			outHashes[name] = normalizeServiceSnapshotText(h)
		}
	}
	// Include hash-only keys as a fallback for older agents, still bounded.
	if len(outHashes) < nodeSvcMaxEntries {
		keys := make([]string, 0, len(hashes))
		for k := range hashes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, raw := range keys {
			if nodeSvcMaxEntries > 0 && len(outHashes) >= nodeSvcMaxEntries {
				break
			}
			name := normalizeServiceSnapshotText(raw)
			if name == "" {
				continue
			}
			if _, exists := outHashes[name]; exists {
				continue
			}
			outHashes[name] = normalizeServiceSnapshotText(hashes[raw])
		}
	}

	nodeSvcMu.Lock()
	cleanupNodeServicesLocked(nowMs)
	nodeSvcs[nodeID] = &nodeSvcSnapshot{names: m, timeMs: timeMs, hashes: outHashes}
	nodeSvcMu.Unlock()
}

func getNodeServiceSnapshot(nodeID int64) (names map[string]struct{}, hashes map[string]string, timeMs int64, ok bool) {
	nodeSvcMu.RLock()
	s, exists := nodeSvcs[nodeID]
	nodeSvcMu.RUnlock()
	if !exists || s == nil {
		return nil, nil, 0, false
	}
	return s.names, s.hashes, s.timeMs, true
}

func cleanupNodeServicesLocked(nowMs int64) {
	if nodeSvcTTL <= 0 {
		return
	}
	cutoff := nowMs - nodeSvcTTL.Milliseconds()
	for nodeID, snap := range nodeSvcs {
		if snap == nil || snap.timeMs < cutoff {
			delete(nodeSvcs, nodeID)
		}
	}
}

func normalizeServiceSnapshotText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return truncateTailString(s, nodeSvcMaxText)
}

func minInt(a, b int) int {
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
