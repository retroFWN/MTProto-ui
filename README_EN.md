# MTProxy Panel

A web panel for managing Telegram MTProto proxy servers. Inspired by [3x-ui](https://github.com/MHSanaei/3x-ui), built specifically for MTProto.

## Features

- **Dashboard** — real-time CPU, RAM, disk, and network monitoring
- **Multi-proxy** — multiple proxy instances on different ports
- **Multi-client** — multiple secrets (users) per proxy
- **Engine selection** — Official C (`telegrammessenger/proxy`) or Rust (`seriyps/mtproto-proxy`)
- **Fake TLS** — traffic disguised as HTTPS
- **Limits** — per-client traffic caps and expiry dates
- **Authentication** — JWT + bcrypt, password management
- **Auto-SSL** — automatic Let's Encrypt certificate when a domain is configured
- **Telegram Bot** — built-in aiogram bot for managing proxies from Telegram
- **Docker** — each proxy is a container managed from the panel
- **Links** — auto-generated `tg://proxy?...` one-click connection links
- **Single binary** — Go, ~25 MB, zero dependencies

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/YOUR_USER/mtproto.git
cd mtproto
docker compose up -d --build
```

### Local Development

Requires [Go 1.23+](https://go.dev/dl/), Docker, Python 3.10+ (for bot).

```bash
go mod tidy
pip install -r bot/requirements.txt
go run .
```

The panel starts at `http://localhost:8080`.

## Default Credentials

| | |
|---|---|
| URL | `http://SERVER_IP:8080` |
| Login | `admin` |
| Password | `admin` |

**Change the default password after first login!**

## Proxy Engine Selection

Choose between two MTProto proxy engines in the Settings page:

| | Official (C) | telemt (Rust) |
|---|---|---|
| Image | `telegrammessenger/proxy` | `seriyps/mtproto-proxy` |
| Fake TLS | v1 | v2 |
| Per-user metrics | no | Prometheus |
| Multi-secret | limited | full |
| Management API | no | HTTP API |
| Stability | stable | stable |

## Auto-SSL (Let's Encrypt)

To enable automatic HTTPS:

1. Set your domain in **Settings > SSL Certificate** (e.g. `panel.example.com`)
2. Open ports **80** and **443** on your server
3. Restart the panel

Or set via environment variable:
```bash
PANEL_DOMAIN=panel.example.com
```

The panel will automatically obtain and renew a Let's Encrypt certificate.

## Telegram Bot

Built-in Telegram bot for proxy management directly from the messenger. The bot runs as a subprocess launched by the panel — no separate setup required.

### Setup

1. Create a bot via [@BotFather](https://t.me/BotFather)
2. Go to **Settings > Telegram Bot**, paste the token and your Telegram user ID
3. Click **Save**, then **Start Bot**

### Bot Commands

| Command | Description | Access |
|---------|-------------|--------|
| `/start` | Welcome message and help | All |
| `/help` | List available commands | All |
| `/proxies` | List proxy servers | All |
| `/connect <id>` | Get tg:// connection links | All |
| `/status <id>` | Proxy status and clients | All |
| `/traffic <id>` | Traffic statistics | All |
| `/addclient <pid> <name> [gb] [days]` | Create client | Admin |
| `/delclient <pid> <cid>` | Delete client | Admin |
| `/resettraffic <pid> <cid>` | Reset client traffic | Admin |

Admin commands are restricted to users whose Telegram IDs are listed in settings.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go, Gin |
| Database | SQLite (GORM) |
| Auth | JWT (HS256), bcrypt |
| Frontend | HTML, CSS, vanilla JS |
| Proxy | Docker, Official C / telemt Rust |
| SSL | Let's Encrypt (autocert) |
| Telegram Bot | Python, aiogram v3 |
| Metrics | gopsutil |

## Project Structure

```
mtproto/
├── main.go                 # Entry point
├── config/config.go        # Configuration
├── database/database.go    # Models + SQLite
├── auth/auth.go            # JWT + bcrypt
├── proxy/
│   ├── manager.go          # Docker container management
│   ├── backend.go          # Backend interface + registry
│   ├── official.go         # Official C backend
│   └── telemt.go           # telemt Rust backend
├── botmanager/
│   └── botmanager.go       # Bot process management
├── web/
│   ├── router.go           # Gin routes
│   ├── middleware.go       # Auth middleware (Page, API, Bot)
│   └── handlers.go         # API handlers
├── bot/                    # Telegram bot (Python, aiogram)
│   ├── main.py             # Bot entry point
│   ├── config.py           # Configuration from env
│   ├── api.py              # Panel HTTP client
│   └── handlers/           # Bot commands
│       ├── start.py        # /start, /help
│       ├── proxy.py        # /proxies, /connect, /status, /traffic
│       └── admin.py        # /addclient, /delclient, /resettraffic
├── templates/              # HTML templates
├── static/                 # CSS, JS
├── Dockerfile
└── docker-compose.yml
```

## API

All `/api/` endpoints require authentication (cookie or `Authorization: Bearer <token>`).

### Auth
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/login` | Sign in (username, password) |
| POST | `/api/logout` | Sign out |
| POST | `/api/change-password` | Change password |

### System
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/system/status` | CPU, RAM, Disk, Network, Uptime |

### Proxies
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/proxies` | List all proxies |
| POST | `/api/proxies` | Create proxy |
| PUT | `/api/proxies/:id` | Update proxy |
| DELETE | `/api/proxies/:id` | Delete proxy |
| POST | `/api/proxies/:id/start` | Start container |
| POST | `/api/proxies/:id/stop` | Stop container |
| POST | `/api/proxies/:id/restart` | Restart container |
| GET | `/api/proxies/:id/stats` | Container CPU/RAM/Net |
| GET | `/api/proxies/:id/live` | Per-user live stats (telemt) |
| GET | `/api/proxies/:id/logs` | Container logs |

### Clients
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/proxies/:id/clients` | List proxy clients |
| POST | `/api/proxies/:id/clients` | Add client |
| PUT | `/api/proxies/:id/clients/:cid` | Update client |
| DELETE | `/api/proxies/:id/clients/:cid` | Delete client |
| POST | `/api/proxies/:id/clients/:cid/reset-traffic` | Reset traffic |

### Settings & Bot
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/settings` | Get settings |
| POST | `/api/settings` | Update settings |
| POST | `/api/pull-image` | Pull latest proxy Docker image |
| GET | `/api/backends` | List available engines |
| GET | `/api/bot/status` | Telegram bot status |
| POST | `/api/bot/start` | Start bot |
| POST | `/api/bot/stop` | Stop bot |

### Bot Internal API
| Method | Path | Description |
|--------|------|-------------|
| GET | `/bot/api/proxies` | List proxies (for bot) |
| GET | `/bot/api/proxies/:id/clients` | Proxy clients (for bot) |
| POST | `/bot/api/proxies/:id/clients` | Create client (for bot) |
| DELETE | `/bot/api/proxies/:id/clients/:cid` | Delete client (for bot) |
| GET | `/bot/api/settings` | Settings (for bot) |

Authorization: `X-Bot-Token` header with the panel's SECRET_KEY.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PANEL_HOST` | `0.0.0.0` | Panel host |
| `PANEL_PORT` | `8080` | Panel port |
| `PANEL_DOMAIN` | — | Domain for auto-SSL (Let's Encrypt) |
| `SECRET_KEY` | auto | JWT key (persisted to `data/.secret_key`) |
| `PROXY_BACKEND` | `official` | Default engine (`official` or `telemt`) |

## Links & Sources

### Project
- [3x-ui](https://github.com/MHSanaei/3x-ui) — inspiration for the panel (Xray + Go + Gin)
- [MTProxyMax](https://github.com/SamNet-dev/MTProxyMax) — MTProto manager built on telemt

### Proxy Engines
- [telegrammessenger/proxy](https://hub.docker.com/r/telegrammessenger/proxy/) — official MTProto proxy (C)
- [telemt/telemt](https://github.com/telemt/telemt) — MTProxy on Rust + Tokio (source code)
- [An0nX/telemt-docker](https://github.com/An0nX/telemt-docker) — Docker image for telemt
- [whn0thacked/telemt-docker](https://hub.docker.com/r/whn0thacked/telemt-docker) — Docker Hub image for telemt
- [telemt API docs](https://github.com/telemt/telemt/blob/main/docs/API.md)
- [telemt config reference](https://github.com/telemt/telemt/blob/main/docs/CONFIG_PARAMS.en.md)

### Tech Stack
- [Go](https://go.dev/) — panel language
- [Gin](https://github.com/gin-gonic/gin) — HTTP framework
- [GORM](https://gorm.io/) — SQLite ORM
- [aiogram](https://docs.aiogram.dev/) — Telegram Bot Framework (Python)
- [Let's Encrypt](https://letsencrypt.org/) — Auto-SSL

## License

MIT
