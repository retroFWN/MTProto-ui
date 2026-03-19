"""Admin-only commands (restricted by Telegram user ID)."""

from aiogram import Router, types
from aiogram.filters import Command

from api import panel
from config import cfg

router = Router()


def is_admin(user_id: int) -> bool:
    return user_id in cfg.admin_ids


@router.message(Command("addclient"))
async def cmd_add_client(msg: types.Message) -> None:
    if not is_admin(msg.from_user.id):
        await msg.answer("⛔ Нет доступа.")
        return

    args = (msg.text or "").split()
    if len(args) < 3:
        await msg.answer(
            "Использование: /addclient &lt;proxy_id&gt; &lt;name&gt; [traffic_gb] [expiry_days]",
            parse_mode="HTML",
        )
        return

    try:
        proxy_id = int(args[1])
    except ValueError:
        await msg.answer("Неверный proxy_id.")
        return

    name = args[2]
    traffic_limit = 0
    expiry_days = 0

    if len(args) >= 4:
        try:
            traffic_limit = int(float(args[3]) * 1024 * 1024 * 1024)  # GB → bytes
        except ValueError:
            pass

    if len(args) >= 5:
        try:
            expiry_days = int(args[4])
        except ValueError:
            pass

    try:
        result = await panel.create_client(proxy_id, name, traffic_limit, expiry_days)
        secret = result.get("secret", "—")
        cid = result.get("ID") or result.get("id")
        await msg.answer(
            f"✅ Клиент создан!\n\n"
            f"ID: <b>{cid}</b>\n"
            f"Имя: <b>{name}</b>\n"
            f"Secret: <code>{secret}</code>",
            parse_mode="HTML",
        )
    except Exception as e:
        await msg.answer(f"Ошибка: {e}")


@router.message(Command("delclient"))
async def cmd_del_client(msg: types.Message) -> None:
    if not is_admin(msg.from_user.id):
        await msg.answer("⛔ Нет доступа.")
        return

    args = (msg.text or "").split()
    if len(args) < 3:
        await msg.answer(
            "Использование: /delclient &lt;proxy_id&gt; &lt;client_id&gt;",
            parse_mode="HTML",
        )
        return

    try:
        proxy_id = int(args[1])
        client_id = int(args[2])
    except ValueError:
        await msg.answer("Неверный ID.")
        return

    try:
        await panel.delete_client(proxy_id, client_id)
        await msg.answer(f"✅ Клиент #{client_id} удалён.")
    except Exception as e:
        await msg.answer(f"Ошибка: {e}")


@router.message(Command("resettraffic"))
async def cmd_reset_traffic(msg: types.Message) -> None:
    if not is_admin(msg.from_user.id):
        await msg.answer("⛔ Нет доступа.")
        return

    args = (msg.text or "").split()
    if len(args) < 3:
        await msg.answer(
            "Использование: /resettraffic &lt;proxy_id&gt; &lt;client_id&gt;",
            parse_mode="HTML",
        )
        return

    try:
        proxy_id = int(args[1])
        client_id = int(args[2])
    except ValueError:
        await msg.answer("Неверный ID.")
        return

    try:
        await panel.reset_traffic(proxy_id, client_id)
        await msg.answer(f"✅ Трафик клиента #{client_id} сброшен.")
    except Exception as e:
        await msg.answer(f"Ошибка: {e}")
