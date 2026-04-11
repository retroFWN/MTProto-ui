package proxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
func generateConfigTOML(port int, clients []ClientEntry, domain string, adTag string) string {
	var sb strings.Builder

	sb.WriteString("[general]\n")
	sb.WriteString("use_middle_proxy = true\n")
	if adTag != "" {
		sb.WriteString(fmt.Sprintf("tag = \"%s\"\n", adTag))
	}
	sb.WriteString("log_level = \"normal\"\n\n")

	sb.WriteString("[general.modes]\n")
	sb.WriteString("classic = true\n")
	sb.WriteString("secure = true\n")
	sb.WriteString("tls = true\n\n")

	sb.WriteString("[general.telemetry]\n")
	sb.WriteString("core_enabled = true\n")
	sb.WriteString("user_enabled = true\n")
	sb.WriteString("me_level = \"normal\"\n\n")

	sb.WriteString("[server]\n")
	sb.WriteString(fmt.Sprintf("port = %d\n", port))
	sb.WriteString(fmt.Sprintf("metrics_port = %d\n", port+20000))
	sb.WriteString("metrics_whitelist = [\"127.0.0.0/8\", \"172.16.0.0/12\", \"10.0.0.0/8\", \"192.168.0.0/16\", \"::1/128\"]\n\n")

	sb.WriteString("[server.api]\n")
	sb.WriteString("enabled = true\n")
	sb.WriteString(fmt.Sprintf("listen = \"0.0.0.0:%d\"\n", port+10000))
	sb.WriteString("whitelist = [\"127.0.0.0/8\", \"172.16.0.0/12\", \"10.0.0.0/8\", \"192.168.0.0/16\", \"::1/128\"]\n\n")

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
	for _, cl := range clients {
		sb.WriteString(fmt.Sprintf("user_%d = \"%s\"\n", cl.ID, ExtractKey(cl.Secret)))
	}

	return sb.String()
}

// DataHostPath is the host-side path to the data directory.
// Set via DATA_HOST_PATH env when running inside Docker (DinD via socket).
var DataHostPath string

func (b *TelemtBackend) BuildRunArgs(containerName string, port int, clients []ClientEntry, domain string, adTag string) []string {
	// Generate config and write to data directory.
	// Dir 0777 + file 0666: telemt API does atomic saves via .tmp files in the same dir.
	configDir := filepath.Join("data", "telemt", containerName)
	os.MkdirAll(configDir, 0777)
	configPath := filepath.Join(configDir, "telemt.toml")
	configContent := generateConfigTOML(port, clients, domain, adTag)
	os.WriteFile(configPath, []byte(configContent), 0666)

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
	log.Printf("Building telemt image (this may take several minutes)...")
	out, err := exec.Command("docker", "build", "-t", telemtImage, "https://github.com/telemt/telemt.git").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build failed: %v\n%s", err, string(out))
	}
	log.Printf("telemt image built successfully")
	return nil
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

// UpdateTelemtConfig regenerates the telemt config file on disk.
// This ensures container restarts pick up the current user list.
func UpdateTelemtConfig(proxyID uint, port int, clients []ClientEntry, domain string, adTag string) {
	containerName := ContainerName(proxyID)
	configDir := filepath.Join("data", "telemt", containerName)
	os.MkdirAll(configDir, 0777)
	configPath := filepath.Join(configDir, "telemt.toml")
	configContent := generateConfigTOML(port, clients, domain, adTag)
	os.WriteFile(configPath, []byte(configContent), 0666)
}

// syncTelemtUsersAfterStart waits for the telemt API to become available,
// then re-applies quotas and expiry for users that have them.
// Config-loaded users have no limits; this removes+adds them with full settings.
func syncTelemtUsersAfterStart(proxyPort int, clients []ClientEntry) {
	apiPort := TelemtAPIPort(proxyPort)

	// Wait for API with increasing backoff, retry up to 2 minutes
	var ready bool
	delays := []time.Duration{2, 2, 3, 3, 5, 5, 10, 10, 15, 15, 20, 30}
	for _, d := range delays {
		time.Sleep(d * time.Second)
		if _, err := telemtListUsers(apiPort); err == nil {
			ready = true
			break
		}
	}
	if !ready {
		log.Printf("ERROR: telemt API not ready after 2min for port %d — users have NO quotas/expiry!", proxyPort)
		// Keep retrying in background every 30s for 10 more minutes
		go func() {
			for i := 0; i < 20; i++ {
				time.Sleep(30 * time.Second)
				if _, err := telemtListUsers(apiPort); err == nil {
					log.Printf("telemt API finally ready for port %d, syncing users...", proxyPort)
					applyUserSync(apiPort, proxyPort, clients)
					return
				}
			}
			log.Printf("CRITICAL: telemt API never became ready for port %d after 12min", proxyPort)
		}()
		return
	}

	applyUserSync(apiPort, proxyPort, clients)
}

func applyUserSync(apiPort, proxyPort int, clients []ClientEntry) {
	synced := 0
	for _, cl := range clients {
		if cl.TrafficLimit > 0 || cl.ExpiryTime > 0 {
			username := fmt.Sprintf("user_%d", cl.ID)
			telemtRemoveUser(apiPort, username)
			telemtAddUser(apiPort, username, cl.Secret, 0, cl.TrafficLimit, cl.ExpiryTime)
			synced++
		}
	}
	if synced > 0 {
		log.Printf("Synced %d user(s) with quotas/expiry for port %d", synced, proxyPort)
	}
}
