package scheduler

import (
	"testing"

	"network-panel/golang-backend/internal/app/model"

	sqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPruneTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.NodeOpLog{},
		&model.NodeDiagResult{},
		&model.EasyTierResult{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestPruneOldDataOnceCleansStreamingDiagTablesByTime(t *testing.T) {
	db := newPruneTestDB(t)
	cutoff := int64(1000)
	if err := db.Create(&model.NodeDiagResult{NodeID: 1, RequestID: "old-diag", Type: "diag", Content: "old", TimeMs: 999}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.NodeDiagResult{NodeID: 1, RequestID: "new-diag", Type: "diag", Content: "new", TimeMs: 1001}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.EasyTierResult{NodeID: 1, RequestID: "old-et", Op: "install", Content: "old", TimeMs: 999}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.EasyTierResult{NodeID: 1, RequestID: "new-et", Op: "install", Content: "new", TimeMs: 1001}).Error; err != nil {
		t.Fatal(err)
	}

	pruneOldDataOnce(db, cutoff, 0)

	var diag []model.NodeDiagResult
	if err := db.Order("time_ms asc").Find(&diag).Error; err != nil {
		t.Fatal(err)
	}
	if len(diag) != 1 || diag[0].RequestID != "new-diag" {
		t.Fatalf("unexpected diag rows after prune: %#v", diag)
	}
	var et []model.EasyTierResult
	if err := db.Order("time_ms asc").Find(&et).Error; err != nil {
		t.Fatal(err)
	}
	if len(et) != 1 || et[0].RequestID != "new-et" {
		t.Fatalf("unexpected easytier rows after prune: %#v", et)
	}
}

func TestPruneOldDataOnceKeepsLatestRowsPerNode(t *testing.T) {
	db := newPruneTestDB(t)
	for i := 1; i <= 5; i++ {
		if err := db.Create(&model.NodeOpLog{NodeID: 7, RequestID: string(rune('a' + i - 1)), TimeMs: int64(i)}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.NodeDiagResult{NodeID: 7, RequestID: string(rune('A' + i - 1)), Type: "diag", TimeMs: int64(i)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Another node should be capped independently.
	for i := 1; i <= 2; i++ {
		if err := db.Create(&model.NodeOpLog{NodeID: 8, RequestID: string(rune('x' + i - 1)), TimeMs: int64(i)}).Error; err != nil {
			t.Fatal(err)
		}
	}

	pruneOldDataOnce(db, -1, 3)

	var logs []model.NodeOpLog
	if err := db.Where("node_id = ?", 7).Order("time_ms asc").Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if got := len(logs); got != 3 {
		t.Fatalf("node 7 op logs count=%d, want 3: %#v", got, logs)
	}
	if logs[0].TimeMs != 3 || logs[2].TimeMs != 5 {
		t.Fatalf("node 7 op logs should keep latest time_ms 3..5: %#v", logs)
	}
	var node8Count int64
	if err := db.Model(&model.NodeOpLog{}).Where("node_id = ?", 8).Count(&node8Count).Error; err != nil {
		t.Fatal(err)
	}
	if node8Count != 2 {
		t.Fatalf("node 8 should keep both rows, got %d", node8Count)
	}

	var diag []model.NodeDiagResult
	if err := db.Where("node_id = ?", 7).Order("time_ms asc").Find(&diag).Error; err != nil {
		t.Fatal(err)
	}
	if got := len(diag); got != 3 || diag[0].TimeMs != 3 || diag[2].TimeMs != 5 {
		t.Fatalf("diag rows should keep latest 3, got %#v", diag)
	}
}
