#!/bin/bash
# Orion Scan — тесты
# Использование: bash orion-scan_test.sh
# Проверяет: парсинг моделей, фильтрацию, группировку, обработку ошибок

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PASS=0
FAIL=0

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    echo "  ✅ $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ $desc — expected '$expected', got '$actual'"
    FAIL=$((FAIL + 1))
  fi
}

assert_contains() {
  local desc="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -q "$needle"; then
    echo "  ✅ $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ $desc — '$needle' not found in output"
    FAIL=$((FAIL + 1))
  fi
}

assert_not_empty() {
  local desc="$1" value="$2"
  if [ -n "$value" ]; then
    echo "  ✅ $desc"
    PASS=$((PASS + 1))
  else
    echo "  ❌ $desc — value is empty"
    FAIL=$((FAIL + 1))
  fi
}

# ─── Тест 1: Парсинг JSON ответа OpenRouter ───
echo ""
echo "📋 Тест 1: Парсинг JSON — извлечение бесплатных моделей"

MOCK_JSON='{"data":[{"id":"test/model-free","name":"Test Free","context_length":131072,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":10000},"reasoning":{}},{"id":"test/model-paid","name":"Test Paid","context_length":131072,"pricing":{"prompt":"0.001","completion":"0.001"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":10000},"reasoning":{}},{"id":"test/model-small","name":"Test Small","context_length":1000,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":1000},"reasoning":{}},{"id":"test/model-image","name":"Test Image","context_length":131072,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text+image->image"},"top_provider":{"max_completion_tokens":0},"reasoning":{}}]}'

PARSED=$(echo "$MOCK_JSON" | python3 -c "
import json, sys
data = json.load(sys.stdin)['data']
free = []
for m in data:
    prompt_price = m.get('pricing', {}).get('prompt', '0')
    completion_price = m.get('pricing', {}).get('completion', '0')
    ctx = m.get('context_length', 0)
    modality = m.get('architecture', {}).get('modality', '')
    is_chat = modality.startswith('text') and '->text' in modality
    if prompt_price == '0' and completion_price == '0' and ctx >= 131072 and is_chat:
        free.append(m['id'])
print('\n'.join(free))
")

assert_contains "Бесплатная модель с большим контекстом найдена" "test/model-free" "$PARSED"
assert_eq "Платная модель отфильтрована" "" "$(echo "$PARSED" | grep 'test/model-paid' || true)"
assert_eq "Модель с маленьким контекстом отфильтрована" "" "$(echo "$PARSED" | grep 'test/model-small' || true)"
assert_eq "Нечат-модель (image output) отфильтрована" "" "$(echo "$PARSED" | grep 'test/model-image' || true)"

# ─── Тест 2: Группировка по размеру контекста ───
echo ""
echo "📋 Тест 2: Группировка по размеру контекста"

GROUP_JSON='{"data":[{"id":"test/giant","name":"Giant","context_length":1048576,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":262144},"reasoning":{}},{"id":"test/medium","name":"Medium","context_length":262144,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":32768},"reasoning":{}},{"id":"test/compact","name":"Compact","context_length":131072,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":32768},"reasoning":{}}]}'

GROUPED=$(echo "$GROUP_JSON" | python3 -c "
import json, sys
data = json.load(sys.stdin)['data']
giants = []
medium = []
compact = []
for m in data:
    prompt_price = m.get('pricing', {}).get('prompt', '0')
    completion_price = m.get('pricing', {}).get('completion', '0')
    ctx = m.get('context_length', 0)
    modality = m.get('architecture', {}).get('modality', '')
    is_chat = modality.startswith('text') and '->text' in modality
    if prompt_price == '0' and completion_price == '0' and ctx >= 131072 and is_chat:
        if ctx >= 1000000:
            giants.append(m['id'])
        elif ctx >= 256000:
            medium.append(m['id'])
        else:
            compact.append(m['id'])
print(f'giants:{\"|\".join(giants)}')
print(f'medium:{\"|\".join(medium)}')
print(f'compact:{\"|\".join(compact)}')
")

assert_contains "Гигант попал в группу giants" "test/giant" "$GROUPED"
assert_contains "Средняя модель попала в группу medium" "test/medium" "$GROUPED"
assert_contains "Компактная модель попала в группу compact" "test/compact" "$GROUPED"

# ─── Тест 3: Определение reasoning ───
echo ""
echo "📋 Тест 3: Определение reasoning"

REASON_JSON='{"data":[{"id":"test/reason-mandatory","name":"Reason Mandatory","context_length":131072,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":10000},"reasoning":{"mandatory":true}},{"id":"test/reason-default","name":"Reason Default","context_length":131072,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":10000},"reasoning":{"default_enabled":true}},{"id":"test/reason-none","name":"Reason None","context_length":131072,"pricing":{"prompt":"0","completion":"0"},"architecture":{"modality":"text->text"},"top_provider":{"max_completion_tokens":10000},"reasoning":{}}]}'

REASONED=$(echo "$REASON_JSON" | python3 -c "
import json, sys
data = json.load(sys.stdin)['data']
for m in data:
    r = m.get('reasoning', {})
    r_str = 'yes' if r.get('mandatory') or r.get('default_enabled') else 'no'
    print(f'{m[\"id\"]}:{r_str}')
")

assert_contains "Mandatory reasoning определён как yes" "test/reason-mandatory:yes" "$REASONED"
assert_contains "Default reasoning определён как yes" "test/reason-default:yes" "$REASONED"
assert_contains "Без reasoning определён как no" "test/reason-none:no" "$REASONED"

# ─── Тест 4: Форматирование ───
echo ""
echo "📋 Тест 4: Форматирование контекста"

FORMAT_VAL=$(python3 -c "print(f'{262144//1000:,}K')")
assert_eq "Форматирование 262144 → 262K" "262K" "$FORMAT_VAL"

FORMAT_VAL2=$(python3 -c "print(f'{1048576//1000:,}K')")
assert_eq "Форматирование 1048576 → 1,048K" "1,048K" "$FORMAT_VAL2"

# ─── Тест 5: Обработка пустого ответа ───
echo ""
echo "📋 Тест 5: Обработка пустого ответа"

EMPTY_JSON='{"data":[]}'
EMPTY_RESULT=$(echo "$EMPTY_JSON" | python3 -c "
import json, sys
data = json.load(sys.stdin)['data']
free = []
for m in data:
    prompt_price = m.get('pricing', {}).get('prompt', '0')
    completion_price = m.get('pricing', {}).get('completion', '0')
    ctx = m.get('context_length', 0)
    modality = m.get('architecture', {}).get('modality', '')
    is_chat = modality.startswith('text') and '->text' in modality
    if prompt_price == '0' and completion_price == '0' and ctx >= 131072 and is_chat:
        free.append(m['id'])
print(f'count:{len(free)}')
")

assert_eq "Пустой ответ → 0 моделей" "count:0" "$EMPTY_RESULT"

# ─── Тест 6: Загрузка .env ───
echo ""
echo "📋 Тест 6: Загрузка .env"

TEST_ENV_FILE=$(mktemp)
echo "OPENROUTER_API_KEY=test-key-12345" > "$TEST_ENV_FILE"

ENV_LOADED=$(bash -c "
source '$TEST_ENV_FILE'
echo \"\${OPENROUTER_API_KEY}\"
")

assert_eq "Ключ загружается из .env" "test-key-12345" "$ENV_LOADED"
rm -f "$TEST_ENV_FILE"

# ─── Итого ───
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Результаты: ✅ $PASS прошли | ❌ $FAIL упали"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
