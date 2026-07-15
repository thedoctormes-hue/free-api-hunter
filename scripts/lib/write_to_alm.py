#!/usr/bin/env python3
"""
Bridge: write VERIFIED search answers into AnythingLLM (semantic memory / ALM).

Hard gate (single source of truth) -- NO write happens unless ALL hold:
    answer_status        == "verified"
    url_overlap_verified is True
    answer_grounding     >= 0.75   (ALM_BRIDGE_GND, default 0.75)

Any other status (answer_ungrounded / unverified_synthesis /
single_source_unverified / both_sources_unavailable) or low grounding is
rejected with a log line and exit 0. Never raises into the orchestrator.

Transport: ALM REST API
    POST /api/v1/document/upload        (multipart file=@ + workspaceSlug)
    POST /api/v1/workspace/:slug/update-embeddings  {adds:[location]}
Target workspace is dedicated (ALM_BRIDGE_WORKSPACE, default raven-search-bridge)
so it never collides with anythingllm-sync (which mirrors git docs only).
"""
import os
import sys
import json
import time
import tempfile
import argparse

import requests

ALM_BASE = os.environ.get("ALM_BASE", "http://127.0.0.1:3002/api/v1")
WORKSPACE = os.environ.get("ALM_BRIDGE_WORKSPACE", "raven-search-bridge")
GND_GATE = float(os.environ.get("ALM_BRIDGE_GND", "0.75"))


def _read_token() -> str:
    p = "/root/LabDoctorM/workspaces/streikbrecher/secrets/anythingllm_token.txt"
    try:
        return open(p, encoding="utf-8").read().strip()
    except Exception:
        return ""


TOKEN = os.environ.get("ALM_TOKEN") or _read_token()


def log(m: str) -> None:
    print(f"[bridge-alm] {m}", file=sys.stderr)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--workspace", default=WORKSPACE)
    ap.add_argument("--dry-run", action="store_true",
                    help="log what would be written, do not upload")
    args = ap.parse_args()

    raw = sys.stdin.read()
    try:
        d = json.loads(raw, strict=False)
    except Exception as e:  # noqa: BLE001
        log(f"invalid JSON from orchestrator: {e}")
        return 0

    v = (d.get("_meta") or {}).get("verification") or {}
    status = v.get("answer_status")
    url_ok = v.get("url_overlap_verified")
    gnd = v.get("answer_grounding")
    answer = d.get("answer") or ""
    query = os.environ.get("QUERY", "")

    # ---- HARD GATE ----
    if status != "verified":
        log(f"GATE BLOCK: answer_status={status!r} (need 'verified') -> no write")
        return 0
    if not url_ok:
        log("GATE BLOCK: url_overlap_verified=False -> no write")
        return 0
    if gnd is None or gnd < GND_GATE:
        log(f"GATE BLOCK: answer_grounding={gnd} < {GND_GATE} -> no write")
        return 0
    if not answer.strip():
        log("GATE BLOCK: empty answer -> no write")
        return 0

    urls = v.get("overlapping_urls") or []
    src_line = "\n".join(f"- {u}" for u in urls) or "- (none)"
    doc = (
        "# Verified insight (Search->Memory bridge)\n"
        f"Query: {query}\n"
        f"Answer: {answer}\n"
        "Sources (overlapping URLs):\n"
        f"{src_line}\n"
        f"Grounding: {gnd} | Status: verified\n"
        "Bridge-generated: "
        f"{time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}\n"
    )

    if args.dry_run:
        log(f"DRY-RUN: would write {len(doc)} chars to workspace {args.workspace}")
        return 0

    headers = {"Authorization": f"Bearer {TOKEN}"}
    tf = tempfile.NamedTemporaryFile(
        "w", suffix=".md", delete=False, encoding="utf-8"
    )
    tf.write(doc)
    tf.close()
    try:
        r = requests.post(
            f"{ALM_BASE}/document/upload",
            headers=headers,
            files={"file": open(tf.name, "rb")},
            data={"workspaceSlug": args.workspace},
            timeout=30,
        )
        r.raise_for_status()
        body = r.json()
        loc = body["documents"][0].get("location") or body["documents"][0].get("url")
        requests.post(
            f"{ALM_BASE}/workspace/{args.workspace}/update-embeddings",
            headers={**headers, "Content-Type": "application/json"},
            json={"adds": [loc]},
            timeout=60,
        ).raise_for_status()
        log(f"WROTE verified insight to workspace {args.workspace} (loc={loc})")
    except Exception as e:  # noqa: BLE001
        log(f"UPLOAD FAILED: {e}")
        return 1
    finally:
        os.unlink(tf.name)
    return 0


if __name__ == "__main__":
    sys.exit(main())
