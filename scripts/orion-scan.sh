#!/bin/bash
# Orion Scan — сканирование бесплатных моделей OpenRouter
# Шаг 1: Получить список бесплатных моделей (context >= 128K)
# Шаг 2: Проверить каждую на живучесть
# Шаг 3: Вывести отчёт

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HUNTER_DIR="$(dirname "$SCRIPT_DIR")"

# Приоритет: env var > vault file > .env fallback
if [ -n "${OPENROUTER_API_KEY:-}" ]; then
  KEY="$OPENROUTER_API_KEY"
elif [ -f "/root/LabDoctorM/vault/free-api-hunter/openrouter/api.key" ]; then
  KEY=$(cat "/root/LabDoctorM/vault/free-api-hunter/openrouter/api.key" | tr -d '[:space:]')
elif [ -f "$HUNTER_DIR/.env" ]; then
  source "$HUNTER_DIR/.env"
  KEY="${OPENROUTER_API_KEY:-}"
fi

if [ -z "$KEY" ]; then
  echo "ERROR: OPENROUTER_API_KEY not set (checked env, vault, .env)"
  exit 1
fi

API="https://openrouter.ai/api/v1"
TIMEOUT=15
MIN_CTX=131072

echo "🔍 Шаг 1: Получаю список моделей с OpenRouter..."
MODELS_JSON=$(curl -s --max-time 30 "$API/models")

# Парсим бесплатные модели с контекстом >= 128K
FREE_MODELS=$(echo "$MODELS_JSON" | python3 -c "
import json, sys
data = json.load(sys.stdin)['data']
free = []
for m in data:
    prompt_price = m.get('pricing', {}).get('prompt', '0')
    completion_price = m.get('pricing', {}).get('completion', '0')
    ctx = m.get('context_length', 0)
    arch = m.get('architecture', {}) or {}
    out_mods = arch.get('output_modalities')
    if out_mods:
        # только текстовый выход — исключаем музыку/картинки (lyria и т.п.)
        is_text_only = 'text' in out_mods and 'audio' not in out_mods and 'image' not in out_mods
    else:
        modality = arch.get('modality', '') or ''
        is_text_only = modality.startswith('text') and modality.endswith('->text')
    if prompt_price == '0' and completion_price == '0' and ctx >= $MIN_CTX and is_text_only:
        free.append({
            'id': m['id'],
            'name': m.get('name', m['id']),
            'ctx': ctx,
            'max_out': m.get('top_provider', {}).get('max_completion_tokens') or 0,
            'reasoning': m.get('reasoning', {}),
        })
free.sort(key=lambda x: x['ctx'], reverse=True)
for m in free:
    print(f\"{m['id']}|{m['name']}|{m['ctx']}|{m['max_out']}|{m['reasoning']}\")
")

TOTAL=$(echo "$FREE_MODELS" | wc -l)
echo "📊 Найдено $TOTAL бесплатных моделей (context >= 128K)"
echo ""
echo "🔍 Шаг 2: Проверяю модели на живучесть..."

OK_COUNT=0
FAIL_COUNT=0
OK_MODELS=""
FAIL_MODELS=""

while IFS='|' read -r id name ctx max_out reasoning; do
  [ -z "$id" ] && continue

  result=$(curl -s --max-time $TIMEOUT -X POST "$API/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $KEY" \
    -d "{\"model\":\"$id\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with exactly: OK\"}],\"max_tokens\":10}" 2>&1) || true

  if echo "$result" | python3 -c "import json,sys; d=json.load(sys.stdin); exit(0 if 'choices' in d else 1)" 2>/dev/null; then
    echo "  ✅ $id"
    OK_MODELS="${OK_MODELS}${id}|${name}|${ctx}|${max_out}|${reasoning}\n"
    OK_COUNT=$((OK_COUNT + 1))
  else
    err=$(echo "$result" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('error',{}).get('message','timeout'))" 2>/dev/null || echo "timeout")
    echo "  ❌ $id — $err"
    FAIL_MODELS="${FAIL_MODELS}${id}|${name}|${err}\n"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi

  sleep 0.3
done <<< "$FREE_MODELS"

echo ""
echo "🔍 Шаг 3: Формирую отчёт..."
echo ""

# Формируем отчёт
SCAN_DATE=$(date -u '+%Y-%m-%d %H:%M UTC')

REPORT=$(python3 -c "
import sys

ok_models = '''${OK_MODELS}'''.strip()
fail_models = '''${FAIL_MODELS}'''.strip()
scan_date = '''${SCAN_DATE}'''

print('🆓 Бесплатные модели OpenRouter')
print('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━')
print(f'📅 Снимок на: {scan_date} | Найдено: $TOTAL | ✅ Отвечают на 1 тест: $OK_COUNT')
print()
print('⚠️ Проверка — ОДИН пробный вызов на модель. Под реальной нагрузкой (миллиарды токенов/мес) free-модели часто таймаутятся и лимитятся. Галочка ≠ стабильная работа.')
print()

if ok_models:
    print('✅ ОТВЕЧАЮТ НА ТЕСТ:')
    print('──────────────────────────────────────')

    # Группируем по размеру контекста
    giants = []
    medium = []
    compact = []

    for line in ok_models.split('\n'):
        if not line.strip():
            continue
        parts = line.split('|')
        if len(parts) < 4:
            continue
        mid, name, ctx, max_out = parts[0], parts[1], int(parts[2]), parts[3]
        reasoning = parts[4] if len(parts) > 4 else ''

        r_str = '🧠' if 'mandatory' in reasoning or 'default_enabled' in reasoning else '—'
        out_str = f'{int(max_out)//1000:,}K' if max_out and int(max_out) > 0 else '?'

        entry = (mid, name, ctx, out_str, r_str)

        if ctx >= 1000000:
            giants.append(entry)
        elif ctx >= 256000:
            medium.append(entry)
        else:
            compact.append(entry)

    if giants:
        print()
        print('🥇 Гигант-модели (1M+ контекст):')
        for mid, name, ctx, out_str, r_str in giants:
            print(f'  • {name}')
            print(f'    {mid}')
            print(f'    Контекст: {ctx:,} | Выход: {out_str} | Reasoning: {r_str}')

    if medium:
        print()
        print('🥈 Средние (256K-512K):')
        for mid, name, ctx, out_str, r_str in medium:
            print(f'  • {name}')
            print(f'    {mid}')
            print(f'    Контекст: {ctx:,} | Выход: {out_str} | Reasoning: {r_str}')

    if compact:
        print()
        print('🥉 Компактные (128K-256K):')
        for mid, name, ctx, out_str, r_str in compact:
            print(f'  • {name}')
            print(f'    {mid}')
            print(f'    Контекст: {ctx:,} | Выход: {out_str} | Reasoning: {r_str}')

print()
print('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━')
print('🤖 Orion Scan • OpenRouter API')
")

echo "$REPORT"

# Аннотация стабильности по истории прогонов (state-файл в scripts/)
STATE_FILE="$SCRIPT_DIR/.orion-scan-state.json"
STABLE_DOWN_THRESHOLD=3
python3 - <<PY
import json, os
ok_raw = '''${OK_MODELS}'''.strip()
fail_raw = '''${FAIL_MODELS}'''.strip()
state_path = '''${STATE_FILE}'''
threshold = $STABLE_DOWN_THRESHOLD
state = {}
if os.path.exists(state_path):
    try:
        state = json.load(open(state_path))
    except Exception:
        state = {}
new_state = {}
for line in ok_raw.split('\n'):
    if not line.strip():
        continue
    mid = line.split('|')[0]
    prev = state.get(mid, {})
    new_state[mid] = {'down_streak': 0, 'up_streak': prev.get('up_streak', 0) + 1, 'status': 'up'}
for line in fail_raw.split('\n'):
    if not line.strip():
        continue
    mid = line.split('|')[0]
    prev = state.get(mid, {})
    new_state[mid] = {'down_streak': prev.get('down_streak', 0) + 1, 'up_streak': 0, 'status': 'down'}
try:
    json.dump(new_state, open(state_path, 'w'), ensure_ascii=False, indent=2)
except Exception:
    pass
notes = []
for mid, st in new_state.items():
    if st.get('down_streak', 0) >= threshold:
        notes.append(f"  ⚠️ СТАБИЛЬНО DOWN: {mid} ({st['down_streak']} прогонов подряд)")
    elif st.get('up_streak', 0) == 1 and state.get(mid, {}).get('down_streak', 0) >= threshold:
        notes.append(f"  ✅ ВОССТАНОВИЛАСЬ: {mid}")
if notes:
    print()
    print('📊 СТАБИЛЬНОСТЬ (по истории прогонов):')
    print('──────────────────────────────────────')
    for n in notes:
        print(n)
PY
