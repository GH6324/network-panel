package controller

import (
    "time"

    dbpkg "network-panel/golang-backend/internal/db"
    "network-panel/golang-backend/internal/app/model"
)

// EnsureObserverOnce scans all forwards and pushes an UpdateService patch to attach the shared observer
// to entry services. It is safe to call repeatedly; agent will merge observers without duplicating.
func EnsureObserverOnce() {
    // For each node, recompute desired port-forward entry services (with observer) and add/upsert.
    var nodes []model.Node
    dbpkg.DB.Find(&nodes)
    for _, n := range nodes {
        svcs := desiredServices(n.ID) // port-forward only; includes observer injection
        if len(svcs) == 0 { continue }
        _ = sendWSCommand(n.ID, "AddService", svcs)
    }
}

// EnsureObserverLoop runs periodically to enforce observer attachment.
func EnsureObserverLoop(interval time.Duration) {
    if interval <= 0 { interval = 60 * time.Second }
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        EnsureObserverOnce()
        <-ticker.C
    }
}

// Build observer-only patches for tunnel-forward entry services on a node.
// Returns []map suitable for UpdateService: {name, handler:{observer}, _observers:[...]}
func BuildTunnelEntryObserverPatches(nodeID int64) []map[string]any {
    type row struct {
        ID       int64
        UserID   int64
        TunnelID int64
        InNodeID int64
        TType    int
    }
    var rows []row
    dbpkg.DB.Table("forward f").
        Select("f.id, f.user_id, f.tunnel_id, t.in_node_id, t.type as t_type").
        Joins("left join tunnel t on t.id = f.tunnel_id").
        Where("t.type = 2 AND t.in_node_id = ?", nodeID).
        Scan(&rows)
    patches := make([]map[string]any, 0, len(rows))
    for _, r := range rows {
        name := buildServiceName(r.ID, r.UserID, r.TunnelID)
        if obsName, spec := buildObserverPluginSpec(nodeID, name); obsName != "" && spec != nil {
            patches = append(patches, map[string]any{
                "name":       name,
                "observer":   obsName,
                "_observers": []any{spec},
            })
        }
    }
    return patches
}
