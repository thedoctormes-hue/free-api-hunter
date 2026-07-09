# SPDX-License-Identifier: AGPL-3.0-or-later
"""TinyFish search engine for SearXNG (Unified Search Gateway)."""

import typing as t

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

api_key: str = ""

categories = ["general", "web", "ai"]
paging = False
safesearch = False
time_range_support = False

base_url = "https://api.search.tinyfish.ai"


def init(_):
    if not api_key:
        raise SearxEngineAPIException("No API key provided for TinyFish")


def request(query: str, params: "OnlineParams") -> None:
    from urllib.parse import urlencode
    qs = urlencode({"query": query, "location": "US", "language": "en"})
    params["url"] = base_url + "?" + qs
    params["method"] = "GET"
    params["headers"]["X-API-Key"] = api_key
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
