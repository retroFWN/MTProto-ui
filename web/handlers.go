package web

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"sync"
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

// ── CPU cache ────────────────────────────────────────────────────────────

var (
	cpuCacheMu   sync.Mutex
	cpuCacheVal  float64
	cpuCacheTime time.Time
)

func getCachedCPU() float64 {
	cpuCacheMu.Lock()
	defer cpuCacheMu.Unlock()
	if time.Since(cpuCacheTime) < 2*time.Second {
		return cpuCacheVal
	}
	pct, _ := cpu.Percent(200*time.Millisecond, false)
	if len(pct) > 0 {
		cpuCacheVal = pct[0]
	}
	cpuCacheTime = time.Now()
	return cpuCacheVal
}

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

		c.SetCookie("access_token", token, 3600*6, "/", "", cfg.Domain != "", true)
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
	cpuPct := getCachedCPU()
	cpuCount, _ := cpu.Counts(true)
	memInfo, _ := mem.VirtualMemory()

	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}
	diskInfo, _ := disk.Usage(diskPath)
	netInfo, _ := net.IOCounters(false)
	hostInfo, _ := host.Info()

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

	// Batch client counts in one query
	type countResult struct {
		ProxyID uint  `gorm:"column:proxy_id"`
		Count   int64 `gorm:"column:count"`
	}
	var counts []countResult
	database.DB.Model(&database.Client{}).
		Select("proxy_id, count(*) as count").
		Group("proxy_id").
		Find(&counts)
	countMap := make(map[uint]int64)
	for _, cc := range counts {
		countMap[cc.ProxyID] = cc.Count
	}

	result := make([]gin.H, 0, len(proxies))
	for _, p := range proxies {
		status := proxy.GetContainerStatus(p.ID)

		result = append(result, gin.H{
			"id":                 p.ID,
			"name":               p.Name,
			"port":               p.Port,
			"fake_tls_domain":    p.FakeTLSDomain,
			"ad_tag":             p.AdTag,
			"backend":            p.Backend,
			"enabled":            p.Enabled,
			"container_id":       p.ContainerID,
			"traffic_up":         p.TrafficUp,
			"traffic_down":       p.TrafficDown,
			"traffic_total_limit": p.TrafficLimit,
			"created_at":         p.CreatedAt,
			"status":             status,
			"client_count":       countMap[p.ID],
		})
	}
	c.JSON(http.StatusOK, result)
}

func CreateProxy(c *gin.Context) {
	var req struct {
		Name          string `json:"name" binding:"required"`
		Port          int    `json:"port" binding:"required"`
		FakeTLSDomain string `json:"fake_tls_domain"`
		AdTag         string `json:"ad_tag"`
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
		AdTag:         req.AdTag,
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

	dbClients := database.GetEnabledClients(p.ID)
	entries := proxy.ClientEntriesFromDB(dbClients)
	containerID, err := proxy.StartProxy(p.ID, p.Port, entries, p.FakeTLSDomain, p.Backend, p.AdTag)
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
		AdTag         *string `json:"ad_tag"`
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
	if req.AdTag != nil && *req.AdTag != p.AdTag {
		p.AdTag = *req.AdTag
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
			dbClients := database.GetEnabledClients(p.ID)
			entries := proxy.ClientEntriesFromDB(dbClients)
			if len(entries) > 0 {
				cid, _ := proxy.RestartProxy(p.ID, p.Port, entries, p.FakeTLSDomain, p.Backend, p.AdTag)
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

	dbClients := database.GetEnabledClients(p.ID)
	entries := proxy.ClientEntriesFromDB(dbClients)
	if len(entries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "No enabled client secrets"})
		return
	}

	cid, err := proxy.StartProxy(p.ID, p.Port, entries, p.FakeTLSDomain, p.Backend, p.AdTag)
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

	dbClients := database.GetEnabledClients(p.ID)
	entries := proxy.ClientEntriesFromDB(dbClients)
	if len(entries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "No enabled client secrets"})
		return
	}

	cid, err := proxy.RestartProxy(p.ID, p.Port, entries, p.FakeTLSDomain, p.Backend, p.AdTag)
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
			"tg_link":       proxy.BuildTgLink(serverIP, p.Port, cl.Secret, p.Backend, p.FakeTLSDomain),
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

	// Sync to proxy engine
	backend := proxy.GetBackend(p.Backend)
	if um, ok := backend.(proxy.UserManager); ok {
		// Telemt: add via API + update config for future restarts
		username := fmt.Sprintf("user_%d", cl.ID)
		um.AddUser(p.Port, username, secret, 0, req.TrafficLimit, req.ExpiryTime)
		proxy.RefreshConfig(&p)
	} else if p.Enabled {
		// Official: restart container to pick up new secret
		dbClients := database.GetEnabledClients(p.ID)
		entries := proxy.ClientEntriesFromDB(dbClients)
		if len(entries) > 0 {
			cid, err := proxy.RestartProxy(p.ID, p.Port, entries, p.FakeTLSDomain, p.Backend, p.AdTag)
			if err == nil {
				database.DB.Model(&p).Update("container_id", cid)
			}
		}
	}

	serverIP := database.GetServerIP()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      cl.ID,
		"secret":  secret,
		"tg_link": proxy.BuildTgLink(serverIP, p.Port, secret, p.Backend, p.FakeTLSDomain),
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

	// Track what changed for proxy sync
	enabledChanged := req.Enabled != nil && *req.Enabled != cl.Enabled
	limitsChanged := (req.TrafficLimit != nil && *req.TrafficLimit != cl.TrafficLimit) ||
		(req.ExpiryTime != nil && *req.ExpiryTime != cl.ExpiryTime)

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

	// Sync changes to proxy engine
	if enabledChanged || limitsChanged {
		var p database.Proxy
		if database.DB.First(&p, proxyID).Error == nil && p.Enabled {
			backend := proxy.GetBackend(p.Backend)
			if um, ok := backend.(proxy.UserManager); ok {
				username := fmt.Sprintf("user_%d", cl.ID)
				if req.Enabled != nil && !*req.Enabled {
					// Disabling client: remove from running proxy
					um.RemoveUser(p.Port, username)
				} else {
					// Re-enabling or changing limits: remove + re-add with new params
					newLimit := cl.TrafficLimit
					if req.TrafficLimit != nil {
						newLimit = *req.TrafficLimit
					}
					newExpiry := cl.ExpiryTime
					if req.ExpiryTime != nil {
						newExpiry = *req.ExpiryTime
					}
					um.RemoveUser(p.Port, username)
					um.AddUser(p.Port, username, cl.Secret, 0, newLimit, newExpiry)
				}
				// Update config for future container restarts
				proxy.RefreshConfig(&p)
			} else if enabledChanged {
				// Official: restart container with updated secret list
				dbClients := database.GetEnabledClients(p.ID)
				entries := proxy.ClientEntriesFromDB(dbClients)
				if len(entries) > 0 {
					cid, err := proxy.RestartProxy(p.ID, p.Port, entries, p.FakeTLSDomain, p.Backend, p.AdTag)
					if err == nil {
						database.DB.Model(&p).Update("container_id", cid)
					}
				} else {
					proxy.StopProxy(p.ID)
					database.DB.Model(&p).Updates(map[string]interface{}{
						"enabled": false, "container_id": "",
					})
				}
			}
		}
	}

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

	// Remove from proxy engine
	var p database.Proxy
	if database.DB.First(&p, proxyID).Error == nil {
		backend := proxy.GetBackend(p.Backend)
		if um, ok := backend.(proxy.UserManager); ok {
			// Telemt: remove user via API
			username := fmt.Sprintf("user_%d", clientID)
			um.RemoveUser(p.Port, username)
		}
	}

	result := database.DB.Where("id = ? AND proxy_id = ?", clientID, proxyID).Delete(&database.Client{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Client not found"})
		return
	}

	// Update proxy state after deletion
	if p.ID > 0 && p.Enabled {
		backend := proxy.GetBackend(p.Backend)
		if _, ok := backend.(proxy.UserManager); ok {
			// Telemt: update config file for future container restarts
			proxy.RefreshConfig(&p)
		} else {
			// Official: restart container without deleted secret
			dbClients := database.GetEnabledClients(p.ID)
			entries := proxy.ClientEntriesFromDB(dbClients)
			if len(entries) > 0 {
				cid, err := proxy.RestartProxy(p.ID, p.Port, entries, p.FakeTLSDomain, p.Backend, p.AdTag)
				if err == nil {
					database.DB.Model(&p).Update("container_id", cid)
				}
			} else {
				proxy.StopProxy(p.ID)
				database.DB.Model(&p).Updates(map[string]interface{}{
					"enabled": false, "container_id": "",
				})
			}
		}
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

	var cl database.Client
	if database.DB.Where("id = ? AND proxy_id = ?", clientID, proxyID).First(&cl).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Client not found"})
		return
	}

	wasDisabled := !cl.Enabled
	database.DB.Model(&cl).Updates(map[string]interface{}{
		"traffic_up": 0, "traffic_down": 0, "enabled": true,
	})

	// Sync with proxy engine
	var p database.Proxy
	if database.DB.First(&p, proxyID).Error == nil && p.Enabled {
		backend := proxy.GetBackend(p.Backend)
		if um, ok := backend.(proxy.UserManager); ok {
			username := fmt.Sprintf("user_%d", cl.ID)
			// Always remove+add to reset telemt-side quota counter
			um.RemoveUser(p.Port, username)
			um.AddUser(p.Port, username, cl.Secret, 0, cl.TrafficLimit, cl.ExpiryTime)
			if wasDisabled {
				proxy.RefreshConfig(&p)
			}
		} else if wasDisabled {
			// Official: restart to re-add the secret
			dbClients := database.GetEnabledClients(p.ID)
			entries := proxy.ClientEntriesFromDB(dbClients)
			if len(entries) > 0 {
				cid, err := proxy.RestartProxy(p.ID, p.Port, entries, p.FakeTLSDomain, p.Backend, p.AdTag)
				if err == nil {
					database.DB.Model(&p).Update("container_id", cid)
				}
			}
		}
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

// ── SSL Certificates ────────────────────────────────────────────────────

func UploadCert(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		certFile, err := c.FormFile("cert")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Missing cert file"})
			return
		}
		keyFile, err := c.FormFile("key")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Missing key file"})
			return
		}

		certDir := filepath.Join(cfg.DataDir, "custom-certs")
		os.MkdirAll(certDir, 0700)
		certPath := filepath.Join(certDir, "cert.pem")
		keyPath := filepath.Join(certDir, "key.pem")

		if err := c.SaveUploadedFile(certFile, certPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to save cert"})
			return
		}
		if err := c.SaveUploadedFile(keyFile, keyPath); err != nil {
			os.Remove(certPath)
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to save key"})
			return
		}

		// Validate cert+key pair
		_, err = tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			os.Remove(certPath)
			os.Remove(keyPath)
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid certificate/key pair: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Certificate uploaded. Restart required."})
	}
}

func DeleteCert(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		certDir := filepath.Join(cfg.DataDir, "custom-certs")
		os.Remove(filepath.Join(certDir, "cert.pem"))
		os.Remove(filepath.Join(certDir, "key.pem"))
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Certificate removed. Restart required."})
	}
}

func CertInfo(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		certPath := filepath.Join(cfg.DataDir, "custom-certs", "cert.pem")
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"installed": false})
			return
		}

		block, _ := pem.Decode(certPEM)
		if block == nil {
			c.JSON(http.StatusOK, gin.H{"installed": true, "error": "Failed to parse PEM"})
			return
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"installed": true, "error": err.Error()})
			return
		}

		domains := cert.DNSNames
		if len(domains) == 0 && cert.Subject.CommonName != "" {
			domains = []string{cert.Subject.CommonName}
		}

		c.JSON(http.StatusOK, gin.H{
			"installed":  true,
			"domains":    domains,
			"issuer":     cert.Issuer.CommonName,
			"not_before": cert.NotBefore.Format("2006-01-02"),
			"not_after":  cert.NotAfter.Format("2006-01-02"),
			"expired":    time.Now().After(cert.NotAfter),
		})
	}
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
