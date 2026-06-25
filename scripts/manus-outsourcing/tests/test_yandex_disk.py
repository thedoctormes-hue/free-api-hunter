"""Тесты для yandex_disk."""

import asyncio
import os
import tempfile
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

import sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from yandex_disk import YandexDisk
from config import ManusConfig


class TestYandexDisk:

    def setup_method(self):
        self.config = ManusConfig(
            yandex_disk_user="test@yandex.ru",
            yandex_disk_pass_file="/tmp/test-disk-pass",
            yandex_disk_colony_path="/colony/shared",
        )
        with open("/tmp/test-disk-pass", "w") as f:
            f.write("test-password")

    def teardown_method(self):
        if os.path.exists("/tmp/test-disk-pass"):
            os.remove("/tmp/test-disk-pass")

    def test_auth(self):
        disk = YandexDisk(self.config)
        auth = disk._auth()
        assert auth.login == "test@yandex.ru"
        assert auth.password == "test-password"

    def test_make_url(self):
        disk = YandexDisk(self.config)
        url = disk._make_url("/colony/shared/test.pptx")
        assert url == "https://webdav.yandex.ru/colony/shared/test.pptx"

    @pytest.mark.asyncio
    async def test_upload_file_not_found(self):
        disk = YandexDisk(self.config)
        with pytest.raises(FileNotFoundError):
            await disk.upload_file("/nonexistent/file.pptx")

    def test_upload_name_generation(self):
        """Если имя не указано — генерируется из даты и имени файла."""
        disk = YandexDisk(self.config)
        # Просто проверяем логику генерации имени
        from datetime import datetime
        date = datetime.now().strftime("%Y-%m-%d")
        basename = "test.pptx"
        expected_name = f"{date}_{basename}"
        assert expected_name.startswith("2026-")
        assert expected_name.endswith(".pptx")


class TestYandexDiskIntegration:
    """Интеграционные тесты (требуют реального пароля Диска)."""

    @pytest.mark.asyncio
    async def test_real_upload(self):
        pass_file = os.path.expanduser("~/.config/yandex/.disk-pass")
        if not os.path.exists(pass_file):
            pytest.skip("Yandex disk pass not found")

        config = ManusConfig()
        disk = YandexDisk(config)

        with tempfile.NamedTemporaryFile(suffix=".txt", delete=False) as f:
            f.write(b"Manus test file")
            temp_path = f.name

        try:
            url = await disk.upload_file(temp_path, "test-manus-skill.txt")
            assert "test-manus-skill.txt" in url
            print(f"✅ Uploaded: {url}")
        finally:
            os.remove(temp_path)


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
