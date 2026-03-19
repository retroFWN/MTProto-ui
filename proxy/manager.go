package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"mtproxy-panel/database"
)

var ContainerPfx = "mtproxy-"

func ContainerName(proxyID uint) string {
	return fmt.Sprintf("%s%d", ContainerPfx, proxyID)
}

// ── Secret generation ────────────────────────────────────────────────────

func GenerateSecret(fakeTLSDomain string) string {
	domainHex := hex.EncodeToString([]byte(fakeTLSDomain))
	// Total: ee (2) + 30 hex chars = 32 chars
	needed := 30 - len(domainHex)
	if needed > 0 {
		randBytes := make([]byte, 15)
		rand.Read(randBytes)
		randHex := hex.EncodeToString(randBytes)[:needed]
		return "ee" + domainHex + randHex
	}
	return "ee" + domainHex[:30]
}

// ── Container lifecycle ──────────────────────────────────────────────────

func StartProxy(proxyID uint, port int, secrets []string, domain string, backendID string) (string, error) {
	if len(secrets) == 0 {
		return "", fmt.Errorf("no secrets provided")
	}

	name := ContainerName(proxyID)
	StopProxy(proxyID)

	backend := GetBackend(backendID)
	args := backend.BuildRunArgs(name, port, secrets, domain)

	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		return "", fmt.Errorf("docker run failed: %w", err)
	}

	containerID := strings.TrimSpace(string(out))
	log.Printf("Started container %s (ID: %.12s) with %d secret(s) [%s]",
		name, containerID, len(secrets), backend.Info().Name)
	return containerID, nil
}

func StopProxy(proxyID uint) {
	name := ContainerName(proxyID)
	exec.Command("docker", "stop", name).Run()
	exec.Command("docker", "rm", name).Run()
}

func RestartProxy(proxyID uint, port int, secrets []string, domain string, backendID string) (string, error) {
	return StartProxy(proxyID, port, secrets, domain, backendID)
}

// ── Container info ───────────────────────────────────────────────────────

type ContainerStatus struct {
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	Running   bool   `json:"running"`
}

func GetContainerStatus(proxyID uint) ContainerStatus {
	name := ContainerName(proxyID)
	out, err := exec.Command(
		"docker", "inspect", "--format",
		"{{.State.Status}}|{{.State.StartedAt}}|{{.State.Running}}",
		name,
	).Output()
	if err != nil {
		return ContainerStatus{Status: "stopped"}
	}

	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	status := ContainerStatus{Status: parts[0]}
	if len(parts) > 1 {
		status.StartedAt = parts[1]
	}
	if len(parts) > 2 {
		status.Running = parts[2] == "true"
	}
	return status
}

type ContainerStats struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	NetRx  string `json:"net_rx"`
	NetTx  string `json:"net_tx"`
}

func GetContainerStats(proxyID uint) ContainerStats {
	name := ContainerName(proxyID)
	out, err := exec.Command(
		"docker", "stats", "--no-stream", "--format",
		"{{.CPUPerc}}|{{.MemUsage}}|{{.NetIO}}",
		name,
	).Output()
	if err != nil {
		return ContainerStats{CPU: "0%", Memory: "0", NetRx: "0B", NetTx: "0B"}
	}

	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	stats := ContainerStats{
		CPU:    strings.TrimSpace(parts[0]),
		Memory: "0",
		NetRx:  "0B",
		NetTx:  "0B",
	}
	if len(parts) > 1 {
		stats.Memory = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		netParts := strings.Split(parts[2], " / ")
		stats.NetRx = strings.TrimSpace(netParts[0])
		if len(netParts) > 1 {
			stats.NetTx = strings.TrimSpace(netParts[1])
		}
	}
	return stats
}

func GetContainerLogs(proxyID uint, tail int) string {
	name := ContainerName(proxyID)
	out, err := exec.Command("docker", "logs", "--tail", strconv.Itoa(tail), name).CombinedOutput()
	if err != nil {
		return ""
	}
	return string(out)
}

// ── Net bytes parser ─────────────────────────────────────────────────────

func ParseNetBytes(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	// Ordered longest-suffix-first to avoid "GB" matching "B"
	suffixes := []struct {
		suffix string
		mult   float64
	}{
		{"TIB", math.Pow(1024, 4)},
		{"TB", math.Pow(1024, 4)},
		{"GIB", math.Pow(1024, 3)},
		{"GB", math.Pow(1024, 3)},
		{"MIB", math.Pow(1024, 2)},
		{"MB", math.Pow(1024, 2)},
		{"KIB", 1024},
		{"KB", 1024},
		{"B", 1},
	}
	for _, entry := range suffixes {
		if strings.HasSuffix(s, entry.suffix) {
			numStr := strings.TrimSpace(s[:len(s)-len(entry.suffix)])
			if val, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int64(val * entry.mult)
			}
		}
	}
	return 0
}

// ── Background tasks ─────────────────────────────────────────────────────

func TrafficCollector(intervalSec int) {
	for {
		time.Sleep(time.Duration(intervalSec) * time.Second)

		var proxies []database.Proxy
		database.DB.Where("enabled = ?", true).Find(&proxies)

		for _, p := range proxies {
			backend := GetBackend(p.Backend)

			// If backend supports per-user stats (telemt), use its API
			if um, ok := backend.(UserManager); ok {
				users, err := um.ListUsers(p.Port)
				if err != nil {
					// Fallback to docker stats
					collectDockerStats(&p)
					continue
				}

				var totalOctets int64
				// Update per-client traffic from telemt live data
				var clients []database.Client
				database.DB.Where("proxy_id = ?", p.ID).Find(&clients)

				for _, cl := range clients {
					for _, u := range users {
						if MatchSecret(cl.Secret, u.Secret) {
							// total_octets is bidirectional — split evenly as approximation
							half := u.TotalOctets / 2
							database.DB.Model(&cl).Updates(map[string]interface{}{
								"traffic_down": half,
								"traffic_up":   u.TotalOctets - half,
							})
							totalOctets += u.TotalOctets
							// Disable client if traffic limit exceeded
							if cl.TrafficLimit > 0 && u.TotalOctets >= cl.TrafficLimit && cl.Enabled {
								database.DB.Model(&cl).Update("enabled", false)
								um.RemoveUser(p.Port, u.Username)
								log.Printf("Client %s (proxy %d) disabled: traffic limit exceeded (%d/%d)",
									cl.Name, p.ID, u.TotalOctets, cl.TrafficLimit)
							}
							break
						}
					}
				}

				if totalOctets > 0 {
					database.DB.Model(&p).Updates(map[string]interface{}{
						"traffic_down": totalOctets / 2,
						"traffic_up":   totalOctets - totalOctets/2,
					})
				}
			} else {
				collectDockerStats(&p)
			}
		}
	}
}

func collectDockerStats(p *database.Proxy) {
	stats := GetContainerStats(p.ID)
	rx := ParseNetBytes(stats.NetRx)
	tx := ParseNetBytes(stats.NetTx)
	if rx > 0 || tx > 0 {
		database.DB.Model(p).Updates(map[string]interface{}{
			"traffic_down": rx,
			"traffic_up":   tx,
		})
	}
}

// MatchSecret compares a panel secret (ee + domain_hex + padding) with a telemt raw secret.
func MatchSecret(panelSecret, telemtSecret string) bool {
	// Panel stores "ee" + 30 hex chars, telemt stores 32 hex chars (without "ee")
	raw := panelSecret
	if len(raw) > 2 && raw[:2] == "ee" {
		raw = raw[2:]
	}
	// Pad to 32 for comparison
	for len(raw) < 32 {
		raw += "0"
	}
	if len(raw) > 32 {
		raw = raw[:32]
	}
	return strings.EqualFold(raw, telemtSecret)
}

func ExpiryChecker() {
	for {
		time.Sleep(60 * time.Second)
		if n := database.DisableExpiredClients(); n > 0 {
			log.Printf("Disabled %d expired client(s)", n)
		}
	}
}
