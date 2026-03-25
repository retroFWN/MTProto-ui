package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

var DockerHostIP = "127.0.0.1"

func telemtBaseURL(apiPort int) string {
	return fmt.Sprintf("http://%s:%d", DockerHostIP, apiPort)
}

// ── List users ──────────────────────────────────────────────────────────

type telemtUsersResponse struct {
	OK   bool `json:"ok"`
	Data []struct {
		Username           string   `json:"username"`
		Secret             string   `json:"secret"`
		CurrentConnections int      `json:"current_connections"`
		TotalOctets        int64    `json:"total_octets"`
		ActiveUniqueIPs    int      `json:"active_unique_ips"`
		ActiveIPList       []string `json:"active_unique_ips_list"`
	} `json:"data"`
}

func telemtListUsers(apiPort int) ([]UserStats, error) {
	resp, err := httpClient.Get(telemtBaseURL(apiPort) + "/v1/users")
	if err != nil {
		return nil, fmt.Errorf("telemt API unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result telemtUsersResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("telemt API parse error: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telemt API error")
	}

	users := make([]UserStats, 0, len(result.Data))
	for _, u := range result.Data {
		users = append(users, UserStats{
			Username:           u.Username,
			Secret:             u.Secret,
			CurrentConnections: u.CurrentConnections,
			TotalOctets:        u.TotalOctets,
			ActiveUniqueIPs:    u.ActiveUniqueIPs,
			ActiveIPList:       u.ActiveIPList,
		})
	}
	return users, nil
}

// ── Add user ────────────────────────────────────────────────────────────

type telemtCreateUserRequest struct {
	Username      string `json:"username"`
	Secret        string `json:"secret,omitempty"`
	MaxTCPConns   int    `json:"max_tcp_conns,omitempty"`
	DataQuota     int64  `json:"data_quota_bytes,omitempty"`
	ExpirationRFC string `json:"expiration_rfc3339,omitempty"`
}

func telemtAddUser(apiPort int, username, secret string, maxConns int, quotaBytes int64, expiryUnix int64) error {
	req := telemtCreateUserRequest{
		Username: username,
	}

	// Strip "ee" prefix for telemt — it wants raw 32-hex secret
	if len(secret) > 2 && secret[:2] == "ee" {
		req.Secret = secret[2:]
	} else {
		req.Secret = secret
	}
	// Pad/trim to 32
	for len(req.Secret) < 32 {
		req.Secret += "0"
	}
	if len(req.Secret) > 32 {
		req.Secret = req.Secret[:32]
	}

	if maxConns > 0 {
		req.MaxTCPConns = maxConns
	}
	if quotaBytes > 0 {
		req.DataQuota = quotaBytes
	}
	if expiryUnix > 0 {
		req.ExpirationRFC = time.Unix(expiryUnix, 0).UTC().Format(time.RFC3339)
	}

	body, _ := json.Marshal(req)
	resp, err := httpClient.Post(
		telemtBaseURL(apiPort)+"/v1/users",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("telemt API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		// User already exists — not an error
		return nil
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telemt add user failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ── Remove user ─────────────────────────────────────────────────────────

func telemtRemoveUser(apiPort int, username string) error {
	req, _ := http.NewRequest(http.MethodDelete, telemtBaseURL(apiPort)+"/v1/users/"+username, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telemt API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil // already gone
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telemt remove user failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ── Summary stats ───────────────────────────────────────────────────────

type telemtSummaryResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		UptimeSeconds    int   `json:"uptime_seconds"`
		ConnectionsTotal int64 `json:"connections_total"`
		ConfiguredUsers  int   `json:"configured_users"`
	} `json:"data"`
}

type telemtConnectionsResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Totals struct {
			CurrentConnections int64 `json:"current_connections"`
			ActiveUsers        int   `json:"active_users"`
		} `json:"totals"`
	} `json:"data"`
}

func telemtGetSummary(apiPort int) (*ProxySummary, error) {
	// Get summary
	resp, err := httpClient.Get(telemtBaseURL(apiPort) + "/v1/stats/summary")
	if err != nil {
		return nil, fmt.Errorf("telemt API unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var summary telemtSummaryResponse
	if err := json.Unmarshal(body, &summary); err != nil {
		return nil, err
	}

	result := &ProxySummary{
		UptimeSeconds:    summary.Data.UptimeSeconds,
		ConnectionsTotal: summary.Data.ConnectionsTotal,
		ConfiguredUsers:  summary.Data.ConfiguredUsers,
	}

	// Get live connections
	resp2, err := httpClient.Get(telemtBaseURL(apiPort) + "/v1/runtime/connections/summary")
	if err == nil {
		defer resp2.Body.Close()
		body2, _ := io.ReadAll(resp2.Body)
		var conns telemtConnectionsResponse
		if json.Unmarshal(body2, &conns) == nil {
			result.CurrentConnections = conns.Data.Totals.CurrentConnections
		}
	}

	return result, nil
}
