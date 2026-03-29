package web

import (
	"path/filepath"

	"mtproxy-panel/auth"
	"mtproxy-panel/config"

	"github.com/gin-gonic/gin"
)

func NewRouter(cfg *config.Config) *gin.Engine {
	auth.Init(cfg.SecretKey)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.LoadHTMLGlob(filepath.Join(cfg.BaseDir, "templates", "*"))
	r.Static("/static", filepath.Join(cfg.BaseDir, "static"))

	// Public pages
	r.GET("/", LoginPage)

	// Auth API
	r.POST("/api/login", Login(cfg))
	r.POST("/api/logout", Logout)

	// Protected pages
	pages := r.Group("/panel")
	pages.Use(PageAuth())
	{
		pages.GET("", DashboardPage)
		pages.GET("/proxies", ProxiesPage)
		pages.GET("/settings", SettingsPage)
	}

	// Protected API
	api := r.Group("/api")
	api.Use(APIAuth())
	{
		api.POST("/change-password", ChangePassword)

		api.GET("/system/status", SystemStatus)

		api.GET("/proxies", ListProxies)
		api.POST("/proxies", CreateProxy)
		api.PUT("/proxies/:id", UpdateProxy)
		api.DELETE("/proxies/:id", DeleteProxy)
		api.POST("/proxies/:id/start", StartProxyHandler)
		api.POST("/proxies/:id/stop", StopProxyHandler)
		api.POST("/proxies/:id/restart", RestartProxyHandler)
		api.GET("/proxies/:id/stats", ProxyStatsHandler)
		api.GET("/proxies/:id/live", ProxyLiveHandler)
		api.GET("/proxies/:id/logs", ProxyLogsHandler)

		api.GET("/proxies/:id/clients", ListClients)
		api.POST("/proxies/:id/clients", CreateClient)
		api.PUT("/proxies/:id/clients/:cid", UpdateClient)
		api.DELETE("/proxies/:id/clients/:cid", DeleteClient)
		api.POST("/proxies/:id/clients/:cid/reset-traffic", ResetClientTraffic)

		api.GET("/settings", GetSettings)
		api.POST("/settings", UpdateSettings)
		api.POST("/pull-image", PullImageHandler)

		api.GET("/backends", ListBackends)

		api.POST("/upload-cert", UploadCert(cfg))
		api.DELETE("/custom-cert", DeleteCert(cfg))
		api.GET("/cert-info", CertInfo(cfg))

		api.GET("/bot/status", BotStatus)
		api.POST("/bot/start", BotStart(cfg))
		api.POST("/bot/stop", BotStop)
	}

	// Bot API — authenticated via X-Bot-Token header (= panel's secret key)
	bot := r.Group("/bot/api")
	bot.Use(BotAuth(cfg.SecretKey))
	{
		bot.GET("/proxies", ListProxies)
		bot.POST("/proxies/:id/start", StartProxyHandler)
		bot.POST("/proxies/:id/stop", StopProxyHandler)
		bot.POST("/proxies/:id/restart", RestartProxyHandler)
		bot.GET("/proxies/:id/clients", ListClients)
		bot.POST("/proxies/:id/clients", CreateClient)
		bot.PUT("/proxies/:id/clients/:cid", UpdateClient)
		bot.DELETE("/proxies/:id/clients/:cid", DeleteClient)
		bot.POST("/proxies/:id/clients/:cid/reset-traffic", ResetClientTraffic)
		bot.GET("/settings", GetSettings)
	}

	return r
}
