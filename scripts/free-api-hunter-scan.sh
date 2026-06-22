#!/bin/bash
# Free API Hunter — сканирование + отправка отчёта через Бестию (OpenClaw cron)
set -euo pipefail

HUNTER_DIR="/root/LabDoctorM/projects/free-api-hunter"
LOG_DIR="/var/log/free-api-hunter"
TIMESTAMP=$(date +%Y-%m-%d_%H:%M)
BINARY="$HUNTER_DIR/bin/hunter"

cd "$HUNTER_DIR"

# Собрать бинарник если не существует или старше 1 часа
if [ ! -f "$BINARY" ] || [ "$(find "$BINARY" -mmin +60 2>/dev/null | wc -l)" -gt 0 ]; then
    echo "[scan] Building hunter binary..."
    go build -ldflags "-X main.Version=$(git describe --tags --always 2>/dev/null || echo dev)" -o "$BINARY" ./cmd/hunter
fi

# Запуск сканера без алертов
OUTPUT=$("$BINARY" -no-alerts -limit 10 2>&1)

# Подсчёт результатов
RAW=$(echo "$OUTPUT" | grep "Сырых находок:" | awk '{print $NF}' || echo "0")
FILTERED=$(echo "$OUTPUT" | grep "После фильтра:" | awk '{print $NF}' || echo "0")
PROVIDERS=$(echo "$OUTPUT" | grep "Провайдеров в базе:" | awk '{print $NF}' || echo "0")

# Формирование отчёта
REPORT="🔍 Free API Hunter — Scan Report

📊 Сырых находок: $RAW
✅ После фильтра: $FILTERED
🏦 Провайдеров в базе: $PROVIDERS
⏰ $TIMESTAMP UTC"

# Извлечение топ-находок
TOP=$(echo "$OUTPUT" | grep -A5 "Топ.*находок" | head -20 || echo "")

if [ -n "$TOP" ]; then
    REPORT="$REPORT

🆕 Топ находок:
$TOP"
fi

# Сохраняем в лог
mkdir -p "$LOG_DIR"
echo "$REPORT" > "$LOG_DIR/scan-report-$TIMESTAMP.txt"

# Вывод для cron (будет отправлен через OpenClaw)
echo "$REPORT"
