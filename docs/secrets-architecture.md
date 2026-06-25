# Архитектура управления секретами для агентов

## Проблема

Сейчас знания о бесплатных API ключах хранятся только у Штрейкбрехера:
- Ключи на диске в `/root/LabDoctorM/vault/free-api-hunter/`
- Документация в `docs/providers-status.md`
- Другие агенты не имеют доступа к этой информации

Это единая точка отказа — если Штрейкбрехер недоступен, ключи теряются.

## Решение: OpenClaw SecretRef + lab-vault

OpenClaw имеет встроенную систему управления секретами — **SecretRef**.

### Как работает

1. **Секреты хранятся в lab-vault** (зашифрованный snapshot)
2. **Агенты получают доступ через SecretRef** — ссылку на секрет, а не сам ключ
3. **Gateway разрешает ссылки при старте** — если секрет недоступен, запуск падает
4. **Ключи не хранятся в конфиге агентов** — только ссылки

### Типы провайдеров SecretRef

**env** — переменные окружения:
```json
{ "source": "env", "provider": "default", "id": "OPENAI_API_KEY" }
```

**file** — JSON-файл с секретами:
```json
{ "source": "file", "provider": "filemain", "id": "/providers/openai/apiKey" }
```

**exec** — внешняя программа (наш случай — lab-vault):
```json
{ "source": "exec", "provider": "vault", "id": "providers/cohere/apiKey" }
```

### Exec provider — интеграция с lab-vault

lab-vault должен реализовать протокол exec provider:

**Запрос (stdin):**
```json
{ "protocolVersion": 1, "provider": "vault", "ids": ["providers/cohere/apiKey"] }
```

**Ответ (stdout):**
```json
{ "protocolVersion": 1, "values": { "providers/cohere/apiKey": "eaNrIYAqRw..." } }
```

**Ошибка:**
```json
{ "protocolVersion": 1, "values": {}, "errors": { "providers/cohere/apiKey": { "message": "not found" } } }
```

## План реализации

### Шаг 1: Настроить exec provider для lab-vault

Создать скрипт-обёртку `/usr/local/bin/openclaw-vault-resolver`:
- Принимает JSON на stdin
- Вызывает `curl http://127.0.0.1:8301/access/{token}` для каждого id
- Возвращает JSON с значениями на stdout

### Шаг 2: Зарегистрировать провайдера в openclaw.json

```json
{
  "secrets": {
    "providers": {
      "labvault": {
        "source": "exec",
        "command": "/usr/local/bin/openclaw-vault-resolver",
        "args": ["--profile", "default"],
        "passEnv": ["PATH"],
        "jsonOnly": true
      }
    }
  }
}
```

### Шаг 3: Мигрировать ключи на SecretRef

Заменить plaintext ключи в конфигах агентов на SecretRef:
```json
{
  "models": {
    "providers": {
      "cohere": {
        "apiKey": { "source": "exec", "provider": "labvault", "id": "providers/cohere/apiKey" }
      }
    }
  }
}
```

### Шаг 4: Аудит

```bash
openclaw secrets audit --check
```

## Текущие секреты для миграции

**Cohere** (4 ключа):
- eaNrIYAqRwjjL9CVc6XgJ4ZVOZs3Of8i4RgFjGx6
- 5OkipXnQ0R3IDivxPKq2WDzZODGnli7WN0nIsd8h
- JVU0mq9jtGyPTQTB2DO2d16pvHtPs3l28p9Br6qk
- 7cGV7Zre9QWehKoG7wWOlHdAEPgnVEO6n6KzTr86

**Cerebras** (5 ключей):
- csk-***MASKED***
- csk-***MASKED***
- csk-***MASKED***
- csk-***MASKED***
- csk-***MASKED***

**Gemini, Groq** — ключи в vault, нужно перенести в lab-vault

## Безопасность

- Ключи НЕ хранятся в конфигах агентов
- Ключи НЕ коммитятся в git
- Доступ через lab-vault с аутентификацией
- Аудит через `openclaw secrets audit`
- При компрометации — отзыв токена в lab-vault

## Ссылки

- [OpenClaw Secrets Management](https://docs.openclaw.ai/gateway/secrets)
- [LumaDock: OpenClaw Secrets Guide](https://lumadock.com/tutorials/openclaw-secrets-management)
- [GitHub Issue #7916: Encrypted API keys](https://github.com/openclaw/openclaw/issues/7916)
