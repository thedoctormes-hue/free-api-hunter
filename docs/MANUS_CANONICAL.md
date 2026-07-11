# КАНОНИЧЕСКАЯ ИНСТРУКЦИЯ: Работа с Манусом (Manus API v2)

> **Статус:** canonical · **Дата:** 2026-07-11 · **Автор:** Сова (owl), free-api-hunter / KRV
> **Provenance (фактчек):** прямые live-вызовы Совы 2026-07-11 + официальные доки `open.manus.ai/docs/v2` (web-verify сабагент) + мерж семантической памяти лаборатории (Kotolizator 06-28, Dominika 06-25, Raven 06-25/06-29).
> **Теги верификации:** `[LIVE]` — проверено прямым вызовом · `[DOCS]` — проверено по офиц. докам · `[SPEC]` — из OpenAPI-спека, независимо не верифицировано (web-verify-2 в процессе).

---

## 0. Что такое Манус (для лаборатории)

Манус — **не просто LLM, а мощный автономный агент** (tool-use, браузер, код, файлы) с LLM-мозгом. У ЗавЛаба **5 аккаунтов** → это **5 независимых агентских каналов** (параллельные задачи).

Две роли в лаборатории:
1. **KRV-валидация неизвестных ключей** (основной пайплайн, см. §7) — Манус выступает «агентным LLM-слоем», который умеет находить документацию провайдера и извлекать endpoint map + инструкции для ключа, который бинарник не распознал.
2. **Аутсорсинг задач→артефактов** (презентации, research, документы, код, скрапинг) — агенты колонии дают Манусу задачу, получают файл/документ.

---

## 1. Доступ к Манусу

- **Ключи:** хранятся в vault `/root/LabDoctorM/vault/free-api-hunter/manus/api.key` … `api.key.4` (права 600) И в `configs/manus-keys.json` (`free-api-hunter`, 5 аккаунтов, поле `"key"`). Зарегистрированы в KRV Registry как `manus/api.key` … `manus/api.key.4` со `live_status=valid`.
- **Auth:** HTTP-заголовок `x-manus-api-key: <key>` `[LIVE][DOCS]`. (Bearer-схема не поддерживается `[DOCS]`.)
- **Base URL:** `https://api.manus.ai` (путь `/v2/...`) `[LIVE][DOCS]`. Доки — на `open.manus.ai/docs/v2`.

---

## 2. Базовый паттерн работы (task-based async)

```
1. POST /v2/task.create   → task_id
2. GET  /v2/task.listMessages?task_id=...  → опрашивать статус/ответы
3. (опц.) POST /v2/task.sendMessage → продолжить диалог тем же task_id
4. результат: текст в assistant_message.content + файлы в attachments[] (пре-сигнованные URL)
```

- **Опрос:** задача асинхронная, завершение 15–30 c (Dominika, live 06-25). Статусы: `running` / `stopped` (готово) / `waiting` (нужен ввод) / `error`.
- **Retrieval БЕСПЛАТЕН:** `task.list` / `task.listMessages` / скачивание аттачментов НЕ расходуют кредиты. Даже при 0/отрицательных кредитах результаты ДОСТУПНЫ (проверено 07-11 при отрицательном балансе).
- **Ссылки на файлы — временные** (TTL sandbox ~42–48 ч, HTTP 410 `gone` после). Для постоянного хранения грузи на Яндекс Диск `/colony/shared` (см. §9).

---

## 3. Проверенный API-референс (ядро)

| Метод | Эндпоинт | Назначение | Вериф. |
| :-- | :-- | :-- | :-- |
| GET | `/v2/usage.availableCredits` | баланс кредитов | `[LIVE][DOCS]` |
| POST | `/v2/task.create` | создать задачу | `[LIVE][DOCS]` |
| GET | `/v2/task.list` | список задач (пагинация `has_more`/`next_cursor`) | `[LIVE][DOCS]` |
| GET | `/v2/task.listMessages` | лента событий задачи | `[LIVE][DOCS]` |
| GET | `/v2/task.detail` | метаданные задачи | `[DOCS]` |
| POST | `/v2/task.sendMessage` | продолжить диалог | `[DOCS]` (live: Dominika 06-25) |
| POST | `/v2/task.confirmAction` | подтвердить `waiting`-действие | `[DOCS]` |
| POST | `/v2/task.stop` | остановить задачу | `[LIVE]` (Dominika 06-25) |
| GET | `/v2/agent.list` | профили агентов (`manus-1.6`/`lite`/`max`) | `[DOCS]` |
| POST | `/v2/webhook.create` | создать webhook | `[DOCS]` |
| GET | `/v2/webhook.publicKey` | публичный ключ (RSA) для верификации | `[DOCS]` |
| POST | `/v2/file.upload` | загрузить файл (2-step presigned) | `[DOCS]` |
| GET | `/v2/file.detail` | детали файла | `[DOCS]` |
| POST | `/v2/project.create` | создать проект | `[DOCS]` |
| GET | `/v2/project.list` | список проектов | `[DOCS]` |
| GET | `/v2/skill.list` | доступные навыки | `[DOCS]` |
| GET | `/v2/connector.list` | коннекторы | `[DOCS]` |

**Rate limits (req/min) — `[LIVE][DOCS]`:**
`task.create` 10 · `task.sendMessage` 10 · `task.listMessages` 100 · `task.detail` 100 · `file.upload` 40 · `project.create` 40 · `webhook.create` 40 · `task.confirmAction` 40.
При 429 `rate_limited` → exponential backoff + jitter; предпочтительнее webhooks, чем опрос `listMessages` (быстро съедает 100/min).

**`task.create` тело:** `{"message":{"content":[...]}, "agent_profile":"manus-1.6-lite"}`. `message.content` канонично — **массив** `{type:"text", text:"..."}` `[DOCS]`; **простая строка тоже принимается** `[LIVE]`. Опции: `interactive_mode` (bool), `hide_in_task_list` (bool) `[DOCS]`.

---

## 4. Полный каталог эндпоинтов (из OpenAPI-спека, Kotolizator 06-28)

Ниже — полный список 32 эндпоинтов из спецификации. Все VERIFIED по `open.manus.ai/docs/v2` (web-verify-2, 07-11).

| Эндпоинт | Назначение | Вериф. |
| :-- | :-- | :-- |
| `POST /v2/task.update` | изменить задачу | `[DOCS]` |
| `POST /v2/task.delete` | удалить задачу | `[DOCS]` |
| `GET /v2/agent.detail` | детали агента | `[DOCS]` |
| `POST /v2/agent.update` | обновить агента | `[DOCS]` |
| `POST /v2/file.delete` | удалить файл | `[DOCS]` |
| `POST /v2/webhook.delete` | удалить webhook | `[DOCS]` |
| `GET /v2/webhook.list` | список webhooks | `[DOCS]` |
| `GET /v2/usage.list` | история расхода (600/min) | `[DOCS]` |
| `GET /v2/usage.teamLog` | лог команды (задачи+кредиты) | `[DOCS]` |
| `GET /v2/usage.teamStatistic` | статистика команды (600/min) | `[DOCS]` |
| `GET /v2/browser.onlineList` | активные браузер-клиенты (для task.confirmAction) | `[DOCS]` |
| `GET /v2/website.*` | веб-скрейпинг: `website.listCheckpoints`, `website.status`, `website.publish`, `website.update` | `[DOCS]` |

> Все VERIFIED по `open.manus.ai/docs/v2` (web-verify-2, 07-11). Примечание: буквального `website.list` нет — семейство `website.*` использует `website.listCheckpoints` и др. Лимиты: `usage.list`/`usage.teamStatistic` — 600/min.

---

## 5. Кредитная модель — ФАКТЧЕК (важно)

**Актуально по прямым наблюдениям Совы, 2026-07-11 (5 аккаунтов, live `GET /v2/usage.availableCredits`):**
- Стартовый `total_credits` ≈ **300** на аккаунт.
- При исчерпании `total_credits` **уходит в отрицательные значения** (наблюдалось −1 … −7).
- В ответе есть поле **`next_refresh_time`** (unix-таймстамп) — время пополнения.
- Поле `refresh_credits` / `refresh_interval` **отсутствует**.

**УСТАРЕВШЕЕ / НЕПОДТВЕРЖДЁННОЕ (помечено к удалению из памяти):**
- Заметки 25.06 (Dominika, Raven) и сгенерированный гайд `ZPJj` утверждали «**1,500 free + 300 daily refresh**» / «баланс 1330». LIVE 07-11 этому **противоречит**. Вероятно, агенты ошибочно интерпретировали (ZPJj доказанно фабриковал `refresh_credits:1500`). Авторитетный источник — прямые наблюдения 07-11.
- **Каноническая модель:** ~300 кредитов/день/аккаунт × 5 = ~1500/день суммарно (но НЕ как «буфер 1500 сверху», а как дневной лимит, который может уйти в минус).

---

## 6. Как отправить задачу (канонический путь)

**Операционный путь сегодня — `dispatch_direct.py`** (прямой вызов API, без Redis/воркера; используется KRV `validate-pending`):

```bash
# Ручной запуск (печатает результат в stdout: текст + содержимое текстовых аттачментов .md/.txt)
python3 /root/LabDoctorM/projects/free-api-hunter/scripts/manus-outsourcing/dispatch_direct.py "Сделай презентацию про X, 10 слайдов"

# Явно указать конфиг с ключами (иначе берётся configs/manus-keys.json)
python3 scripts/manus-outsourcing/dispatch_direct.py "промпт" configs/manus-keys.json
```

- Ротирует 5 аккаунтов, при 429 `rate_limited` делает паузу и переключает ключ, таймаут ожидания ~280 с.
- **НЕ передаёт сырой ключ в промпт.**

**Альтернатива — скилл `manus-outsourcing`** (Dominika 06-25, Raven 06-29): богатый набор (`TaskManager`, `AccountManager` с Redis-балансами и token-bucket 10 RPM, `YandexDisk` WebDAV, `webhook_handler` FastAPI с RSA-подписью). ⚠️ Его `cli.py run --wait` **требует запущенного воркера** (локальный `TaskManager` через Redis) — не standalone. Модули можно импортировать, но Redis обязателен. Для большинства задач достаточно `dispatch_direct.py`.

---

## 7. KRV-пайплайн: Манус как валидатор неизвестных ключей

Это основное боевое применение Мануса в лаборатории (реализовано 2026-07-11):

```
keydrop (таймер) → pending_validation.json (неизвестный провайдер/ключ)
   → hunter validate-pending (таймер 15 мин)
      → для каждого pending: dispatch_direct.py "<промпт: найди доку и извлеки endpoint map + инструкции>"
         → Манус (агентный LLM-слой) исследует провайдера, возвращает текст + .md-аттачмент
      → Go-код парсит stdout → регистрирует ключ в KRV Registry (SQLite)
         с live_status=valid, instructions=извлечённое, added_by=krv-agent-manus
   → MCP apikeys отдаёт агентам валидированный ключ + инструкцию
```

- Решает «1 блокер»: бинарник не находит endpoint'ы/capabilities без LLM+человека → Манус (агент с LLM) = слой рассуждений/действий.
- Успешно отработано 07-11: извлечён и зарегистрирован реальный результат по Manus API v2 (ZPJj), а также 5 ключей Мануса валидированы через `usage.availableCredits`.

---

## 8. Возможности Манус (live-тест Dominika 06-25)

- **Презентации:** PPTX, PDF, интерактивный HTML (Chart.js, Mermaid, D2), до 12 слайдов.
- **Генерация изображений:** latent diffusion, Mermaid.js, D2, Matplotlib/Seaborn.
- **Веб-браузер:** полный Chromium, JS-рендеринг, скрапинг, многостраничные workflow'ы.
- **Код:** Python 3.11, Node.js 22, Bash; sandbox Ubuntu 24.04 (Turing-complete).
- **Документы:** PDF, XLSX, CSV, JSON, XML, YAML, PNG, SVG, WebP.
- **Исследования:** итеративный multi-source search + cross-reference → структурированные отчёты.
- **Редактирование файлов** (загрузка→модификация→выдача) и **API integration** (потребляет/создаёт endpoints).
- **Ограничения:** нет генерации видео, нет GPU, эфемерный sandbox (ссылки истекают), нет доступа к физ. миру.

---

## 9. Webhooks (push вместо опроса)

- События: `task_created`, `task_stopped` (завершён/ждёт ввода), `task_completed`, `task_failed`.
- Подпись: заголовки `x-manus-signature` + `x-manus-timestamp`; RSA-PKCS1v15-SHA256; публичный ключ `GET /v2/webhook.publicKey` (кэш 1 ч). Допуск ±5 мин.
- Рекомендуется для продакшена (экономит 100/min бюджет `listMessages`).

---

## 10. Уроки / подводные камни

1. **Кредиты:** ~300/акк/день, могут уйти в минус; retrieval БЕСПЛАТЕН даже при 0. Не верь заявлениям «1500 free buffer» — опровергнуто live.
2. **`message.content`:** массив по докам, строка тоже работает.
3. **429:** backoff + ротация ключей; не спамь `task.create` (>10/мин).
4. **Ссылки на файлы временные** — перекладывай на Яндекс Диск (`/colony/shared`) для постоянства.
5. **Не переусердствуй с харвестом:** массовая выгрузка ВСЕХ задач аккаунта тащит личную историю ЗавЛаба — фильтруй по заголовку/дате.
6. **фиктивные ключи** → `unauthenticated`; только реальные ключи из vault дают live-ответы.

---

## 11. Fact-check log (источники и решения)

- **VERIFIED (live 07-11 + docs):** ядро API (§3), rate limits, auth, base URL. Подтверждено web-verify сабагентом по `open.manus.ai/docs/v2` (11 VERIFIED, 1 CONTRADICTED — `message.content` оказался массивом, но строка тоже работает).
- **CORRECTED:** `refresh_credits:1500 / refresh_interval:"daily"` (гайд ZPJj) — отсутствует в live и доках. Удалено.
- **SUPERSEDED:** модель «1,500 free + 300 daily» (Dominika/Raven 06-25) — противоречит live 07-11; заменена на ~300/день/акк.
- **MERGED from semantic memory:** Kotolizator `manus-api-v2-research.md` (32 эндпоинта из спека), Dominika `manus-api-research-2026-06-25.md` (live-паттерн, возможности, вебхуки), Raven `fah-manus-docs.md` (архитектура скилла), Dominika `manus-skill-final.md` (структура скилла).
- **RESOLVED (web-verify-2, 07-11):** все 8 расширенных эндпоинтов §4 VERIFIED по `open.manus.ai/docs/v2` (usage.list/teamLog/teamStatistic, agent.detail/update, file.delete, webhook.delete/list, browser.onlineList, website.*). Теперь ВЕСЬ каталог (§3+§4) — VERIFIED.

---
*См. также: `docs/MANUS_API_v2_VERIFIED.md` (сжатый проверенный референс), `docs/ADR-KRV-pipeline.md` (KRV-пайплайн).*
