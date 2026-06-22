#!/bin/sh
# ============================================================
# lab-commit.sh — race-free атрибуция коммитов LabDoctorM
# ------------------------------------------------------------
# Проблема: все агенты коммитят из общего worktree /root/LabDoctorM.
# Запись identity в общий .git/config (git config user.name) приводит
# к гонке — параллельный агент перетирает её между add и commit.
#
# Решение: автор задаётся через GIT_AUTHOR_*/GIT_COMMITTER_* env
# в момент коммита. Env процесс-локален → гонки нет, и перебивает
# любое значение в config.
#
# Использование:
#   lab-commit.sh <agent> -m "сообщение" [git commit args...]
#   LAB_AGENT=antcat lab-commit.sh -m "сообщение"
#
# Агент берётся из первого аргумента (если он — известный id) или
# из переменной окружения LAB_AGENT.
# ============================================================

# Фиксированный путь к git-authors.json — общий для всех кабинетов
AUTHORS_JSON="$(dirname "$0")/../git-authors.json"

if [ ! -f "$AUTHORS_JSON" ]; then
    echo "lab-commit: не найден $AUTHORS_JSON" >&2
    exit 1
fi

# Определяем агента: первый аргумент, если это известный id, иначе $LAB_AGENT
AGENT=""

if [ $# -gt 0 ]; then
    # Проверяем, является ли первый аргумент известным агентом
    if command -v jq >/dev/null 2>&1 && jq -e --arg id "$1" 'has($id)' "$AUTHORS_JSON" >/dev/null 2>&1; then
        AGENT="$1"
        shift
    fi
fi

if [ -z "$AGENT" ] && [ -n "${LAB_AGENT:-}" ]; then
    AGENT="$LAB_AGENT"
fi

if [ -z "$AGENT" ]; then
    echo "lab-commit: агент не указан." >&2
    if command -v jq >/dev/null 2>&1; then
        KNOWN=$(jq -r 'keys | join(", ")' "$AUTHORS_JSON")
    else
        KNOWN="antcat, bestia, dominika, kotolizator, mangust, owl, raven, streikbrecher"
    fi
    echo "   Использование: $0 <agent> -m \"...\"   или   LAB_AGENT=<agent> $0 -m \"...\"" >&2
    echo "   Известные агенты: $KNOWN" >&2
    exit 1
fi

GIT_NAME=$(command -v jq >/dev/null 2>&1 && jq -r --arg id "$AGENT" '.[$id].name // empty' "$AUTHORS_JSON")
GIT_EMAIL=$(command -v jq >/dev/null 2>&1 && jq -r --arg id "$AGENT" '.[$id].email // empty' "$AUTHORS_JSON")

if [ -z "$GIT_NAME" ] || [ -z "$GIT_EMAIL" ]; then
    echo "lab-commit: агент '$AGENT' не найден в git-authors.json" >&2
    if command -v jq >/dev/null 2>&1; then
        KNOWN=$(jq -r 'keys | join(", ")' "$AUTHORS_JSON")
    else
        KNOWN="antcat, bestia, dominika, kotolizator, mangust, owl, raven, streikbrecher"
    fi
    echo "   Известные агенты: $KNOWN" >&2
    exit 1
fi

echo "lab-commit: автор = $GIT_NAME <$GIT_EMAIL>"

# Автор и коммиттер задаются локально для этого процесса git commit.
export GIT_AUTHOR_NAME="$GIT_NAME"
export GIT_AUTHOR_EMAIL="$GIT_EMAIL"
export GIT_COMMITTER_NAME="$GIT_NAME"
export GIT_COMMITTER_EMAIL="$GIT_EMAIL"

exec git commit "$@"
