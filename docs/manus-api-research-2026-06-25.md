---
title: Manus API Research — Inter-Agent Integration Capabilities
date: 2026-06-25
author: Dominika (Scout)
version: 1.0.0
status: active
tags: [manus, api, inter-agent, outsourcing, research]
---

# Manus API Research

Исследование возможностей Manus API v2 для интеграции с агентами LabDoctorM в качестве аутсорсинга.

## Обзор

Manus — автономный AI-агент на базе Claude (Anthropic), работающий в sandbox-среде. Имеет REST API для программного взаимодействия. У ЗавЛаба доступно 4-5 аккаунтов Manus, каждый с ~300 кредитами/день.

**Базовый URL:** https://api.manus.ai/v2/
**Авторизация:** `x-manus-api-key` header

## Что проверено (live API)

### Способ взаимодействия

Task-based асинхронный паттерн:

1. `POST /v2/task.create` — создать задачу с описанием
2. `GET /v2/task.listMessages` — опрашивать статус и ответы
3. `POST /v2/task.sendMessage` — продолжить диалог
4. `POST /v2/task.stop` — остановить задачу

Время ответа: 15-30 секунд (зависит от сложности).
Результаты: текстовые сообщения + файлы в sandbox (manuscdn.com, временные ссылки).

### Подтверждённые возможности

| Возможность | Детали |
|---|---|
| Презентации | PPTX, PDF, интерактивный HTML. До 12 слайдов. Chart.js, Mermaid, D2, AI images |
| Генерация изображений | Latent diffusion, Mermaid.js, D2, Matplotlib/Seaborn |
| Веб-браузер | Полный Chromium, JS-rendering, скрапинг, многостраничные workflows |
| Код | Python 3.11, Node.js 22, Bash. Sandbox Ubuntu 24.04 |
| Документы | PDF, XLSX, CSV, JSON, XML, YAML, PNG, SVG, WebP |
| Исследования | Итеративный multi-source search, cross-reference, структурированные отчёты |
| Редактирование файлов | Загрузка → модификация → выдача результата |
| API integration | Потребление REST/GraphQL + создание endpoints через WebDev |

### Ограничения

- Нет генерации видео
- Sandbox эфемерный (нужен WebDev для 24/7 сервисов)
- Нет доступа к GPU
- Нет физического мира
- Временные ссылки на файлы (истекают)

### Модель использования

**Manus = аутсорсер для задач, которые нашим агентам тяжело или невозможно:**

- Презентации и слайды (мы не умеем)
- Сложные мульти-источниковые исследования
- Генерация медиа (изображения, графики, диаграммы)
- Документы и отчёты (PPTX, XLSX, PDF)
- Веб-скрапинг сложных JS-сайтов

**Паттерн интеграции:**

```
Наш агент → task.create(задача) → Manus выполняет → результат файлом → мы забираем
```

**Параллелизм:** 4-5 аккаунтов можно запускать одновременно.
**Квота:** ~300 кредитов/день/аккаунт = 1200-1500 кредитов/день суммарно.

## Примеры API вызовов

### Создание задачи

```bash
curl -X POST https://api.manus.ai/v2/task.create \
  -H "Content-Type: application/json" \
  -H "x-manus-api-key: $MANUS_API_KEY" \
  -d '{
    "message": {
      "content": "Описание задачи"
    }
  }'
```

Response:
```json
{
  "ok": true,
  "task_id": "LxEDhQoMGP8PXWjTHLJrBy",
  "task_title": "...",
  "task_url": "https://manus.im/app/LxEDhQoMGP8PXWjTHLJrBy"
}
```

### Получение ответа

```bash
curl "https://api.manus.ai/v2/task.listMessages?task_id=$TASK_ID&order=desc&limit=5" \
  -H "x-manus-api-key: $MANUS_API_KEY"
```

### Продолжение диалога

```bash
curl -X POST https://api.manus.ai/v2/task.sendMessage \
  -H "Content-Type: application/json" \
  -H "x-manus-api-key: $MANUS_API_KEY" \
  -d '{
    "task_id": "$TASK_ID",
    "message": { "content": "Продолжение задачи" }
  }'
```

## Webhooks

Manus поддерживает push-уведомления о событиях:

- `task_created` — задача создана
- `task_stopped` — задача завершена или ждёт ввода

Payload включает: event_id, event_type, task_detail (task_id, title, url, message, attachments, stop_reason).

Webhook Security: подпись через public key (GET /v2/webhook.publicKey).

## Дополнительные эндпоинты

| Эндпоинт | Описание |
|---|---|
| `/v2/agent.list` | Список агентов |
| `/v2/agent.detail` | Детали агента |
| `/v2/task.list` | Список задач (с фильтрацией по scope) |
| `/v2/task.detail` | Метаданные задачи |
| `/v2/task.stop` | Остановить задачу |
| `/v2/file.upload` | Загрузить файл |
| `/v2/file.detail` | Детали файла |
| `/v2/webhook.create` | Создать webhook |
| `/v2/webhook.list` | Список webhooks |
| `/v2/webhook.publicKey` | Публичный ключ для верификации |
| `/v2/usage.availableCredits` | Баланс кредитов |
| `/v2/skill.list` | Доступные навыки |
| `/v2/project.create` | Создать проект |
| `/v2/connector.list` | Список коннекторов |

## Отчёт от Manus (capabilities)

Manus предоставил детальный отчёт о своих возможностях (файл `capability_evaluation_report.md`). Ключевые тезисы:

- Presentation engine: HTML-based, export в PDF/PPTX, до 12 слайдов
- Image generation: динамический выбор модели под задачу
- Code: Turing-complete sandbox, Python/Node.js/Bash
- Research: broad search → targeted dives → synthesis
- API: может потреблять и создавать endpoints
- Ограничения: нет видео, нет GPU, эфемерный sandbox

## Результаты тестов

### Презентация "AI Agents in Healthcare" (gaQPyMZ7w6wkiB8UfxCgsy)

✅ Завершена. 10 слайдов, профессиональное качество:

1. AI Agents in Healthcare: The Next Frontier
2. Beyond Chatbots: What Are AI Agents?
3. Clinical Agent Architecture (ReAct loop, MCP, FHIR)
4. Market Growth ($500M → $5B by 2030, 45% CAGR)
5. Clinical Use Cases (documentation, decision support, prior auth)
6. Patient Empowerment (24/7 care, triage, scheduling)
7. Safety Guardrails (Human-in-the-Loop, HIPAA, confidence scoring)
8. Implementation Challenges (EHR integration, trust, regulation)
9. Future of Agentic Healthcare (multi-agent, embodied AI)
10. Conclusion: A Collaborative Future

**Стоимость:** 373 кредита
**Файлы:** `manus-presentation-ai-healthcare.json` (интерактивный HTML), `manus-presentation-ai-healthcare-notes.md`

### Кредитная система

- **Баланс:** 1330 кредитов осталось
- **Дневной лимит:** 300 кредитов/день
- **Обновление:** ежедневно (next_refresh: 1782421200)
- **4-5 аккаунтов** × 300 = 1200-1500 кредитов/день суммарно

### Вопросы ЗавЛаба (ответы из тестов)

**Q: Если кредитов не хватит, Manus доделает на следующий день?**
A: Не проверено напрямую, но судя по документации — task остаётся в системе. Вероятно, при нехватке кредитов task будет ждать следующего дня или остановится с ошибкой rate_limited. Нужен дополнительный тест.

**Q: Каждый запрос = новая сессия Manus?**
A: Да. Каждый task.create создаёт новый контекст. Для продолжения диалога нужно использовать task.sendMessage с тем же task_id. Это НЕ постоянная сессия — это серия связанных сообщений в рамках одного task.

## Статус тестовых задач

| Task ID | Описание | Статус |
|---|---|---|
| LxEDhQoMGP8PXWjTHLJrBy | Первый контакт | ✅ Завершён |
| gctUhjnFhL8v2yJs8b3wqf | Capabilities research | ✅ Завершён |
| gaQPyMZ7w6wkiB8UfxCgsy | Презентация (AI agents in healthcare) | ✅ Завершён (373 кредита) |

## Следующие шаги

- [ ] Протестировать поведение при нехватке кредитов
- [ ] Протестировать webhooks для push-уведомлений
- [ ] Оформить воркфлоу аутсорсинга в стандарт колонии
- [ ] Протестировать параллельный запуск нескольких аккаунтов
- [ ] Определить оптимальные юзкейсы для free-api-hunter

---

_Исследование проведено 25.06.2026 через live API Manus v2._
