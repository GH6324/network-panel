package controller

import (
    "fmt"
    "net/http"
    "time"
    "strings"

    "network-panel/golang-backend/internal/app/dto"
    "network-panel/golang-backend/internal/app/model"
    "network-panel/golang-backend/internal/app/response"
    "network-panel/golang-backend/internal/db"

    "github.com/gin-gonic/gin"
    "log"
)

// POST /api/v1/tunnel/create
func TunnelCreate(c *gin.Context) {
	var req dto.TunnelDto
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	// unique name
	var cnt int64
	db.DB.Model(&model.Tunnel{}).Where("name = ?", req.Name).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusOK, response.ErrMsg("隧道名称已存在"))
		return
	}
	// in node exists
	var in model.Node
	if err := db.DB.First(&in, req.InNodeID).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("入口节点不存在"))
		return
	}
	// set entity
	now := time.Now().UnixMilli()
	status := 1
	t := model.Tunnel{BaseEntity: model.BaseEntity{CreatedTime: now, UpdatedTime: now, Status: &status},
		Name: req.Name, InNodeID: req.InNodeID, InIP: in.IP, Type: req.Type, Flow: req.Flow,
		Protocol: req.Protocol, TrafficRatio: req.TrafficRatio, TCPListenAddr: req.TCPListenAddr, UDPListenAddr: req.UDPListenAddr, InterfaceName: req.InterfaceName,
	}
	if req.OutNodeID != nil {
		var out model.Node
		if db.DB.First(&out, *req.OutNodeID).Error == nil {
			t.OutNodeID = req.OutNodeID
			t.OutIP = &out.ServerIP
		}
	}
	if err := db.DB.Create(&t).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("隧道创建失败"))
		return
	}
	c.JSON(http.StatusOK, response.OkMsg("隧道创建成功"))
}

// POST /api/v1/tunnel/list
func TunnelList(c *gin.Context) {
	var list []model.Tunnel
	db.DB.Find(&list)
	c.JSON(http.StatusOK, response.Ok(list))
}

// POST /api/v1/tunnel/update
func TunnelUpdate(c *gin.Context) {
	var req dto.TunnelUpdateDto
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	var t model.Tunnel
	if err := db.DB.First(&t, req.ID).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("隧道不存在"))
		return
	}
	// name unique
	var cnt int64
	db.DB.Model(&model.Tunnel{}).Where("name = ? AND id <> ?", req.Name, req.ID).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusOK, response.ErrMsg("隧道名称已存在"))
		return
	}
	t.Name = req.Name
	t.Flow = int(req.Flow)
	t.TCPListenAddr, t.UDPListenAddr, t.Protocol, t.InterfaceName, t.TrafficRatio = req.TCPListenAddr, req.UDPListenAddr, req.Protocol, req.InterfaceName, req.TrafficRatio
	t.UpdatedTime = time.Now().UnixMilli()
	if err := db.DB.Save(&t).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("隧道更新失败"))
		return
	}
	c.JSON(http.StatusOK, response.OkMsg("隧道更新成功"))
}

// POST /api/v1/tunnel/delete
func TunnelDelete(c *gin.Context) {
	var p struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	// usage: forwards and user_tunnel
	var cnt int64
	db.DB.Model(&model.Forward{}).Where("tunnel_id = ?", p.ID).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusOK, response.ErrMsg("该隧道还有转发在使用，请先删除相关转发"))
		return
	}
	db.DB.Model(&model.UserTunnel{}).Where("tunnel_id = ?", p.ID).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusOK, response.ErrMsg("该隧道还有用户权限关联，请先取消用户权限分配"))
		return
	}
	if err := db.DB.Delete(&model.Tunnel{}, p.ID).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("隧道删除失败"))
		return
	}
	c.JSON(http.StatusOK, response.OkMsg("隧道删除成功"))
}

// POST /api/v1/tunnel/user/tunnel
func TunnelUserTunnel(c *gin.Context) {
	roleID, _ := c.Get("role_id")
	userID, _ := c.Get("user_id")
	var tunnels []model.Tunnel
	if roleID == 0 || roleID == nil { // admin or no token
		db.DB.Where("status = ?", 1).Find(&tunnels)
	} else {
		// only those user has permission and active
		db.DB.Raw(`select t.* from tunnel t join user_tunnel ut on ut.tunnel_id=t.id where ut.user_id=? and ut.status=1`, userID).Scan(&tunnels)
	}
	c.JSON(http.StatusOK, response.Ok(tunnels))
}

// ========== user-tunnel management ==========

// POST /api/v1/tunnel/user/assign
func TunnelUserAssign(c *gin.Context) {
	var req dto.UserTunnelDto
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	var cnt int64
	db.DB.Model(&model.UserTunnel{}).Where("user_id=? and tunnel_id=?", req.UserID, req.TunnelID).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusOK, response.ErrMsg("该用户已拥有此隧道权限"))
		return
	}
	ut := model.UserTunnel{UserID: req.UserID, TunnelID: req.TunnelID, Flow: req.Flow, Num: req.Num, FlowResetTime: req.FlowResetTime, ExpTime: req.ExpTime, SpeedID: req.SpeedID, Status: val(req.Status, 1)}
	if err := db.DB.Create(&ut).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("用户隧道权限分配失败"))
		return
	}
	c.JSON(http.StatusOK, response.OkMsg("用户隧道权限分配成功"))
}

// POST /api/v1/tunnel/user/list
func TunnelUserList(c *gin.Context) {
	var req dto.UserTunnelQueryDto
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	// join tunnel and speed_limit to enrich names and tunnel flow type
	var items []struct {
		model.UserTunnel
		TunnelName     string  `json:"tunnelName"`
		SpeedLimitName *string `json:"speedLimitName,omitempty"`
		TunnelFlow     *int    `json:"tunnelFlow,omitempty"`
	}
	db.DB.Table("user_tunnel ut").
		Select("ut.*, t.name as tunnel_name, sl.name as speed_limit_name, t.flow as tunnel_flow").
		Joins("left join tunnel t on t.id = ut.tunnel_id").
		Joins("left join speed_limit sl on sl.id = ut.speed_id").
		Where("ut.user_id = ?", req.UserID).
		Scan(&items)
	c.JSON(http.StatusOK, response.Ok(items))
}

// POST /api/v1/tunnel/user/remove {id}
func TunnelUserRemove(c *gin.Context) {
	var p struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	// delete user forwards on this tunnel
	var ut model.UserTunnel
	if err := db.DB.First(&ut, p.ID).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("未找到对应的用户隧道权限记录"))
		return
	}
	db.DB.Where("user_id = ? and tunnel_id = ?", ut.UserID, ut.TunnelID).Delete(&model.Forward{})
	db.DB.Delete(&ut)
	c.JSON(http.StatusOK, response.OkMsg("用户隧道权限删除成功"))
}

// POST /api/v1/tunnel/user/update
func TunnelUserUpdate(c *gin.Context) {
	var req dto.UserTunnelUpdateDto
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	var ut model.UserTunnel
	if err := db.DB.First(&ut, req.ID).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("用户隧道权限不存在"))
		return
	}
	ut.Flow, ut.Num = req.Flow, req.Num
	if req.FlowResetTime != nil {
		ut.FlowResetTime = req.FlowResetTime
	}
	if req.ExpTime != nil {
		ut.ExpTime = req.ExpTime
	}
	if req.Status != nil {
		ut.Status = *req.Status
	}
	ut.SpeedID = req.SpeedID
	if err := db.DB.Save(&ut).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("用户隧道权限更新失败"))
		return
	}
	c.JSON(http.StatusOK, response.OkMsg("用户隧道权限更新成功"))
}

// POST /api/v1/tunnel/diagnose
// Returns a structured diagnosis result the frontend can render.
func TunnelDiagnose(c *gin.Context) {
	var p struct {
		TunnelID int64 `json:"tunnelId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	var t model.Tunnel
	if err := db.DB.First(&t, p.TunnelID).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("隧道不存在"))
		return
	}
	// load nodes
	var inNode model.Node
	_ = db.DB.First(&inNode, t.InNodeID).Error
	var outNode model.Node
	if t.OutNodeID != nil {
		_ = db.DB.First(&outNode, *t.OutNodeID).Error
	}
	// build results
	results := make([]map[string]interface{}, 0, 6)
	// 入口节点连通性（用节点在线状态代替）
	results = append(results, map[string]interface{}{
		"success":     inNode.Status != nil && *inNode.Status == 1,
		"description": "入口节点连通性",
		"nodeName":    inNode.Name,
		"nodeId":      inNode.ID,
		"targetIp":    t.InIP,
		"message":     ifThen(inNode.Status != nil && *inNode.Status == 1, "节点在线", "节点离线"),
	})
	// 出口节点连通性（隧道转发时）
	if t.Type == 2 {
		results = append(results, map[string]interface{}{
			"success":     outNode.Status != nil && *outNode.Status == 1,
			"description": "出口节点连通性",
			"nodeName":    outNode.Name,
			"nodeId":      outNode.ID,
			"targetIp":    orString(ptrString(t.OutIP), outNode.ServerIP),
			"message":     ifThen(outNode.Status != nil && *outNode.Status == 1, "节点在线", "节点离线或未配置"),
		})
	}
    // 多级隧道：逐跳 ICMP（入口->每个中间->出口）
    if t.Type == 2 {
        path := getTunnelPathNodes(t.ID)
        if len(path) == 0 {
            jlog(map[string]any{"event":"iperf3_no_path","tunnelId": t.ID})
        }
        // build pairs
        hops := make([]int64, 0, 2+len(path))
        hops = append(hops, t.InNodeID)
        hops = append(hops, path...)
        if t.OutNodeID != nil { hops = append(hops, *t.OutNodeID) }
        for i := 0; i+1 < len(hops); i++ {
            var srcN, dstN model.Node
            _ = db.DB.First(&srcN, hops[i]).Error
            _ = db.DB.First(&dstN, hops[i+1]).Error
            targetIP := dstN.ServerIP
            avg0, loss0, ok0, msg0, rid0 := diagnosePingFromNodeCtx(srcN.ID, targetIP, 3, 1500, map[string]interface{}{"src": "tunnel", "step": "path", "tunnelId": t.ID, "hopIndex": i})
            results = append(results, map[string]interface{}{
                "success":     ok0,
                "description": "逐跳连通性 (ICMP)",
                "nodeName":    srcN.Name,
                "nodeId":      srcN.ID,
                "targetIp":    targetIP,
                "averageTime": avg0,
                "packetLoss":  loss0,
                "message":     msg0,
                "reqId":       rid0,
            })
        }
    }
	// 基础配置校验
	cfgOK := true
	msg := "配置正常"
	if t.TCPListenAddr == nil || *t.TCPListenAddr == "" || t.UDPListenAddr == nil || *t.UDPListenAddr == "" {
		cfgOK = false
		msg = "TCP/UDP监听地址未完整配置"
	}
	results = append(results, map[string]interface{}{
		"success":     cfgOK,
		"description": "基础配置校验",
		"nodeName":    inNode.Name,
		"nodeId":      inNode.ID,
		"targetIp":    t.InIP,
		"message":     msg,
	})

    // 入口节点外网连通性：仅端口转发类型检查
    if t.Type == 1 {
        avg, loss, ok2, msg2, rid1 := diagnosePingFromNodeCtx(inNode.ID, "1.1.1.1", 3, 1500, map[string]interface{}{"src": "tunnel", "step": "entryPublic", "tunnelId": t.ID})
        results = append(results, map[string]interface{}{
            "success":     ok2,
            "description": "入口节点外网连通性 (ICMP 1.1.1.1)",
            "nodeName":    inNode.Name,
            "nodeId":      inNode.ID,
            "targetIp":    "1.1.1.1",
            "averageTime": avg,
            "packetLoss":  loss,
            "message":     msg2,
            "reqId":       rid1,
        })
    }
	// 出口节点外网连通性（隧道转发时）
	if t.Type == 2 && outNode.ID != 0 {
		avg2, loss2, ok3, msg3, rid2 := diagnosePingFromNodeCtx(outNode.ID, "1.1.1.1", 3, 1500, map[string]interface{}{"src": "tunnel", "step": "exitPublic", "tunnelId": t.ID})
		results = append(results, map[string]interface{}{
			"success":     ok3,
			"description": "出口节点外网连通性 (ICMP 1.1.1.1)",
			"nodeName":    outNode.Name,
			"nodeId":      outNode.ID,
			"targetIp":    "1.1.1.1",
			"averageTime": avg2,
			"packetLoss":  loss2,
			"message":     msg3,
			"reqId":       rid2,
		})
	}

	out := map[string]interface{}{
		"tunnelName": t.Name,
		"tunnelType": ifThen(t.Type == 1, "端口转发", "隧道转发"),
		"timestamp":  time.Now().UnixMilli(),
		"results":    results,
	}
	c.JSON(http.StatusOK, response.Ok(out))
}

// POST /api/v1/tunnel/diagnose-step {tunnelId, step: entry|entryExit|exitPublic}
func TunnelDiagnoseStep(c *gin.Context) {
	var p struct {
		TunnelID int64  `json:"tunnelId" binding:"required"`
		Step     string `json:"step" binding:"required"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("参数错误"))
		return
	}
	var t model.Tunnel
	if err := db.DB.First(&t, p.TunnelID).Error; err != nil {
		c.JSON(http.StatusOK, response.ErrMsg("隧道不存在"))
		return
	}
	var inNode, outNode model.Node
	_ = db.DB.First(&inNode, t.InNodeID).Error
	if t.OutNodeID != nil {
		_ = db.DB.First(&outNode, *t.OutNodeID).Error
	}

    // avoid nil deref: compute outNodeId safely
    outId := int64(0)
    if t.OutNodeID != nil { outId = *t.OutNodeID }
    log.Printf("API /tunnel/diagnose-step tunnelId=%d step=%s inNode=%d outNode=%d", p.TunnelID, p.Step, t.InNodeID, outId)
	var res map[string]interface{}
	switch p.Step {
	case "entry":
		avg, loss, ok, msg := diagnosePingFromNode(inNode.ID, "1.1.1.1", 3, 1500)
		res = map[string]interface{}{
			"success": ok, "description": "入口节点外网连通性 (ICMP 1.1.1.1)", "nodeName": inNode.Name, "nodeId": inNode.ID,
			"targetIp": "1.1.1.1", "averageTime": avg, "packetLoss": loss, "message": msg,
		}
    case "entryExit":
        // 选择出口IP：若显式配置了 OutIP 则使用；否则优先选择 IPv4
        exitIP := ptrString(t.OutIP)
        if exitIP == "" { exitIP = preferIPv4(outNode) }
        avg, loss, ok, msg := diagnosePingFromNode(inNode.ID, exitIP, 3, 1500)
        res = map[string]interface{}{
            "success": ok, "description": "入口到出口连通性 (ICMP)", "nodeName": inNode.Name, "nodeId": inNode.ID,
            "targetIp": exitIP, "averageTime": avg, "packetLoss": loss, "message": msg,
        }
    case "path":
        if t.Type != 2 {
            c.JSON(http.StatusOK, response.ErrMsg("非隧道转发不支持路径检查"))
            return
        }
        path := getTunnelPathNodes(t.ID)
        hops := make([]int64, 0, 2+len(path))
        hops = append(hops, t.InNodeID)
        hops = append(hops, path...)
        if t.OutNodeID != nil { hops = append(hops, *t.OutNodeID) }
        items := make([]map[string]any, 0, len(hops)-1)
        for i := 0; i+1 < len(hops); i++ {
            var srcN, dstN model.Node
            _ = db.DB.First(&srcN, hops[i]).Error
            _ = db.DB.First(&dstN, hops[i+1]).Error
            target := dstN.ServerIP
            avg, loss, ok, msg := diagnosePingFromNode(srcN.ID, target, 3, 1500)
            items = append(items, map[string]any{
                "success": ok, "description": "逐跳连通性 (ICMP)", "nodeName": srcN.Name, "nodeId": srcN.ID,
                "targetIp": target, "averageTime": avg, "packetLoss": loss, "message": msg,
            })
        }
        // 返回打包的结果数组
        c.JSON(http.StatusOK, response.Ok(map[string]any{"results": items}))
        return
    case "exitPublic":
		if outNode.ID == 0 {
			c.JSON(http.StatusOK, response.ErrMsg("无出口节点"))
			return
		}
		avg, loss, ok, msg := diagnosePingFromNode(outNode.ID, "1.1.1.1", 3, 1500)
		res = map[string]interface{}{
			"success": ok, "description": "出口节点外网连通性 (ICMP 1.1.1.1)", "nodeName": outNode.Name, "nodeId": outNode.ID,
			"targetIp": "1.1.1.1", "averageTime": avg, "packetLoss": loss, "message": msg,
		}
    case "iperf3":
        // 仅隧道转发才进行 iperf3 测速：出口节点启动服务，入口节点作为客户端 -R 连接出口
        if t.Type != 2 || outNode.ID == 0 {
            c.JSON(http.StatusOK, response.ErrMsg("仅隧道转发支持iperf3"))
            return
        }
        exitIP := orString(ptrString(t.OutIP), outNode.ServerIP)
        // 1) 出口节点启动 iperf3 server（随机端口）
        srvReq := map[string]interface{}{"requestId": RandUUID(), "mode": "iperf3", "server": true, "port": 0, "ctx": map[string]any{"src": "tunnel", "step": "iperf3_server", "tunnelId": t.ID}}
        _ = sendWSCommand(outNode.ID, "Diagnose", srvReq)
        srvRes, ok := RequestDiagnose(outNode.ID, srvReq, 8*time.Second)
        if !ok {
            c.JSON(http.StatusOK, response.ErrMsg("出口节点未响应iperf3服务启动"))
            return
        }
        srvPort := 0
        if data, _ := srvRes["data"].(map[string]interface{}); data != nil {
            if p2, ok2 := data["port"].(float64); ok2 {
                srvPort = int(p2)
            }
        }
        if srvPort == 0 {
            c.JSON(http.StatusOK, response.ErrMsg("iperf3服务未返回端口"))
            return
        }
        // 2) 沿路径临时搭建端口直转链路（入口 → 中间... → 最后 → 出口的 iperf3 端口）
        path := getTunnelPathNodes(t.ID)
        fNodes := make([]int64, 0, 1+len(path))
        fNodes = append(fNodes, inNode.ID)
        fNodes = append(fNodes, path...)
        // 为每个节点分配临时端口
        tmpPorts := make([]int, len(fNodes))
        rid := RandUUID()
        tmpNames := make([]string, len(fNodes))
        jlog(map[string]any{"event":"iperf3_path_nodes","tunnelId": t.ID, "nodes": fNodes, "ports": tmpPorts})
        for i, nid := range fNodes {
            var n model.Node
            _ = db.DB.First(&n, nid).Error
            minP, maxP := 10000, 65535
            if n.PortSta > 0 { minP = n.PortSta }
            if n.PortEnd > 0 { maxP = n.PortEnd }
            prefer := n.PortSta
            if prefer <= 0 { prefer = minP }
            tmpPorts[i] = findFreePortOnNode(nid, prefer, minP, maxP)
            if tmpPorts[i] == 0 { tmpPorts[i] = minP }
            tmpNames[i] = fmt.Sprintf("tmp_iperf3_%d_%s_%d", t.ID, rid, i)
            jlog(map[string]any{"event":"iperf3_tmp_port_pick","tunnelId": t.ID, "nodeId": nid, "name": tmpNames[i], "port": tmpPorts[i]})
        }
        // 部署临时直转服务链
        for i := 0; i < len(fNodes); i++ {
            nid := fNodes[i]
            var target string
            if i < len(fNodes)-1 {
                var nx model.Node
                if db.DB.First(&nx, fNodes[i+1]).Error != nil { continue }
                targetIP := preferIPv4(nx)
                target = safeHostPort(targetIP, tmpPorts[i+1])
            } else {
                target = safeHostPort(exitIP, srvPort)
            }
            svc := buildServiceConfig(tmpNames[i], tmpPorts[i], target, nil)
            _ = sendWSCommand(nid, "AddService", []map[string]any{svc})
            jlog(map[string]any{"event":"iperf3_tmp_add","tunnelId": t.ID, "nodeId": nid, "name": tmpNames[i], "listen": tmpPorts[i], "target": target})
        }
        // 下发 RestartGost 以确保临时配置立即生效
        for i := 0; i < len(fNodes); i++ {
            _ = sendWSCommand(fNodes[i], "RestartGost", map[string]any{"reason": "iperf3_tmp"})
        }
        // 主动轮询各节点临时服务是否生效（最多 8 秒）
        readyAll := true
        for i := 0; i < len(fNodes); i++ {
            if !waitServiceOnNode(fNodes[i], tmpNames[i], tmpPorts[i], 8*time.Second) {
                readyAll = false
                jlog(map[string]any{"event":"iperf3_tmp_wait_timeout","tunnelId": t.ID, "nodeId": fNodes[i], "name": tmpNames[i], "port": tmpPorts[i]})
            }
        }
        // 额外：逐跳 TCP 连通性验证（上一跳直连下一跳端口，最多重试3次）
        if readyAll {
            for i := 0; i+1 < len(fNodes); i++ {
                var nx model.Node
                _ = db.DB.First(&nx, fNodes[i+1]).Error
                host := preferIPv4(nx)
                okc := false
                for r := 0; r < 3 && !okc; r++ {
                    _, _, okc, _ = diagnoseFromNode(fNodes[i], host, tmpPorts[i+1], 1, 1200)
                    if !okc { time.Sleep(250 * time.Millisecond) }
                }
                if !okc {
                    readyAll = false
                    jlog(map[string]any{"event":"iperf3_tmp_tcp_fail","tunnelId": t.ID, "from": fNodes[i], "to": host, "port": tmpPorts[i+1]})
                    break
                }
            }
        }
        if !readyAll {
            jlog(map[string]any{"event":"iperf3_tmp_ready_partial","tunnelId": t.ID})
            c.JSON(http.StatusOK, response.ErrMsg("临时通道未完全就绪，已中止 iperf3 测试"))
            return
        }
        // 3) 入口作为 iperf3 客户端，连接本机临时入口端口
        cliReq := map[string]interface{}{"requestId": RandUUID(), "mode": "iperf3", "client": true, "host": "127.0.0.1", "port": tmpPorts[0], "duration": 5, "reverse": true, "ctx": map[string]any{"src": "tunnel", "step": "iperf3_client_path", "tunnelId": t.ID}}
        _ = sendWSCommand(inNode.ID, "Diagnose", cliReq)
        cliRes, ok := RequestDiagnose(inNode.ID, cliReq, 20*time.Second)
        if !ok {
            c.JSON(http.StatusOK, response.ErrMsg("入口节点未响应iperf3客户端"))
            return
        }
        data, _ := cliRes["data"].(map[string]interface{})
        success := false
        msgI := ""
        bw := 0.0
        if data != nil {
            if v, ok2 := data["success"].(bool); ok2 {
                success = v
            }
            if m, ok2 := data["message"].(string); ok2 {
                msgI = m
            }
            if b, ok2 := data["bandwidthMbps"].(float64); ok2 {
                bw = b
            }
        }
        // 暂不清理临时服务，便于排查中间节点通道部署问题（后续可恢复清理）
        res = map[string]interface{}{
            "success": success, "description": "iperf3 反向带宽测试", "nodeName": inNode.Name, "nodeId": inNode.ID,
            "targetIp": exitIP, "targetPort": srvPort, "message": msgI, "bandwidthMbps": bw,
        }
	default:
		c.JSON(http.StatusOK, response.ErrMsg("未知诊断步骤"))
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// waitServiceOnNode polls QueryServices via WS to check if a temporary service is present.
// It matches by service name or by listening port.
func waitServiceOnNode(nodeID int64, name string, port int, timeout time.Duration) bool {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        svcs := queryNodeServicesRaw(nodeID)
        // match by name or port
        for _, s := range svcs {
            listening := false
            if b, ok := s["listening"].(bool); ok { listening = b }
            if n, ok := s["name"].(string); ok && n == name && listening {
                return true
            }
            if v, ok := s["addr"].(string); ok {
                if parsePort(v) == port && listening { return true }
            }
            if lst, ok := s["listener"].(map[string]any); ok {
                if v, ok2 := lst["addr"].(string); ok2 {
                    if parsePort(v) == port && listening { return true }
                }
            }
        }
        time.Sleep(200 * time.Millisecond)
    }
    return false
}

// preferIPv4 chooses an IPv4 address from Node fields if available; otherwise returns ServerIP
func preferIPv4(n model.Node) string {
    // Node.IP may contain comma-separated list
    if n.IP != "" {
        parts := strings.Split(n.IP, ",")
        for _, p := range parts {
            p = strings.TrimSpace(p)
            if p != "" && !strings.Contains(p, ":") { // IPv4
                return p
            }
        }
    }
    // fallback: if ServerIP is IPv4
    if n.ServerIP != "" && !strings.Contains(n.ServerIP, ":") {
        return n.ServerIP
    }
    // last resort: ServerIP itself (IPv6 acceptable when both ends support it)
    return n.ServerIP
}

func ifThen[T any](cond bool, a T, b T) T {
	if cond {
		return a
	}
	return b
}
func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func orString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
func defaultPortForProtocol(p *string) int {
	if p == nil {
		return 443
	}
	switch *p {
	case "tls", "wss", "mtls", "mwss":
		return 443
	case "tcp", "mtcp":
		return 80
	default:
		return 443
	}
}

func val(p *int, d int) int {
	if p == nil {
		return d
	}
	return *p
}
