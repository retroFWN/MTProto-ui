import os


class Settings:
    def __init__(self) -> None:
        self.bot_token: str = os.environ.get("BOT_BOT_TOKEN", "")
        self.panel_url: str = os.environ.get("BOT_PANEL_URL", "http://127.0.0.1:8080")
        self.panel_secret: str = os.environ.get("BOT_PANEL_SECRET", "")
        raw_ids = os.environ.get("BOT_ADMIN_IDS", "")
        self.admin_ids: list[int] = []
        if raw_ids:
            for part in raw_ids.split(","):
                part = part.strip()
                if part.isdigit():
                    self.admin_ids.append(int(part))


cfg = Settings()
