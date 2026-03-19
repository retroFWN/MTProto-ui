# MTProxy Panel

A web panel for managing Telegram MTProto proxy servers. Inspired by [3x-ui](https://github.com/MHSanaei/3x-ui), built specifically for MTProto.

## Features

- **Dashboard** — real-time CPU, RAM, disk, and network monitoring
- **Multi-proxy** — multiple proxy instances on different ports
- **Multi-client** — multiple secrets (users) per proxy
- **Fake TLS** — traffic disguised as HTTPS (`ee` prefix)
- **Limits** — per-client traffic caps and expiry dates
- **Authentication** — JWT + bcrypt, password management
- **Docker** — each proxy is a `telegrammessenger/proxy` container
- **Links** — auto-generated `tg://proxy?...` one-click connection links
- **Single binary** — Go, ~25 MB, zero dependencies

## Quick Start

### VPS Installation (Ubuntu 20.04+)

```bash
git clone https://github.com/YOUR_USER/mtproto.git /opt/mtproxy-panel
cd /opt/mtproxy-panel
sudo bash scripts/install.sh
```

The panel will be available at `http://SERVER_IP:8080`.

### Docker Compose

```bash
git clone https://github.com/YOUR_USER/mtproto.git
cd mtproto
docker compose up -d --build
```

### Local Development

Requires [Go 1.23+](https://go.dev/dl/) and Docker.

```bash
go mod tidy
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

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go, Gin |
| Database | SQLite (GORM) |
| Auth | JWT (HS256), bcrypt |
| Frontend | HTML, Tailwind CSS, vanilla JS |
| Proxy | Docker, `telegrammessenger/proxy` |
| Metrics | gopsutil |

## Project Structure

```
mtproto/
├── main.go                 # Entry point
├── config/config.go        # Configuration
├── database/database.go    # Models + SQLite
├── auth/auth.go            # JWT + bcrypt
├── proxy/manager.go        # Docker container management
├── web/
│   ├── router.go           # Gin routes
│   ├── middleware.go        # Auth middleware
│   └── handlers.go         # API handlers
├── templates/              # HTML templates
├── static/                 # CSS, JS
├── Dockerfile
├── docker-compose.yml
└── scripts/install.sh
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
| GET | `/api/proxies/:id/logs` | Container logs |

### Clients
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/proxies/:id/clients` | List proxy clients |
| POST | `/api/proxies/:id/clients` | Add client |
| PUT | `/api/proxies/:id/clients/:cid` | Update client |
| DELETE | `/api/proxies/:id/clients/:cid` | Delete client |
| POST | `/api/proxies/:id/clients/:cid/reset-traffic` | Reset traffic |

### Settings
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/settings` | Get settings |
| POST | `/api/settings` | Update settings |
| POST | `/api/pull-image` | Pull latest proxy Docker image |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PANEL_HOST` | `0.0.0.0` | Panel host |
| `PANEL_PORT` | `8080` | Panel port |
| `SECRET_KEY` | auto | JWT key (persisted to `data/.secret_key`) |

## License

MIT
