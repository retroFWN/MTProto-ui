package proxy

// Feature describes a single capability of a proxy backend.
type Feature struct {
	Name      string `json:"name"`
	Supported bool   `json:"supported"`
}

// BackendInfo holds metadata about a proxy backend for UI display.
type BackendInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Lang        string    `json:"lang"`
	Image       string    `json:"image"`
	Description string    `json:"description"`
	Features    []Feature `json:"features"`
}

// Backend is the interface every proxy engine must implement.
type Backend interface {
	Info() BackendInfo
	BuildRunArgs(containerName string, port int, clients []ClientEntry, domain string, adTag string) []string
	PullImage() error
}

// UserManager is an optional interface for backends that support per-user management via API.
type UserManager interface {
	// AddUser creates a user on the running proxy. Returns error if proxy not reachable.
	AddUser(proxyPort int, username, secret string, maxConns int, quotaBytes int64, expiryUnix int64) error
	// RemoveUser deletes a user from the running proxy.
	RemoveUser(proxyPort int, username string) error
	// ListUsers returns live per-user stats from the running proxy.
	ListUsers(proxyPort int) ([]UserStats, error)
	// GetSummary returns overall proxy stats.
	GetSummary(proxyPort int) (*ProxySummary, error)
}

// UserStats holds live per-user data from telemt API.
type UserStats struct {
	Username           string   `json:"username"`
	Secret             string   `json:"secret"`
	CurrentConnections int      `json:"current_connections"`
	TotalOctets        int64    `json:"total_octets"`
	ActiveUniqueIPs    int      `json:"active_unique_ips"`
	ActiveIPList       []string `json:"active_unique_ips_list"`
}

// ProxySummary holds overall proxy stats.
type ProxySummary struct {
	UptimeSeconds      int   `json:"uptime_seconds"`
	ConnectionsTotal   int64 `json:"connections_total"`
	ConfiguredUsers    int   `json:"configured_users"`
	CurrentConnections int64 `json:"current_connections"`
}

// ClientEntry carries per-client data needed by backends (config + API sync).
type ClientEntry struct {
	ID           uint
	Secret       string
	TrafficLimit int64
	ExpiryTime   int64
}

// ── Registry ────────────────────────────────────────────────────────────

var backends = map[string]Backend{}

func RegisterBackend(b Backend) {
	backends[b.Info().ID] = b
}

func GetBackend(id string) Backend {
	if b, ok := backends[id]; ok {
		return b
	}
	return backends["official"]
}

func AllBackends() []BackendInfo {
	order := []string{"official", "telemt"}
	result := make([]BackendInfo, 0, len(order))
	for _, id := range order {
		if b, ok := backends[id]; ok {
			result = append(result, b.Info())
		}
	}
	return result
}

func init() {
	RegisterBackend(&OfficialBackend{})
	RegisterBackend(&TelemtBackend{})
}
