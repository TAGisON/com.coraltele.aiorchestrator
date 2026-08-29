# Sarvam lab gateways

Shared config for Phase F vendors: `sarvam-stt`, `sarvam-llm`, `sarvam-tts`.

## Secrets

- Env: `SARVAM_API_KEY`
- Or file: `.agent/secrets.local.json` → `sarvam.api_key` (copy from `secrets.example.json`)
- Optional: `AIORCH_SECRETS_FILE` to point at an alternate secrets path in tests

Never commit secrets. Gateways never log the key.

## Registration

`cmd/aiorchestrator` calls `sarvam.LoadConfig()` and registers the three gateways only when `Configured()` is true.

## Failover

Profile ordered lists e.g. `[sarvam-stt, fake-listen]`. If Sarvam is unregistered or Probe unhealthy, `router.Select` picks the fake.

See `docs/architecture/OPERATIONS.md` §10.
