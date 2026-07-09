# SPDX-License-Identifier: AGPL-3.0-or-later
"""TinyFish search engine for SearXNG (key-rotating pool)."""

import threading
import typing as t
from urllib.parse import urlencode

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults

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

_api_idx = 0
_api_lock = threading.Lock()

categories = ["general", "web", "ai"]
paging = False
safesearch = False
time_range_support = False

base_url = "https://api.search.tinyfish.ai"


def init(_):
    global api_key
    if not api_keys:
        raise SearxEngineAPIException("No API keys provided for TinyFish")
    api_key = api_keys[0]


def _next_key() -> str:
    global _api_idx
    with _api_lock:
        k = api_keys[_api_idx % len(api_keys)]
        _api_idx += 1
    return k


def request(query: str, params: "OnlineParams") -> None:
    qs = urlencode({"query": query, "location": "US", "language": "en"})
    params["url"] = base_url + "?" + qs
    params["method"] = "GET"
    params["headers"]["X-API-Key"] = _next_key()
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
