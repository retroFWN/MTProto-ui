package main

import (
	"fmt"
	"log"

	"mtproxy-panel/config"
	"mtproxy-panel/database"
	"mtproxy-panel/proxy"
	"mtproxy-panel/web"
)

func main() {
	cfg := config.Load()

	database.Init(cfg.DBPath)
	database.Seed(cfg.DefaultUser, cfg.DefaultPass)
	proxy.InitConfig(cfg.ProxyImage, cfg.ContainerPfx)

	// Background tasks
	go proxy.TrafficCollector(cfg.StatsInterval)
	go proxy.ExpiryChecker()

	router := web.NewRouter(cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("MTProxy Panel starting on http://%s", addr)
	log.Printf("Default credentials: %s / %s", cfg.DefaultUser, cfg.DefaultPass)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
