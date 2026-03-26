<div align="center">

<img src="image/logo.png" alt="MTProxy Panel" width="300">

**Modern web panel for managing Telegram MTProto proxy servers**

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Bot-26A5E4?logo=telegram&logoColor=white)](#-telegram-bot)

**English** | [Русский](README.md)

---

<img src="image/1.png" alt="Screenshot" width="800">

</div>

## Features

| | Feature | Description |
|---|---|---|
| :chart_with_upwards_trend: | **Dashboard** | Real-time CPU, RAM, disk, network monitoring |
| :shield: | **Multi-proxy** | Multiple proxy instances on different ports |
| :busts_in_silhouette: | **Multi-client** | Per-user secrets, traffic limits, expiry dates |
| :gear: | **Dual Engine** | Official C or telemt Rust — choose per proxy |
| :lock: | **Fake TLS** | Traffic disguised as HTTPS to bypass blocks |
| :globe_with_meridians: | **Auto-SSL** | Automatic Let's Encrypt certificates |
| :robot: | **Telegram Bot** | Manage proxies directly from Telegram |
| :link: | **One-click Links** | Auto-generated `tg://proxy` connection links |
| :whale: | **Docker Native** | Each proxy = isolated container |
| :zap: | **Single Binary** | Go, ~25 MB, zero external dependencies |

## Quick Start

### 1. Install Docker

```bash
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker
```

### 2. Deploy the panel

```bash
git clone https://github.com/retroFWN/MTProto-ui.git
cd MTProto-ui
docker compose up -d --build
```

### 3. Open the panel

```
http://YOUR_SERVER_IP:8080
```

| | |
|---|---|
| **Login** | `admin` |
| **Password** | `admin` |

> :warning: **Change the default password after first login!**

## Proxy Engines

| | Official (C) | telemt (Rust) |
|---|:---:|:---:|
| **Image** | `telegrammessenger/proxy` | `whn0thacked/telemt-docker` |
| **Fake TLS** | v1 | v2 |
| **Per-user stats** | :x: | :white_check_mark: |
| **Management API** | :x: | :white_check_mark: |
| **Dynamic secrets** | :x: | :white_check_mark: |
| **Anti-replay** | :x: | :white_check_mark: |

## :robot: Telegram Bot

1. Create a bot via [@BotFather](https://t.me/BotFather)
2. In panel **Settings**, paste the bot token and your Telegram ID
3. Click **Save** then **Start Bot**

| Command | Description |
|---|---|
| `/proxies` | List proxy servers |
| `/connect <id>` | Get connection links |
| `/status <id>` | Proxy status & clients |
| `/addclient <pid> <name> [gb] [days]` | Create client |
| `/delclient <pid> <cid>` | Delete client |

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PANEL_PORT` | `8080` | Panel port |
| `PANEL_DOMAIN` | — | Domain for auto-SSL |
| `SECRET_KEY` | auto-generated | JWT signing key |
| `PROXY_BACKEND` | `official` | Default engine |
| `DOCKER_HOST_IP` | `127.0.0.1` | Docker host IP |

### Auto-SSL

```yaml
# docker-compose.yml
environment:
  - PANEL_DOMAIN=panel.example.com
ports:
  - "80:80"
  - "443:443"
```

## Update

```bash
cd /opt/MTProto-ui
git pull
docker compose up -d --build
```

## Tech Stack

<table>
<tr><td><b>Backend</b></td><td>Go, Gin, GORM, SQLite</td></tr>
<tr><td><b>Frontend</b></td><td>HTML, CSS, Vanilla JS</td></tr>
<tr><td><b>Auth</b></td><td>JWT (HS256), bcrypt</td></tr>
<tr><td><b>Bot</b></td><td>Python, aiogram v3</td></tr>
<tr><td><b>SSL</b></td><td>Let's Encrypt (autocert)</td></tr>
<tr><td><b>Proxy</b></td><td>Docker containers</td></tr>
</table>

## Credits

- [3x-ui](https://github.com/MHSanaei/3x-ui) — panel design inspiration
- [telegrammessenger/proxy](https://github.com/TelegramMessenger/MTProxy) — official MTProto proxy (C)
- [telemt-docker](https://gitlab.com/An0nX/telemt-docker) — Rust MTProto proxy engine

## License

[MIT](LICENSE)
