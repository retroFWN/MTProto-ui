package proxy

import (
	"fmt"
	"os/exec"
)

// OfficialBackend wraps the official telegrammessenger/proxy (C).
type OfficialBackend struct{}

func (b *OfficialBackend) Info() BackendInfo {
	return BackendInfo{
		ID:    "official",
		Name:  "Official MTProto Proxy",
		Lang:  "C",
		Image: "telegrammessenger/proxy",
		Description: "be_official_desc",
		Features: []Feature{
			{"be_feat_basic_proxy", true},
			{"be_feat_fake_tls", true},
			{"be_feat_multi_secret", true},
			{"be_feat_auto_config", true},
			{"be_feat_per_user_stats", false},
			{"be_feat_prometheus", false},
			{"be_feat_management_api", false},
			{"be_feat_dynamic_secrets", false},
			{"be_feat_device_limit", false},
			{"be_feat_anti_replay", false},
		},
	}
}

func (b *OfficialBackend) BuildRunArgs(containerName string, port int, secrets []string, domain string) []string {
	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"-p", fmt.Sprintf("%d:443", port),
		"-e", fmt.Sprintf("SECRET=%s", secrets[0]),
		"telegrammessenger/proxy",
	}
	for _, s := range secrets[1:] {
		args = append(args, "-S", s)
	}
	return args
}

func (b *OfficialBackend) PullImage() error {
	return exec.Command("docker", "pull", "telegrammessenger/proxy").Run()
}
