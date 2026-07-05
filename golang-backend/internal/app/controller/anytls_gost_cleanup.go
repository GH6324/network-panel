package controller

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"network-panel/golang-backend/internal/app/model"
	dbpkg "network-panel/golang-backend/internal/db"
)

func anyTLSRuntimePortSet(nodeID int64) map[int]struct{} {
	ports := map[int]struct{}{}
	if nodeID <= 0 {
		return ports
	}
	for _, pm := range listAnyTLSPortMappings(nodeID) {
		if pm.Port > 0 {
			ports[pm.Port] = struct{}{}
		}
	}
	var at model.AnyTLSSetting
	if err := dbpkg.DB.Where("node_id = ?", nodeID).First(&at).Error; err == nil && at.Port > 0 {
		ports[at.Port] = struct{}{}
	}
	return ports
}

func findAnyTLSGostLoopServiceNames(nodeID int64, services []map[string]any) []string {
	ports := anyTLSRuntimePortSet(nodeID)
	if len(ports) == 0 || len(services) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, svc := range services {
		if svc == nil {
			continue
		}
		port := getServicePort(svc)
		if port <= 0 {
			continue
		}
		if _, ok := ports[port]; !ok {
			continue
		}
		if !isPanelManagedGostForwardService(svc) {
			continue
		}
		name, _ := svc["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		base := strings.TrimSuffix(name, "_rudp")
		if base == "" {
			base = name
		}
		if _, ok := seen[base]; ok {
			continue
		}
		seen[base] = struct{}{}
		out = append(out, base)
	}
	sort.Strings(out)
	return out
}

func isPanelManagedGostForwardService(svc map[string]any) bool {
	if svc == nil {
		return false
	}
	meta, _ := svc["metadata"].(map[string]any)
	managedBy, _ := meta["managedBy"].(string)
	if strings.TrimSpace(managedBy) != "network-panel" {
		return false
	}
	handler, _ := svc["handler"].(map[string]any)
	if typ, _ := handler["type"].(string); strings.TrimSpace(typ) != "forward" {
		return false
	}
	listener, _ := svc["listener"].(map[string]any)
	lt := strings.ToLower(strings.TrimSpace(anyString(listener["type"])))
	return lt == "tcp" || lt == "rudp"
}

func anyString(v any) string {
	s, _ := v.(string)
	return s
}

func cleanupAnyTLSGostLoopServices(nodeID int64) []string {
	services := queryNodeServicesRaw(nodeID)
	names := findAnyTLSGostLoopServiceNames(nodeID, services)
	if len(names) == 0 {
		return nil
	}
	payload := map[string]any{"services": expandNamesWithRUDP(names)}
	success := 1
	msg := "cleanup anytls gost rudp loop services"
	if err := sendWSCommand(nodeID, "DeleteService", payload); err != nil {
		success = 0
		msg = msg + ": " + err.Error()
	}
	stdout := ""
	if b, err := json.Marshal(map[string]any{"matched": names, "sent": payload["services"]}); err == nil {
		stdout = string(b)
	}
	_ = dbpkg.DB.Create(&model.NodeOpLog{
		TimeMs:  time.Now().UnixMilli(),
		NodeID:  nodeID,
		Cmd:     "AnyTLSGostLoopCleanup",
		Success: success,
		Message: msg,
		Stdout:  &stdout,
	}).Error
	return names
}
