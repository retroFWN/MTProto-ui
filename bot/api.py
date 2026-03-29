"""HTTP client for the MTProxy Panel /bot/api endpoints."""

import time

import aiohttp
from config import cfg


class PanelAPI:
    def __init__(self) -> None:
        self._base = cfg.panel_url.rstrip("/")
        self._headers = {"X-Bot-Token": cfg.panel_secret}

    async def _get(self, path: str) -> dict | list:
        async with aiohttp.ClientSession() as s:
            async with s.get(f"{self._base}{path}", headers=self._headers) as r:
                r.raise_for_status()
                return await r.json()

    async def _post(self, path: str, json: dict | None = None) -> dict:
        async with aiohttp.ClientSession() as s:
            async with s.post(f"{self._base}{path}", headers=self._headers, json=json) as r:
                r.raise_for_status()
                return await r.json()

    async def _put(self, path: str, json: dict | None = None) -> dict:
        async with aiohttp.ClientSession() as s:
            async with s.put(f"{self._base}{path}", headers=self._headers, json=json) as r:
                r.raise_for_status()
                return await r.json()

    async def _delete(self, path: str) -> dict:
        async with aiohttp.ClientSession() as s:
            async with s.delete(f"{self._base}{path}", headers=self._headers) as r:
                r.raise_for_status()
                return await r.json()

    # --- Proxies ---

    async def list_proxies(self) -> list:
        return await self._get("/bot/api/proxies")

    async def start_proxy(self, proxy_id: int) -> dict:
        return await self._post(f"/bot/api/proxies/{proxy_id}/start")

    async def stop_proxy(self, proxy_id: int) -> dict:
        return await self._post(f"/bot/api/proxies/{proxy_id}/stop")

    async def restart_proxy(self, proxy_id: int) -> dict:
        return await self._post(f"/bot/api/proxies/{proxy_id}/restart")

    # --- Clients ---

    async def list_clients(self, proxy_id: int) -> list:
        return await self._get(f"/bot/api/proxies/{proxy_id}/clients")

    async def create_client(
        self, proxy_id: int, name: str, traffic_limit: int = 0, expiry_days: int = 0
    ) -> dict:
        payload: dict = {"name": name}
        if traffic_limit > 0:
            payload["traffic_limit"] = traffic_limit
        if expiry_days > 0:
            payload["expiry_time"] = int(time.time()) + expiry_days * 86400
        return await self._post(f"/bot/api/proxies/{proxy_id}/clients", json=payload)

    async def update_client(self, proxy_id: int, client_id: int, **fields) -> dict:
        return await self._put(
            f"/bot/api/proxies/{proxy_id}/clients/{client_id}", json=fields
        )

    async def delete_client(self, proxy_id: int, client_id: int) -> dict:
        return await self._delete(f"/bot/api/proxies/{proxy_id}/clients/{client_id}")

    async def reset_traffic(self, proxy_id: int, client_id: int) -> dict:
        return await self._post(
            f"/bot/api/proxies/{proxy_id}/clients/{client_id}/reset-traffic"
        )

    # --- Settings ---

    async def get_settings(self) -> dict:
        return await self._get("/bot/api/settings")


panel = PanelAPI()
