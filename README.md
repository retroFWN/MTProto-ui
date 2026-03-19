# MTProxy Panel

Web-панель для управления MTProto прокси-серверами Telegram. Вдохновлена [3x-ui](https://github.com/MHSanaei/3x-ui), но заточена под MTProto.

## Возможности

- **Dashboard** — мониторинг CPU, RAM, диска, сети в реальном времени
- **Мульти-прокси** — несколько прокси-инстансов на разных портах
- **Мульти-клиент** — несколько секретов (пользователей) на один прокси
- **Fake TLS** — маскировка трафика под HTTPS (`ee` prefix)
- **Лимиты** — трафик и срок действия на клиента
- **Авторизация** — JWT + bcrypt, смена пароля
- **Docker** — каждый прокси = контейнер `telegrammessenger/proxy`
- **Ссылки** — автогенерация `tg://proxy?...` для подключения в один клик
- **Один бинарник** — Go, ~25 MB, zero dependencies

## Быстрый старт

### Установка на VPS (Ubuntu 20.04+)

```bash
git clone https://github.com/YOUR_USER/mtproto.git /opt/mtproxy-panel
cd /opt/mtproxy-panel
sudo bash scripts/install.sh
```

Панель будет доступна на `http://SERVER_IP:8080`.

### Docker Compose

```bash
git clone https://github.com/YOUR_USER/mtproto.git
cd mtproto
docker compose up -d --build
```

### Локальная разработка

Требуется [Go 1.23+](https://go.dev/dl/) и Docker.

```bash
go mod tidy
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

## Стек

| Компонент | Технология |
|-----------|-----------|
| Backend | Go, Gin |
| Database | SQLite (GORM) |
| Auth | JWT (HS256), bcrypt |
| Frontend | HTML, Tailwind CSS, vanilla JS |
| Proxy | Docker, `telegrammessenger/proxy` |
| Метрики | gopsutil |

## Структура проекта

```
mtproto/
├── main.go                 # Точка входа
├── config/config.go        # Конфигурация
├── database/database.go    # Модели + SQLite
├── auth/auth.go            # JWT + bcrypt
├── proxy/manager.go        # Docker-управление контейнерами
├── web/
│   ├── router.go           # Маршруты Gin
│   ├── middleware.go        # Auth middleware
│   └── handlers.go         # API-хэндлеры
├── templates/              # HTML-шаблоны
├── static/                 # CSS, JS
├── Dockerfile
├── docker-compose.yml
└── scripts/install.sh
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
| GET | `/api/proxies/:id/logs` | Логи контейнера |

### Clients
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/proxies/:id/clients` | Список клиентов прокси |
| POST | `/api/proxies/:id/clients` | Добавить клиента |
| PUT | `/api/proxies/:id/clients/:cid` | Обновить клиента |
| DELETE | `/api/proxies/:id/clients/:cid` | Удалить клиента |
| POST | `/api/proxies/:id/clients/:cid/reset-traffic` | Сбросить трафик |

### Settings
| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/settings` | Получить настройки |
| POST | `/api/settings` | Обновить настройки |
| POST | `/api/pull-image` | Обновить Docker-образ прокси |

## Переменные окружения

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `PANEL_HOST` | `0.0.0.0` | Хост панели |
| `PANEL_PORT` | `8080` | Порт панели |
| `SECRET_KEY` | auto | Ключ JWT (сохраняется в `data/.secret_key`) |

## Лицензия

MIT
