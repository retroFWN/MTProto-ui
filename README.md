# MTProxy Panel

Web-панель для управления MTProto прокси-серверами Telegram. Вдохновлена [3x-ui](https://github.com/MHSanaei/3x-ui), но заточена под MTProto.

## Возможности

- **Dashboard** — мониторинг CPU, RAM, диска, сети в реальном времени
- **Мульти-прокси** — несколько прокси-инстансов на разных портах
- **Мульти-клиент** — несколько секретов (пользователей) на один прокси
- **Выбор движка** — Official C (`telegrammessenger/proxy`) или Rust (`seriyps/mtproto-proxy`)
- **Fake TLS** — маскировка трафика под HTTPS
- **Лимиты** — трафик и срок действия на клиента
- **Авторизация** — JWT + bcrypt, смена пароля
- **Auto-SSL** — автоматический сертификат Let's Encrypt при указании домена
- **Telegram Bot** — встроенный бот на aiogram для управления прокси из Telegram
- **Docker** — каждый прокси = контейнер, управляемый из панели
- **Ссылки** — автогенерация `tg://proxy?...` для подключения в один клик
- **Один бинарник** — Go, ~25 MB, zero dependencies

## Быстрый старт

### Docker Compose (рекомендуется)

```bash
git clone https://github.com/YOUR_USER/mtproto.git
cd mtproto
docker compose up -d --build
```

### Локальная разработка

Требуется [Go 1.23+](https://go.dev/dl/), Docker, Python 3.10+ (для бота).

```bash
go mod tidy
pip install -r bot/requirements.txt
go run .
```

Панель запустится на `http://localhost:8080`.

## Учётные данные по умолчанию

| | |
|---|---|
| URL | `http://SERVER_IP:8080` |
| Login | `admin` |
| Password | `admin` |

**Смените пароль после первого входа!**

## Выбор прокси-движка

В настройках панели можно выбрать один из двух движков:

| | Official (C) | telemt (Rust) |
|---|---|---|
| Образ | `telegrammessenger/proxy` | `seriyps/mtproto-proxy` |
| Fake TLS | v1 | v2 |
| Per-user метрики | нет | Prometheus |
| Мульти-секрет | ограничено | полноценно |
| Management API | нет | HTTP API |
| Готовность | stable | stable |

## Auto-SSL (Let's Encrypt)

Для автоматического HTTPS-сертификата:

1. Укажите домен в **Settings > SSL Certificate** (например `panel.example.com`)
2. Откройте порты **80** и **443** на сервере
3. Перезапустите панель

Или задайте через переменную окружения:
```bash
PANEL_DOMAIN=panel.example.com
```

Панель автоматически получит и будет обновлять сертификат от Let's Encrypt.

## Telegram Bot

Встроенный Telegram-бот для управления прокси прямо из мессенджера. Бот запускается из панели как subprocess — отдельная настройка не требуется.

### Настройка

1. Создайте бота через [@BotFather](https://t.me/BotFather)
2. В **Settings > Telegram Bot** вставьте токен и ваш Telegram ID
3. Нажмите **Save**, затем **Start Bot**

### Команды бота

| Команда | Описание | Доступ |
|---------|----------|--------|
| `/start` | Приветствие и справка | Все |
| `/help` | Список команд | Все |
| `/proxies` | Список прокси-серверов | Все |
| `/connect <id>` | Получить tg:// ссылки подключения | Все |
| `/status <id>` | Статус и клиенты прокси | Все |
| `/traffic <id>` | Статистика трафика | Все |
| `/addclient <pid> <name> [gb] [days]` | Создать клиента | Админ |
| `/delclient <pid> <cid>` | Удалить клиента | Админ |
| `/resettraffic <pid> <cid>` | Сбросить трафик клиента | Админ |

Админ-команды доступны только пользователям, чей Telegram ID указан в настройках.

## Стек

| Компонент | Технология |
|-----------|-----------|
| Backend | Go, Gin |
| Database | SQLite (GORM) |
| Auth | JWT (HS256), bcrypt |
| Frontend | HTML, CSS, vanilla JS |
| Proxy | Docker, Official C / telemt Rust |
| SSL | Let's Encrypt (autocert) |
| Telegram Bot | Python, aiogram v3 |
| Метрики | gopsutil |

## Структура проекта

```
mtproto/
├── main.go                 # Точка входа
├── config/config.go        # Конфигурация
├── database/database.go    # Модели + SQLite
├── auth/auth.go            # JWT + bcrypt
├── proxy/
│   ├── manager.go          # Docker-управление контейнерами
│   ├── backend.go          # Backend-интерфейс + реестр
│   ├── official.go         # Official C backend
│   └── telemt.go           # telemt Rust backend
├── botmanager/
│   └── botmanager.go       # Управление процессом бота
├── web/
│   ├── router.go           # Маршруты Gin
│   ├── middleware.go       # Auth middleware (Page, API, Bot)
│   └── handlers.go         # API-хэндлеры
├── bot/                    # Telegram бот (Python, aiogram)
│   ├── main.py             # Точка входа бота
│   ├── config.py           # Конфигурация из env
│   ├── api.py              # HTTP-клиент к панели
│   └── handlers/           # Команды бота
│       ├── start.py        # /start, /help
│       ├── proxy.py        # /proxies, /connect, /status, /traffic
│       └── admin.py        # /addclient, /delclient, /resettraffic
├── templates/              # HTML-шаблоны
├── static/                 # CSS, JS
├── Dockerfile
└── docker-compose.yml
```

## API

Все эндпоинты под `/api/` требуют авторизации (cookie или `Authorization: Bearer <token>`).

### Auth
| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/login` | Вход (username, password) |
| POST | `/api/logout` | Выход |
| POST | `/api/change-password` | Смена пароля |

### System
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/system/status` | CPU, RAM, Disk, Network, Uptime |

### Proxies
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/proxies` | Список всех прокси |
| POST | `/api/proxies` | Создать прокси |
| PUT | `/api/proxies/:id` | Обновить прокси |
| DELETE | `/api/proxies/:id` | Удалить прокси |
| POST | `/api/proxies/:id/start` | Запустить контейнер |
| POST | `/api/proxies/:id/stop` | Остановить контейнер |
| POST | `/api/proxies/:id/restart` | Перезапустить контейнер |
| GET | `/api/proxies/:id/stats` | CPU/RAM/Net контейнера |
| GET | `/api/proxies/:id/live` | Per-user live данные (telemt) |
| GET | `/api/proxies/:id/logs` | Логи контейнера |

### Clients
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/proxies/:id/clients` | Список клиентов прокси |
| POST | `/api/proxies/:id/clients` | Добавить клиента |
| PUT | `/api/proxies/:id/clients/:cid` | Обновить клиента |
| DELETE | `/api/proxies/:id/clients/:cid` | Удалить клиента |
| POST | `/api/proxies/:id/clients/:cid/reset-traffic` | Сбросить трафик |

### Settings & Bot
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/settings` | Получить настройки |
| POST | `/api/settings` | Обновить настройки |
| POST | `/api/pull-image` | Обновить Docker-образ прокси |
| GET | `/api/backends` | Список доступных движков |
| GET | `/api/bot/status` | Статус Telegram-бота |
| POST | `/api/bot/start` | Запустить бота |
| POST | `/api/bot/stop` | Остановить бота |

### Bot Internal API
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/bot/api/proxies` | Список прокси (для бота) |
| GET | `/bot/api/proxies/:id/clients` | Клиенты прокси (для бота) |
| POST | `/bot/api/proxies/:id/clients` | Создать клиента (для бота) |
| DELETE | `/bot/api/proxies/:id/clients/:cid` | Удалить клиента (для бота) |
| GET | `/bot/api/settings` | Настройки (для бота) |

Авторизация: заголовок `X-Bot-Token` с SECRET_KEY панели.

## Переменные окружения

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `PANEL_HOST` | `0.0.0.0` | Хост панели |
| `PANEL_PORT` | `8080` | Порт панели |
| `PANEL_DOMAIN` | — | Домен для auto-SSL (Let's Encrypt) |
| `SECRET_KEY` | auto | Ключ JWT (сохраняется в `data/.secret_key`) |
| `PROXY_BACKEND` | `official` | Движок по умолчанию (`official` или `telemt`) |

## Ссылки и источники

### Проект
- [3x-ui](https://github.com/MHSanaei/3x-ui) — вдохновение для панели (Xray + Go + Gin)
- [MTProxyMax](https://github.com/SamNet-dev/MTProxyMax) — менеджер MTProto на базе telemt

### Прокси-движки
- [telegrammessenger/proxy](https://hub.docker.com/r/telegrammessenger/proxy/) — официальный MTProto прокси (C)
- [telemt/telemt](https://github.com/telemt/telemt) — MTProxy на Rust + Tokio (исходники)
- [An0nX/telemt-docker](https://github.com/An0nX/telemt-docker) — Docker-образ для telemt
- [whn0thacked/telemt-docker](https://hub.docker.com/r/whn0thacked/telemt-docker) — Docker Hub образ telemt
- [Документация telemt API](https://github.com/telemt/telemt/blob/main/docs/API.md)
- [Параметры конфигурации telemt](https://github.com/telemt/telemt/blob/main/docs/CONFIG_PARAMS.en.md)

### Стек
- [Go](https://go.dev/) — язык панели
- [Gin](https://github.com/gin-gonic/gin) — HTTP-фреймворк
- [GORM](https://gorm.io/) — ORM для SQLite
- [aiogram](https://docs.aiogram.dev/) — Telegram Bot Framework (Python)
- [Let's Encrypt](https://letsencrypt.org/) — Auto-SSL

## Лицензия

MIT
