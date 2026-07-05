package controller

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	sqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"network-panel/golang-backend/internal/app/model"
	dbpkg "network-panel/golang-backend/internal/db"
)

func setupTunnelExternalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := dbpkg.DB
	t.Cleanup(func() { dbpkg.DB = oldDB })
	name := regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(t.Name(), "_")
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&model.Node{}, &model.Tunnel{}, &model.ExitNodeExternal{}, &model.ViteConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	dbpkg.DB = db
	return db
}

func callTunnelCreate(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/tunnel/create", bytes.NewReader(b))
	c.Request.Header.Set("Content-Type", "application/json")
	TunnelCreate(c)
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return resp
}

func TestTunnelCreate_AllowsExternalExitOnlyLine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTunnelExternalTestDB(t)
	now := time.Now().UnixMilli()
	status := 1
	proto := "anytls"
	config := `{"password":"pw","sni":"edge.example.com"}`
	ext := model.ExitNodeExternal{
		BaseEntity: model.BaseEntity{ID: 91, CreatedTime: now, UpdatedTime: now, Status: &status},
		Name:       "外部-anytls-香港",
		Host:       "hk.example.com",
		Port:       443,
		Protocol:   &proto,
		Config:     &config,
	}
	if err := db.Create(&ext).Error; err != nil {
		t.Fatalf("create external exit: %v", err)
	}

	resp := callTunnelCreate(t, map[string]any{
		"name":      "外部直连线路",
		"type":      2,
		"flow":      1,
		"inNodeId":  nil,
		"outExitId": float64(91),
		"protocol":  "anytls",
	})
	if code, _ := resp["code"].(float64); code != 0 {
		t.Fatalf("expected create success, got response: %#v", resp)
	}
	var got model.Tunnel
	if err := db.Where("name = ?", "外部直连线路").First(&got).Error; err != nil {
		t.Fatalf("created tunnel not found: %v", err)
	}
	if got.InNodeID != 0 {
		t.Fatalf("external-only line should not require entry node, got in_node_id=%d", got.InNodeID)
	}
	if got.OutExitID == nil || *got.OutExitID != 91 {
		t.Fatalf("expected out_exit_id=91, got %#v", got.OutExitID)
	}
	if got.OutNodeID != nil {
		t.Fatalf("external-only line must not set managed out node: %#v", got.OutNodeID)
	}
}

func TestTunnelCreate_RejectsManagedLineWithoutEntryNode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = setupTunnelExternalTestDB(t)
	resp := callTunnelCreate(t, map[string]any{
		"name": "无入口普通线路",
		"type": 2,
		"flow": 1,
	})
	if code, _ := resp["code"].(float64); code == 0 {
		t.Fatalf("expected managed line without entry to fail, got response: %#v", resp)
	}
}
