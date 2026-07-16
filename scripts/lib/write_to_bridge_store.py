#!/usr/bin/env python3
"""
Bridge: persist VERIFIED search answers into RAVEN'S OWN local store.

WHY THIS EXISTS (INC-2026-07-16-000300-raven-alm-trespass + RUL-009):
  The laboratory's canonical semantic memory is the `memory-gateway` MCP
  server (OpenClaw-managed, ANT-VERIFIED 2026-07-16). It is a READ-ONLY
  gateway: it exposes only `search_memory`, `get_document`, `gateway_health`
  and has NO write/ingest tool. The deprecated standalone AnythingLLM instance is
  outlawed, and owned by another agent. Raven is explicitly forbidden from
  writing to or indexing shared/legacy memory (it must not be a "second
  indexer"). The stolen ALM token is never read.

  Therefore this bridge does NOT write to that legacy instance and does NOT touch any other
  agent's credentials. Its sanctioned destination is RAVEN'S OWN store, with
  an OPTIONAL owner-gated handoff queue that the ALM owner can ingest through
  the sanctioned sync pipeline. This is exactly the lesson recorded in the
  incident: "verified answer -> into its own storage or handoff to owner via
  a sanctioned path."

Hard gate (single source of truth) -- NO write happens unless ALL hold:
    answer_status        == "verified"
    url_overlap_verified is True
    answer_grounding     >= 0.75   (BRIDGE_GND, default 0.75)

Any other status (answer_ungrounded / unverified_synthesis /
single_source_unverified / both_sources_unavailable) or low grounding is
rejected with a log line and exit 0. Never raises into the orchestrator.

Transport:
  Primary (always, compliant):
      write a markdown insight + sidecar JSON into BRIDGE_STORE_DIR
      (default: <repo>/data/verified-insights). This is raven's own store.

  Optional owner-gated handoff (disabled by default):
      if MEMORY_GATEWAY_WRITE_URL is set AND a token is supplied via
      MEMORY_GATEWAY_WRITE_TOKEN (env) or MEMORY_GATEWAY_WRITE_TOKEN_FILE
      (path), also POST the insight to the OWNER-SANCTIONED memory-gateway
      write endpoint. Nothing is hardcoded; the hook is inert unless the
      owner explicitly configures it. Absence of this config is normal and
      never an error.
"""
import os
import sys
import json
import time
import hashlib
import argparse

# ── Config (env-driven; no hardcoded endpoints, no hardcoded tokens) ──────
STORE_DIR = os.environ.get(
    "BRIDGE_STORE_DIR",
    os.path.join(
        os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
        "data", "verified-insights",
    ),
)
HANDOFF_DIR = os.environ.get("BRIDGE_HANDOFF_DIR", "")  # optional owner queue
GND_GATE = float(os.environ.get("BRIDGE_GND", "0.75"))

# Owner-gated, opt-in handoff to a sanctioned memory-gateway write endpoint.
# Both the URL and the token MUST be provided by the owner via env; we never
# hardcode them and never fall back to another agent's secret file.
MG_WRITE_URL = os.environ.get("MEMORY_GATEWAY_WRITE_URL", "")
MG_WRITE_TOKEN = os.environ.get("MEMORY_GATEWAY_WRITE_TOKEN", "")
MG_WRITE_TOKEN_FILE = os.environ.get("MEMORY_GATEWAY_WRITE_TOKEN_FILE", "")


def log(m: str) -> None:
    print(f"[bridge-store] {m}", file=sys.stderr)


def _slug(query: str, answer: str) -> str:
    h = hashlib.sha256(f"{query}|{answer}".encode("utf-8")).hexdigest()[:12]
    stamp = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
    safe_q = "".join(c if c.isalnum() else "_" for c in query[:40]).strip("_")
    return f"{stamp}_{safe_q}_{h}"


def _load_handoff_token() -> str:
    if MG_WRITE_TOKEN:
        return MG_WRITE_TOKEN
    if MG_WRITE_TOKEN_FILE:
        try:
            return open(MG_WRITE_TOKEN_FILE, encoding="utf-8").read().strip()
        except Exception:
            return ""
    return ""


def _handoff_to_owner(doc: dict, meta: dict) -> bool:
    """Owner-gated forward to a sanctioned memory-gateway write endpoint.

    Returns True if a handoff was attempted & succeeded, False otherwise.
    Silently no-ops (returns False) when the owner has not sanctioned it.
    """
    if not MG_WRITE_URL:
        return False
    token = _load_handoff_token()
    if not token:
        log("HANDOFF SKIP: MEMORY_GATEWAY_WRITE_URL set but no token "
            "(MEMORY_GATEWAY_WRITE_TOKEN / _TOKEN_FILE) -> not forwarding")
        return False
    try:
        import requests  # local import: only needed for the opt-in hook
    except Exception as e:  # noqa: BLE001
        log(f"HANDOFF SKIP: requests unavailable ({e})")
        return False
    payload = {
        "query": meta.get("query", ""),
        "answer": meta.get("answer", ""),
        "sources": meta.get("sources", []),
        "grounding": meta.get("grounding"),
        "status": "verified",
        "generated_at": meta.get("generated_at", ""),
        "bridge": "raven/search-memory-bridge",
    }
    try:
        r = requests.post(
            MG_WRITE_URL,
            headers={"Authorization": f"Bearer {token}",
                     "Content-Type": "application/json"},
            json=payload,
            timeout=30,
        )
        r.raise_for_status()
        log(f"HANDOFF OK -> owner-sanctioned endpoint {MG_WRITE_URL}")
        return True
    except Exception as e:  # noqa: BLE001
        log(f"HANDOFF FAILED: {e} (insight still persisted to own store)")
        return False


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true",
                    help="log what would be written, do not persist")
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
    ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    doc_md = (
        "# Verified insight (Search->Memory bridge)\n"
        f"Query: {query}\n"
        f"Answer: {answer}\n"
        "Sources (overlapping URLs):\n"
        f"{src_line}\n"
        f"Grounding: {gnd} | Status: verified\n"
        f"Bridge-generated: {ts}\n"
        "\n> Stored in raven's OWN store (RUL-009 / INC-2026-07-16-000300).\n"
        "> Not written to shared/legacy memory. Owner may ingest via sanctioned path.\n"
    )

    if args.dry_run:
        log(f"DRY-RUN: would write {len(doc_md)} chars to {STORE_DIR}")
        return 0

    # ---- Persist to raven's own store (compliant primary path) ----
    try:
        os.makedirs(STORE_DIR, exist_ok=True)
    except Exception as e:  # noqa: BLE001
        log(f"STORE MKDIR FAILED: {e}")
        return 1

    slug = _slug(query, answer)
    md_path = os.path.join(STORE_DIR, f"{slug}.md")
    json_path = os.path.join(STORE_DIR, f"{slug}.json")
    sidecar = {
        "query": query,
        "answer": answer,
        "sources": urls,
        "grounding": gnd,
        "status": "verified",
        "generated_at": ts,
        "bridge": "raven/search-memory-bridge",
        "owner_sanctioned_ingest": bool(MG_WRITE_URL),
    }
    try:
        with open(md_path, "w", encoding="utf-8") as f:
            f.write(doc_md)
        with open(json_path, "w", encoding="utf-8") as f:
            json.dump(sidecar, f, ensure_ascii=False, indent=2)
        log(f"WROTE verified insight to OWN store: {md_path}")
    except Exception as e:  # noqa: BLE001
        log(f"STORE WRITE FAILED: {e}")
        return 1

    # ---- Optional owner-gated handoff queue (separate artifact) ----
    if HANDOFF_DIR:
        try:
            os.makedirs(HANDOFF_DIR, exist_ok=True)
            hop = os.path.join(HANDOFF_DIR, f"{slug}.json")
            with open(hop, "w", encoding="utf-8") as f:
                json.dump(sidecar, f, ensure_ascii=False, indent=2)
            log(f"HANDOFF QUEUED for owner ingest: {hop}")
        except Exception as e:  # noqa: BLE001
            log(f"HANDOFF QUEUE FAILED: {e}")

    # ---- Optional owner-sanctioned live forward (opt-in, inert by default) ----
    _handoff_to_owner(doc_md, sidecar)

    return 0


if __name__ == "__main__":
    sys.exit(main())
