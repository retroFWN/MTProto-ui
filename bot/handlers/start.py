from aiogram import Router, types
from aiogram.filters import CommandStart, Command

router = Router()


@router.message(CommandStart())
async def cmd_start(msg: types.Message) -> None:
    await msg.answer(
        "<b>MTProxy Panel Bot</b>\n\n"
        "Команды:\n"
        "/proxies — список прокси-серверов\n"
        "/connect &lt;proxy_id&gt; — получить ссылку подключения\n"
        "/status &lt;proxy_id&gt; — статус прокси и клиентов\n"
        "/traffic &lt;proxy_id&gt; — статистика трафика\n\n"
        "<i>Админ-команды:</i>\n"
        "/addclient &lt;proxy_id&gt; &lt;name&gt; — добавить клиента\n"
        "/delclient &lt;proxy_id&gt; &lt;client_id&gt; — удалить клиента\n"
        "/resettraffic &lt;proxy_id&gt; &lt;client_id&gt; — сбросить трафик\n"
        "/help — эта справка",
        parse_mode="HTML",
    )


@router.message(Command("help"))
async def cmd_help(msg: types.Message) -> None:
    await cmd_start(msg)
