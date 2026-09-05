# L3 — CI.3 Optional golangci-lint

| Field | Value |
|---|---|
| **id** | `CI.3` |
| **title** | Job D golangci-lint (fail on issues) |
| **status** | **Closed** — Job D fail mode |
| **parent_plan** | [11_CI_AND_CD.md](../11_CI_AND_CD.md) § OD-11-5 / phase CI.3 |
| **depends_on** | CI.2 Closed (`36e0146`) |

## architecture_refs

- [11_CI_AND_CD.md](../11_CI_AND_CD.md) — OD-11-5; CI.3 sketch
- [10_CODING_PRINCIPLES.md](../10_CODING_PRINCIPLES.md)
- Existing Job A `gofmt` (kept; lint does not replace it)

## goal

Add a pinned golangci-lint config and CI job. V1 may be **warn-only** (non-blocking) if the tree has legacy findings; document the mode in this file.

## in_scope

- `.golangci.yml` with a small enable set (govet, errcheck, staticcheck, ineffassign, unused)
- CI job `golangci` using official action + Go 1.22.x
- Docs: this file + README
- Choose fail vs warn-only after first local/CI-shaped run; record choice here

## out_scope

- Mass autofix of entire tree beyond what is needed to choose mode
- CD.0 / CD.1
- Enabling every default linter
- Replacing Job A gofmt

## forbidden

- Auto-deploy
- Live vendor calls
- Absorbing CD.0

## exit_criteria

- [x] `.golangci.yml` present and pinned version documented (`v1.64.8`)
- [x] Job present on push/PR
- [x] Mode recorded: **fail**
- [x] Local/CI-shaped run completes clean

## mode

**fail** — only five findings on first run; fixed in-tree (unused helpers, errcheck in test, empty branch). Job is blocking.

## verification

```text
golangci-lint run ./...   # v1.64.8
```

## handoff

Next: **CD.0** lab promote checklist.
