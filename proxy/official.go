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
		Description: "Официальный прокси-сервер от команды Telegram. " +
			"Стабильный и проверенный, но без расширенного управления.",
		Features: []Feature{
			{"Базовое проксирование", true},
			{"Fake TLS (маскировка)", true},
			{"Мульти-секрет", true},
			{"Авто-конфигурация", true},
			{"Per-user статистика", false},
			{"Prometheus метрики", false},
			{"Management API", false},
			{"Динамическое добавление секретов", false},
			{"Лимит устройств", false},
			{"Anti-replay защита", false},
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
