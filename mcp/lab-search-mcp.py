#!/usr/bin/env python3
"""Lab Search MCP — native web search + deep research for OpenClaw agents.

Wraps SearXNG (Unified Search Gateway) for web_search and the /research
orchestrator for deep_research. Engine pool is explicit so we bypass
SearXNG's default engine selection (which ignores our custom general engines).
"""
import asyncio
import json
import os
import subprocess
import urllib.parse
import urllib.request
from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import Tool, TextContent

SEARXNG = os.environ.get("SEARXNG_URL", "http://localhost:8889/search")
POOL = os.environ.get("LABSEARCH_POOL", "exa,exa2,exa3,exa4,exa5")
ORCHESTRATOR = "/root/LabDoctorM/projects/free-api-hunter/scripts/search-orchestrator.sh"

app = Server("lab-search")


@app.list_tools()
async def list_tools():
    return [
        Tool(
            name="web_search",
            description="Web search via SearXNG pool (wiby, marginalia, bing, seznam, exa, wikipedia). Returns merged results with title/url/snippet.",
            inputSchema={
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Search query"},
                    "max_results": {"type": "integer", "default": 10, "description": "Max results to return"},
                },
                "required": ["query"],
            },
        ),
        Tool(
            name="deep_research",
            description="Deep research via /research orchestrator (verify + merge + synthesis). Returns a synthesized answer.",
            inputSchema={
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Research question"},
                },
                "required": ["query"],
            },
        ),
    ]


@app.call_tool()
async def call_tool(name: str, args: dict):
    if name == "web_search":
        q = args.get("query", "")
        mr = int(args.get("max_results", 10))
        params = urllib.parse.urlencode({"q": q, "format": "json", "engines": POOL})
        url = f"{SEARXNG}?{params}"
        try:
            with urllib.request.urlopen(url, timeout=20) as r:
                data = json.load(r)
            results = data.get("results", [])[:mr]
            if not results:
                return [TextContent(type="text", text="No results from search pool.")]
            blocks = []
            for res in results:
                title = res.get("title", "")
                link = res.get("url", "")
                snippet = (res.get("content") or "")[:300]
                blocks.append(f"{title}\n{link}\n{snippet}")
            return [TextContent(type="text", text="\n\n".join(blocks))]
        except Exception as e:
            return [TextContent(type="text", text=f"Search error: {e}")]

    if name == "deep_research":
        q = args.get("query", "")
        try:
            res = subprocess.run(
                [ORCHESTRATOR, "deep_research", q],
                capture_output=True, text=True, timeout=180,
            )
            out = res.stdout.strip() or res.stderr.strip()
            return [TextContent(type="text", text=out or "No research output.")]
        except Exception as e:
            return [TextContent(type="text", text=f"Research error: {e}")]

    return [TextContent(type="text", text=f"Unknown tool: {name}")]


async def main():
    async with stdio_server() as (read, write):
        await app.run(read, write, app.create_initialization_options())


if __name__ == "__main__":
    asyncio.run(main())
