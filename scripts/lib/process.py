#!/usr/bin/env python3
# process.py — постобработка результатов поиска для free-api-hunter оркестратора.
# Фичи: freshness-скоринг, нормализация URL, дедуп/мерж across-providers,
# детект противоречий, эвристическая декомпозиция запроса.
# Используется оркестратором через: echo "$JSON" | python3 lib/process.py <action>
import json
import sys
import re
import os
from datetime import datetime, timezone
from urllib.parse import urlparse

NOW = datetime.now(timezone.utc)


def normalize_url(u):
    if not u:
        return ""
    u = u.strip()
    u = re.sub(r'^https?://', '', u)
    u = re.sub(r'^www\.', '', u)
    u = u.split('#')[0]
    u = u.split('?')[0]
    u = u.rstrip('/')
    return u.lower()


def host_of(url):
    if not url:
        return ""
    try:
        return urlparse(url).netloc.lower()
    except Exception:
        return ""


_POLICY_CACHE = None


def load_domain_policy():
    """Загрузить configs/domain_policy.json (O4). Graceful: нет файла -> None."""
    global _POLICY_CACHE
    if _POLICY_CACHE is not None:
        return _POLICY_CACHE
    path = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                         '..', '..', 'configs', 'domain_policy.json')
    try:
        with open(path) as fh:
            p = json.load(fh)
    except Exception:
        p = None
    _POLICY_CACHE = p
    return p


def _match(host, pat):
    if not pat:
        return False
    pat = pat.lower()
    if pat.startswith('*.'):
        return host == pat[2:] or host.endswith('.' + pat[2:])
    if pat.endswith('.*'):
        return host.startswith(pat[:-2] + '.')
    return host == pat


def domain_score(host, policy):
    """O4: +балл за promote, -балл за demote, block -> сильный штраф. Нет политики -> 0."""
    if not host or not policy:
        return 0.0
    scores = policy.get('scores', {}) or {}
    promo = float(scores.get('promote', 0.0))
    demo = float(scores.get('demote', 0.0))
    blk = float(scores.get('block', -1.0))
    for pat in (policy.get('block', []) or []):
        if _match(host, pat):
            return blk
    for pat in (policy.get('promote', []) or []):
        if _match(host, pat):
            return promo
    for pat in (policy.get('demote', []) or []):
        if _match(host, pat):
            return demo
    return 0.0


def extract_date(text):
    if not text:
        return None
    m = re.search(r'(\d{4}-\d{2}-\d{2})', text)
    if m:
        try:
            return datetime.strptime(m.group(1), '%Y-%m-%d').replace(tzinfo=timezone.utc)
        except Exception:
            pass
    m = re.search(r'([A-Za-z]+ \d{1,2},? \d{4})', text)
    if m:
        for fmt in ('%b %d, %Y', '%B %d, %Y', '%b %d %Y', '%B %d %Y'):
            try:
                return datetime.strptime(m.group(1), fmt).replace(tzinfo=timezone.utc)
            except Exception:
                pass
    m = re.search(r'\b(19|20)\d{2}\b', text)
    if m:
        try:
            return datetime(int(m.group(0)), 1, 1, tzinfo=timezone.utc)
        except Exception:
            pass
    return None


def freshness_score(dt):
    if not dt:
        return None
    age = (NOW - dt).days
    if age < 0:
        age = 0
    half_life = 180
    return round(0.5 ** (age / half_life), 3)


def add_freshness(results):
    for r in results:
        text = " ".join(str(r.get(k, '')) for k in ('title', 'content', 'snippet', 'url'))
        dt = extract_date(text)
        if dt:
            r.setdefault('_meta', {})
            r['_meta']['published_date'] = dt.strftime('%Y-%m-%d')
            r['_meta']['age_days'] = (NOW - dt).days
            r['_meta']['freshness_score'] = freshness_score(dt)
    return results


def normalize_results(provider_name, data):
    out = []
    items = []
    if isinstance(data, dict):
        if 'results' in data and isinstance(data['results'], list):
            items = data['results']
        elif 'data' in data and isinstance(data['data'], list):
            items = data['data']
    for it in items:
        if not isinstance(it, dict):
            continue
        url = it.get('url') or it.get('link') or ''
        title = it.get('title') or (str(it.get('content', ''))[:80])
        snippet = it.get('content') or it.get('snippet') or it.get('description') or ''
        out.append({
            'provider': provider_name,
            'url': url,
            'title': title,
            'snippet': snippet,
        })
    return out


def merge_results(providers_dict):
    norm = []
    for name, p in providers_dict.items():
        if isinstance(p, dict) and p.get('status') == 'ok':
            norm.extend(normalize_results(name, p.get('data', {})))
    seen = {}
    merged = []
    for r in norm:
        key = normalize_url(r['url'])
        if not key:
            key = 't:' + re.sub(r'\W+', ' ', (r['title'] or '')).strip().lower()[:120]
        if key in seen:
            e = seen[key]
            e['provider_count'] = e.get('provider_count', 1) + 1
            e['providers'] = sorted(set(e.get('providers', [e['provider']]) + [r['provider']]))
            if len(r['snippet']) > len(e['snippet']):
                e['snippet'] = r['snippet']
        else:
            r['provider_count'] = 1
            r['providers'] = [r['provider']]
            seen[key] = r
            merged.append(r)
    add_freshness(merged)
    policy = load_domain_policy()
    for r in merged:
        f = r.get('_meta', {}).get('freshness_score')
        f = f if f is not None else 0.5
        pc = r.get('provider_count', 1)
        dp = domain_score(host_of(r.get('url', '')), policy)
        r['_domain_priority'] = dp
        conf = 0.4 + 0.2 * (pc - 1) + 0.4 * f + dp
        r['_confidence'] = round(min(1.0, max(0.0, conf)), 3)
    merged.sort(key=lambda x: (x.get('provider_count', 1),
                                x.get('_domain_priority', 0),
                                x.get('_meta', {}).get('freshness_score') or 0),
                reverse=True)
    return merged


def detect_contradictions(merged, query):
    contradictions = []
    ver_re = re.compile(r'\b(v?\d+\.\d+(?:\.\d+)?)\b')
    year_re = re.compile(r'\b(19|20)\d{2}\b')
    versions = set()
    years = set()
    for r in merged:
        text = (r.get('title', '') + ' ' + r.get('snippet', ''))
        for m in ver_re.findall(text):
            versions.add(m)
        for m in year_re.findall(text):
            years.add(m)
    if len(versions) > 1:
        contradictions.append({'type': 'version_conflict',
                               'values': sorted(versions),
                               'note': 'разные версии в источниках — требует ручной проверки'})
    if len(years) > 2:
        contradictions.append({'type': 'year_spread',
                               'values': sorted(years),
                               'note': 'большой разброс по годам — уточни актуальность'})
    return contradictions


def decompose(query):
    parts = re.split(r'\bvs\.?\b|\bversus\b|\bcompared to\b|\bor\b', query, flags=re.I)
    parts = [p.strip() for p in parts if p.strip()]
    return parts[:4]


def synthesize(d):
    """Построить answer из агрегированных результатов, если он не задан провайдером."""
    if not isinstance(d, dict):
        return d
    if d.get('answer'):
        d.setdefault('_meta', {})
        d['_meta']['synthesis'] = 'provider'
        return d
    results = d.get('results', [])
    if not results:
        d['answer'] = None
        d.setdefault('_meta', {})
        d['_meta']['synthesis'] = 'no_results'
        return d
    top = results[:5]
    bullets = []
    for r in top:
        title = (r.get('title') or '').strip()
        url = (r.get('url') or '').strip()
        snip = (r.get('snippet') or r.get('content') or '').strip().replace('\n', ' ')
        line = f"- {title}" if title else f"- {url}"
        if snip:
            line += f": {snip[:220]}"
        bullets.append(line)
    header = f"Synthesized from {len(results)} aggregated sources (top {len(top)} shown):"
    d['answer'] = header + "\n" + "\n".join(bullets)
    d.setdefault('_meta', {})
    d['_meta']['synthesis'] = 'generated'
    d['_meta']['synthesis_source_count'] = len(results)
    return d


def main():
    raw = sys.stdin.read()
    try:
        d = json.loads(raw)
    except Exception:
        print(raw)
        sys.exit(0)
    action = sys.argv[1] if len(sys.argv) > 1 else 'merge'
    if action == 'merge' and isinstance(d, dict) and 'providers' in d:
        merged = merge_results(d['providers'])
        d['results'] = merged
        d.setdefault('_meta', {})
        d['_meta']['merged_count'] = len(merged)
        d['_meta']['contradictions'] = detect_contradictions(merged, d.get('query', ''))
        print(json.dumps(d, ensure_ascii=False))
    elif action == 'freshness' and isinstance(d, dict):
        if 'results' in d and isinstance(d['results'], list):
            add_freshness(d['results'])
        elif 'data' in d and isinstance(d['data'], list):
            add_freshness(d['data'])
        print(json.dumps(d, ensure_ascii=False))
    elif action == 'decompose':
        q = d.get('query', '') if isinstance(d, dict) else str(d)
        print(json.dumps({'subqueries': decompose(q)}, ensure_ascii=False))
    elif action == 'merge_subs' and isinstance(d, dict):
        subfile = sys.argv[2] if len(sys.argv) > 2 else None
        subs = []
        if subfile and os.path.exists(subfile):
            for line in open(subfile):
                line = line.strip()
                if line:
                    try:
                        subs.append(json.loads(line))
                    except Exception:
                        pass
        combined = list(d.get('results', []))
        for s in subs:
            if isinstance(s, list):
                combined.extend(s)
        seen = {}
        merged = []
        for r in combined:
            key = normalize_url(r.get('url', ''))
            if not key:
                key = 't:' + re.sub(r'\W+', ' ', (r.get('title', '') or '')).strip().lower()[:120]
            if key in seen:
                seen[key]['provider_count'] = seen[key].get('provider_count', 1) + 1
            else:
                r['provider_count'] = 1
                seen[key] = r
                merged.append(r)
        add_freshness(merged)
        policy = load_domain_policy()
        for r in merged:
            f = r.get('_meta', {}).get('freshness_score')
            f = f if f is not None else 0.5
            pc = r.get('provider_count', 1)
            dp = domain_score(host_of(r.get('url', '')), policy)
            r['_domain_priority'] = dp
            conf = 0.4 + 0.2 * (pc - 1) + 0.4 * f + dp
            r['_confidence'] = round(min(1.0, max(0.0, conf)), 3)
        merged.sort(key=lambda x: (x.get('provider_count', 1),
                                    x.get('_domain_priority', 0),
                                    x.get('_meta', {}).get('freshness_score') or 0),
                    reverse=True)
        d['results'] = merged
        d.setdefault('_meta', {})
        d['_meta']['merged_count'] = len(merged)
        d['_meta']['contradictions'] = detect_contradictions(merged, d.get('query', ''))
        d['_meta']['decomposed'] = True
        print(json.dumps(d, ensure_ascii=False))
    elif action == 'synthesize' and isinstance(d, dict):
        synthesize(d)
        print(json.dumps(d, ensure_ascii=False))


if __name__ == '__main__':
    main()
