#!/usr/bin/env python3
"""Install English-authored coral-tfn, set lab matrix extensions, publish v5."""
from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8011"


def api(method: str, path: str, body=None):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            raw = r.read().decode("utf-8")
            return r.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        try:
            j = json.loads(raw)
        except json.JSONDecodeError:
            j = {"raw": raw}
        return e.code, j


def main() -> int:
    code, inst = api("POST", "/v1/desk-presets/coral-tfn", {"tenant_id": "default"})
    print("install", code, "publishable", (inst.get("checklist") or {}).get("publishable"))
    if code >= 400:
        print(inst)
        return 1

    code, desk = api("GET", "/v1/desks/coral-tfn")
    doc = desk["document"]
    ext = {
        "sales_enquiry": ("sales", "5002"),
        "product_information": ("sales", "5002"),
        "technical_support": ("technical_support", "5004"),
        "service_complaint": ("service", "5004"),
    }
    matrix = []
    for row in doc.get("matrix") or []:
        intent = row.get("intent")
        row = dict(row)
        if intent in ext:
            q, n = ext[intent]
            row["target"] = q
            row["number"] = n
        matrix.append(row)
    doc["matrix"] = matrix
    cx = dict(doc.get("cx") or {})
    cx["welcome_barge_allowed"] = False
    cx["locale_synthesis"] = True
    cx["primary_locale"] = "en-IN"
    cx["listen_while_speak"] = True
    cx["barge_in"] = True
    cx["rtp_settle_ms"] = 400
    cx["min_barge_chars"] = 3
    cx["min_barge_ms"] = 280
    cx["barge_partial_confidence"] = 0.70
    # India multilingual runtime; authoring languages stay English-only in doc.languages
    cx["runtime_languages"] = [
        "en-IN", "hi-IN", "bn-IN", "ta-IN", "te-IN", "mr-IN",
        "gu-IN", "kn-IN", "ml-IN", "pa-IN", "or-IN", "as-IN",
    ]
    doc["cx"] = cx
    doc["default_language"] = "en-IN"
    doc["languages"] = ["en-IN"]

    code, saved = api("PUT", "/v1/desks/coral-tfn/draft", {"document": doc})
    print("save", code, "publishable", (saved.get("checklist") or {}).get("publishable"))
    if code >= 400:
        print(saved)
        return 1

    code, pub = api("POST", "/v1/desks/coral-tfn/publish", {"published_by": "automation-v5"})
    print("publish", code)
    print(json.dumps({k: pub.get(k) for k in ("desk_version", "profile_version", "content_hash")}, indent=2))
    if code >= 400:
        print(pub)
        return 1

    code, desk = api("GET", "/v1/desks/coral-tfn")
    print("published", desk.get("published"))
    print("welcome", desk["document"]["prompts"]["welcome"]["text"])
    print("default", desk["document"]["default_language"])
    print("barge", desk["document"]["cx"].get("welcome_barge_allowed"))
    print("synth", desk["document"]["cx"].get("locale_synthesis"))
    print("runtime", len(desk["document"]["cx"].get("runtime_languages") or []))
    for m in desk["document"]["matrix"]:
        print(" ", m.get("intent"), m.get("target"), m.get("number"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
