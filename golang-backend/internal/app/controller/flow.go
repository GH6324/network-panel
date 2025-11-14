package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"network-panel/golang-backend/internal/app/dto"
	"network-panel/golang-backend/internal/app/model"
	dbpkg "network-panel/golang-backend/internal/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func FlowConfig(c *gin.Context) { c.String(http.StatusOK, "ok") }
func FlowTest(c *gin.Context)   { c.String(http.StatusOK, "test") }

// POST /flow/upload?secret=...
// Updates forward/user/usertunnel flow counters and pauses when limits exceeded.
func FlowUpload(c *gin.Context) {

	fmt.Println("FlowUpload called")
	secret := c.Query("secret")
	// validate node by secret (silent fail to avoid leaking info)
	var nodeCount int64
	dbpkg.DB.Model(&model.Node{}).Where("secret = ?", secret).Count(&nodeCount)
	if nodeCount == 0 {
		c.String(http.StatusOK, "ok")
		return
	}

	// read raw body once; support old and new formats
	body, _ := io.ReadAll(c.Request.Body)
	fmt.Println("FlowUpload called params:", string(body))

	// Try new observer events format first
	type obsStats struct {
		TotalConns   int   `json:"totalConns"`
		CurrentConns int   `json:"currentConns"`
		InputBytes   int64 `json:"inputBytes"`
		OutputBytes  int64 `json:"outputBytes"`
		TotalErrs    int   `json:"totalErrs"`
	}
	type obsEvent struct {
		Kind    string   `json:"kind"`
		Service string   `json:"service"`
		Type    string   `json:"type"`
		Stats   obsStats `json:"stats"`
	}
	var obsPayload struct {
		Events []obsEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &obsPayload); err == nil && len(obsPayload.Events) > 0 {
		// sum bytes across events of type stats
		var inBytes, outBytes int64
		var serviceName string
		for _, e := range obsPayload.Events {
			if strings.ToLower(e.Type) != "stats" {
				continue
			}
			inBytes += e.Stats.InputBytes
			outBytes += e.Stats.OutputBytes
			if e.Service != "" {
				serviceName = e.Service
			}
		}
		if inBytes == 0 && outBytes == 0 {
			c.String(http.StatusOK, "ok")
			return
		}
		// resolve forward id
		var fwdID int64
		if v := strings.TrimSpace(c.Query("id")); v != "" {
			fwdID, _ = strconv.ParseInt(v, 10, 64)
		}
		if fwdID == 0 && serviceName != "" {
			if i := strings.Index(serviceName, "_"); i > 0 {
				fwdID, _ = strconv.ParseInt(serviceName[:i], 10, 64)
			} else {
				fwdID, _ = strconv.ParseInt(serviceName, 10, 64)
			}
		}
		if fwdID == 0 {
			c.String(http.StatusOK, "ok")
			return
		}
		// load forward and tunnel
		var fwd model.Forward
		if err := dbpkg.DB.First(&fwd, fwdID).Error; err != nil {
			c.String(http.StatusOK, "ok")
			return
		}
		var tun model.Tunnel
		_ = dbpkg.DB.First(&tun, fwd.TunnelID).Error

		inInc, outInc := inBytes, outBytes
		if tun.Flow == 1 {
			outInc = inBytes + outBytes
			inInc = 0
		}

		// apply increments (forward, user, user_tunnel)
		dbpkg.DB.Model(&model.Forward{}).Where("id = ?", fwdID).
			Updates(map[string]any{"in_flow": gorm.Expr("in_flow + ?", inInc), "out_flow": gorm.Expr("out_flow + ?", outInc), "updated_time": time.Now().UnixMilli()})
		dbpkg.DB.Model(&model.User{}).Where("id = ?", fwd.UserID).
			Updates(map[string]any{"in_flow": gorm.Expr("in_flow + ?", inInc), "out_flow": gorm.Expr("out_flow + ?", outInc), "updated_time": time.Now().UnixMilli()})
		// user_tunnel
		var ut model.UserTunnel
		if err := dbpkg.DB.Where("user_id=? and tunnel_id=?", fwd.UserID, fwd.TunnelID).First(&ut).Error; err == nil && ut.ID > 0 {
			dbpkg.DB.Model(&model.UserTunnel{}).Where("id = ?", ut.ID).
				Updates(map[string]any{"in_flow": gorm.Expr("in_flow + ?", inInc), "out_flow": gorm.Expr("out_flow + ?", outInc)})
		}
		// limits
		var user model.User
		if err := dbpkg.DB.First(&user, fwd.UserID).Error; err == nil {
			if overUserLimit(user) || expired(user.ExpTime) || user.Status != nil && *user.Status != 1 {
				pauseAllUserForwards(user.ID)
				s := 0
				user.Status = &s
				_ = dbpkg.DB.Save(&user).Error
			}
		}
		if ut.ID != 0 {
			if overUTunnelLimit(ut) || expired(ut.ExpTime) || ut.Status != 1 {
				pauseUserTunnelForwards(ut.UserID, ut.TunnelID)
				ut.Status = 0
				_ = dbpkg.DB.Save(&ut).Error
			}
		}
		c.String(http.StatusOK, "ok")
		return
	}

	// Fallback to old simple format
	var payload dto.FlowDto
	if json.Unmarshal(body, &payload) != nil || payload.N == "" {
		c.String(http.StatusOK, "ok")
		return
	}
	if payload.N == "web_api" {
		c.String(http.StatusOK, "ok")
		return
	}
	parts := strings.Split(payload.N, "_")
	if len(parts) < 3 {
		c.String(http.StatusOK, "ok")
		return
	}
	fwdID, _ := strconv.ParseInt(parts[0], 10, 64)
	userID, _ := strconv.ParseInt(parts[1], 10, 64)
	utID, _ := strconv.ParseInt(parts[2], 10, 64)
	var fwd model.Forward
	if err := dbpkg.DB.First(&fwd, fwdID).Error; err != nil {
		c.String(http.StatusOK, "ok")
		return
	}
	var tun model.Tunnel
	_ = dbpkg.DB.First(&tun, fwd.TunnelID).Error
	inInc, outInc := payload.U, payload.D
	if tun.Flow == 1 {
		outInc = payload.U + payload.D
		inInc = 0
	}
	dbpkg.DB.Model(&model.Forward{}).Where("id = ?", fwdID).Updates(map[string]any{"in_flow": gorm.Expr("in_flow + ?", inInc), "out_flow": gorm.Expr("out_flow + ?", outInc), "updated_time": time.Now().UnixMilli()})
	dbpkg.DB.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{"in_flow": gorm.Expr("in_flow + ?", inInc), "out_flow": gorm.Expr("out_flow + ?", outInc), "updated_time": time.Now().UnixMilli()})
	if utID != 0 {
		dbpkg.DB.Model(&model.UserTunnel{}).Where("id = ?", utID).Updates(map[string]any{"in_flow": gorm.Expr("in_flow + ?", inInc), "out_flow": gorm.Expr("out_flow + ?", outInc)})
	}
	var user model.User
	if err := dbpkg.DB.First(&user, userID).Error; err == nil {
		if overUserLimit(user) || expired(user.ExpTime) || user.Status != nil && *user.Status != 1 {
			pauseAllUserForwards(user.ID)
			s := 0
			user.Status = &s
			_ = dbpkg.DB.Save(&user).Error
		}
	}
	if utID != 0 {
		var ut model.UserTunnel
		if err := dbpkg.DB.First(&ut, utID).Error; err == nil {
			if overUTunnelLimit(ut) || expired(ut.ExpTime) || ut.Status != 1 {
				pauseUserTunnelForwards(ut.UserID, ut.TunnelID)
				ut.Status = 0
				_ = dbpkg.DB.Save(&ut).Error
			}
		}
	}
	c.String(http.StatusOK, "ok")
}

// Over user limit if flow(GiB) <= in + out
func overUserLimit(u model.User) bool {
	limit := u.Flow * 1024 * 1024 * 1024
	return limit > 0 && (u.InFlow+u.OutFlow) > limit
}
func overUTunnelLimit(ut model.UserTunnel) bool {
	limit := ut.Flow * 1024 * 1024 * 1024
	return limit > 0 && (ut.InFlow+ut.OutFlow) > limit
}
func expired(ts *int64) bool { return ts != nil && *ts > 0 && *ts <= time.Now().UnixMilli() }

func pauseAllUserForwards(userID int64) {
	var forwards []model.Forward
	dbpkg.DB.Where("user_id = ?", userID).Find(&forwards)
	for _, f := range forwards {
		dbpkg.DB.Model(&model.Forward{}).Where("id = ?", f.ID).Update("status", 0)
		var t model.Tunnel
		if err := dbpkg.DB.First(&t, f.TunnelID).Error; err == nil {
			name := buildServiceName(f.ID, f.UserID, f.TunnelID)
			_ = sendWSCommand(t.InNodeID, "PauseService", map[string]interface{}{"services": []string{name}})
			if t.Type == 2 {
				_ = sendWSCommand(outNodeIDOr0(t), "PauseService", map[string]interface{}{"services": []string{name}})
			}
		}
	}
}
func pauseUserTunnelForwards(userID, tunnelID int64) {
	var forwards []model.Forward
	dbpkg.DB.Where("user_id = ? AND tunnel_id = ?", userID, tunnelID).Find(&forwards)
	for _, f := range forwards {
		dbpkg.DB.Model(&model.Forward{}).Where("id = ?", f.ID).Update("status", 0)
		var t model.Tunnel
		if err := dbpkg.DB.First(&t, f.TunnelID).Error; err == nil {
			name := buildServiceName(f.ID, f.UserID, f.TunnelID)
			_ = sendWSCommand(t.InNodeID, "PauseService", map[string]interface{}{"services": []string{name}})
			if t.Type == 2 {
				_ = sendWSCommand(outNodeIDOr0(t), "PauseService", map[string]interface{}{"services": []string{name}})
			}
		}
	}
}
