# Call outcomes — recording, fallback prompts, transfer

Three things decide what a caller actually experiences when a call does not end
in the happy path. All three are operator-controlled and all three are audited.

---

## 1. Call recording

Every live session is recorded to local disk on **the host running the
orchestrator**, as one stereo WAV plus a JSON sidecar.

```
<recording.root>/<YYYY-MM-DD>/<session-id>[-<call-id>]/
    session.wav    stereo s16le — LEFT = caller, RIGHT = agent
    meta.json      identity, timing, disposition, counters
```

`<call-id>` is the FreeSWITCH channel UUID from session metadata; when it is
absent the directory is just `<session-id>`. Both ids are sanitised, so a hostile
SIP header cannot escape the root.

### Configuration

| Property | Env | Default |
|---|---|---|
| `recording.enabled` | `RECORDING_ENABLED` | `true` |
| `recording.root` | `RECORDING_ROOT` | `/usr/local/recordings/ai-orchestrator` |
| `recording.retention_days` | `RECORDING_RETENTION_DAYS` | `30` |

The default root is written with a leading `/` deliberately: on Linux it is
absolute, and on Windows Go resolves it against the current drive
(`C:\usr\local\recordings\ai-orchestrator`). The same configured value therefore
works unchanged on every OS the orchestrator runs on.

### How the two legs stay aligned

A muxer ticks at the session frame cadence (20 ms) and writes one stereo frame
per tick, taking whatever each leg has buffered and padding with silence. The
number of frames due is derived from **elapsed wall time**, not from tick count,
so a scheduling hiccup cannot make the file drift out of sync.

Because the agent leg is captured where it is *played out* — not where it was
synthesised — the recording reflects what the caller actually heard, including
gaps.

### Guarantees

- Recording never affects the call. Every failure (no disk, permission denied,
  disk full) logs once, stops the recorder, and leaves the call running.
- RIFF sizes are re-patched every 5 s, so a recording left behind by a crash is
  still playable.
- `meta.json` is written via temp-file + rename, so a reader never sees a partial
  file.
- The path is stored on the session as `recording_ref`, which flows into
  `GET /v1/sessions/{id}`, postcall jobs and the warm-transfer payload.

### Retention

A sweeper runs at boot and daily, removing whole date directories older than
`recording.retention_days`. Today's directory is never touched, and directories
that are not `YYYY-MM-DD` are left alone. Set retention to `0` to keep forever —
be deliberate: call audio fills disks, and a full disk takes the switch with it.

---

## 2. Fallback prompts

When the AI pipeline cannot serve a call, the caller hears an operator-recorded
announcement and the call is released — never dead air, never a silent drop.

### Scenarios

| Scenario | Triggered by |
|---|---|
| `ai_unavailable` | An engine (STT/LLM/TTS) is down or unroutable (`CodeUnavailable`) |
| `credits_exhausted` | Vendor rejected us for auth/quota/billing (`CodeAuth`, `CodeRateLimit`) |
| `timeout` | Engine accepted the request but did not answer (`CodeTimeout`) |
| `system_busy` | We refused the work ourselves (`CodeBadRequest`, `CodeBadAudio`, `CodeUnsupported`) |
| `internal_error` | A bug, or an error we could not classify |
| `generic` | Catch-all used when no specific prompt is uploaded |

Unclassified errors deliberately land on `internal_error` rather than being
treated as a normal end of call.

### Managing prompts

```bash
# Upload (raw WAV body)
curl -X PUT --data-binary @out_of_credit.wav \
     -H 'Content-Type: audio/wav' \
     http://127.0.0.1:8011/v1/tenant/fallback/credits_exhausted

# Or as a multipart form
curl -X PUT -F file=@out_of_credit.wav \
     http://127.0.0.1:8011/v1/tenant/fallback/credits_exhausted

# What is configured, and what each scenario resolves to
curl http://127.0.0.1:8011/v1/tenant/fallback

# Download what would actually play
curl -o played.wav \
     'http://127.0.0.1:8011/v1/tenant/fallback/timeout?download=true'

# Remove a tenant override
curl -X DELETE http://127.0.0.1:8011/v1/tenant/fallback/timeout
```

Uploads must be **uncompressed 16-bit PCM WAVE**, mono or stereo, 8000–48000 Hz,
≤10 MB and ≤60 s (the edge's downlink queue depth — a longer prompt would have
its tail silently discarded at playout instead of being rejected here). Stereo is downmixed and the normalised mono form is stored, so
playback is deterministic. Anything else is rejected at upload time — a prompt
that fails to decode while a caller is on the line is the one failure mode this
feature exists to prevent.

### Resolution order

```
<tenant>/<scenario>  →  <tenant>/generic  →  _default/<scenario>  →  _default/generic
```

So a single uploaded `_default/generic.wav` covers every tenant and every
scenario. Start there, then add specific prompts where the wording matters.

### What happens on failure

1. In-flight Think/Speak is interrupted and the sink flushed.
2. The resolved prompt is streamed to the caller at the session sample rate
   (and captured on the recording's agent leg).
3. The edge is told to hang up; it drains the prompt before releasing.
4. Disposition `failed_<scenario>` is persisted and the session stopped.

This is **idempotent per session** — a cascade of engine errors produces exactly
one announcement. The hangup cause reflects the scenario
(`NORMAL_TEMPORARY_FAILURE` for capacity/credits, `NETWORK_OUT_OF_ORDER` for an
engine outage) so upstream CDRs can tell them apart.

If no prompt is uploaded, the call is still released cleanly — with a warning in
the log naming the scenario and tenant.

### Orchestrator completely unreachable

The prompts above live in the orchestrator, so they cannot help when the
orchestrator itself is down. `ai_voice_bot.lua` covers that case from
FreeSWITCH, playing a local file and releasing:

```
/usr/local/share/ai-orchestrator/fallback/{ai_unavailable,credits_exhausted,internal_error,generic}.wav
```

Override the directory with the `ai_fallback_dir` channel variable.

---

## 3. Transfer

`coral-transfer` moves the caller's leg to a human. It does two independent
things:

1. **Notifies Coral** (`POST <coral.base_url>/skills/warm-transfer`) with intent,
   summary, transcript excerpt and `recording_ref`.
2. **Moves the leg** — the in-process equivalent of
   `uuid_transfer <call-uuid> <destination> <dialplan> <context>`.

A Coral outage must not strand a caller who was promised a human, so the leg is
transferred whether or not the notification succeeds; the notification result is
reported in the skill output for the audit trail.

### Destination

Taken from the first present skill arg: `destination`, `number`, `extension`,
`dest`, `transfer_to` — then `DefaultDestination`. With no destination the skill
fails with `bad_request` rather than transferring somewhere arbitrary.

### Defaults

| Property | Env | Default |
|---|---|---|
| `transfer.dialplan` | `TRANSFER_DIALPLAN` | `XML` |
| `transfer.context` | `TRANSFER_CONTEXT` | `calltransfer` |

Both may be overridden per call via skill args.

### Ordering

Queued speech is played out first — the edge holds the transfer until its
playout buffer drains (bounded, 10 s from our side and 15 s at the module), so
"connecting you now" is never cut off. Disposition `transferred` is then
persisted.

Sessions with no telephony leg (playback jobs, lab clocks) report
`transferred: false` with a reason instead of failing: there is nothing to move.
