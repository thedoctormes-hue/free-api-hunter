# Unified Search Gateway — добавление бесплатных API-ключей

## Что уже готово (инфраструктура)
- Движки-шаблоны в `searxng/engines/`: `exa.py`, `serper.py`, `serpapi.py`, `olostep.py`
  (написаны по эталону `braveapi.py`, читают ключ из env).
- `settings.yml` уже содержит эти движки с `api_key: !env EXA_API_KEY` и т.д.
- `docker-compose.searxng.yml` монтирует папку `engines/` и читает `.env.searxng`.
- Скрипт `bin/lab-search-gateway-addkey.sh` раскатывает ключ + движок + перезапуск.

Движки НЕ активны, пока не задан env-ключ (SearXNG пропустит их с warning).

## Где ты регаешься (Вариант А)
- **Exa** — https://exa.ai (ключ: https://dashboard.exa.ai/api-keys) — free 1000/мес
- **Serper** — https://serper.dev/signup (ключ: https://serper.dev/dashboard) — free 2500 запросов
- **SerpApi** — https://serpapi.com/users/sign_up (ключ: https://serpapi.com/dashboard) — free 250/мес
- **Olostep** — https://www.olostep.com/ (ключ в дашборде после реги) — free 500 credits

Не регай: Tavily / Firecrawl / TinyFish (уже есть у нас).
Научные (arxiv, semantic_scholar, crossref, openalex) уже в дефолтном списке
SearXNG (`use_default_settings: true`) — ключи не нужны.

## Как вписать ключ (после реги)
Один вызов на провайдера:
```
bash bin/lab-search-gateway-addkey.sh exa      <TBOY_EXA_KEY>
bash bin/lab-search-gateway-addkey.sh serper   <TBOY_SERPER_KEY>
bash bin/lab-search-gateway-addkey.sh serpapi  <TBOY_SERPAPI_KEY>
bash bin/lab-search-gateway-addkey.sh olostep  <TBOY_OLOSTEP_KEY>
```
Скрипт:
1. Пишет ключ в `/root/.openclaw/.api-keys.json` (chmod 600).
2. Копирует движок в контейнер (также смонтирован через volume).
3. Прописывает `EXA_API_KEY=...` в `.env.searxng`.
4. Перезапускает `searxng`.
5. Запускает healthcheck.

## Проверка
```
bash bin/searxng-health.sh
# ожидаем: OK <N> (N должно вырасти за счёт новых движков)
```
Живой тест через оркестратор:
```
bash scripts/search-orchestrator.sh "тестовый запрос" broad 10
```

## Гибрид (если Serper/Exa требуют подтверждение email)
Ты регаешь, пересылаешь мне confirmation-ссылку из письма — я «докликиваю»
её через `curl` (HTTPS к сервису работает), и дальше вписываю ключ скриптом.
