package controller

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"network-panel/golang-backend/internal/app/model"
	dbpkg "network-panel/golang-backend/internal/db"
)

func TestShouldSkipGostServiceForForward_LocalAnyTLSPortUsesRuntimeOnly(t *testing.T) {
	setupForwardAnyTLSTestDB(t)
	if err := dbpkg.DB.Create(&model.AnyTLSSetting{NodeID: 116, Port: 10087, Password: "secret"}).Error; err != nil {
		t.Fatalf("seed anytls setting: %v", err)
	}
	forward := model.Forward{InPort: 10087, RemoteAddr: "82.158.226.99:10087"}
	tunnel := model.Tunnel{Type: 1, InNodeID: 116}

	if !shouldSkipGostServiceForForward(tunnel, forward) {
		t.Fatalf("local AnyTLS runtime port should not create a GOST :%d forward", forward.InPort)
	}
}

func TestShouldSkipGostServiceForForward_LocalNonAnyTLSPortStillUsesGost(t *testing.T) {
	setupForwardAnyTLSTestDB(t)
	if err := dbpkg.DB.Create(&model.AnyTLSSetting{NodeID: 116, Port: 10087, Password: "secret"}).Error; err != nil {
		t.Fatalf("seed anytls setting: %v", err)
	}
	forward := model.Forward{InPort: 10088, RemoteAddr: "82.158.226.99:10087"}
	tunnel := model.Tunnel{Type: 1, InNodeID: 116}

	if shouldSkipGostServiceForForward(tunnel, forward) {
		t.Fatalf("non-AnyTLS local port should still create a GOST forward")
	}
}

func TestShouldSkipGostServiceForForward_AnyTLSTunnelUsesRuntimeOnly(t *testing.T) {
	setupForwardAnyTLSTestDB(t)
	protocol := "anytls"
	outNodeID := int64(116)
	outPort := 10087
	forward := model.Forward{InPort: 10087, OutPort: &outPort, RemoteAddr: "82.158.226.99:10087"}
	tunnel := model.Tunnel{Type: 1, InNodeID: 116, OutNodeID: &outNodeID, Protocol: &protocol}

	if !shouldSkipGostServiceForForward(tunnel, forward) {
		t.Fatalf("AnyTLS forwards should be served by flux-agent runtime only, not by a GOST :%d forward", forward.InPort)
	}
}

func TestShouldSkipGostServiceForForward_AnyTLSTunnelDifferentPortsStillUsesGost(t *testing.T) {
	setupForwardAnyTLSTestDB(t)
	if err := dbpkg.DB.Create(&model.AnyTLSSetting{NodeID: 116, Port: 10087, Password: "secret"}).Error; err != nil {
		t.Fatalf("seed anytls setting: %v", err)
	}
	protocol := "anytls"
	outNodeID := int64(116)
	outPort := 10088
	forward := model.Forward{InPort: 10087, OutPort: &outPort, RemoteAddr: "82.158.226.99:10087"}
	tunnel := model.Tunnel{Type: 1, InNodeID: 116, OutNodeID: &outNodeID, Protocol: &protocol}

	if !shouldSkipGostServiceForForward(tunnel, forward) {
		t.Fatalf("AnyTLS runtime entry port should skip GOST even when the selected exit port differs")
	}
}

func TestShouldSkipGostServiceForForward_AnyTLSDifferentEntryNodeStillUsesGost(t *testing.T) {
	setupForwardAnyTLSTestDB(t)
	protocol := "anytls"
	outNodeID := int64(116)
	outPort := 10087
	forward := model.Forward{InPort: 10087, OutPort: &outPort, RemoteAddr: "82.158.226.99:10087"}
	tunnel := model.Tunnel{Type: 1, InNodeID: 115, OutNodeID: &outNodeID, Protocol: &protocol}

	if shouldSkipGostServiceForForward(tunnel, forward) {
		t.Fatalf("different entry/exit nodes still need the entry GOST forward")
	}
}

func TestShouldSkipGostServiceForForward_NormalTCPForwardStillUsesGost(t *testing.T) {
	setupForwardAnyTLSTestDB(t)
	outPort := 10087
	forward := model.Forward{InPort: 10088, OutPort: &outPort, RemoteAddr: "82.158.226.99:10087"}
	tunnel := model.Tunnel{Type: 1, InNodeID: 116}

	if shouldSkipGostServiceForForward(tunnel, forward) {
		t.Fatalf("normal TCP forward should still use GOST")
	}
}

func TestFindAnyTLSGostLoopServiceNames_FindsPanelManagedRUDPLoopsOnly(t *testing.T) {
	setupForwardAnyTLSTestDB(t)
	if err := dbpkg.DB.Create(&model.AnyTLSSetting{NodeID: 116, Port: 10087, Password: "secret"}).Error; err != nil {
		t.Fatalf("seed anytls setting: %v", err)
	}

	services := []map[string]any{
		{
			"name":     "42_1_0",
			"addr":     ":10087",
			"listener": map[string]any{"type": "tcp"},
			"handler":  map[string]any{"type": "forward"},
			"metadata": map[string]any{"managedBy": "network-panel"},
		},
		{
			"name":     "42_1_0_rudp",
			"addr":     ":10087",
			"listener": map[string]any{"type": "rudp"},
			"handler":  map[string]any{"type": "forward"},
			"metadata": map[string]any{"managedBy": "network-panel"},
		},
		{
			"name":     "normal_forward",
			"addr":     ":10088",
			"listener": map[string]any{"type": "rudp"},
			"handler":  map[string]any{"type": "forward"},
			"metadata": map[string]any{"managedBy": "network-panel"},
		},
		{
			"name":     "manual_service",
			"addr":     ":10087",
			"listener": map[string]any{"type": "rudp"},
			"handler":  map[string]any{"type": "forward"},
		},
	}

	got := findAnyTLSGostLoopServiceNames(116, services)
	if len(got) != 1 || got[0] != "42_1_0" {
		t.Fatalf("expected base service to be selected once, got %#v", got)
	}
}

func setupForwardAnyTLSTestDB(t *testing.T) {
	t.Helper()
	oldDB := dbpkg.DB
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ViteConfig{}, &model.AnyTLSSetting{}, &model.AnyTLSPortEgress{}, &model.ExitSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	dbpkg.DB = db
	t.Cleanup(func() { dbpkg.DB = oldDB })
}
