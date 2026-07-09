# SPDX-License-Identifier: AGPL-3.0-or-later
"""Engine to search using the Serper (Google SERP) API.

.. _Serper API: https://serper.dev/

Configuration (yaml)
=====================

  - name: serper
    engine: serper
    api_key: '!env SERPER_API_KEY'   # required
    results_per_page: 10             # optional

Serper returns Google-fidelity organic results.
"""

import json
import typing as t

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults

if t.TYPE_CHECKING:
    from searx.extended_types import SXNG_Response
    from searx.search.processors import OnlineParams

about = {
    "website": "https://serper.dev/",
    "wikidata_id": None,
    "official_api_documentation": "https://serper.dev/",
    "use_official_api": True,
    "require_api_key": True,
    "results": "JSON",
}

api_key: str = ""
"""API key for Serper (required, from SERPER_API_KEY env)."""

categories = ["general", "web"]
paging = True
safesearch = False
time_range_support = False

results_per_page: int = 10
"""Maximum number of results per page (default 10)."""

base_url = "https://google.serper.dev/search"
"""Serper search endpoint (Google SERP)."""


def init(_):
    """Initialize the engine."""
    if not api_key:
        raise SearxEngineAPIException("No API key provided (SERPER_API_KEY)")


def request(query: str, params: "OnlineParams") -> None:
    """Create the API request (POST, JSON body)."""
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["X-API-KEY"] = api_key
    body: dict[str, t.Any] = {
        "q": query,
        "num": results_per_page,
    }
    if paging and params["pageno"] > 1:
        body["page"] = params["pageno"] - 1
    params["data"] = json.dumps(body)


def response(resp: "SXNG_Response") -> EngineResults:
    """Process the API response and return results."""
    res = EngineResults()
    data = resp.json()
    for r in data.get("organic", []):
        url = r.get("link", "")
        if not url:
            continue
        res.add(
            res.types.MainResult(
                url=url,
                title=r.get("title", "") or url,
                content=r.get("snippet", ""),
            )
        )
    return res
