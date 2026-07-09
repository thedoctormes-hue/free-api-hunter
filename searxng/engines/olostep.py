# SPDX-License-Identifier: AGPL-3.0-or-later
"""Engine to search using the Olostep Web Data API (search + extract).

.. _Olostep API: https://www.olostep.com/

Configuration (yaml)
=====================

  - name: olostep
    engine: olostep
    api_key: '!env OLOSTEP_API_KEY'   # required
    results_per_page: 10              # optional

Olostep returns search results + structured extraction (search + scrape).
NOTE: endpoint/response shape verified against Olostep docs; adjust if API changes.
"""

import json
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

api_key: str = ""
"""API key for Olostep (required, from OLOSTEP_API_KEY env)."""

categories = ["general", "web"]
paging = True
safesearch = False
time_range_support = False

results_per_page: int = 10
"""Maximum number of results per page (default 10)."""

# Olostep Search endpoint (verify against current docs if API changes)
base_url = "https://api.olostep.com/v1/search"


def init(_):
    """Initialize the engine."""
    if not api_key:
        raise SearxEngineAPIException("No API key provided (OLOSTEP_API_KEY)")


def request(query: str, params: "OnlineParams") -> None:
    """Create the API request (POST, JSON body, Bearer auth)."""
    params["url"] = base_url
    params["method"] = "POST"
    params["headers"]["Content-Type"] = "application/json"
    params["headers"]["Authorization"] = f"Bearer {api_key}"
    body: dict[str, t.Any] = {
        "query": query,
        "num_results": results_per_page,
    }
    if paging and params["pageno"] > 1:
        body["offset"] = (params["pageno"] - 1) * results_per_page
    params["data"] = json.dumps(body)


def response(resp: "SXNG_Response") -> EngineResults:
    """Process the API response and return results."""
    res = EngineResults()
    data = resp.json()
    # Olostep wraps results in "results" (or "data"/"organic" depending on call)
    items = data.get("results", data.get("data", data.get("organic", [])))
    for r in items:
        url = r.get("url", "")
        if not url:
            continue
        res.add(
            res.types.MainResult(
                url=url,
                title=r.get("title", "") or url,
                content=r.get("text", "") or r.get("snippet", ""),
            )
        )
    return res
