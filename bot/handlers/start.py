"""Main menu and /start command."""

from aiogram import Router, types, F
from aiogram.filters import CommandStart, Command
from aiogram.types import InlineKeyboardMarkup, InlineKeyboardButton

from config import cfg

router = Router()


def is_admin(user_id: int) -> bool:
    return user_id in cfg.admin_ids


def main_menu_kb() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="📡 Прокси-серверы", callback_data="proxies")],
    ])


WELCOME = (
    "🔹 <b>MTProxy Panel Bot</b>\n\n"
    "Управление MTProto прокси-серверами\n"
    "прямо из Telegram."
)


@router.message(CommandStart())
async def cmd_start(msg: types.Message) -> None:
    if not is_admin(msg.from_user.id):
        await msg.answer("⛔ Нет доступа.")
        return
    await msg.answer(WELCOME, reply_markup=main_menu_kb())


@router.message(Command("help"))
async def cmd_help(msg: types.Message) -> None:
    await cmd_start(msg)


@router.callback_query(F.data == "menu")
async def cb_menu(cq: types.CallbackQuery) -> None:
    await cq.answer()
    await cq.message.edit_text(WELCOME, reply_markup=main_menu_kb())
