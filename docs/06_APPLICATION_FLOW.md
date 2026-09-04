# 06 — Application flow (whole system)

This document is the **single map** of the application we are building: telephony, media, brain, tools, and evidence.  
 complementary detail: [03_BRAIN_AND_GRAPH.md](./03_BRAIN_AND_GRAPH.md), [04_LIVE_TURN_MACHINE.md](./04_LIVE_TURN_MACHINE.md).

---

## 1. System context

```mermaid
flowchart TB
  subgraph Channels
    PSTN[PSTN / Mobile / SIP]
  end

  subgraph CoralTel["Coral telephony"]
    PABX[Coral PABX / SIP core]
    FS[FreeSWITCH]
    MOD[mod_audio_stream<br/>dumb PCM pipe]
  end

  subgraph Orch["aiorchestrator — LLM Call Centre"]
    CTRL[Control API / Session]
    GRAPH[Conversation Graph Runtime]
    TURN[Live Turn Machine]
    STT[Listen gateway — STT]
    LLM[Think gateway — LLM]
    TTS[Speak gateway — TTS]
    TOOLS[Tools: transfer / hangup]
    EVID[Transcript + Audit + Recording]
  end

  subgraph Config["Configuration"]
    ADMIN[Admin / API<br/>graph · prompts · matrix · bindings · voice]
  end

  subgraph Downstream["Coral CC / later"]
    ACD[Queues / ACD / agents]
    CRM[CRM — Next]
    POST[Post-call summary — Next]
  end

  PSTN --> PABX --> FS --> MOD
  MOD <-->|PCM up/down| TURN
  MOD -->|transfer / hangup verbs| TOOLS
  ADMIN --> GRAPH
  CTRL --> GRAPH
  GRAPH --> TURN
  TURN --> STT
  TURN --> LLM
  TURN --> TTS
  TURN --> TOOLS
  TURN --> EVID
  TOOLS -->|blind transfer| ACD
  EVID -.-> POST
  EVID -.-> CRM
```

**Keep:** FS ↔ orch PCM path.  
**Rebuild:** dialogue brain (graph + turn machine).  
**Defer:** rich CRM push, summarization, agent assist UI.

---

## 2. End-to-end call lifecycle

```mermaid
sequenceDiagram
  participant C as Caller
  participant FS as FreeSWITCH + mod
  participant O as Orchestrator
  participant G as Graph + Turn machine
  participant V as STT / LLM / TTS

  C->>FS: SIP invite DID
  FS->>O: create session + WSS media
  O->>G: Entry → Speak welcome
  G->>V: TTS welcome
  V->>FS: PCM downlink
  FS->>C: hears bot

  loop Until Tool end
    C->>FS: speech
    FS->>O: PCM uplink
    O->>V: STT
    V->>G: final text
    alt Actionable and legal edge
      G->>V: LLM classify within node
      G->>V: TTS reply or closing line
      V->>FS: PCM downlink
    else Repair / suppress
      G->>G: retry / transcript-only
    end
  end

  alt Transfer
    G->>FS: transfer verb after arm+TTS
    FS->>C: connected to queue/agent
  else Hangup
    G->>FS: hangup verb after arm+TTS
    FS->>C: call cleared
  end

  O->>O: stop recording, finalize transcript, session terminal
```

---

## 3. Brain: graph cursor

```mermaid
flowchart LR
  E[Entry] --> W[Speak welcome]
  W --> L{Language<br/>locked?}
  L -->|no| Lang[ListenLanguage]
  L -->|yes| M[ListenChoice department]
  Lang --> M
  M -->|sales / corporate / support| T[Tool transfer]
  M -->|faq intent| I[Inform KB]
  M -->|unclear ×N| H[Tool hangup]
  I --> M2[ListenChoice anything else]
  M2 -->|back| M
  M2 -->|done| End1[End]
  T --> X[Leg leaves / End]
  H --> End2[End]

  GL[Global: language_switch] -.-> Lang
  GL -.-> M
  GL -.-> I
```

- Jumps only on **drawn edges** (including globals the node opts into).
- Unclear / incomplete / disallowed jump → **node repair** (retry → exhaust).

---

## 4. Live turn machine (decision vs transcript)

```mermaid
stateDiagram-v2
  [*] --> Idle
  Idle --> Speaking: welcome / prompt
  Speaking --> Listening: playout done
  Listening --> Thinking: actionable final
  Speaking --> Thinking: barge commit allowed
  Thinking --> Speaking: reply no tool
  Thinking --> ToolArmed: tool edge
  ToolArmed --> Speaking: closing line
  Speaking --> ToolExecuting: drain + verb
  ToolExecuting --> Ending: settled
  Ending --> [*]

  note right of ToolArmed
    barge OFF
    STT transcript-only
  end note
```

```mermaid
flowchart TD
  F[STT final] --> Q{State?}
  Q -->|Listening| Filt[Filters: length echo noise]
  Q -->|Speaking + barge OK| Filt
  Q -->|ToolArmed / Thinking / Ending| TO[Transcript-only]
  Filt -->|pass| ACT[Actionable → Thinking]
  Filt -->|fail| TO
  ACT --> LLM[LLM / matcher<br/>allowlisted edges only]
  LLM -->|edge| MOVE[Move cursor / Speak / Tool arm]
  LLM -->|unclear| REP[Node repair retry]
  LLM -->|exhausted| TOOLH[Tool hangup edge]
```

---

## 5. Tool lock (transfer and hangup)

```mermaid
flowchart TD
  A[Graph selects Tool edge] --> B[ARM params from matrix/slots]
  B --> C[barge disabled]
  C --> D[Speak closing prompt]
  D --> E[Wait playout drain]
  E --> F{transfer or hangup?}
  F -->|transfer| G[Edge uuid_transfer]
  F -->|hangup| H[Edge hangup]
  G --> I[Wait call control gone]
  H --> I
  I --> J[Ending: stop recording + terminal session]
```

---

## 6. Configuration vs runtime vs bindings

```mermaid
flowchart TB
  subgraph ConfigTime
    AG[Admin draws graph]
    PR[Prompts per language]
    MX[Routing matrix]
    KB[Knowledge / FAQ dump or retrieve binding]
    SK[Skill bindings optional CRM]
    VC[Voice id + language allowlist]
  end

  subgraph Publish
    PUB[Published profile version]
  end

  subgraph Runtime
    CUR[Cursor on node]
    TM[Turn machine]
    GW[STT LLM TTS gateways]
  end

  AG --> PUB
  PR --> PUB
  MX --> PUB
  KB --> PUB
  SK --> PUB
  VC --> PUB
  PUB --> CUR
  CUR --> TM
  TM --> GW
  KB -.->|Inform node| TM
  MX -.->|Tool transfer params| TM
```

---

## 7. Evidence path

```mermaid
flowchart LR
  CALL[Live call] --> TR[Transcript events]
  CALL --> AU[Audit events]
  CALL --> REC[Session recording]
  TR --> SUP[Supervisor review]
  AU --> SUP
  REC --> SUP
  TR --> PC[Post-call Next]
  AU --> PC
```

Recording must **stop** when the session/leg ends.  
Transcript includes actionable and suppressed/ignored speech with reasons.

---

## 8. V1 build boundary (on this map)

| Layer | V1 |
|---|---|
| Telephony PCM + control verbs | Keep / harden |
| Graph runtime + turn machine | **Build (replace old desk hybrid)** |
| STT/LLM/TTS gateways | Keep adapters; constrain by graph |
| Transfer / hangup tools | Harden to tool-lock |
| Transcript + audit + recording stop | Build to match turn machine |
| Inform + FAQ binding | Thin V1 |
| CRM / summary / agent assist | Next |

---

## 9. Reading order for implementers

1. This file — whole picture  
2. [01_VISION_AND_SCOPE.md](./01_VISION_AND_SCOPE.md) — what to tick  
3. [02_CURRENT_STATE.md](./02_CURRENT_STATE.md) — keep vs discard  
4. [03_BRAIN_AND_GRAPH.md](./03_BRAIN_AND_GRAPH.md) — node/edge law  
5. [04_LIVE_TURN_MACHINE.md](./04_LIVE_TURN_MACHINE.md) — live behaviour  
6. [05_MEDIA_AND_VENDORS.md](./05_MEDIA_AND_VENDORS.md) — engines  

JSON schema and admin UI wireframes are **intentionally not** in this documentation set yet.
