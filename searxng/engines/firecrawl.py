# SPDX-License-Identifier: AGPL-3.0-or-later
"""Firecrawl search engine for SearXNG (key-rotating pool with failover)."""

import json
import typing as t

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults
from searx.engines._poolkeys import KeyPool, probe_post_json

if t.TYPE_CHECKING:
    from searx.extended_types import SXNG_Response
    from searx.search.processors import OnlineParams

about = {
    "website": "https://www.firecrawl.dev/",
    "wikidata_id": None,
    "official_api_documentation": "https://docs.firecrawl.dev/",
    "use_official_api": True,
    "require_api_key": True,
    "results": "JSON",
}

api_keys: list[str] = []
api_key: str = ""

categories = ["general", "web", "ai"]
paging = False
safesearch = False
time_range_support = False

timeout: float = 10.0

base_url = "https://api.firecrawl.dev/v1/search"

_pool: KeyPool | None = None


def init(_):
    global _pool
    if not api_keys:
        raise SearxEngineAPIException("No API keys provided for Firecrawl")

    def _validate(key: str, timeout: float) -> bool:
        return probe_post_json(
            base_url,
            {"Authorization": "Bearer " + key},
            {"query": "health", "limit": 1},
            timeout,
        )

    _pool = KeyPool(api_keys, _validate)


def request(query: str, params: "OnlineParams") -> None:
    key = _pool.pick()
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["Authorization"] = "Bearer " + key
    body = {
        "query": query,
        "limit": 10,
    }
    params["data"] = json.dumps(body)


def response(resp: "SXNG_Response") -> EngineResults:
    res = EngineResults()
    data = resp.json()
    for r in data.get("data", []):
        url = r.get("url", "")
        if not url:
            continue
        res.add(
            res.types.MainResult(
                url=url,
                title=r.get("title", "") or url,
                content=r.get("description", "") or r.get("markdown", ""),
            )
        )
    return res
