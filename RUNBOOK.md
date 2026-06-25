# Free API Hunter — Runbook

## 🚀 Быстрый старт

### Запуск всех сервисов
```bash
systemctl start free-api-hunter-api.service
systemctl start free-api-hunter-scan.timer
```

### Проверка статуса
```bash
# API сервер
systemctl status free-api-hunter-api.service

# Таймер сканирования
systemctl list-timers | grep hunter

# Логи
journalctl -u free-api-hunter-api.service -f
journalctl -u free-api-hunter-scan.service -f
```

## 🔍 Эндпоинты API

### Health checks
- `GET /health` — базовый статус
- `GET /health/full` — расширенный статус (время последнего скана, состояние логов, доступность Orex)

### Метрики
- `GET /metrics` — Prometheus формат
- `GET /metrics/json` — JSON формат (обратная совместимость)

### Данные
- `GET /api/v1/providers` — список всех провайдеров
- `GET /api/v1/providers/{id}` — конкретный провайдер
- `GET /api/v1/findings` — все находки
- `GET /api/v1/stats` — статистика (JSON)

## 🔐 Безопасность

### Vault
- Ключи хранятся в `/root/LabDoctorM/vault/free-api-hunter/<provider>/<key_name>.key`
- Права: директории `700`, файлы ключей `600`
- Audit log: `/var/log/free-api-hunter/vault-audit.log`

### Ротация ключей
```bash
# Запуск вручную
/root/LabDoctorM/scripts/rotate-vault-keys.sh

# Автоматически через cron (добавьте в crontab -e)
# 0 2 1 */3 * /root/LabDoctorM/scripts/rotate-vault-keys.sh  # каждые 3 месяца
```

## 📊 Мониторинг

### Prometheus метрики (пример)
```
# HELP free_api_hunter_providers_total Total number of providers
# TYPE free_api_hunter_providers_total gauge
free_api_hunter_providers_total 19
free_api_hunter_providers_status{status="verified"} 8
free_api_hunter_providers_status{status="confirmed"} 8
free_api_hunter_providers_status{status="deprioritized"} 3

# HELP free_api_hunter_findings_total Total number of findings
# TYPE free_api_hunter_findings_total gauge
free_api_hunter_findings_total 9
```

### Алерты
- Telegram алерты при успешном сканировании
- Алерты при ротации ключей (через скрипт)
- Orex интеграция (если доступен)

## 🛠️ Обслуживание

### Перезапуск после изменений конфигурации
```bash
systemctl daemon-reload
systemctl restart free-api-hunter-api.service
systemctl restart free-api-hunter-scan.timer
```

### Проверка логов сканирования
```bash
tail -f /var/log/free-api-hunter/scan.log
```

### Проверка vault audit
```bash
tail -f /var/log/free-api-hunter/vault-audit.log
```

## 🔧 Разработка

### Сборка
```bash
cd /root/LabDoctorM/projects/free-api-hunter
go build -o bin/hunter cmd/hunter/main.go
```

### Тесты
```bash
go test ./...
go test ./... -cover
```

### Запуск в режиме отладки
```bash
./bin/hunter -api :8090 -no-alerts  # только API
./bin/hunter -scan -no-alerts       # одноразовое сканирование
./bin/hunter -no-alerts             # полный цикл (scan + API)
```

## 📞 Контакты
- ЗавЛаб: @DoctorMES в Telegram
- Системные логи: `/var/log/free-api-hunter/`
- Конфиги: `/root/LabDoctorM/projects/free-api-hunter/config/`