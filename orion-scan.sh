#!/bin/bash
# Orion Scan — сканирование бесплатных моделей OpenRouter
# Шаг 1: Получить список бесплатных моделей (context >= 128K)
# Шаг 2: Проверить каждую на живучесть
# Шаг 3: Вывести отчёт

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -z "${OPENROUTER_API_KEY:-}" ] && [ -f "$SCRIPT_DIR/.env" ]; then
  source "$SCRIPT_DIR/.env"
fi

KEY="${OPENROUTER_API_KEY:-}"
if [ -z "$KEY" ]; then
  echo "ERROR: OPENROUTER_API_KEY not set (checked env and $SCRIPT_DIR/.env)"
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
    modality = m.get('architecture', {}).get('modality', '')
    # Только чат-модели (text->text или text+image->text)
    is_chat = modality.startswith('text') and '->text' in modality
    if prompt_price == '0' and completion_price == '0' and ctx >= $MIN_CTX and is_chat:
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

print('🆓 Бесплатные модели OpenRouter — отчёт')
print('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━')
print(f'📅 Дата: {scan_date}')
print(f'📊 Всего найдено: $TOTAL | ✅ Работают: $OK_COUNT | ❌ Не работают: $FAIL_COUNT')
print()

if ok_models:
    print('✅ РАБОТАЮТ:')
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
        
        r_str = '✅' if 'mandatory' in reasoning or 'default_enabled' in reasoning else '—'
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

if fail_models:
    print()
    print('❌ НЕ РАБОТАЮТ:')
    print('──────────────────────────────────────')
    for line in fail_models.split('\n'):
        if not line.strip():
            continue
        parts = line.split('|')
        if len(parts) < 3:
            continue
        mid, name, err = parts[0], parts[1], parts[2]
        print(f'  • {name} — {err}')

print()
print('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━')
print('🤖 Orion Scan • OpenRouter API')
")

echo "$REPORT"
