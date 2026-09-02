#!/usr/bin/env python3
"""Multi-language text-call / simulate matrix against published coral-tfn."""
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
        with urllib.request.urlopen(req, timeout=120) as r:
            raw = r.read().decode("utf-8")
            return r.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        try:
            j = json.loads(raw)
        except json.JSONDecodeError:
            j = {"raw": raw}
        return e.code, j


def trunc(s: str, n: int) -> str:
    s = s or ""
    return s if len(s) <= n else s[:n] + "…"


def sim(title, lang, turns, expect_disp=None, expect_transfer_num=None):
    code, out = api("POST", "/v1/desks/coral-tfn/simulate", {"language": lang, "turns": turns})
    ok = code < 400
    disp = out.get("disposition")
    attrs = out.get("attributes") or {}
    transfer_num = attrs.get("transfer_number") or ""
    issues = []
    if not ok:
        issues.append(f"http={code}")
    if expect_disp and disp != expect_disp:
        issues.append(f"disp want={expect_disp} got={disp}")
    if expect_transfer_num is not None and transfer_num != expect_transfer_num:
        issues.append(f"tnum want={expect_transfer_num} got={transfer_num}")
    print(f"\n=== {title} lang={lang} ended={out.get('ended')} disp={disp} tnum={transfer_num} ===")
    for s in (out.get("steps") or [])[:14]:
        if s.get("user"):
            print(f"  C [{s.get('language')}]: {trunc(s['user'], 90)}")
        print(f"  A [{s.get('language')}]: {trunc(s.get('assistant') or '', 110)}")
    status = "PASS" if not issues else "FAIL: " + "; ".join(issues)
    print(" ->", status)
    return (not issues, out)


def main() -> int:
    results = []

    results.append(
        sim(
            "01 sales EN",
            "en-IN",
            ["I want a quotation for IP phones", "We need about 20 phones for Delhi office"],
            expect_disp="transferred_sales",
            expect_transfer_num="5002",
        )
    )
    results.append(
        sim(
            "02 sales HI",
            "hi-IN",
            ["मुझे sales enquiry करनी है", "IP Phone चाहिए 10 quantity"],
            expect_disp="transferred_sales",
            expect_transfer_num="5002",
        )
    )
    results.append(
        sim(
            "03 tech EN",
            "en-IN",
            [
                "I need technical support",
                "IP Phone",
                "Calls are not connecting",
                "single user",
                "none",
                "yes",
            ],
            expect_disp="transferred_tech",
            expect_transfer_num="5004",
        )
    )
    results.append(
        sim(
            "04 tech HI",
            "hi-IN",
            [
                "technical support chahiye",
                "IP Phone",
                "call nahi lag rahi",
                "ek user",
                "none",
                "haan",
            ],
            expect_disp="transferred_tech",
            expect_transfer_num="5004",
        )
    )
    results.append(
        sim(
            "05 product EN",
            "en-IN",
            ["I need product information about Media Gateway", "yes connect me to sales"],
            expect_disp="transferred_sales",
            expect_transfer_num="5002",
        )
    )
    results.append(
        sim(
            "06 hinglish tech",
            "hi-IN",
            [
                "mera IP phone kaam nahi kar raha technical support chahiye",
                "IP Phone",
                "display blank hai",
                "single",
                "none",
                "yes",
            ],
            expect_disp="transferred_tech",
            expect_transfer_num="5004",
        )
    )
    results.append(
        sim(
            "07 Hindi on EN session",
            "en-IN",
            [
                "मुझे technical support चाहिए",
                "IP Phone",
                "network error",
                "multiple users",
                "none",
                "yes",
            ],
            expect_disp="transferred_tech",
            expect_transfer_num="5004",
        )
    )
    results.append(
        sim(
            "08 sales hinglish",
            "hi-IN",
            ["sales enquiry hai quotation chahiye", "Call Center 5 licenses"],
            expect_disp="transferred_sales",
            expect_transfer_num="5002",
        )
    )
    results.append(
        sim(
            "09 clarify then sales",
            "en-IN",
            ["help", "sales enquiry please", "IP phones 50 units"],
            expect_disp="transferred_sales",
            expect_transfer_num="5002",
        )
    )

    code, out = api("POST", "/v1/desks/coral-tfn/simulate", {"language": "hi-IN", "turns": []})
    w = ((out.get("steps") or [{}])[0].get("assistant") or "")
    print("\n=== 10 welcome HI only ===")
    print("  A:", w)
    ok = ("स्वागत" in w or "नमस्ते" in w) and len(w) < 120
    print(" ->", "PASS" if ok else f"FAIL welcome={w!r} len={len(w)}")
    results.append((ok, out))

    code, sb = api("POST", "/v1/desk-calls", {"desk_id": "coral-tfn", "language": "en-IN", "ani": "9800111001"})
    cid = sb.get("id")
    print(f"\n=== 11 desk-call EN sales {cid} ===")
    print("  welcome:", trunc(((sb.get("turns") or [{}])[0].get("assistant") or ""), 100))
    for ut in ["I have a sales enquiry", "Need 15 IP phones"]:
        code, sb = api("POST", f"/v1/desk-calls/{cid}/turn", {"text": ut})
        last = (sb.get("turns") or [])[-1]
        print(f"  C: {ut}")
        print(
            f"  A: {trunc(last.get('assistant') or '', 100)} disp={last.get('disposition')} end={last.get('end')}"
        )
    code, sb = api("GET", f"/v1/desk-calls/{cid}")
    ok = False
    for t in sb.get("turns") or []:
        for sk in t.get("skills") or []:
            if sk.get("name") == "transfer_to_queue":
                print("  skill", sk)
                ok = sk.get("status") == "ok"
        if t.get("disposition") == "transferred_sales":
            ok = True
    print("  final disp", ((sb.get("turns") or [])[-1].get("disposition")))
    print(" ->", "PASS" if ok else "FAIL")
    results.append((ok, sb))

    code, sb = api("POST", "/v1/desk-calls", {"desk_id": "coral-tfn", "language": "hi-IN", "ani": "9800111002"})
    cid = sb.get("id")
    print(f"\n=== 12 desk-call HI tech {cid} ===")
    print("  welcome:", trunc(((sb.get("turns") or [{}])[0].get("assistant") or ""), 100))
    for ut in [
        "technical support chahiye",
        "IP Phone",
        "call drop ho rahi hai",
        "kai users",
        "none",
        "haan sahi hai",
    ]:
        code, sb = api("POST", f"/v1/desk-calls/{cid}/turn", {"text": ut})
        last = (sb.get("turns") or [])[-1]
        print(
            f"  C: {ut}\n  A: {trunc(last.get('assistant') or '', 90)} end={last.get('end')} disp={last.get('disposition')}"
        )
    last = (sb.get("turns") or [])[-1]
    ok = last.get("disposition") == "transferred_tech" or bool(last.get("end"))
    print(" ->", "PASS" if ok else "FAIL", "disp=", last.get("disposition"))
    results.append((ok, sb))

    results.append(
        sim(
            "13 product HI",
            "hi-IN",
            ["product information chahiye IP Phone ke baare mein", "haan sales se jodiye"],
            expect_disp="transferred_sales",
            expect_transfer_num="5002",
        )
    )
    results.append(
        sim(
            "14 complaint start HI",
            "hi-IN",
            ["मुझे शिकायत दर्ज करानी है", "आईपी फोन"],
        )
    )

    # Empty-matrix regression via local simulate is covered by unit tests;
    # confirm published attributes still expose dialable numbers on transfer.
    results.append(
        sim(
            "15 product EN Media Gateway transfer",
            "en-IN",
            ["Tell me about Coral Media Gateway pricing", "yes please transfer to sales"],
            expect_disp="transferred_sales",
            expect_transfer_num="5002",
        )
    )

    passed = sum(1 for r, _ in results if r)
    print(f"\n======== SUMMARY {passed}/{len(results)} PASS ========")
    return 0 if passed == len(results) else 1


if __name__ == "__main__":
    sys.exit(main())
