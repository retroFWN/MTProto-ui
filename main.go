package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/crypto/acme/autocert"

	"mtproxy-panel/config"
	"mtproxy-panel/database"
	"mtproxy-panel/proxy"
	"mtproxy-panel/web"
)

func main() {
	testMode := flag.Bool("test", false, "spin up C + Rust proxies on test ports, print tg:// links, wait for Ctrl+C")
	testIP := flag.String("test-ip", "", "server IP for test tg:// links (auto-detect if empty)")
	testDomain := flag.String("test-domain", "google.com", "fake-TLS domain for test proxies")
	testPortC := flag.Int("test-port-c", 10443, "port for C proxy in test mode")
	testPortRust := flag.Int("test-port-rust", 10444, "port for Rust proxy in test mode")
	flag.Parse()

	if *testMode {
		// Minimal init — just need proxy backends + DataHostPath
		cfg := config.Load()
		proxy.ContainerPfx = cfg.ContainerPfx
		proxy.DockerHostIP = cfg.DockerHostIP
		proxy.DataHostPath = cfg.DataHostPath

		ip := *testIP
		if ip == "" {
			// Quick auto-detect
			ip = os.Getenv("TEST_SERVER_IP")
		}
		if ip == "" {
			ip = "YOUR_SERVER_IP"
		}

		proxy.RunTestMode(proxy.TestConfig{
			ServerIP:     ip,
			Domain:       *testDomain,
			PortOfficial: *testPortC,
			PortTelemt:   *testPortRust,
		})
		return
	}

	cfg := config.Load()

	database.Init(cfg.DBPath)
	database.Seed(cfg.DefaultUser, cfg.DefaultPass)
	proxy.ContainerPfx = cfg.ContainerPfx
	proxy.DockerHostIP = cfg.DockerHostIP
	proxy.DataHostPath = cfg.DataHostPath

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
	cfg.Domain = domain

	// Check for custom SSL certificate
	customCert := filepath.Join(cfg.DataDir, "custom-certs", "cert.pem")
	customKey := filepath.Join(cfg.DataDir, "custom-certs", "key.pem")
	_, certErr := os.Stat(customCert)
	hasCustomCert := certErr == nil

	if hasCustomCert {
		// Custom SSL certificate
		// HTTP :80 — redirect to HTTPS
		go func() {
			log.Printf("HTTP :80 → HTTPS redirect")
			redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				target := "https://" + r.Host + r.URL.RequestURI()
				http.Redirect(w, r, target, http.StatusMovedPermanently)
			})
			if err := http.ListenAndServe(":80", redirect); err != nil {
				log.Printf("HTTP listener error: %v", err)
			}
		}()

		// Internal HTTP API for bot/internal access
		go func() {
			addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
			log.Printf("Internal API on http://%s", addr)
			if err := http.ListenAndServe(addr, router.Handler()); err != nil {
				log.Printf("Internal HTTP listener error: %v", err)
			}
		}()

		log.Printf("MTProxy Panel starting on https://:443 (custom certificate)")
		log.Printf("Default credentials: %s / %s", cfg.DefaultUser, cfg.DefaultPass)

		server := &http.Server{
			Addr:    ":443",
			Handler: router.Handler(),
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}
		if err := server.ListenAndServeTLS(customCert, customKey); err != nil {
			log.Fatalf("HTTPS server failed: %v", err)
		}
	} else if domain != "" {
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

		// Internal HTTP API for bot/internal access
		go func() {
			addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
			log.Printf("Internal API on http://%s", addr)
			if err := http.ListenAndServe(addr, router.Handler()); err != nil {
				log.Printf("Internal HTTP listener error: %v", err)
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
