---
title: Пошаговый воркфлоу скилла manus-outsourcing
date: 2026-06-25
author: Dominika (Scout)
version: 1.0.0
status: active
tags: [manus, outsourcing, workflow, skill, yandex-disk]
---

# Пошаговый воркфлоу скилла manus-outsourcing

> Понятным языком: что происходит от момента "агент получил задачу" до "результат на Яндекс Диске".

## Полная схема

```
Данные → Спек → task.create → Ждём → Скачиваем файл → Диск → Обновляем баланс → Отдаём результат
```

## Шаг 1: Агент получает задачу и данные

**Что происходит:** ЗавЛаб или другой агент говорит: *"Сделай презентацию по бесплатным API для медицины"*.

**Что делает агент:**
- Собирает данные (список API, описания, категории)
- Формирует их в структурированный формат (JSON, Markdown)
- Понимает что это задача для Manus (тяжёлая, мультимодальная)

**Пример данных на входе:**
```json
{
  "topic": "Бесплатные API для медицины",
  "apis": [
    {"name": "OpenFDA", "category": "Препараты", "url": "https://open.fda.gov", "limit": "1000/час"},
    {"name": "HealthData.gov", "category": "Здоровье", "url": "https://healthdata.gov", "limit": "безлимит"}
  ],
  "requirements": "10 слайдов, графики рынка, рекомендации"
}
```

## Шаг 2: Скилл выбирает аккаунт Manus

**Что происходит:** Скилл проверяет баланс всех 5 ключей через `usage.availableCredits`.

**Как работает маршрутизация:**
```
manus-1: 1303 кредита ✅ (максимальный баланс → выбираем этот)
manus-2: 300 кредита
manus-3: 394 кредита
manus-4: 300 кредита
manus-5: 300 кредита
```

**Стратегия:** least-used (ключ с наибольшим балансом). Если все ключи <500 кредитов — используем максимальный и ждём рефреша.

**Если ключ не работает (401):** Переключаемся на следующий по балансу.

## Шаг 3: Скилл отправляет задачу Manus

**Что происходит:** Скилл формирует детальное описание и отправляет через `task.create`.

**Пример спека для Manus:**
```
Создай профессиональную презентацию на тему "Бесплатные API для медицины".

Данные:
- Список из 15 API в формате JSON (прилагается как файл)
- Категории: Препараты, Здоровье, Диагностика, Исследования
- Требуется: графики роста рынка, сравнительная таблица

Формат вывода:
- 10 слайдов в формате PPTX
- Markdown-файл с заметками к каждому слайду
- Стиль: профессиональный, медицинская тематика, синие тона

Сохрани результат как файлы в sandbox.
```

**API-запрос:**
```python
response = await manus_client.create_task(
    message=spec_text,
    structured_output_schema={
        "type": "object",
        "properties": {
            "pptx_url": {"type": "string"},
            "notes_md_url": {"type": "string"},
            "slide_count": {"type": "number"},
            "cost_credits": {"type": "number"}
        },
        "required": ["pptx_url", "notes_md_url"]
    }
)
# response = {"task_id": "xxx", "task_url": "https://manus.im/app/xxx"}
```

**Rate limiting:** Перед запросом скилл проверяет Token Bucket в Redis. Если токенов нет — ждёт.

## Шаг 4: Скилл ждёт результат

**Два варианта:**

### Вариант А — Polling (для простых случаев)

```python
while True:
    messages = await manus_client.list_messages(task_id)
    status = messages["messages"][0]["status_update"]["agent_status"]
    
    if status == "stopped":
        break
    elif status == "running":
        await asyncio.sleep(5)  # ждём 5 секунд
    
# Или если задача спрашивает пользователя (stop_reason: "ask"):
# → сообщаем ЗавЛабу, ждём ответа, отправляем task.sendMessage
```

### Вариант Б — Webhooks (для продакшена)

Manus сам отправляет POST на наш endpoint:
```
POST /webhooks
Headers: x-manus-signature: <JWT>, x-manus-timestamp: <ts>
Body: {"event_type": "task_stopped", "task_detail": {...}}
```

Скилл проверяет подпись через публичный ключ → обрабатывает событие.

**Важно:** Endpoint должен отвечать HTTP 200 за <10 секунд.

## Шаг 5: Скилл скачивает файлы из Manus

**Что происходит:** Manus завершил задачу. В ответе — ссылки на файлы в sandbox (manuscdn.com).

**Скачивание:**
```python
# Скачиваем презентацию
await download_file(
    url=result["pptx_url"],
    local_path="/tmp/manus_output/api-healthcare.pptx"
)

# Скачиваем заметки
await download_file(
    url=result["notes_md_url"],
    local_path="/tmp/manus_output/api-healthcare-notes.md"
)
```

**Важно:** Файлы в sandbox Manus живут только 48 часов. Скачать нужно сразу.

## Шаг 6: Скилл отправляет файлы на Яндекс Диск

**Что происходит:** Скилл загружает скачанные файлы в папку shared на Яндекс Диске через WebDAV.

**Путь на Диске:**
```
/colony/shared/2026-06-25_manus-api-healthcare-pptx
/colony/shared/2026-06-25_manus-api-healthcare-notes.md
```

**WebDAV-запрос:**
```bash
curl -s -X PUT \
  -u "moscowskiymichi@yandex.ru:$YANDEX_DISK_PASS" \
  -T /tmp/manus_output/api-healthcare.pptx \
  "https://webdav.yandex.ru/colony/shared/2026-06-25_manus-api-healthcare.pptx"
```

**Зачем Диск:**
- Постоянное хранилище (не 48 часов как у Manus)
- Доступ для всех агентов колонии
- Можно отправить ссылку ЗавЛабу или опубликовать

## Шаг 7: Скилл обновляет баланс кредитов

**Что происходит:** После завершения задачи скилл обновляет баланс ключа в Redis.

```python
# Было: manus-1 = 1303
# Стало: manus-1 = 930 (минус ~373 за презентацию)
await redis_client.hset("manus:credits", "manus-1", 930)
```

**Зачем:** Следующая задача пойдёт на другой ключ (least-used стратегия).

## Шаг 8: Скилл отдаёт результат

**Что происходит:** Скилл возвращает агенту финальный результат.

```python
return {
    "status": "success",
    "task_id": "xxx",
    "files": {
        "pptx": "https://webdav.yandex.ru/colony/shared/2026-06-25_manus-api-healthcare.pptx",
        "notes": "https://webdav.yandex.ru/colony/shared/2026-06-25_manus-api-healthcare-notes.md"
    },
    "local_paths": {
        "pptx": "/root/LabDoctorM/projects/free-api-hunter/output/api-healthcare.pptx",
        "notes": "/root/LabDoctorM/projects/free-api-hunter/output/api-healthcare-notes.md"
    },
    "credits_used": 373,
    "account_used": "manus-1"
}
```

## Обработка ошибок

| Ошибка | Что делает скилл |
|---|---|
| **429 rate_limited** | Exponential backoff (1с→2с→4с→8с), retry. Если retries > 5 → переключить ключ |
| **401 unauthorized** | Переключиться на следующий ключ по балансу |
| **404 not_found** | Task не существует → вернуть ошибку агенту |
| **410 gone** | Файл в sandbox удалён (прошло 48ч) → запросить повторно |
| **success: false** | Manus не смог по схеме → повторить с уточнением |
| **stop_reason: ask** | Manus спрашивает → скилл спрашивает ЗавЛаба → отправляет ответ |
| **Все ключи <500** | Использовать максимальный, записать в лог, что нужен рефреш |

## Полный пример одной задачи

**Задача:** Презентация "Бесплатные API для медицины"

```
1. Агент собрал 15 API → сформировал JSON
2. Скилл выбрал manus-1 (1303 кредита)
3. Отправил task.create с детальным спеком
4. Ждал 25 секунд (polling каждые 5 сек)
5. Manus вернул pptx_url + notes_md_url
6. Скилл скачал файлы в /tmp/
7. Скилл загрузил на Яндекс Диск: /colony/shared/2026-06-25_api-healthcare.pptx
8. Скилл обновил баланс: manus-1 = 930
9. Скилл вернул агенту ссылки на Диск
10. Агент отдал ЗавЛабу: "Готово: [ссылка на Диск]"
```

**Стоимость:** 373 кредита из 1303.
**Время:** ~30 секунд.
**Результат:** Профессиональная презентация на Яндекс Диске.

---

_Воркфлоу описан для скилла manus-outsourcing v1.0._
