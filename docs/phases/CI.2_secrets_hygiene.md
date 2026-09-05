# L3 — CI.2 Secrets hygiene (Job C)

| Field | Value |
|---|---|
| **id** | `CI.2` |
| **title** | Job C `secrets-hygiene` path deny |
| **status** | **Closed** — Job C path deny |
| **parent_plan** | [11_CI_AND_CD.md](../11_CI_AND_CD.md) § Job C / phase CI.2 |
| **depends_on** | CI.0/CI.1 Closed (`14b1965`); Doc 11 Locked |

## architecture_refs

- [11_CI_AND_CD.md](../11_CI_AND_CD.md) — Job C; OD path deny
- [10_CODING_PRINCIPLES.md](../10_CODING_PRINCIPLES.md) — no commit of secrets.local
- [AGENTS.md](../../AGENTS.md) — never commit `.agent/secrets.local.json`
- Existing: `.gitignore` entry; `tests/agent/scenarios/F-lock-no-secrets-in-git.yaml`

## goal

Add always-on CI Job C that fails if forbidden secret paths are tracked in git, plus a small Go lock test mirroring the agent harness rule.

## in_scope

- `.github/workflows/ci.yml` job `secrets-hygiene` (Job C)
- Path deny: `.agent/secrets.local.json`, `.agent/secrets/**`, root `.env`, `credentials.json` if tracked
- Assert `.gitignore` still lists `.agent/secrets.local.json`
- Go test package `internal/cihygiene` (git ls-files + gitignore)
- Docs: this file + README
- V1: **no** gitleaks (optional later)

## out_scope

- gitleaks / Step Security full scan
- CI.3 golangci-lint
- CD.0 / CD.1
- Rotating or deleting lab secrets on disk (gitignored files may exist locally)

## forbidden

- Committing real secrets to demonstrate failure
- Auto-deploy
- Live vendor calls in Job C

## exit_criteria

- [x] Job C present; always runs on push/PR
- [x] Clean tree passes path deny + Go lock test
- [x] Documented: tracked `.agent/secrets.local.json` would fail Job C
- [x] No gitleaks required for V1 close

## verification

```text
git ls-files .agent/secrets.local.json .agent/secrets .env credentials.json
go test ./internal/cihygiene/... -count=1
```

## planted-failure note

If `.agent/secrets.local.json` (or other deny paths) is force-added and tracked, Job C step `path deny (tracked secrets)` exits non-zero. Do not plant real keys in the repo to prove this.

## handoff

Next: **CI.3** optional lint, or **CD.0** lab promote checklist.
