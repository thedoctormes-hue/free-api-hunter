# SPDX-License-Identifier: AGPL-3.0-or-later
"""Firecrawl search engine for SearXNG (Unified Search Gateway)."""

import json
import typing as t

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults

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

api_key: str = ""

categories = ["general", "web", "ai"]
paging = False
safesearch = False
time_range_support = False

timeout: float = 10.0

base_url = "https://api.firecrawl.dev/v1/search"


def init(_):
    if not api_key:
        raise SearxEngineAPIException("No API key provided for Firecrawl")


def request(query: str, params: "OnlineParams") -> None:
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["Authorization"] = "Bearer " + api_key
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
