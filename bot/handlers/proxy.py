"""Proxy list, detail and control via inline keyboards."""

from aiogram import Router, types, F
from aiogram.types import InlineKeyboardMarkup, InlineKeyboardButton

from api import panel
from config import cfg

router = Router()


def is_admin(user_id: int) -> bool:
    return user_id in cfg.admin_ids


def format_bytes(b: int) -> str:
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if abs(b) < 1024:
            return f"{b:.1f} {unit}" if b != int(b) else f"{int(b)} {unit}"
        b /= 1024
    return f"{b:.1f} PB"


# ── Proxy List ────────────────────────────────────────────────────────────


@router.callback_query(F.data == "proxies")
async def cb_proxy_list(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔ Нет доступа", show_alert=True)
        return
    await cq.answer()

    try:
        proxies = await panel.list_proxies()
    except Exception as e:
        await cq.message.edit_text(f"❌ Ошибка: {e}")
        return

    if not proxies:
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text="🔙 Меню", callback_data="menu")],
        ])
        await cq.message.edit_text("📡 Нет прокси-серверов.", reply_markup=kb)
        return

    text = "📡 <b>Прокси-серверы</b>\n"
    buttons = []
    for p in proxies:
        running = p.get("status", {}).get("running", False)
        icon = "🟢" if running else "🔴"
        name = p.get("name", "—")
        port = p.get("port", "?")
        pid = p.get("id")
        clients = p.get("client_count", 0)
        backend = "Rust" if p.get("backend") == "telemt" else "C"
        total = format_bytes(p.get("traffic_up", 0) + p.get("traffic_down", 0))

        text += f"\n{icon} <b>{name}</b> — :{port} ({backend})\n"
        text += f"    {total} | {clients} кл.\n"

        buttons.append([InlineKeyboardButton(
            text=f"{icon} {name} — :{port}",
            callback_data=f"px:{pid}",
        )])

    buttons.append([InlineKeyboardButton(text="🔙 Меню", callback_data="menu")])
    await cq.message.edit_text(text, reply_markup=InlineKeyboardMarkup(inline_keyboard=buttons))


# ── Proxy Detail ──────────────────────────────────────────────────────────


@router.callback_query(F.data.startswith("px:"))
async def cb_proxy_detail(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔ Нет доступа", show_alert=True)
        return
    await cq.answer()

    proxy_id = int(cq.data.split(":")[1])

    try:
        proxies = await panel.list_proxies()
    except Exception as e:
        await cq.message.edit_text(f"❌ Ошибка: {e}")
        return

    p = next((x for x in proxies if x.get("id") == proxy_id), None)
    if not p:
        await cq.message.edit_text("Прокси не найден.")
        return

    running = p.get("status", {}).get("running", False)
    status = "🟢 Работает" if running else "🔴 Остановлен"
    backend = "Rust" if p.get("backend") == "telemt" else "C"
    up = format_bytes(p.get("traffic_up", 0))
    down = format_bytes(p.get("traffic_down", 0))
    total = format_bytes(p.get("traffic_up", 0) + p.get("traffic_down", 0))
    limit = p.get("traffic_total_limit", 0)
    limit_str = format_bytes(limit) if limit > 0 else "∞"
    clients = p.get("client_count", 0)

    text = (
        f"📡 <b>{p.get('name', '—')}</b>\n"
        f"Port: {p.get('port')} | {p.get('fake_tls_domain', '')} | {backend}\n"
        f"{status}\n\n"
        f"↑ {up}  ↓ {down}  Σ {total}\n"
        f"Лимит: {limit_str}\n"
        f"Клиентов: {clients}"
    )

    row1 = [
        InlineKeyboardButton(text="👥 Клиенты", callback_data=f"clients:{proxy_id}"),
        InlineKeyboardButton(text="➕ Добавить", callback_data=f"cl_add:{proxy_id}"),
    ]

    if running:
        row2 = [
            InlineKeyboardButton(text="⟳ Рестарт", callback_data=f"px_restart:{proxy_id}"),
            InlineKeyboardButton(text="⏹ Стоп", callback_data=f"px_stop:{proxy_id}"),
        ]
    else:
        row2 = [
            InlineKeyboardButton(text="▶️ Запуск", callback_data=f"px_start:{proxy_id}"),
        ]

    row3 = [InlineKeyboardButton(text="🔙 К списку", callback_data="proxies")]

    kb = InlineKeyboardMarkup(inline_keyboard=[row1, row2, row3])
    await cq.message.edit_text(text, reply_markup=kb)


# ── Proxy Actions ─────────────────────────────────────────────────────────


@router.callback_query(F.data.startswith("px_start:"))
async def cb_proxy_start(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return
    proxy_id = int(cq.data.split(":")[1])
    try:
        await panel.start_proxy(proxy_id)
        await cq.answer("✅ Запущен")
    except Exception as e:
        await cq.answer(f"❌ {e}", show_alert=True)
        return
    # Refresh proxy detail
    cq.data = f"px:{proxy_id}"
    await cb_proxy_detail(cq)


@router.callback_query(F.data.startswith("px_stop:"))
async def cb_proxy_stop(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return
    proxy_id = int(cq.data.split(":")[1])
    try:
        await panel.stop_proxy(proxy_id)
        await cq.answer("✅ Остановлен")
    except Exception as e:
        await cq.answer(f"❌ {e}", show_alert=True)
        return
    cq.data = f"px:{proxy_id}"
    await cb_proxy_detail(cq)


@router.callback_query(F.data.startswith("px_restart:"))
async def cb_proxy_restart(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return
    proxy_id = int(cq.data.split(":")[1])
    try:
        await panel.restart_proxy(proxy_id)
        await cq.answer("✅ Перезапущен")
    except Exception as e:
        await cq.answer(f"❌ {e}", show_alert=True)
        return
    cq.data = f"px:{proxy_id}"
    await cb_proxy_detail(cq)
