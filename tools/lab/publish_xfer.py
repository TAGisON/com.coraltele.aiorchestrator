#!/usr/bin/env python3
"""Install English-authored coral-xfer, keep lab matrix extensions, publish."""
from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8011"


def api(method: str, path: str, body: dict | None = None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(
        BASE + path,
        data=data,
        method=method,
        headers={"Content-Type": "application/json", "Accept": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read().decode()
            return resp.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            payload = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            payload = {"raw": raw}
        return e.code, payload


def main() -> int:
    code, inst = api("POST", "/v1/desk-presets/coral-xfer", {"tenant_id": "default"})
    print("install", code, json.dumps(inst.get("desk", {}), indent=2)[:400])
    if code not in (200, 201):
        print("FAIL install", inst)
        return 1

    code, desk = api("GET", "/v1/desks/coral-xfer")
    if code != 200:
        print("FAIL get", desk)
        return 1
    doc = desk.get("document") or desk.get("draft") or {}
    if not doc:
        # Some APIs nest under desk
        doc = (desk.get("desk") or {}).get("document") or {}
    # Prefer draft document from install response shape
    code2, full = api("GET", "/v1/desks/coral-xfer")
    doc = full.get("document") or doc

    # Ensure lab transfer extensions (Configurator may have overridden).
    matrix = []
    for row in doc.get("matrix") or []:
        intent = row.get("intent")
        if intent == "sales":
            row = {**row, "number": "5002"}
        elif intent == "corporate":
            row = {**row, "number": "5003"}
        elif intent == "support":
            row = {**row, "number": "5004"}
        elif intent == "domain_faq":
            row = {**row, "number": "5002"}
        matrix.append(row)
    doc["matrix"] = matrix

    code, saved = api("PUT", "/v1/desks/coral-xfer/draft", {"document": doc})
    print("save draft", code)
    if code not in (200, 201):
        print("FAIL draft", saved)
        return 1

    code, pub = api("POST", "/v1/desks/coral-xfer/publish", {"published_by": "automation-xfer"})
    print("publish", code, json.dumps(pub, indent=2)[:800])
    if code not in (200, 201):
        print("FAIL publish", pub)
        return 1

    code, desk = api("GET", "/v1/desks/coral-xfer")
    print("desk", code, "version", (desk.get("desk") or desk).get("published_version") or desk.get("version"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
