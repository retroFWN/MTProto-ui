package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	telemtImage   = "whn0thacked/telemt-docker:latest"
	telemtAPIPort = 9091
	telemtMetrics = 9090
)

// TelemtBackend wraps the telemt Rust-based MTProto proxy.
type TelemtBackend struct{}

func (b *TelemtBackend) Info() BackendInfo {
	return BackendInfo{
		ID:    "telemt",
		Name:  "telemt (Rust)",
		Lang:  "Rust",
		Image: telemtImage,
		Description: "Продвинутый прокси на Rust с полным API управления, " +
			"per-user статистикой, Prometheus метриками и защитой от replay-атак.",
		Features: []Feature{
			{"Базовое проксирование", true},
			{"Fake TLS v2 (улучшенная маскировка)", true},
			{"Мульти-секрет", true},
			{"Авто-конфигурация", true},
			{"Per-user статистика", true},
			{"Prometheus метрики", true},
			{"Management API", true},
			{"Динамическое добавление секретов", true},
			{"Лимит устройств", true},
			{"Anti-replay защита", true},
		},
	}
}

// TelemtAPIPort returns the host port for the management API for a given proxy port.
func TelemtAPIPort(proxyPort int) int {
	return proxyPort + 10000
}

// TelemtMetricsPort returns the host port for Prometheus metrics.
func TelemtMetricsPort(proxyPort int) int {
	return proxyPort + 20000
}

// generateConfigTOML creates a telemt config.toml with users and settings.
func generateConfigTOML(secrets []string, domain string) string {
	var sb strings.Builder

	sb.WriteString("[general]\n")
	sb.WriteString("use_middle_proxy = true\n")
	sb.WriteString("log_level = \"normal\"\n\n")

	sb.WriteString("[general.modes]\n")
	sb.WriteString("classic = false\n")
	sb.WriteString("secure = false\n")
	sb.WriteString("tls = true\n\n")

	sb.WriteString("[general.telemetry]\n")
	sb.WriteString("core_enabled = true\n")
	sb.WriteString("user_enabled = true\n")
	sb.WriteString("me_level = \"normal\"\n\n")

	sb.WriteString("[server]\n")
	sb.WriteString("port = 443\n")
	sb.WriteString("metrics_port = 9090\n")
	sb.WriteString("metrics_whitelist = [\"0.0.0.0/0\", \"::0/0\"]\n\n")

	sb.WriteString("[server.api]\n")
	sb.WriteString("enabled = true\n")
	sb.WriteString("listen = \"0.0.0.0:9091\"\n")
	sb.WriteString("whitelist = [\"0.0.0.0/0\", \"::0/0\"]\n\n")

	sb.WriteString("[[server.listeners]]\n")
	sb.WriteString("ip = \"0.0.0.0\"\n\n")

	sb.WriteString("[censorship]\n")
	if domain == "" {
		domain = "google.com"
	}
	sb.WriteString(fmt.Sprintf("tls_domain = \"%s\"\n", domain))
	sb.WriteString("mask = true\n")
	sb.WriteString("tls_emulation = true\n\n")

	sb.WriteString("[access.users]\n")
	for i, s := range secrets {
		// Strip "ee" prefix and domain hex — telemt wants raw 32-hex secret
		rawSecret := s
		if len(rawSecret) > 2 && strings.HasPrefix(rawSecret, "ee") {
			rawSecret = rawSecret[2:]
		}
		// Pad to 32 hex chars if needed
		for len(rawSecret) < 32 {
			rawSecret += "0"
		}
		if len(rawSecret) > 32 {
			rawSecret = rawSecret[:32]
		}
		sb.WriteString(fmt.Sprintf("user_%d = \"%s\"\n", i, rawSecret))
	}

	return sb.String()
}

func (b *TelemtBackend) BuildRunArgs(containerName string, port int, secrets []string, domain string) []string {
	// Generate config.toml and write to temp directory
	configDir := filepath.Join("data", "telemt", containerName)
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "telemt.toml")
	configContent := generateConfigTOML(secrets, domain)
	os.WriteFile(configPath, []byte(configContent), 0644)

	absConfigDir, _ := filepath.Abs(configDir)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"--cap-drop", "ALL",
		"--cap-add", "NET_BIND_SERVICE",
		"--ulimit", "nofile=65536:65536",
		"-p", fmt.Sprintf("%d:443", port),
		"-p", fmt.Sprintf("%d:%d", TelemtAPIPort(port), telemtAPIPort),
		"-p", fmt.Sprintf("%d:%d", TelemtMetricsPort(port), telemtMetrics),
		"-v", fmt.Sprintf("%s:/etc/telemt", absConfigDir),
		telemtImage,
		"/etc/telemt/telemt.toml",
	}
	return args
}

func (b *TelemtBackend) PullImage() error {
	return exec.Command("docker", "pull", telemtImage).Run()
}

// ── UserManager interface ───────────────────────────────────────────────

func (b *TelemtBackend) AddUser(proxyPort int, username, secret string, maxConns int, quotaBytes int64, expiryUnix int64) error {
	apiPort := TelemtAPIPort(proxyPort)
	return telemtAddUser(apiPort, username, secret, maxConns, quotaBytes, expiryUnix)
}

func (b *TelemtBackend) RemoveUser(proxyPort int, username string) error {
	apiPort := TelemtAPIPort(proxyPort)
	return telemtRemoveUser(apiPort, username)
}

func (b *TelemtBackend) ListUsers(proxyPort int) ([]UserStats, error) {
	apiPort := TelemtAPIPort(proxyPort)
	return telemtListUsers(apiPort)
}

func (b *TelemtBackend) GetSummary(proxyPort int) (*ProxySummary, error) {
	apiPort := TelemtAPIPort(proxyPort)
	return telemtGetSummary(apiPort)
}
