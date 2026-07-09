# SPDX-License-Identifier: AGPL-3.0-or-later
"""Engine to search using the Exa AI Search API.

.. _Exa API: https://docs.exa.ai/

Configuration (yaml)
=====================

  - name: exa
    engine: exa
    api_key: '!env EXA_API_KEY'   # required
    results_per_page: 10          # optional

Exa returns semantic web results + extracted contents (AI-native retrieval).
"""

import json
import typing as t

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

api_key: str = ""
"""API key for Exa (required, from EXA_API_KEY env)."""

categories = ["general", "web", "ai"]
paging = True
safesearch = False
time_range_support = False

results_per_page: int = 10
"""Maximum number of results per page (default 10)."""

base_url = "https://api.exa.ai/search"
"""Exa search endpoint."""


def init(_):
    """Initialize the engine."""
    if not api_key:
        raise SearxEngineAPIException("No API key provided (EXA_API_KEY)")


def request(query: str, params: "OnlineParams") -> None:
    """Create the API request (POST, JSON body)."""
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["x-api-key"] = api_key
    body: dict[str, t.Any] = {
        "query": query,
        "numResults": results_per_page,
        "type": "keyword",
    }
    if paging and params["pageno"] > 1:
        # Exa uses cursor/offset via numResults window; simple page shift
        body["numResults"] = results_per_page
    params["data"] = json.dumps(body)


def response(resp: "SXNG_Response") -> EngineResults:
    """Process the API response and return results."""
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
                content=r.get("text", "") or r.get("summary", ""),
                publishedDate=r.get("publishedDate"),
            )
        )
    return res
