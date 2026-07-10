# SPDX-License-Identifier: AGPL-3.0-or-later
"""Engine to search using the Exa AI Search API (key-rotating pool with failover).

One SearXNG engine, N API accounts. Each request validates the next key with a
minimal API call *before* handing it to the framework, so a dead/rate-limited
key never reaches the framework (which would suspend the whole engine).
"""

import json
import typing as t
from datetime import datetime

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults
from searx.engines._poolkeys import KeyPool, probe_post_json

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

categories = ["general", "web", "ai"]
paging = True
safesearch = False
time_range_support = False

results_per_page: int = 10

base_url = "https://api.exa.ai/search"

_pool: KeyPool | None = None


def init(_):
    """Initialize the engine."""
    global _pool
    if not api_keys:
        raise SearxEngineAPIException("No API keys provided for Exa")

    def _validate(key: str, timeout: float) -> bool:
        return probe_post_json(
            base_url,
            {"x-api-key": key},
            {"query": "health", "numResults": 1, "type": "keyword"},
            timeout,
        )

    _pool = KeyPool(api_keys, _validate)


def request(query: str, params: "OnlineParams") -> None:
    """Create the API request (POST, JSON body) using a validated key."""
    key = _pool.pick()
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["x-api-key"] = key
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
