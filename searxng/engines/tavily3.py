# SPDX-License-Identifier: AGPL-3.0-or-later
"""Tavily search engine for SearXNG (Unified Search Gateway)."""

import json
import typing as t

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults

if t.TYPE_CHECKING:
    from searx.extended_types import SXNG_Response
    from searx.search.processors import OnlineParams

about = {
    "website": "https://tavily.com/",
    "wikidata_id": None,
    "official_api_documentation": "https://docs.tavily.com/",
    "use_official_api": True,
    "require_api_key": True,
    "results": "JSON",
}

api_key: str = ""

categories = ["general", "web", "ai"]
paging = False
safesearch = False
time_range_support = False

base_url = "https://api.tavily.com/search"


def init(_):
    if not api_key:
        raise SearxEngineAPIException("No API key provided for Tavily")


def request(query: str, params: "OnlineParams") -> None:
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["Authorization"] = "Bearer " + api_key
    body = {
        "query": query,
        "max_results": 10,
        "search_depth": "basic",
    }
    params["data"] = json.dumps(body)


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
                content=r.get("content", ""),
            )
        )
    return res
