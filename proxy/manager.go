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

// GenerateSecret creates an ee-prefixed 32-char secret for the official C proxy.
// Format: ee + domain_hex + random = 32 chars total.
func GenerateSecret(fakeTLSDomain string) string {
	domainHex := hex.EncodeToString([]byte(fakeTLSDomain))
	needed := 30 - len(domainHex)
	if needed > 0 {
		randBytes := make([]byte, (needed+1)/2)
		rand.Read(randBytes)
		return "ee" + domainHex + hex.EncodeToString(randBytes)[:needed]
	}
	return "ee" + domainHex[:30]
}

// GenerateTelemtSecret creates a raw 32-hex secret for telemt Rust proxy.
// Telemt handles the ee prefix and domain in its link generation internally.
func GenerateTelemtSecret() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// BuildTgLink generates the tg:// proxy link for the appropriate backend.
func BuildTgLink(serverIP string, port int, secret, backend, domain string) string {
	if backend == "telemt" {
		// telemt format: ee + raw_secret + hex(domain)
		domainHex := hex.EncodeToString([]byte(domain))
		return fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=ee%s%s", serverIP, port, secret, domainHex)
	}
	// official format: secret already contains ee prefix
	return fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", serverIP, port, secret)
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

		for i := range proxies {
			p := &proxies[i]
			backend := GetBackend(p.Backend)

			if um, ok := backend.(UserManager); ok {
				users, err := um.ListUsers(p.Port)
				if err != nil {
					collectDockerStats(p)
					continue
				}

				var clients []database.Client
				database.DB.Where("proxy_id = ?", p.ID).Find(&clients)

				var totalDeltaDown, totalDeltaUp int64
				for j := range clients {
					cl := &clients[j]
					for _, u := range users {
						if MatchSecret(cl.Secret, u.Secret) {
							deltaOctets := u.TotalOctets - cl.LastStatOctets
							if deltaOctets < 0 {
								deltaOctets = u.TotalOctets
							}

							half := deltaOctets / 2
							upDelta := deltaOctets - half
							newDown := cl.TrafficDown + half
							newUp := cl.TrafficUp + upDelta

							database.DB.Model(cl).Updates(map[string]interface{}{
								"traffic_down":     newDown,
								"traffic_up":       newUp,
								"last_stat_octets": u.TotalOctets,
							})

							totalDeltaDown += half
							totalDeltaUp += upDelta

							if cl.TrafficLimit > 0 && (newDown+newUp) >= cl.TrafficLimit && cl.Enabled {
								database.DB.Model(cl).Update("enabled", false)
								um.RemoveUser(p.Port, u.Username)
								log.Printf("Client %s (proxy %d) disabled: traffic limit exceeded (%d/%d)",
									cl.Name, p.ID, newDown+newUp, cl.TrafficLimit)
							}
							break
						}
					}
				}

				if totalDeltaDown > 0 || totalDeltaUp > 0 {
					newProxyDown := p.TrafficDown + totalDeltaDown
					newProxyUp := p.TrafficUp + totalDeltaUp
					database.DB.Model(p).Updates(map[string]interface{}{
						"traffic_down": newProxyDown,
						"traffic_up":   newProxyUp,
					})

					if p.TrafficLimit > 0 && (newProxyDown+newProxyUp) >= p.TrafficLimit {
						database.DB.Model(p).Update("enabled", false)
						StopProxy(p.ID)
						log.Printf("Proxy %d disabled: traffic limit exceeded (%d/%d)",
							p.ID, newProxyDown+newProxyUp, p.TrafficLimit)
					}
				}
			} else {
				collectDockerStats(p)
			}
		}
	}
}

func collectDockerStats(p *database.Proxy) {
	stats := GetContainerStats(p.ID)
	rx := ParseNetBytes(stats.NetRx)
	tx := ParseNetBytes(stats.NetTx)

	deltaRx := rx - p.LastStatDown
	deltaTx := tx - p.LastStatUp
	if deltaRx < 0 {
		deltaRx = rx
	}
	if deltaTx < 0 {
		deltaTx = tx
	}

	if deltaRx > 0 || deltaTx > 0 {
		newDown := p.TrafficDown + deltaRx
		newUp := p.TrafficUp + deltaTx
		database.DB.Model(p).Updates(map[string]interface{}{
			"traffic_down":   newDown,
			"traffic_up":     newUp,
			"last_stat_down": rx,
			"last_stat_up":   tx,
		})

		if p.TrafficLimit > 0 && (newDown+newUp) >= p.TrafficLimit && p.Enabled {
			database.DB.Model(p).Update("enabled", false)
			StopProxy(p.ID)
			log.Printf("Proxy %d disabled: traffic limit exceeded (%d/%d)",
				p.ID, newDown+newUp, p.TrafficLimit)
		}
	}
}

// MatchSecret compares a panel secret with a telemt API secret.
func MatchSecret(panelSecret, telemtSecret string) bool {
	// Direct match (telemt backend stores raw 32-hex)
	if strings.EqualFold(panelSecret, telemtSecret) {
		return true
	}
	// Fallback: strip ee prefix and pad for comparison
	raw := panelSecret
	if len(raw) > 2 && raw[:2] == "ee" {
		raw = raw[2:]
	}
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
		n, expiredClients := database.DisableExpiredClients()
		if n > 0 {
			log.Printf("Disabled %d expired client(s)", n)

			proxyClients := make(map[uint][]database.Client)
			for _, cl := range expiredClients {
				proxyClients[cl.ProxyID] = append(proxyClients[cl.ProxyID], cl)
			}

			for pid, cls := range proxyClients {
				var p database.Proxy
				if database.DB.First(&p, pid).Error != nil || !p.Enabled {
					continue
				}

				backend := GetBackend(p.Backend)
				if um, ok := backend.(UserManager); ok {
					// Telemt: remove expired users via API
					for _, cl := range cls {
						um.RemoveUser(p.Port, fmt.Sprintf("user_%d", cl.ID))
					}
				} else {
					// Official: restart container with remaining secrets
					secrets := database.GetEnabledSecrets(p.ID)
					if len(secrets) > 0 {
						cid, err := StartProxy(p.ID, p.Port, secrets, p.FakeTLSDomain, p.Backend)
						if err == nil {
							database.DB.Model(&p).Update("container_id", cid)
						}
					} else {
						StopProxy(p.ID)
						database.DB.Model(&p).Updates(map[string]interface{}{
							"enabled": false, "container_id": "",
						})
					}
				}
			}
		}
	}
}
