"""MTProxy Panel Telegram Bot — aiogram v3 with inline keyboards."""

import asyncio
import logging

from aiogram import Bot, Dispatcher
from aiogram.client.default import DefaultBotProperties
from aiogram.fsm.storage.memory import MemoryStorage

from config import cfg
from handlers import start, proxy, admin

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
log = logging.getLogger(__name__)


async def main() -> None:
    bot = Bot(token=cfg.bot_token, default=DefaultBotProperties(parse_mode="HTML"))
    dp = Dispatcher(storage=MemoryStorage())

    dp.include_router(start.router)
    dp.include_router(proxy.router)
    dp.include_router(admin.router)

    log.info("Bot starting (panel: %s)", cfg.panel_url)
    await dp.start_polling(bot)


if __name__ == "__main__":
    asyncio.run(main())
