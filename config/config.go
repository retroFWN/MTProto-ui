package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Host          string
	Port          int
	DBPath        string
	SecretKey     string
	DefaultUser   string
	DefaultPass   string
	TokenExpiry   int // minutes
	ProxyImage    string
	ContainerPfx  string
	StatsInterval int // seconds
}

func Load() *Config {
	base := baseDir()
	dataDir := filepath.Join(base, "data")
	os.MkdirAll(dataDir, 0755)

	return &Config{
		Host:          envOr("PANEL_HOST", "0.0.0.0"),
		Port:          envInt("PANEL_PORT", 8080),
		DBPath:        filepath.Join(dataDir, "mtproxy.db"),
		SecretKey:     loadOrCreateSecret(filepath.Join(dataDir, ".secret_key")),
		DefaultUser:   "admin",
		DefaultPass:   "admin",
		TokenExpiry:   360,
		ProxyImage:    "telegrammessenger/proxy",
		ContainerPfx:  "mtproxy-",
		StatsInterval: 10,
	}
}

func baseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func loadOrCreateSecret(path string) string {
	if key := os.Getenv("SECRET_KEY"); key != "" {
		return key
	}
	data, err := os.ReadFile(path)
	if err == nil {
		s := strings.TrimSpace(string(data))
		if len(s) >= 32 {
			return s
		}
	}
	b := make([]byte, 32)
	rand.Read(b)
	key := hex.EncodeToString(b)
	os.WriteFile(path, []byte(key), 0600)
	return key
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
