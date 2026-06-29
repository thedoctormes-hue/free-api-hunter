#!/usr/bin/env python3
"""CLI-утилита для manus-outsourcing v2 — расширенная."""

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
from manus_client import ManusClient, ManusError
from account_manager import AccountManager

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)

# ── Helpers ──────────────────────────────────────────────────────────

def _get_config(args) -> ManusConfig:
    """Загрузить конфиг из файла."""
    return ManusConfig.from_file(args.config)


def _get_api_key(config: ManusConfig, args_key: str = None) -> str:
    """Получить API ключ: из аргумента или из первого аккаунта."""
    if args_key:
        return args_key
    if config.accounts:
        return config.accounts[0].api_key
    raise ManusError("no_api_key", "No API key provided and none in config")


def _format_json(data: dict) -> str:
    """Красивый JSON-вывод."""
    return json.dumps(data, indent=2, ensure_ascii=False)


def _print_header(text: str):
    """Вывести заголовок секции."""
    print(f"\n{'─' * 60}")
    print(f"  {text}")
    print(f"{'─' * 60}")


# ── Commands ─────────────────────────────────────────────────────────

async def cmd_run(args):
    """Отправить задачу на выполнение."""
    config = _get_config(args)
    import redis.asyncio as redis
    r = redis.Redis.from_url(config.redis_url)

    from task_manager import TaskManager
    tm = TaskManager(config, r)

    # Опциональный title
    message = args.message
    if args.title:
        message = f"[Title: {args.title}]\n{message}"

    result = await tm.send_task(
        message=message,
        agent_profile=args.profile,
        locale=args.locale,
    )

    print(f"✅ Задача создана")
    print(f"   task_id:  {result['task_id']}")
    print(f"   url:      {result.get('task_url', 'N/A')}")
    print(f"   account:  {result['account_id']}")

    # --wait
    if args.wait:
        task_id = result["task_id"]
        account_id = result["account_id"]

        _print_header(f"Ожидание результата (таймаут {args.timeout}s)")
        start = time.time()
        final = await tm.get_result(
            task_id=task_id,
            account_id=account_id,
            wait=True,
            timeout=args.timeout,
        )
        elapsed = time.time() - start

        status = final.get("status", "unknown")
        if status == "finished":
            print(f"\n✅ Задачи завершена за {elapsed:.0f}s")
            content = final.get("content", "")
            if content:
                print(f"\n📝 Результат:\n{content}")

            # --download
            if final.get("attachments") and args.download:
                print()
                downloaded = await tm.download_attachments(
                    task_id=task_id,
                    account_id=account_id,
                    output_dir=f"{config.output_dir}/manus-output/{task_id}",
                )
                for f in downloaded:
                    print(f"   📁 {f['filename']} → {f['local_path']}")
        else:
            print(f"\n⚠️  Статус: {status}")
            if final.get("error"):
                print(f"   {final['error']}")

    await r.close()


async def cmd_result(args):
    """Показать результат задачи (из messages)."""
    config = _get_config(args)
    api_key = _get_api_key(config, args.key if hasattr(args, 'key') else None)
    client = ManusClient(api_key=api_key, base_url=config.base_url)

    result = await client.get_result(args.task_id)
    output_text = result.text
    attachments = result.attachments or []
    structured = result.structured_output

    _print_header(f"Результат задачи {args.task_id}")

    if output_text:
        print(f"\n📝 Текст ответа:\n{output_text}")
    else:
        print("\n⚠️  Текстовый ответ не найден")

    if structured:
        print(f"\n📊 Структурированный вывод:")
        print(_format_json(structured))

    if attachments:
        print(f"\n📎 Вложения ({len(attachments)}):")
        for att in attachments:
            fname = att.get('name', att.get('filename', att.get('file_name', 'unknown')))
            print(f"   • {fname}")
            print(f"     URL: {att.get('url', att.get('file_url', 'N/A'))}")
            if att.get('size_bytes') or att.get('size'):
                sz = att.get('size_bytes', att.get('size', 'unknown'))
                print(f"     Size: {sz} bytes")
    else:
        print("\n📎 Вложений нет")

    await client.close()


async def cmd_upload(args):
    """Загрузить файл в Manus storage."""
    config = _get_config(args)
    api_key = _get_api_key(config)
    client = ManusClient(api_key=api_key, base_url=config.base_url)

    if not os.path.isfile(args.filepath):
        print(f"❌ Файл не найден: {args.filepath}")
        await client.close()
        return

    _print_header(f"Загрузка файла: {os.path.basename(args.filepath)}")
    file_id = await client.upload_file(args.filepath)
    await client.close()

    print(f"\n✅ Файл загружен")
    print(f"   file_id:    {file_id}")
    print(f"   filename:   {os.path.basename(args.filepath)}")


async def cmd_project(args):
    """Создать проект в Manus."""
    config = _get_config(args)
    api_key = _get_api_key(config)
    client = ManusClient(api_key=api_key, base_url=config.base_url)

    _print_header(f"Создание проекта: {args.name}")
    result = await client.create_project(args.name, getattr(args, 'instruction', None))
    await client.close()

    project_id = result.get("project_id", result.get("id", ""))
    print(f"\n✅ Проект создан")
    print(f"   project_id: {project_id}")
    print(f"   name:       {args.name}")
    if getattr(args, 'instruction', None):
        print(f"   instruction:{args.instruction}")


async def cmd_usage(args):
    """Показать текущий баланс кредитов."""
    config = _get_config(args)
    import redis.asyncio as redis
    r = redis.Redis.from_url(config.redis_url)

    am = AccountManager(config, r)
    await am.refresh_balances()
    balances = await am.get_all_balances()

    _print_header("Баланс кредитов Manus")
    total = 0
    for acc_id, balance in balances.items():
        print(f"   {acc_id:20s} → {balance:>8} credits")
        total += balance

    print(f"   {'─' * 35}")
    print(f"   {'Итого:':20s}   {total:>8} credits")

    await r.close()


async def cmd_status(args):
    """Проверить статус задачи."""
    config = _get_config(args)
    api_key = _get_api_key(config, args.key if hasattr(args, 'key') else None)
    client = ManusClient(api_key=api_key, base_url=config.base_url)

    response = await client.list_messages(args.task_id)
    status = client.extract_status(response)

    _print_header(f"Статус задачи {args.task_id}")
    print(f"   Статус: {status or 'unknown'}")

    if status == "stopped":
        attachments = client.extract_attachments(response)
        if attachments:
            print(f"\n   Вложения ({len(attachments)}):")
            for att in attachments:
                print(f"     📎 {att['filename']} ({att.get('size_bytes', '?')} bytes)")
                print(f"        {att['url']}")

    await client.close()


# ── Main ─────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="Manus Outsourcing CLI v2",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
%(prog)s run "Сделать презентацию" --wait --download
%(prog)s run "Анализ" --title "Q3 отчёт" --timeout 600
%(prog)s result <task_id>                       # извлечь результат
%(prog)s upload ./data/file.pdf                # загрузить в storage
%(prog)s project "My Project" "Инструкция"     # создать проект
%(prog)s usage                                  # баланс кредитов
%(prog)s status <task_id>                       # статус задачи
        """,
    )
    parser.add_argument(
        "--config",
        default="/root/LabDoctorM/projects/free-api-hunter/config/manus-keys.json",
    )
    subparsers = parser.add_subparsers(dest="command", help="Доступные команды")

    # ── run ────────────────────────────────────────────────────────────
    p_run = subparsers.add_parser("run", help="Отправить задачу на выполнение")
    p_run.add_argument("message", help="Описание задачи")
    p_run.add_argument("--profile", help="Agent profile (lite/standard/max)")
    p_run.add_argument("--locale", help="Locale (ru/en)")
    p_run.add_argument("--wait", action="store_true", help="Ждать завершения")
    p_run.add_argument("--timeout", type=int, default=300, help="Таймаут ожидания в секундах (default: 300)")
    p_run.add_argument("--download", action="store_true", help="Скачать attachments после завершения")
    p_run.add_argument("--title", help="Название задачи")
    p_run.set_defaults(func=cmd_run)

    # ── result ─────────────────────────────────────────────────────────
    p_result = subparsers.add_parser("result", help="Показать результат задачи")
    p_result.add_argument("task_id", help="ID задачи")
    p_result.add_argument("--key", help="API ключ (если не из конфига)")
    p_result.set_defaults(func=cmd_result)

    # ── upload ─────────────────────────────────────────────────────────
    p_upload = subparsers.add_parser("upload", help="Загрузить файл в Manus storage")
    p_upload.add_argument("filepath", help="Путь к локальному файлу")
    p_upload.set_defaults(func=cmd_upload)

    # ── project ────────────────────────────────────────────────────────
    p_project = subparsers.add_parser("project", help="Создать проект")
    p_project.add_argument("name", help="Имя проекта")
    p_project.add_argument("instruction", nargs="?", help="Инструкция для проекта")
    p_project.set_defaults(func=cmd_project)

    # ── usage ──────────────────────────────────────────────────────────
    p_usage = subparsers.add_parser("usage", help="Показать баланс кредитов")
    p_usage.set_defaults(func=cmd_usage)

    # ── status ─────────────────────────────────────────────────────────
    p_status = subparsers.add_parser("status", help="Проверить статус задачи")
    p_status.add_argument("task_id", help="ID задачи")
    p_status.add_argument("--key", help="API ключ (если не из конфига)")
    p_status.set_defaults(func=cmd_status)

    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        sys.exit(1)

    asyncio.run(args.func(args))


if __name__ == "__main__":
    main()
