"""Загрузка файлов на Яндекс Диск через WebDAV."""

import logging
import os
import subprocess
from datetime import datetime
from typing import Optional

import aiohttp

from config import ManusConfig

logger = logging.getLogger(__name__)


class YandexDisk:
    def __init__(self, config: ManusConfig):
        self.config = config
        self.webdav = config.yandex_disk_webdav
        self.user = config.yandex_disk_user
        self.password = config.get_yandex_disk_pass()
        self.colony_path = config.yandex_disk_colony_path

    def _auth(self) -> aiohttp.BasicAuth:
        return aiohttp.BasicAuth(self.user, self.password)

    def _make_url(self, path: str) -> str:
        return f"{self.webdav}{path}"

    async def upload_file(self, local_path: str, remote_name: Optional[str] = None) -> str:
        """Загрузить файл на Диск. Вернуть URL на Диске."""
        if not os.path.exists(local_path):
            raise FileNotFoundError(f"File not found: {local_path}")

        if remote_name is None:
            date = datetime.now().strftime("%Y-%m-%d")
            basename = os.path.basename(local_path)
            remote_name = f"{date}_{basename}"

        remote_path = f"{self.colony_path}/{remote_name}"
        url = self._make_url(remote_path)

        async with aiohttp.ClientSession() as session:
            with open(local_path, "rb") as f:
                data = f.read()
            async with session.put(url, data=data, auth=self._auth()) as resp:
                if resp.status in (200, 201, 204):
                    logger.info(f"Uploaded to Yandex Disk: {remote_path}")
                    return url
                else:
                    text = await resp.text()
                    raise Exception(f"Upload failed: HTTP {resp.status} - {text}")

    async def download_file(self, remote_path: str, local_path: str):
        """Скачать файл с Диска."""
        url = self._make_url(remote_path)

        async with aiohttp.ClientSession() as session:
            async with session.get(url, auth=self._auth()) as resp:
                resp.raise_for_status()
                with open(local_path, "wb") as f:
                    f.write(await resp.read())
                logger.info(f"Downloaded from Yandex Disk: {remote_path} → {local_path}")

    async def list_files(self, path: str = None) -> list:
        """Список файлов в папке на Диске."""
        url = self._make_url(path or self.colony_path)

        async with aiohttp.ClientSession() as session:
            async with session.request("PROPFIND", url, auth=self._auth(), headers={"Depth": "1"}) as resp:
                resp.raise_for_status()
                text = await resp.text()
                # Простой парсинг XML
                import re
                files = re.findall(r"<d:href>([^<]+)</d:href>", text)
                return [f.replace("/disk", "") for f in files if not f.endswith("/")]

    async def delete_file(self, remote_path: str):
        """Удалить файл с Диска."""
        url = self._make_url(remote_path)

        async with aiohttp.ClientSession() as session:
            async with session.delete(url, auth=self._auth()) as resp:
                if resp.status in (200, 204):
                    logger.info(f"Deleted from Yandex Disk: {remote_path}")
                else:
                    logger.warning(f"Delete failed: HTTP {resp.status}")

    async def ensure_path(self, path: str = None):
        """Создать папку на Диске если не существует."""
        url = self._make_url(path or self.colony_path)

        async with aiohttp.ClientSession() as session:
            async with session.request("MKCOL", url, auth=self._auth()) as resp:
                if resp.status in (200, 201):
                    logger.info(f"Created directory: {path or self.colony_path}")
                elif resp.status == 405:
                    pass  # Уже существует
                else:
                    logger.warning(f"MKCOL failed: HTTP {resp.status}")
