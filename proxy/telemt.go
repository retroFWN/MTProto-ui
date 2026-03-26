package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	telemtImage   = "telemt-local"
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
		Description: "be_telemt_desc",
		Features: []Feature{
			{"be_feat_basic_proxy", true},
			{"be_feat_fake_tls_v2", true},
			{"be_feat_multi_secret", true},
			{"be_feat_auto_config", true},
			{"be_feat_per_user_stats", true},
			{"be_feat_prometheus", true},
			{"be_feat_management_api", true},
			{"be_feat_dynamic_secrets", true},
			{"be_feat_device_limit", true},
			{"be_feat_anti_replay", true},
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
func generateConfigTOML(port int, secrets []string, domain string) string {
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
	sb.WriteString(fmt.Sprintf("port = %d\n", port))
	sb.WriteString(fmt.Sprintf("metrics_port = %d\n", port+20000))
	sb.WriteString("metrics_whitelist = [\"0.0.0.0/0\", \"::0/0\"]\n\n")

	sb.WriteString("[server.api]\n")
	sb.WriteString("enabled = true\n")
	sb.WriteString(fmt.Sprintf("listen = \"0.0.0.0:%d\"\n", port+10000))
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
		// Client strips "ee" prefix before auth, so telemt must have the same stripped key
		raw := s
		if len(raw) >= 2 && (raw[:2] == "ee" || raw[:2] == "dd") {
			raw = raw[2:]
		}
		for len(raw) < 32 {
			raw += "0"
		}
		if len(raw) > 32 {
			raw = raw[:32]
		}
		sb.WriteString(fmt.Sprintf("user_%d = \"%s\"\n", i, raw))
	}

	return sb.String()
}

// DataHostPath is the host-side path to the data directory.
// Set via DATA_HOST_PATH env when running inside Docker (DinD via socket).
var DataHostPath string

func (b *TelemtBackend) BuildRunArgs(containerName string, port int, secrets []string, domain string) []string {
	// Generate config and write to data directory
	configDir := filepath.Join("data", "telemt", containerName)
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "telemt.toml")
	configContent := generateConfigTOML(port, secrets, domain)
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Resolve the volume mount path for the host
	var hostConfigDir string
	if DataHostPath != "" {
		// Running in Docker: use the known host path
		hostConfigDir = filepath.Join(DataHostPath, "telemt", containerName)
	} else {
		// Running directly on host
		hostConfigDir, _ = filepath.Abs(configDir)
	}

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"--network", "host",
		"--user", "root",
		"--ulimit", "nofile=65536:65536",
		"-e", "RUST_LOG=info",
		"-v", fmt.Sprintf("%s:/etc/telemt", hostConfigDir),
		telemtImage,
		"/etc/telemt/telemt.toml",
	}
	return args
}

func (b *TelemtBackend) PullImage() error {
	// Build from upstream source (pre-built image has UPX issues on some kernels)
	return exec.Command("docker", "build", "-t", telemtImage, "https://github.com/telemt/telemt.git").Run()
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
