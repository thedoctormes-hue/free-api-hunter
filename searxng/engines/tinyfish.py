# SPDX-License-Identifier: AGPL-3.0-or-later
"""TinyFish search engine for SearXNG (key-rotating pool with failover)."""

import typing as t
from urllib.parse import urlencode

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults
from searx.engines._poolkeys import KeyPool, probe_get

if t.TYPE_CHECKING:
    from searx.extended_types import SXNG_Response
    from searx.search.processors import OnlineParams

about = {
    "website": "https://tinyfish.ai/",
    "wikidata_id": None,
    "official_api_documentation": "https://docs.tinyfish.ai/",
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

base_url = "https://api.search.tinyfish.ai"

_pool: KeyPool | None = None


def init(_):
    global _pool
    if not api_keys:
        raise SearxEngineAPIException("No API keys provided for TinyFish")

    def _validate(key: str, timeout: float) -> bool:
        qs = urlencode({"query": "health", "location": "US", "language": "en"})
        return probe_get(
            base_url + "?" + qs,
            {"X-API-Key": key, "Accept": "application/json"},
            timeout,
        )

    _pool = KeyPool(api_keys, _validate)


def request(query: str, params: "OnlineParams") -> None:
    key = _pool.pick()
    qs = urlencode({"query": query, "location": "US", "language": "en"})
    params["url"] = base_url + "?" + qs
    params["method"] = "GET"
    params["headers"]["X-API-Key"] = key
    params["headers"]["Accept"] = "application/json"


def response(resp: "SXNG_Response") -> EngineResults:
    res = EngineResults()
    data = resp.json()
    for r in data.get("results", []):
        url = r.get("url", "")
        if not url:
            continue
        res.add(
            res.types.MainResult(
                url=url,
                title=r.get("title", "") or url,
                content=r.get("content", "") or r.get("snippet", ""),
            )
        )
    return res
