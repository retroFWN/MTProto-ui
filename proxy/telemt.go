package proxy

import (
	"fmt"
	"os/exec"
)

// TelemtBackend wraps the telemt Rust-based MTProto proxy.
// GitHub: https://github.com/nickolay/teleporter (telemt engine)
// Management wrapper: https://github.com/SamNet-dev/MTProxyMax
type TelemtBackend struct{}

func (b *TelemtBackend) Info() BackendInfo {
	return BackendInfo{
		ID:    "telemt",
		Name:  "telemt (Rust)",
		Lang:  "Rust",
		Image: "nickolay/mtproto-proxy",
		Description: "Продвинутый прокси на Rust с полным API управления, " +
			"per-user статистикой, Prometheus метриками и защитой от replay-атак. " +
			"Используется в MTProxyMax.",
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

func (b *TelemtBackend) BuildRunArgs(containerName string, port int, secrets []string, domain string) []string {
	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"-p", fmt.Sprintf("%d:443", port),
		"-p", fmt.Sprintf("%d:2398", port+10000), // management API
		"-p", fmt.Sprintf("%d:9090", port+20000), // Prometheus metrics
	}

	// Pass secrets via environment
	for i, s := range secrets {
		args = append(args, "-e", fmt.Sprintf("SECRET_%d=%s", i, s))
	}

	// Fake TLS domain
	if domain != "" {
		args = append(args, "-e", fmt.Sprintf("TLS_DOMAIN=%s", domain))
	}

	args = append(args, "nickolay/mtproto-proxy")
	return args
}

func (b *TelemtBackend) PullImage() error {
	return exec.Command("docker", "pull", "nickolay/mtproto-proxy").Run()
}
