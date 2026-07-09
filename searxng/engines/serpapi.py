# SPDX-License-Identifier: AGPL-3.0-or-later
"""Engine to search using the SerpApi (Google SERP) API.

.. _SerpApi: https://serpapi.com/

Configuration (yaml)
=====================

  - name: serpapi
    engine: serpapi
    api_key: '!env SERPAPI_API_KEY'   # required
    results_per_page: 10              # optional

SerpApi returns Google-fidelity organic results (scraper-based).
"""

import typing as t
from urllib.parse import urlencode

from searx.exceptions import SearxEngineAPIException
from searx.result_types import EngineResults

if t.TYPE_CHECKING:
    from searx.extended_types import SXNG_Response
    from searx.search.processors import OnlineParams

about = {
    "website": "https://serpapi.com/",
    "wikidata_id": None,
    "official_api_documentation": "https://serpapi.com/search-api",
    "use_official_api": True,
    "require_api_key": True,
    "results": "JSON",
}

api_key: str = ""
"""API key for SerpApi (required, from SERPAPI_API_KEY env)."""

categories = ["general", "web"]
paging = True
safesearch = False
time_range_support = False

results_per_page: int = 10
"""Maximum number of results per page (default 10)."""

base_url = "https://serpapi.com/search"
"""SerpApi search endpoint."""


def init(_):
    """Initialize the engine."""
    if not api_key:
        raise SearxEngineAPIException("No API key provided (SERPAPI_API_KEY)")


def request(query: str, params: "OnlineParams") -> None:
    """Create the API request (GET, query string)."""
    search_args: dict[str, t.Any] = {
        "q": query,
        "api_key": api_key,
        "num": results_per_page,
        "start": (params["pageno"] - 1) * results_per_page,
    }
    params["url"] = f"{base_url}?{urlencode(search_args)}"


def response(resp: "SXNG_Response") -> EngineResults:
    """Process the API response and return results."""
    res = EngineResults()
    data = resp.json()
    for r in data.get("organic_results", []):
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
