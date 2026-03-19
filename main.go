package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"golang.org/x/crypto/acme/autocert"

	"mtproxy-panel/config"
	"mtproxy-panel/database"
	"mtproxy-panel/proxy"
	"mtproxy-panel/web"
)

func main() {
	cfg := config.Load()

	database.Init(cfg.DBPath)
	database.Seed(cfg.DefaultUser, cfg.DefaultPass)
	proxy.ContainerPfx = cfg.ContainerPfx

	// Background tasks
	go proxy.TrafficCollector(cfg.StatsInterval)
	go proxy.ExpiryChecker()

	router := web.NewRouter(cfg)

	// Check domain: env var first, then DB setting
	domain := cfg.Domain
	if domain == "" {
		var s database.Setting
		if database.DB.Where("`key` = ?", "panel_domain").First(&s).Error == nil {
			domain = s.Value
		}
	}

	if domain != "" {
		// Auto-SSL via Let's Encrypt
		certDir := filepath.Join(cfg.DataDir, "certs")
		manager := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domain),
			Cache:      autocert.DirCache(certDir),
		}

		// HTTP :80 — ACME challenges + redirect to HTTPS
		go func() {
			log.Printf("HTTP  → https://%s (redirect + ACME)", domain)
			if err := http.ListenAndServe(":80", manager.HTTPHandler(nil)); err != nil {
				log.Printf("HTTP listener error: %v", err)
			}
		}()

		server := &http.Server{
			Addr:    ":443",
			Handler: router.Handler(),
			TLSConfig: &tls.Config{
				GetCertificate: manager.GetCertificate,
				MinVersion:     tls.VersionTLS12,
			},
		}

		log.Printf("MTProxy Panel starting on https://%s", domain)
		log.Printf("Default credentials: %s / %s", cfg.DefaultUser, cfg.DefaultPass)

		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Fatalf("HTTPS server failed: %v", err)
		}
	} else {
		// Plain HTTP
		addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		log.Printf("MTProxy Panel starting on http://%s", addr)
		log.Printf("Default credentials: %s / %s", cfg.DefaultUser, cfg.DefaultPass)
		log.Printf("Tip: set PANEL_DOMAIN or panel_domain in settings to enable auto-SSL")

		if err := router.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}
}
