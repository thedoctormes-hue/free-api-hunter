#!/usr/bin/env python3
"""CLI-утилита для manus-outsourcing."""

import argparse
import asyncio
import json
import logging
import os
import sys
import time

# Добавляем родительскую папку в путь
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from config import ManusConfig
from manus_client import ManusError
from account_manager import AccountManager

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)


async def cmd_send(args):
    """Отправить задачу."""
    config = ManusConfig.from_file(args.config)
    import redis.asyncio as redis
    r = redis.Redis.from_url(config.redis_url)

    from task_manager import TaskManager
    tm = TaskManager(config, r)

    result = await tm.send_task(
        message=args.message,
        agent_profile=args.profile,
        locale=args.locale,
    )

    print(json.dumps(result, indent=2, ensure_ascii=False))

    # Сохранить task_id для последующих команд
    if args.wait:
        print(f"\nОжидание результата (таймаут {args.timeout}s)...")
        result = await tm.get_result(
            task_id=result["task_id"],
            account_id=result["account_id"],
            wait=True,
            timeout=args.timeout,
        )
        print(json.dumps(result, indent=2, ensure_ascii=False))

        # Скачать файлы
        if result.get("attachments"):
            print(f"\nСкачивание {len(result['attachments'])} файлов...")
            downloaded = await tm.download_attachments(
                task_id=result["task_id"],
                account_id=result["account_id"],
            )
            for f in downloaded:
                print(f"  ✅ {f['filename']} → {f['local_path']}")

    await r.close()


async def cmd_status(args):
    """Проверить статус задачи."""
    config = ManusConfig.from_file(args.config)
    client = ManusClient(api_key=***"key"], base_url=config.base_url)

    response = await client.list_messages(args.task_id)
    status = client.extract_status(response)
    print(f"Task {args.task_id}: {status}")

    if status == "stopped":
        attachments = client.extract_attachments(response)
        if attachments:
            print(f"\nФайлы ({len(attachments)}):")
            for att in attachments:
                print(f"  📎 {att['filename']} ({att.get('size_bytes', '?')} bytes)")
                print(f"     {att['url']}")

    await client.close()


async def cmd_credits(args):
    """Проверить баланс кредитов."""
    config = ManusConfig.from_file(args.config)
    import redis.asyncio as redis
    r = redis.Redis.from_url(config.redis_url)

    am = AccountManager(config, r)
    await am.refresh_balances()
    balances = await am.get_all_balances()

    print("Балансы Manus:")
    total = 0
    for acc_id, balance in balances.items():
        print(f"  {acc_id}: {balance} кредитов")
        total += balance
    print(f"  ─────────────────")
    print(f"  Итого: {total} кредитов")

    await r.close()


async def cmd_upload(args):
    """Загрузить файл на Яндекс Диск."""
    config = ManusConfig.from_file(args.config)
    disk = __import__("yandex_disk", fromlist=["YandexDisk"]).YandexDisk(config)

    url = await disk.upload_file(args.file, args.name)
    print(f"Загружено: {url}")


async def cmd_download(args):
    """Скачать файл с Яндекс Диска."""
    config = ManusConfig.from_file(args.config)
    disk = __import__("yandex_disk", fromlist=["YandexDisk"]).YandexDisk(config)

    await disk.download_file(args.remote, args.local)
    print(f"Скачано: {args.remote} → {args.local}")


def main():
    parser = argparse.ArgumentParser(description="Manus Outsourcing CLI")
    parser.add_argument("--config", default="/root/LabDoctorM/projects/free-api-hunter/config/manus-keys.json")

    subparsers = parser.add_subparsers(dest="command")

    # send
    p_send = subparsers.add_parser("send", help="Отправить задачу")
    p_send.add_argument("message", help="Описание задачи")
    p_send.add_argument("--profile", help="Agent profile (lite/standard/max)")
    p_send.add_argument("--locale", help="Locale (ru/en)")
    p_send.add_argument("--wait", action="store_true", help="Ждать результата")
    p_send.add_argument("--timeout", type=int, default=600, help="Таймаут ожидания (сек)")
    p_send.set_defaults(func=cmd_send)

    # status
    p_status = subparsers.add_parser("status", help="Проверить статус задачи")
    p_status.add_argument("task_id", help="ID задачи")
    p_status.add_argument("--key", help="API ключ (если не из конфига)")
    p_status.set_defaults(func=cmd_status)

    # credits
    p_credits = subparsers.add_parser("credits", help="Проверить баланс")
    p_credits.set_defaults(func=cmd_credits)

    # upload
    p_upload = subparsers.add_parser("upload", help="Загрузить файл на Диск")
    p_upload.add_argument("file", help="Локальный файл")
    p_upload.add_argument("--name", help="Имя на Диске")
    p_upload.set_defaults(func=cmd_upload)

    # download
    p_download = subparsers.add_parser("download", help="Скачать файл с Диска")
    p_download.add_argument("remote", help="Путь на Диске")
    p_download.add_argument("local", help="Локальный путь")
    p_download.set_defaults(func=cmd_download)

    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        sys.exit(1)

    asyncio.run(args.func(args))


if __name__ == "__main__":
    main()
