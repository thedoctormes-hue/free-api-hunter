# SPDX-License-Identifier: AGPL-3.0-or-later
"""Olostep search engine for SearXNG (key-rotating pool).

Uses Olostep /v1/searches endpoint (AI-powered web search returning
source links with title + description). One engine, N API accounts.
"""

import json
import threading
import typing as t

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults

if t.TYPE_CHECKING:
    from searx.extended_types import SXNG_Response
    from searx.search.processors import OnlineParams

about = {
    "website": "https://www.olostep.com/",
    "wikidata_id": None,
    "official_api_documentation": "https://www.olostep.com/blog/olostep-web-data-api-for-ai-agents",
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

base_url = "https://api.olostep.com/v1/searches"


def init(_):
    """Initialize the engine."""
    global api_key
    if not api_keys:
        raise SearxEngineAPIException("No API keys provided for Olostep")
    api_key = api_keys[0]


def _next_key() -> str:
    global _api_idx
    with _api_lock:
        k = api_keys[_api_idx % len(api_keys)]
        _api_idx += 1
    return k


def request(query: str, params: "OnlineParams") -> None:
    """Create the API request (POST, JSON body, Bearer auth)."""
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["Authorization"] = "Bearer " + _next_key()
    body = {"query": query}
    params["data"] = json.dumps(body)


def response(resp: "SXNG_Response") -> EngineResults:
    """Process the API response and return results."""
    res = EngineResults()
    data = resp.json()
    # /v1/searches returns result.json_content as a JSON string
    jc = data.get("result", {}).get("json_content")
    if isinstance(jc, str):
        try:
            jc = json.loads(jc)
        except Exception:
            jc = {}
    links = []
    if isinstance(jc, dict):
        links = jc.get("links", [])
    elif isinstance(jc, list):
        links = jc
    for r in links:
        url = r.get("url", "")
        if not url:
            continue
        res.add(
            res.types.MainResult(
                url=url,
                title=r.get("title", "") or url,
                content=r.get("description", "") or r.get("snippet", ""),
            )
        )
    return res
