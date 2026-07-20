#!/usr/bin/env python3
"""Обёртка над searxng-gateway.deep_research для KRV-валидации.

Используется из Go-бинаря hunter (cmd/hunter/main.go → runDeepResearch) вместо
агента Manus. Возвращает JSON: {"answer": str, "degraded": bool, "error": str}.

deep_research = веб (SearXNG/orchestrator) + семантическая память лабы
(memory-gateway.hybrid_search). Без внешних кредитов, без агента.
"""
import json
import os
import sys

# Подключаем пакет searxng-gateway (он не установлен в venv, лежит в репозитории).
_SEARXNG_GW = "/root/LabDoctorM/projects/mcp-tools/searxng-gateway"
if _SEARXNG_GW not in sys.path:
    sys.path.insert(0, _SEARXNG_GW)

from searxng_gateway.server import deep_research  # noqa: E402


def main() -> int:
    query = " ".join(sys.argv[1:]).strip()
    if not query:
        print(json.dumps({"answer": "", "degraded": False, "error": "empty query"}, ensure_ascii=False))
        return 0
    try:
        result = deep_research(query=query, count=10)
        if not isinstance(result, dict):
            result = {}
        answer = result.get("answer", "")
        degraded = bool(result.get("degraded", False))
        print(json.dumps({"answer": answer, "degraded": degraded, "error": ""}, ensure_ascii=False))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"answer": "", "degraded": True, "error": str(exc)}, ensure_ascii=False))
        return 1


if __name__ == "__main__":
    sys.exit(main())
