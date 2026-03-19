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
	BuildRunArgs(containerName string, port int, secrets []string, domain string) []string
	PullImage() error
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
