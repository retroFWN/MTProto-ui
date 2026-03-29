"""Client management via inline keyboards + FSM for adding clients."""

from datetime import datetime

from aiogram import Router, types, F
from aiogram.fsm.context import FSMContext
from aiogram.fsm.state import StatesGroup, State
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


# ── Client List ───────────────────────────────────────────────────────────


@router.callback_query(F.data.startswith("clients:"))
async def cb_client_list(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return
    await cq.answer()

    proxy_id = int(cq.data.split(":")[1])

    try:
        clients = await panel.list_clients(proxy_id)
    except Exception as e:
        await cq.message.edit_text(f"❌ Ошибка: {e}")
        return

    if not clients:
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text="➕ Добавить клиента", callback_data=f"cl_add:{proxy_id}")],
            [InlineKeyboardButton(text="🔙 К прокси", callback_data=f"px:{proxy_id}")],
        ])
        await cq.message.edit_text("👥 Нет клиентов.", reply_markup=kb)
        return

    text = "👥 <b>Клиенты</b>\n"
    buttons = []
    for c in clients:
        icon = "✅" if c.get("enabled") else "⛔"
        name = c.get("name", "—")
        cid = c.get("id")
        total = format_bytes(c.get("traffic_up", 0) + c.get("traffic_down", 0))
        limit = c.get("traffic_limit", 0)
        traffic_str = total
        if limit > 0:
            traffic_str += f" / {format_bytes(limit)}"

        buttons.append([InlineKeyboardButton(
            text=f"{icon} {name} — {traffic_str}",
            callback_data=f"cl:{proxy_id}:{cid}",
        )])

    buttons.append([InlineKeyboardButton(text="➕ Добавить", callback_data=f"cl_add:{proxy_id}")])
    buttons.append([InlineKeyboardButton(text="🔙 К прокси", callback_data=f"px:{proxy_id}")])

    await cq.message.edit_text(text, reply_markup=InlineKeyboardMarkup(inline_keyboard=buttons))


# ── Client Detail ─────────────────────────────────────────────────────────


@router.callback_query(F.data.regexp(r"^cl:\d+:\d+$"))
async def cb_client_detail(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return
    await cq.answer()

    parts = cq.data.split(":")
    proxy_id = int(parts[1])
    client_id = int(parts[2])

    try:
        clients = await panel.list_clients(proxy_id)
    except Exception as e:
        await cq.message.edit_text(f"❌ Ошибка: {e}")
        return

    c = next((x for x in clients if x.get("id") == client_id), None)
    if not c:
        await cq.answer("Клиент не найден", show_alert=True)
        return

    enabled = c.get("enabled", True)
    status = "✅ Активен" if enabled else "⛔ Отключён"
    up = format_bytes(c.get("traffic_up", 0))
    down = format_bytes(c.get("traffic_down", 0))
    total = format_bytes(c.get("traffic_up", 0) + c.get("traffic_down", 0))
    limit = c.get("traffic_limit", 0)
    limit_str = format_bytes(limit) if limit > 0 else "∞"

    expiry = c.get("expiry_time", 0)
    if expiry and expiry > 0:
        now = int(datetime.now().timestamp())
        if expiry < now:
            expiry_str = "❌ Истёк"
        else:
            expiry_str = datetime.fromtimestamp(expiry).strftime("%d.%m.%Y")
    else:
        expiry_str = "Бессрочно"

    # Traffic progress bar (text-based)
    bar = ""
    if limit > 0:
        pct = min(100, round((c.get("traffic_up", 0) + c.get("traffic_down", 0)) / limit * 100))
        filled = pct // 10
        bar = f"\n{'█' * filled}{'░' * (10 - filled)} {pct}%"

    secret = c.get("secret", "—")
    tg_link = c.get("tg_link", "")

    text = (
        f"👤 <b>{c.get('name', '—')}</b>\n"
        f"Статус: {status}\n"
        f"Трафик: {total} / {limit_str}{bar}\n"
        f"↑ {up}  ↓ {down}\n"
        f"Срок: {expiry_str}\n\n"
        f"Секрет:\n<code>{secret}</code>\n\n"
        f"Ссылка:\n<code>{tg_link}</code>"
    )

    toggle_text = "⏸ Отключить" if enabled else "▶️ Включить"
    toggle_data = f"cl_toggle:{proxy_id}:{client_id}"

    kb = InlineKeyboardMarkup(inline_keyboard=[
        [
            InlineKeyboardButton(text="⟳ Сброс трафика", callback_data=f"cl_reset:{proxy_id}:{client_id}"),
        ],
        [
            InlineKeyboardButton(text=toggle_text, callback_data=toggle_data),
            InlineKeyboardButton(text="🗑 Удалить", callback_data=f"cl_del:{proxy_id}:{client_id}"),
        ],
        [InlineKeyboardButton(text="🔙 К клиентам", callback_data=f"clients:{proxy_id}")],
    ])

    await cq.message.edit_text(text, reply_markup=kb)


# ── Client Actions ────────────────────────────────────────────────────────


@router.callback_query(F.data.startswith("cl_toggle:"))
async def cb_client_toggle(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return

    parts = cq.data.split(":")
    proxy_id, client_id = int(parts[1]), int(parts[2])

    # Get current state to determine toggle direction
    try:
        clients = await panel.list_clients(proxy_id)
        c = next((x for x in clients if x.get("id") == client_id), None)
        if not c:
            await cq.answer("Клиент не найден", show_alert=True)
            return
        new_state = not c.get("enabled", True)
        await panel.update_client(proxy_id, client_id, enabled=new_state)
        await cq.answer("✅ Включён" if new_state else "⏸ Отключён")
    except Exception as e:
        await cq.answer(f"❌ {e}", show_alert=True)
        return

    # Refresh detail
    cq.data = f"cl:{proxy_id}:{client_id}"
    await cb_client_detail(cq)


@router.callback_query(F.data.startswith("cl_reset:"))
async def cb_client_reset(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return

    parts = cq.data.split(":")
    proxy_id, client_id = int(parts[1]), int(parts[2])

    try:
        await panel.reset_traffic(proxy_id, client_id)
        await cq.answer("✅ Трафик сброшен")
    except Exception as e:
        await cq.answer(f"❌ {e}", show_alert=True)
        return

    cq.data = f"cl:{proxy_id}:{client_id}"
    await cb_client_detail(cq)


@router.callback_query(F.data.startswith("cl_del:"))
async def cb_client_delete_confirm(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return
    await cq.answer()

    parts = cq.data.split(":")
    proxy_id, client_id = int(parts[1]), int(parts[2])

    kb = InlineKeyboardMarkup(inline_keyboard=[
        [
            InlineKeyboardButton(text="✅ Да, удалить", callback_data=f"cl_delok:{proxy_id}:{client_id}"),
            InlineKeyboardButton(text="❌ Отмена", callback_data=f"cl:{proxy_id}:{client_id}"),
        ],
    ])
    await cq.message.edit_text("🗑 <b>Удалить клиента?</b>\n\nЭто действие необратимо.", reply_markup=kb)


@router.callback_query(F.data.startswith("cl_delok:"))
async def cb_client_delete(cq: types.CallbackQuery) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return

    parts = cq.data.split(":")
    proxy_id, client_id = int(parts[1]), int(parts[2])

    try:
        await panel.delete_client(proxy_id, client_id)
        await cq.answer("✅ Удалён")
    except Exception as e:
        await cq.answer(f"❌ {e}", show_alert=True)
        return

    # Go back to client list
    cq.data = f"clients:{proxy_id}"
    await cb_client_list(cq)


# ── Add Client (FSM) ─────────────────────────────────────────────────────


class AddClient(StatesGroup):
    name = State()
    traffic_limit = State()
    expiry_days = State()


@router.callback_query(F.data.startswith("cl_add:"))
async def cb_add_client_start(cq: types.CallbackQuery, state: FSMContext) -> None:
    if not is_admin(cq.from_user.id):
        await cq.answer("⛔", show_alert=True)
        return
    await cq.answer()

    proxy_id = int(cq.data.split(":")[1])
    await state.set_state(AddClient.name)
    await state.update_data(proxy_id=proxy_id, msg_id=cq.message.message_id)

    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="❌ Отмена", callback_data=f"px:{proxy_id}")],
    ])
    await cq.message.edit_text(
        "➕ <b>Новый клиент</b>\n\n"
        "Шаг 1/3 — Введите <b>имя</b> клиента:",
        reply_markup=kb,
    )


@router.message(AddClient.name)
async def fsm_name(msg: types.Message, state: FSMContext) -> None:
    if not is_admin(msg.from_user.id):
        return

    name = msg.text.strip()
    if not name:
        await msg.answer("Имя не может быть пустым. Попробуйте ещё раз:")
        return

    await state.update_data(name=name)
    await state.set_state(AddClient.traffic_limit)

    # Delete user message to keep chat clean
    try:
        await msg.delete()
    except Exception:
        pass

    data = await state.get_data()
    proxy_id = data["proxy_id"]
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="0 — безлимит", callback_data="fsm_limit:0")],
        [InlineKeyboardButton(text="❌ Отмена", callback_data=f"px:{proxy_id}")],
    ])

    # Edit the original bot message
    try:
        await msg.bot.edit_message_text(
            f"➕ <b>Новый клиент:</b> {name}\n\n"
            "Шаг 2/3 — Лимит трафика (GB).\n"
            "Введите число или нажмите кнопку:",
            chat_id=msg.chat.id,
            message_id=data["msg_id"],
            reply_markup=kb,
        )
    except Exception:
        await msg.answer(
            f"Шаг 2/3 — Лимит трафика (GB, 0 = безлимит):",
            reply_markup=kb,
        )


@router.callback_query(F.data.startswith("fsm_limit:"))
async def fsm_limit_btn(cq: types.CallbackQuery, state: FSMContext) -> None:
    await cq.answer()
    value = int(cq.data.split(":")[1])
    await state.update_data(traffic_limit=value)
    await state.set_state(AddClient.expiry_days)

    data = await state.get_data()
    proxy_id = data["proxy_id"]
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="0 — бессрочно", callback_data="fsm_expiry:0")],
        [
            InlineKeyboardButton(text="30 дней", callback_data="fsm_expiry:30"),
            InlineKeyboardButton(text="90 дней", callback_data="fsm_expiry:90"),
        ],
        [InlineKeyboardButton(text="❌ Отмена", callback_data=f"px:{proxy_id}")],
    ])
    await cq.message.edit_text(
        f"➕ <b>Новый клиент:</b> {data['name']}\n"
        f"Лимит: {'безлимит' if value == 0 else f'{value} GB'}\n\n"
        "Шаг 3/3 — Срок действия (дни).\n"
        "Введите число или нажмите кнопку:",
        reply_markup=kb,
    )


@router.message(AddClient.traffic_limit)
async def fsm_traffic(msg: types.Message, state: FSMContext) -> None:
    if not is_admin(msg.from_user.id):
        return

    try:
        value = max(0, int(float(msg.text.strip())))
    except ValueError:
        await msg.answer("Введите число (GB). Например: 5")
        return

    try:
        await msg.delete()
    except Exception:
        pass

    await state.update_data(traffic_limit=value)
    await state.set_state(AddClient.expiry_days)

    data = await state.get_data()
    proxy_id = data["proxy_id"]
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="0 — бессрочно", callback_data="fsm_expiry:0")],
        [
            InlineKeyboardButton(text="30 дней", callback_data="fsm_expiry:30"),
            InlineKeyboardButton(text="90 дней", callback_data="fsm_expiry:90"),
        ],
        [InlineKeyboardButton(text="❌ Отмена", callback_data=f"px:{proxy_id}")],
    ])
    try:
        await msg.bot.edit_message_text(
            f"➕ <b>Новый клиент:</b> {data['name']}\n"
            f"Лимит: {'безлимит' if value == 0 else f'{value} GB'}\n\n"
            "Шаг 3/3 — Срок действия (дни).\n"
            "Введите число или нажмите кнопку:",
            chat_id=msg.chat.id,
            message_id=data["msg_id"],
            reply_markup=kb,
        )
    except Exception:
        await msg.answer("Шаг 3/3 — Срок действия (дни, 0 = бессрочно):", reply_markup=kb)


@router.callback_query(F.data.startswith("fsm_expiry:"))
async def fsm_expiry_btn(cq: types.CallbackQuery, state: FSMContext) -> None:
    await cq.answer()
    value = int(cq.data.split(":")[1])
    await _create_client(cq.message, state, value, cq.from_user.id)


@router.message(AddClient.expiry_days)
async def fsm_expiry(msg: types.Message, state: FSMContext) -> None:
    if not is_admin(msg.from_user.id):
        return

    try:
        value = max(0, int(msg.text.strip()))
    except ValueError:
        await msg.answer("Введите число дней. Например: 30")
        return

    try:
        await msg.delete()
    except Exception:
        pass

    await _create_client(msg, state, value, msg.from_user.id, edit_msg_id=(await state.get_data()).get("msg_id"))


async def _create_client(
    msg_or_target, state: FSMContext, expiry_days: int, user_id: int, edit_msg_id: int | None = None,
) -> None:
    data = await state.get_data()
    await state.clear()

    proxy_id = data["proxy_id"]
    name = data["name"]
    traffic_gb = data.get("traffic_limit", 0)
    traffic_bytes = int(traffic_gb * 1024 * 1024 * 1024) if traffic_gb > 0 else 0

    try:
        result = await panel.create_client(proxy_id, name, traffic_bytes, expiry_days)
    except Exception as e:
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text="🔙 К прокси", callback_data=f"px:{proxy_id}")],
        ])
        text = f"❌ Ошибка: {e}"
        if isinstance(msg_or_target, types.Message) and edit_msg_id:
            try:
                await msg_or_target.bot.edit_message_text(text, chat_id=msg_or_target.chat.id, message_id=edit_msg_id, reply_markup=kb)
            except Exception:
                await msg_or_target.answer(text, reply_markup=kb)
        elif isinstance(msg_or_target, types.Message):
            await msg_or_target.edit_text(text, reply_markup=kb)
        return

    secret = result.get("secret", "—")
    tg_link = result.get("tg_link", "")
    cid = result.get("id")
    limit_str = f"{traffic_gb} GB" if traffic_gb > 0 else "безлимит"
    expiry_str = f"{expiry_days} дней" if expiry_days > 0 else "бессрочно"

    text = (
        f"✅ <b>Клиент создан!</b>\n\n"
        f"Имя: <b>{name}</b>\n"
        f"Лимит: {limit_str}\n"
        f"Срок: {expiry_str}\n\n"
        f"Секрет:\n<code>{secret}</code>\n\n"
        f"Ссылка:\n<code>{tg_link}</code>"
    )

    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="👤 К клиенту", callback_data=f"cl:{proxy_id}:{cid}")],
        [InlineKeyboardButton(text="👥 Все клиенты", callback_data=f"clients:{proxy_id}")],
    ])

    if isinstance(msg_or_target, types.Message) and edit_msg_id:
        try:
            await msg_or_target.bot.edit_message_text(text, chat_id=msg_or_target.chat.id, message_id=edit_msg_id, reply_markup=kb)
            return
        except Exception:
            pass

    if isinstance(msg_or_target, types.Message):
        try:
            await msg_or_target.edit_text(text, reply_markup=kb)
        except Exception:
            await msg_or_target.answer(text, reply_markup=kb)
