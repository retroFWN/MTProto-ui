"""User-facing proxy commands."""

from aiogram import Router, types
from aiogram.filters import Command

from api import panel

router = Router()


def format_bytes(b: int) -> str:
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if abs(b) < 1024:
            return f"{b:.1f} {unit}"
        b /= 1024  # type: ignore[assignment]
    return f"{b:.1f} PB"


@router.message(Command("proxies"))
async def cmd_proxies(msg: types.Message) -> None:
    try:
        proxies = await panel.list_proxies()
    except Exception as e:
        await msg.answer(f"Ошибка: {e}")
        return

    if not proxies:
        await msg.answer("Нет прокси-серверов.")
        return

    lines = ["<b>Прокси-серверы:</b>\n"]
    for p in proxies:
        status = "🟢" if p.get("status") == "running" else "🔴"
        name = p.get("name", "—")
        pid = p.get("id")
        port = p.get("port", "?")
        clients = p.get("client_count", 0)
        lines.append(f"{status} <b>{name}</b> (ID: {pid}) — :{port}, клиентов: {clients}")

    await msg.answer("\n".join(lines), parse_mode="HTML")


@router.message(Command("connect"))
async def cmd_connect(msg: types.Message) -> None:
    args = (msg.text or "").split()
    if len(args) < 2:
        await msg.answer("Использование: /connect &lt;proxy_id&gt;", parse_mode="HTML")
        return

    try:
        proxy_id = int(args[1])
    except ValueError:
        await msg.answer("Неверный proxy_id.")
        return

    try:
        clients = await panel.list_clients(proxy_id)
    except Exception as e:
        await msg.answer(f"Ошибка: {e}")
        return

    if not clients:
        await msg.answer("У этого прокси нет клиентов.")
        return

    lines = [f"<b>Ссылки для прокси #{proxy_id}:</b>\n"]
    for c in clients:
        name = c.get("name", "—")
        enabled = c.get("enabled", True)
        tg_link = c.get("tg_link", "")
        if not enabled:
            lines.append(f"⛔ <s>{name}</s> — отключен")
            continue
        lines.append(f"✅ <b>{name}</b>\n<code>{tg_link}</code>")

    await msg.answer("\n".join(lines), parse_mode="HTML")


@router.message(Command("status"))
async def cmd_status(msg: types.Message) -> None:
    args = (msg.text or "").split()
    if len(args) < 2:
        await msg.answer("Использование: /status &lt;proxy_id&gt;", parse_mode="HTML")
        return

    try:
        proxy_id = int(args[1])
    except ValueError:
        await msg.answer("Неверный proxy_id.")
        return

    try:
        clients = await panel.list_clients(proxy_id)
    except Exception as e:
        await msg.answer(f"Ошибка: {e}")
        return

    if not clients:
        await msg.answer("Нет клиентов.")
        return

    lines = [f"<b>Клиенты прокси #{proxy_id}:</b>\n"]
    for c in clients:
        name = c.get("name", "—")
        cid = c.get("id")
        enabled = "✅" if c.get("enabled", True) else "⛔"
        up = format_bytes(c.get("traffic_up", 0))
        down = format_bytes(c.get("traffic_down", 0))
        limit = c.get("traffic_limit", 0)
        limit_str = format_bytes(limit) if limit > 0 else "∞"
        expiry = c.get("expiry_time", 0)
        if expiry and expiry > 0:
            from datetime import datetime
            expiry_str = datetime.fromtimestamp(expiry).strftime("%Y-%m-%d")
        else:
            expiry_str = "—"
        lines.append(
            f"{enabled} <b>{name}</b> (ID: {cid})\n"
            f"   ↑ {up}  ↓ {down}  лимит: {limit_str}\n"
            f"   истекает: {expiry_str}"
        )

    await msg.answer("\n".join(lines), parse_mode="HTML")


@router.message(Command("traffic"))
async def cmd_traffic(msg: types.Message) -> None:
    args = (msg.text or "").split()
    if len(args) < 2:
        await msg.answer("Использование: /traffic &lt;proxy_id&gt;", parse_mode="HTML")
        return

    try:
        proxy_id = int(args[1])
    except ValueError:
        await msg.answer("Неверный proxy_id.")
        return

    try:
        clients = await panel.list_clients(proxy_id)
    except Exception as e:
        await msg.answer(f"Ошибка: {e}")
        return

    total_up = sum(c.get("traffic_up", 0) for c in clients)
    total_down = sum(c.get("traffic_down", 0) for c in clients)

    lines = [
        f"<b>Трафик прокси #{proxy_id}:</b>\n",
        f"Всего ↑ {format_bytes(total_up)}  ↓ {format_bytes(total_down)}\n",
    ]
    for c in clients:
        name = c.get("name", "—")
        up = format_bytes(c.get("traffic_up", 0))
        down = format_bytes(c.get("traffic_down", 0))
        lines.append(f"  • {name}: ↑ {up}  ↓ {down}")

    await msg.answer("\n".join(lines), parse_mode="HTML")
