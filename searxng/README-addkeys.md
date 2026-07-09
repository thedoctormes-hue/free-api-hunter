# Unified Search Gateway — добавление бесплатных API-ключей

## Что уже готово (инфраструктура)
- Движки-шаблоны в `searxng/engines/`: `exa.py`, `serper.py`, `serpapi.py`, `olostep.py`
  (написаны по эталону `braveapi.py`, читают ключ из `settings.yml`).
- `settings.yml` содержит движок `exa` с прописанным ключом (секрет -> gitignored).
  Остальные (serper/serpapi/olostep) добавляются скриптом при получении ключей.
- `docker-compose.searxng.yml` монтирует только `settings.yml` (ro). Кастомные
  движки копируются в контейнер скриптом через `docker cp` (НЕ монтируются целиком,
  чтобы не затереть встроенные движки SearXNG — иначе `ImportError: cannot import
  name 'wikidata'`).
- Скрипт `bin/lab-search-gateway-addkey.sh` раскатывает ключ + копирует движки + перезапуск.

**ВАЖНО:** SearXNG НЕ поддерживает тег `!env` в `settings.yml` (падает с
`SearxSettingsException`). Ключи прописываются напрямую (inline) в `settings.yml`,
поэтому файл в `.gitignore` (секрет). Общий `server.secret_key` тоже сгенерирован и
лежит только на диске.

## Где ты регаешься (Вариант А)
- **Exa** — https://exa.ai (ключ: https://dashboard.exa.ai/api-keys) — free 1000/мес
- **Serper** — https://serper.dev/signup (ключ: https://serper.dev/dashboard) — free 2500 запросов
- **SerpApi** — https://serpapi.com/users/sign_up (ключ: https://serpapi.com/dashboard) — free 250/мес
- **Olostep** — https://www.olostep.com/ (ключ в дашборде после реги) — free 500 credits

Не регай: Tavily / Firecrawl / TinyFish (уже есть).
Научные (arxiv, semantic_scholar, crossref, openalex) уже в дефолтном списке
SearXNG (`use_default_settings: true`) — ключи не нужны.

## Как вписать ключ (после реги)
Один вызов на провайдера:
```
# один ключ
bash bin/lab-search-gateway-addkey.sh exa      <EXA_KEY>
# ПУЛ для роутинга (несколько аккаунтов Exa -> движки exa/exa2/exa3):
bash bin/lab-search-gateway-addkey.sh exa <KEY1> <KEY2> <KEY3>
# остальные провайдеры — по одному ключу
bash bin/lab-search-gateway-addkey.sh serper   <SERPER_KEY>
bash bin/lab-search-gateway-addkey.sh serpapi  <SERPAPI_KEY>
bash bin/lab-search-gateway-addkey.sh olostep  <OLOSTEP_KEY>
```
Скрипт:
1. Пишет ключ в `/root/.openclaw/.api-keys.json` (chmod 600).
2. Копирует ВСЕ движки (`exa.py`, `serper.py`, ...) в контейнер через `docker cp`.
3. Прописывает ключ в `settings.yml` (inline, т.к. SearXNG НЕ поддерживает `!env`).
4. Перезапускает `searxng`.
5. Запускает healthcheck.

## Проверка
```
bash bin/searxng-health.sh
# ожидаем: OK <N>
```
Живой тест конкретного движка:
```
curl -s "http://localhost:8889/search?q=test&format=json&engines=exa" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('results',[])))"
```

## Гибрид (если Serper/Exa требуют подтверждение email)
Ты регаешь, пересылаешь мне confirmation-ссылку из письма — я «докликаю»
её через `curl` (HTTPS к сервису работает), и дальше вписываю ключ скриптом.
