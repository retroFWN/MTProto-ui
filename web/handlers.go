package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"mtproxy-panel/auth"
	"mtproxy-panel/botmanager"
	"mtproxy-panel/config"
	"mtproxy-panel/database"
	"mtproxy-panel/proxy"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// ── Pages ────────────────────────────────────────────────────────────────

func LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

func DashboardPage(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", nil)
}

func ProxiesPage(c *gin.Context) {
	c.HTML(http.StatusOK, "proxies.html", nil)
}

func SettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", nil)
}

// ── Auth ─────────────────────────────────────────────────────────────────

func Login(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request"})
			return
		}

		var user database.User
		if database.DB.Where("username = ?", req.Username).First(&user).Error != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Invalid credentials"})
			return
		}
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Invalid credentials"})
			return
		}

		token, err := auth.CreateToken(user.Username, cfg.TokenExpiry)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "Token creation failed"})
			return
		}

		c.SetCookie("access_token", token, 3600*6, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"success": true, "token": token})
	}
}

func Logout(c *gin.Context) {
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request"})
		return
	}

	user := currentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Not authenticated"})
		return
	}
	if !auth.CheckPassword(req.OldPassword, user.PasswordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Wrong current password"})
		return
	}

	hash, _ := auth.HashPassword(req.NewPassword)
	database.DB.Model(user).Update("password_hash", hash)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ── System ───────────────────────────────────────────────────────────────

func SystemStatus(c *gin.Context) {
	cpuPercent, _ := cpu.Percent(500*time.Millisecond, false)
	cpuCount, _ := cpu.Counts(true)
	memInfo, _ := mem.VirtualMemory()

	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}
	diskInfo, _ := disk.Usage(diskPath)
	netInfo, _ := net.IOCounters(false)
	hostInfo, _ := host.Info()

	cpuPct := 0.0
	if len(cpuPercent) > 0 {
		cpuPct = cpuPercent[0]
	}

	var netSent, netRecv uint64
	if len(netInfo) > 0 {
		netSent = netInfo[0].BytesSent
		netRecv = netInfo[0].BytesRecv
	}

	c.JSON(http.StatusOK, gin.H{
		"cpu_percent":    cpuPct,
		"cpu_count":      cpuCount,
		"memory_total":   memInfo.Total,
		"memory_used":    memInfo.Used,
		"memory_percent": memInfo.UsedPercent,
		"disk_total":     diskInfo.Total,
		"disk_used":      diskInfo.Used,
		"disk_percent":   diskInfo.UsedPercent,
		"net_sent":       netSent,
		"net_recv":       netRecv,
		"uptime_seconds": hostInfo.Uptime,
		"platform":       runtime.GOOS,
		"hostname":       hostInfo.Hostname,
	})
}

// ── Proxies ──────────────────────────────────────────────────────────────

func ListProxies(c *gin.Context) {
	var proxies []database.Proxy
	database.DB.Find(&proxies)

	result := make([]gin.H, 0, len(proxies))
	for _, p := range proxies {
		var clientCount int64
		database.DB.Model(&database.Client{}).Where("proxy_id = ?", p.ID).Count(&clientCount)
		status := proxy.GetContainerStatus(p.ID)

		result = append(result, gin.H{
			"id":                 p.ID,
			"name":               p.Name,
			"port":               p.Port,
			"fake_tls_domain":    p.FakeTLSDomain,
			"backend":            p.Backend,
			"enabled":            p.Enabled,
			"container_id":       p.ContainerID,
			"traffic_up":         p.TrafficUp,
			"traffic_down":       p.TrafficDown,
			"traffic_total_limit": p.TrafficLimit,
			"created_at":         p.CreatedAt,
			"status":             status,
			"client_count":       clientCount,
		})
	}
	c.JSON(http.StatusOK, result)
}

func CreateProxy(c *gin.Context) {
	var req struct {
		Name          string `json:"name" binding:"required"`
		Port          int    `json:"port" binding:"required"`
		FakeTLSDomain string `json:"fake_tls_domain"`
		Backend       string `json:"backend"`
		TrafficLimit  int64  `json:"traffic_total_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	if req.FakeTLSDomain == "" {
		req.FakeTLSDomain = "google.com"
	}
	if req.Backend == "" {
		req.Backend = activeBackend()
	}

	// Check port uniqueness
	var count int64
	database.DB.Model(&database.Proxy{}).Where("port = ?", req.Port).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("Port %d already in use", req.Port)})
		return
	}

	p := database.Proxy{
		Name:          req.Name,
		Port:          req.Port,
		FakeTLSDomain: req.FakeTLSDomain,
		Backend:       req.Backend,
		TrafficLimit:  req.TrafficLimit,
		Enabled:       true,
	}
	database.DB.Create(&p)

	secret := proxy.GenerateSecret(req.FakeTLSDomain)
	client := database.Client{
		ProxyID: p.ID,
		Name:    "default",
		Secret:  secret,
		Enabled: true,
	}
	database.DB.Create(&client)

	secrets := database.GetEnabledSecrets(p.ID)
	containerID, err := proxy.StartProxy(p.ID, p.Port, secrets, p.FakeTLSDomain, p.Backend)
	if err == nil {
		database.DB.Model(&p).Update("container_id", containerID)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "id": p.ID, "secret": secret})
}

func UpdateProxy(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}

	var req struct {
		Name          *string `json:"name"`
		Port          *int    `json:"port"`
		FakeTLSDomain *string `json:"fake_tls_domain"`
		Enabled       *bool   `json:"enabled"`
		TrafficLimit  *int64  `json:"traffic_total_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	needRestart := false
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Port != nil && *req.Port != p.Port {
		p.Port = *req.Port
		needRestart = true
	}
	if req.FakeTLSDomain != nil && *req.FakeTLSDomain != p.FakeTLSDomain {
		p.FakeTLSDomain = *req.FakeTLSDomain
		needRestart = true
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
		needRestart = true
	}
	if req.TrafficLimit != nil {
		p.TrafficLimit = *req.TrafficLimit
	}
	database.DB.Save(&p)

	if needRestart {
		if p.Enabled {
			secrets := database.GetEnabledSecrets(p.ID)
			if len(secrets) > 0 {
				cid, _ := proxy.RestartProxy(p.ID, p.Port, secrets, p.FakeTLSDomain, p.Backend)
				database.DB.Model(&p).Update("container_id", cid)
			}
		} else {
			proxy.StopProxy(p.ID)
			database.DB.Model(&p).Update("container_id", "")
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteProxy(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}

	proxy.StopProxy(p.ID)
	database.DB.Delete(&p) // CASCADE deletes clients
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func StartProxyHandler(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}

	secrets := database.GetEnabledSecrets(p.ID)
	if len(secrets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "No enabled client secrets"})
		return
	}

	cid, err := proxy.StartProxy(p.ID, p.Port, secrets, p.FakeTLSDomain, p.Backend)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	database.DB.Model(&p).Updates(map[string]interface{}{"container_id": cid, "enabled": true})
	c.JSON(http.StatusOK, gin.H{"success": true, "container_id": cid})
}

func StopProxyHandler(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}

	proxy.StopProxy(p.ID)
	database.DB.Model(&p).Updates(map[string]interface{}{"container_id": "", "enabled": false})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RestartProxyHandler(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}

	secrets := database.GetEnabledSecrets(p.ID)
	if len(secrets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "No enabled client secrets"})
		return
	}

	cid, err := proxy.RestartProxy(p.ID, p.Port, secrets, p.FakeTLSDomain, p.Backend)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	database.DB.Model(&p).Update("container_id", cid)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ProxyStatsHandler(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}
	c.JSON(http.StatusOK, proxy.GetContainerStats(p.ID))
}

func ProxyLogsHandler(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	tail, _ := strconv.Atoi(c.DefaultQuery("tail", "50"))
	var p database.Proxy
	if database.DB.First(&p, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": proxy.GetContainerLogs(p.ID, tail)})
}

func ProxyLiveHandler(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}

	backend := proxy.GetBackend(p.Backend)
	um, isUserManager := backend.(proxy.UserManager)
	if !isUserManager {
		c.JSON(http.StatusOK, gin.H{"supported": false})
		return
	}

	users, err := um.ListUsers(p.Port)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"supported": true, "error": err.Error()})
		return
	}

	summary, _ := um.GetSummary(p.Port)

	// Match telemt users to panel clients by secret
	var clients []database.Client
	database.DB.Where("proxy_id = ?", p.ID).Find(&clients)

	type liveClient struct {
		ClientID           uint     `json:"client_id"`
		Name               string   `json:"name"`
		CurrentConnections int      `json:"current_connections"`
		TotalOctets        int64    `json:"total_octets"`
		ActiveUniqueIPs    int      `json:"active_unique_ips"`
		ActiveIPList       []string `json:"active_ips"`
	}
	result := make([]liveClient, 0)
	for _, cl := range clients {
		for _, u := range users {
			if proxy.MatchSecret(cl.Secret, u.Secret) {
				result = append(result, liveClient{
					ClientID:           cl.ID,
					Name:               cl.Name,
					CurrentConnections: u.CurrentConnections,
					TotalOctets:        u.TotalOctets,
					ActiveUniqueIPs:    u.ActiveUniqueIPs,
					ActiveIPList:       u.ActiveIPList,
				})
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"supported": true,
		"users":     result,
		"summary":   summary,
	})
}

// ── Clients ──────────────────────────────────────────────────────────────

func ListClients(c *gin.Context) {
	proxyID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, proxyID).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}

	serverIP := database.GetServerIP()
	var clients []database.Client
	database.DB.Where("proxy_id = ?", proxyID).Find(&clients)

	result := make([]gin.H, 0, len(clients))
	for _, cl := range clients {
		result = append(result, gin.H{
			"id":            cl.ID,
			"name":          cl.Name,
			"secret":        cl.Secret,
			"enabled":       cl.Enabled,
			"traffic_up":    cl.TrafficUp,
			"traffic_down":  cl.TrafficDown,
			"traffic_limit": cl.TrafficLimit,
			"expiry_time":   cl.ExpiryTime,
			"last_online":   cl.LastOnline,
			"created_at":    cl.CreatedAt,
			"tg_link":       fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", serverIP, p.Port, cl.Secret),
		})
	}
	c.JSON(http.StatusOK, result)
}

func CreateClient(c *gin.Context) {
	proxyID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var p database.Proxy
	if database.DB.First(&p, proxyID).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Proxy not found"})
		return
	}

	var req struct {
		Name         string `json:"name" binding:"required"`
		TrafficLimit int64  `json:"traffic_limit"`
		ExpiryTime   int64  `json:"expiry_time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	secret := proxy.GenerateSecret(p.FakeTLSDomain)
	cl := database.Client{
		ProxyID:      p.ID,
		Name:         req.Name,
		Secret:       secret,
		Enabled:      true,
		TrafficLimit: req.TrafficLimit,
		ExpiryTime:   req.ExpiryTime,
	}
	database.DB.Create(&cl)

	// Sync to telemt API if running
	backend := proxy.GetBackend(p.Backend)
	if um, ok := backend.(proxy.UserManager); ok {
		username := fmt.Sprintf("user_%d", cl.ID)
		um.AddUser(p.Port, username, secret, 0, req.TrafficLimit, req.ExpiryTime)
	}

	serverIP := database.GetServerIP()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      cl.ID,
		"secret":  secret,
		"tg_link": fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", serverIP, p.Port, secret),
	})
}

func UpdateClient(c *gin.Context) {
	proxyID, ok := parseID(c, "id")
	if !ok {
		return
	}
	clientID, ok := parseID(c, "cid")
	if !ok {
		return
	}

	var cl database.Client
	if database.DB.Where("id = ? AND proxy_id = ?", clientID, proxyID).First(&cl).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Client not found"})
		return
	}

	var req struct {
		Name         *string `json:"name"`
		Enabled      *bool   `json:"enabled"`
		TrafficLimit *int64  `json:"traffic_limit"`
		ExpiryTime   *int64  `json:"expiry_time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.TrafficLimit != nil {
		updates["traffic_limit"] = *req.TrafficLimit
	}
	if req.ExpiryTime != nil {
		updates["expiry_time"] = *req.ExpiryTime
	}
	database.DB.Model(&cl).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteClient(c *gin.Context) {
	proxyID, ok := parseID(c, "id")
	if !ok {
		return
	}
	clientID, ok := parseID(c, "cid")
	if !ok {
		return
	}

	// Remove from telemt API if applicable
	var p database.Proxy
	if database.DB.First(&p, proxyID).Error == nil {
		backend := proxy.GetBackend(p.Backend)
		if um, ok := backend.(proxy.UserManager); ok {
			username := fmt.Sprintf("user_%d", clientID)
			um.RemoveUser(p.Port, username)
		}
	}

	result := database.DB.Where("id = ? AND proxy_id = ?", clientID, proxyID).Delete(&database.Client{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Client not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ResetClientTraffic(c *gin.Context) {
	proxyID, ok := parseID(c, "id")
	if !ok {
		return
	}
	clientID, ok := parseID(c, "cid")
	if !ok {
		return
	}

	result := database.DB.Model(&database.Client{}).
		Where("id = ? AND proxy_id = ?", clientID, proxyID).
		Updates(map[string]interface{}{"traffic_up": 0, "traffic_down": 0})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Client not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ── Settings ─────────────────────────────────────────────────────────────

func GetSettings(c *gin.Context) {
	var settings []database.Setting
	database.DB.Find(&settings)

	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	c.JSON(http.StatusOK, result)
}

func UpdateSettings(c *gin.Context) {
	var data map[string]string
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	for key, value := range data {
		var s database.Setting
		if database.DB.Where("`key` = ?", key).First(&s).Error != nil {
			database.DB.Create(&database.Setting{Key: key, Value: value})
		} else {
			database.DB.Model(&s).Update("value", value)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func PullImageHandler(c *gin.Context) {
	backendID := activeBackend()
	backend := proxy.GetBackend(backendID)
	if err := backend.PullImage(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to pull image"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Image pulled: " + backend.Info().Image})
}

// ── Backends ─────────────────────────────────────────────────────────────

func ListBackends(c *gin.Context) {
	active := activeBackend()
	backends := proxy.AllBackends()
	result := make([]gin.H, 0, len(backends))
	for _, b := range backends {
		result = append(result, gin.H{
			"id":          b.ID,
			"name":        b.Name,
			"lang":        b.Lang,
			"image":       b.Image,
			"description": b.Description,
			"features":    b.Features,
			"active":      b.ID == active,
		})
	}
	c.JSON(http.StatusOK, result)
}

// ── Bot Management ──────────────────────────────────────────────────────

func BotStatus(c *gin.Context) {
	running, lastErr := botmanager.Status()
	c.JSON(http.StatusOK, gin.H{"running": running, "error": lastErr})
}

func BotStart(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token, adminIDs string

		var s database.Setting
		if database.DB.Where("`key` = ?", "tg_bot_token").First(&s).Error == nil {
			token = s.Value
		}
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Bot token not configured"})
			return
		}

		if database.DB.Where("`key` = ?", "tg_admin_ids").First(&s).Error == nil {
			adminIDs = s.Value
		}

		panelURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
		botDir := filepath.Join(cfg.BaseDir, "bot")

		if err := botmanager.Start(botDir, token, adminIDs, panelURL, cfg.SecretKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func BotStop(c *gin.Context) {
	if err := botmanager.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ── Helpers ──────────────────────────────────────────────────────────────

func parseID(c *gin.Context, param string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid ID"})
		return 0, false
	}
	return uint(id), true
}

func activeBackend() string {
	var s database.Setting
	if database.DB.Where("`key` = ?", "proxy_backend").First(&s).Error == nil && s.Value != "" {
		return s.Value
	}
	return "official"
}
