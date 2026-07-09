# SPDX-License-Identifier: AGPL-3.0-or-later
"""Engine to search using the Exa AI Search API (key-rotating pool).

One SearXNG engine, N API accounts. Each request picks the next key via
round-robin so the free-tier quota of all accounts is shared/balanced.
"""

import json
import threading
import typing as t
from datetime import datetime

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults

if t.TYPE_CHECKING:
    from searx.extended_types import SXNG_Response
    from searx.search.processors import OnlineParams

about = {
    "website": "https://exa.ai/",
    "wikidata_id": None,
    "official_api_documentation": "https://docs.exa.ai/",
    "use_official_api": True,
    "require_api_key": True,
    "results": "JSON",
}

# List of API keys (accounts). Provided via settings.yml `api_keys:`.
api_keys: list[str] = []
# Kept for SearXNG compatibility (require_api_key check).
api_key: str = ""

_api_idx = 0
_api_lock = threading.Lock()

categories = ["general", "web", "ai"]
paging = True
safesearch = False
time_range_support = False

results_per_page: int = 10

base_url = "https://api.exa.ai/search"


def init(_):
    """Initialize the engine."""
    global api_key
    if not api_keys:
        raise SearxEngineAPIException("No API keys provided for Exa")
    api_key = api_keys[0]


def _next_key() -> str:
    global _api_idx
    with _api_lock:
        k = api_keys[_api_idx % len(api_keys)]
        _api_idx += 1
    return k


def request(query: str, params: "OnlineParams") -> None:
    """Create the API request (POST, JSON body)."""
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["x-api-key"] = _next_key()
    body: dict[str, t.Any] = {
        "query": query,
        "numResults": results_per_page,
        "type": "keyword",
    }
    params["data"] = json.dumps(body)


def response(resp: "SXNG_Response") -> EngineResults:
    """Process the API response and return results."""
    res = EngineResults()
    data = resp.json()
    for r in data.get("results", []):
        url = r.get("url", "")
        if not url:
            continue
        pd = None
        if r.get("publishedDate"):
            try:
                pd = datetime.fromisoformat(r["publishedDate"].replace("Z", "+00:00"))
            except Exception:
                pd = None
        res.add(
            res.types.MainResult(
                url=url,
                title=r.get("title", "") or url,
                content=r.get("text", "") or r.get("summary", ""),
                publishedDate=pd,
            )
        )
    return res
