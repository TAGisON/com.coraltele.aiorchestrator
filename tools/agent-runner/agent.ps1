<#
.SYNOPSIS
  Coral agentic phase pipeline runner (v0: artifact + mail + human control).

.EXAMPLE
  .\tools\agent-runner\agent.ps1 start -From phase-a
  .\tools\agent-runner\agent.ps1 next-prompt
  .\tools\agent-runner\agent.ps1 complete-role -Result pass
  .\tools\agent-runner\agent.ps1 status
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet(
        "start", "status", "audit", "stop", "resume", "restart-phase",
        "continue", "decide", "next-prompt", "complete-role", "mail-test",
        "monitor-start", "monitor-stop", "assign-role"
    )]
    [string]$Command = "status",

    [string]$From,
    [string]$Phase,
    [string]$Id,
    [string]$Answer,
    [ValidateSet("pass", "fail", "blocker")]
    [string]$Result = "pass",
    [int]$Tail = 40,
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
)

$ErrorActionPreference = "Stop"
$AgentDir = Join-Path $RepoRoot ".agent"
$StatusPath = Join-Path $AgentDir "status.json"
$AuditPath = Join-Path $AgentDir "audit.jsonl"
$QueuePath = Join-Path $AgentDir "queue.json"
$ConfigPath = Join-Path $AgentDir "config.yaml"
$MailScript = Join-Path $RepoRoot "tools\notify\Send-AgentMail.ps1"

$RoleOrder = @("planner", "coder", "reviewer", "summarizer")
$SkillMap = @{
    planner    = "coral-phase-planner"
    coder      = "coral-phase-coder"
    reviewer   = "coral-phase-reviewer"
    summarizer = "coral-phase-summarizer"
}

function Get-ConfigPhases {
    if (-not (Test-Path $ConfigPath)) { return @("phase-a", "phase-b", "phase-c", "phase-d", "phase-e", "phase-f") }
    $lines = Get-Content $ConfigPath
    $inPhases = $false
    $list = @()
    foreach ($line in $lines) {
        if ($line -match '^\s*phases:\s*$') { $inPhases = $true; continue }
        if ($inPhases) {
            if ($line -match '^\S') { break }
            if ($line -match '^\s*-\s*(\S+)') { $list += $Matches[1] }
        }
    }
    if ($list.Count -eq 0) { return @("phase-a", "phase-b", "phase-c", "phase-d", "phase-e", "phase-f") }
    return $list
}

function Read-Status {
    if (-not (Test-Path $StatusPath)) {
        return [pscustomobject]@{
            pipeline   = "coral-phase"
            state      = "idle"
            phase      = $null
            role       = $null
            loop       = 0
            updated_at = $null
            message    = "No status yet. Run Install.ps1 then start."
        }
    }
    return Get-Content $StatusPath -Raw | ConvertFrom-Json
}

function Write-Status([hashtable]$Hash) {
    $Hash["updated_at"] = (Get-Date).ToString("o")
    ($Hash | ConvertTo-Json -Depth 6) | Set-Content $StatusPath -Encoding UTF8
}

function Write-Audit([string]$Cmd, [string]$Detail, [string]$ResultName = "ok") {
    New-Item -ItemType Directory -Force -Path $AgentDir | Out-Null
    if (-not (Test-Path $AuditPath)) { New-Item -ItemType File -Path $AuditPath | Out-Null }
    $st = Read-Status
    $row = [ordered]@{
        ts     = (Get-Date).ToString("o")
        cmd    = $Cmd
        phase  = $st.phase
        role   = $st.role
        state  = $st.state
        result = $ResultName
        detail = $Detail
    }
    Add-Content -Path $AuditPath -Value (($row | ConvertTo-Json -Compress)) -Encoding UTF8
}

function Send-Notify([string]$Subject, [string]$Body) {
    if ($env:AGENT_NO_MAIL -eq "1") {
        Write-Host "(AGENT_NO_MAIL=1) skip mail: $Subject"
        return
    }
    if (-not (Test-Path $MailScript)) {
        Write-Warning "Mail script missing: $MailScript"
        return
    }
    $secrets = Join-Path $AgentDir "secrets.local.json"
    if (-not (Test-Path $secrets)) {
        Write-Warning "No secrets.local.json — skip mail. Run Install.ps1"
        return
    }
    try {
        & $MailScript -Subject $Subject -Body $Body -RepoRoot $RepoRoot
    }
    catch {
        Write-Warning "Mail failed: $_"
    }
}

function Ensure-WorkDir([string]$PhaseId) {
    $dir = Join-Path $AgentDir "work\$PhaseId"
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    return $dir
}

function Get-NextPhase([string]$Current) {
    $phases = @(Get-ConfigPhases)
    $idx = [array]::IndexOf($phases, $Current)
    if ($idx -lt 0 -or $idx -ge $phases.Count - 1) { return $null }
    return $phases[$idx + 1]
}

function Get-NextRole([string]$Role) {
    $idx = [array]::IndexOf($RoleOrder, $Role)
    if ($idx -lt 0 -or $idx -ge $RoleOrder.Count - 1) { return $null }
    return $RoleOrder[$idx + 1]
}

function Get-ApprovalPath([string]$PhaseId, [string]$Role) {
    $map = @{
        planner    = "plan.md"
        coder      = "implementation.md"
        reviewer   = "review.md"
        summarizer = "summary.md"
    }
    return Join-Path (Join-Path $AgentDir "work\$PhaseId") $map[$Role]
}

function Parse-Approval([string]$Path) {
    if (-not (Test-Path $Path)) { return $null }
    $raw = Get-Content $Path -Raw
    if ($raw -notmatch '(?s)#\s*agent-approval\s*\r?\n(.*)$') { return $null }
    $block = $Matches[1]
    $result = $null
    if ($block -match '(?m)^\s*result:\s*(\w+)') { $result = $Matches[1].Trim() }
    return [pscustomobject]@{ result = $result; block = $block }
}

function Build-Prompt([string]$PhaseId, [string]$Role) {
    $skill = $SkillMap[$Role]
    $phaseFile = Join-Path $AgentDir "phases\$PhaseId.yaml"
    $work = Join-Path $AgentDir "work\$PhaseId"
    # Single-quoted here-string so leading dashes are not parsed as operators.
    $prompt = @'
You are running the Coral agentic phase pipeline.

ROLE: __ROLE__
SKILL: use skill `__SKILL__` (follow it strictly)
PHASE: __PHASE__
PHASE FILE: __PHASE_FILE__
WORK DIR: __WORK__
REPO: __REPO__

Rules:
* Separate session; do not trust other roles chat.
* Read artifacts and git; write only your role outputs under WORK DIR.
* End your artifact with # agent-approval (see .agent/templates/approval.md).
* Architecture locks: docs/architecture/PLATFORM_FIRST.md and phase docs.

When finished, tell the operator to run:
  .\tools\agent-runner\agent.ps1 complete-role -Result <pass|fail|blocker>
'@
    $prompt = $prompt.Replace('__ROLE__', $Role).Replace('__SKILL__', $skill).
        Replace('__PHASE__', $PhaseId).Replace('__PHASE_FILE__', $phaseFile).
        Replace('__WORK__', $work).Replace('__REPO__', $RepoRoot)
    return $prompt
}

function Start-Pipeline([string]$StartPhase) {
    $phases = @(Get-ConfigPhases)
    if ($phases -notcontains $StartPhase) { throw "Unknown phase: $StartPhase. Known: $($phases -join ', ')" }
    Ensure-WorkDir $StartPhase | Out-Null
    Write-Status @{
        pipeline   = "coral-phase"
        state      = "running"
        phase      = $StartPhase
        role       = "planner"
        loop       = 0
        message    = "Planner ready. Run next-prompt and paste into a new Cursor agent chat."
    }
    Write-Audit "start" "from=$StartPhase role=planner"
    $subj = "[aiorchestrator] $StartPhase · planner · STARTED"
    $body = @"
Pipeline started.

Phase: $StartPhase
Role: planner
Repo: $RepoRoot

Next:
  cd $RepoRoot
  .\tools\agent-runner\agent.ps1 next-prompt
  (paste into new Cursor agent chat)
  .\tools\agent-runner\agent.ps1 complete-role -Result pass

Status: .\tools\agent-runner\agent.ps1 status
Stop:   .\tools\agent-runner\agent.ps1 stop
"@
    Send-Notify $subj $body
    Write-Host $subj
    Write-Host "Run: .\tools\agent-runner\agent.ps1 next-prompt"
}

function Complete-CurrentRole([string]$ForcedResult) {
    $st = Read-Status
    if ($st.state -eq "stopped") { throw "Pipeline stopped. resume first." }
    if ($st.state -eq "waiting_human") { throw "waiting_human — use decide / continue after answering queue." }
    if (-not $st.phase -or -not $st.role) { throw "No active phase/role. start first." }

    $path = Get-ApprovalPath $st.phase $st.role
    $parsed = Parse-Approval $path
    $res = $ForcedResult
    if ($parsed -and $parsed.result) { $res = $parsed.result }
    if (-not $res) { $res = "pass" }

    Write-Audit "complete-role" "file=$path" $res

    if ($res -eq "blocker") {
        Write-Status @{
            pipeline   = "coral-phase"
            state      = "waiting_human"
            phase      = $st.phase
            role       = $st.role
            loop       = [int]$st.loop
            message    = "Blocker reported. See .agent/work/$($st.phase)/blockers.md and decide/continue."
        }
        # try return_to from blockers — default plan -> planner
        $returnTo = "planner"
        if ($st.role -eq "coder") { $returnTo = "planner" }
        if ($st.role -eq "reviewer") { $returnTo = "coder" }
        Send-Notify "[aiorchestrator] $($st.phase) · $($st.role) · BLOCKER" @"
Blocker on $($st.phase) / $($st.role).

Suggested return: $returnTo
Check: .agent\work\$($st.phase)\blockers.md

Commands:
  .\tools\agent-runner\agent.ps1 status
  .\tools\agent-runner\agent.ps1 decide -Id B1 -Answer <text>
  .\tools\agent-runner\agent.ps1 continue
  or restart role via resume after editing status (advanced)
"@
        Write-Host "BLOCKER — mailed. State=waiting_human"
        return
    }

    if ($res -eq "fail") {
        $loop = [int]$st.loop
        if ($st.role -eq "reviewer") {
            $loop++
            $max = 2
            if ($loop -gt $max) {
                Write-Status @{
                    pipeline = "coral-phase"; state = "waiting_human"; phase = $st.phase
                    role = $st.role; loop = $loop
                    message = "Max coder-review loops exceeded. Human required."
                }
                Send-Notify "[aiorchestrator] $($st.phase) · MAX LOOPS" "Reviewer failed after $max loops. Human decision needed."
                return
            }
            Write-Status @{
                pipeline = "coral-phase"; state = "running"; phase = $st.phase
                role = "coder"; loop = $loop
                message = "Reviewer FAIL → coder (loop $loop). Run next-prompt."
            }
            Send-Notify "[aiorchestrator] $($st.phase) · reviewer · FAIL → coder" "Loop=$loop. Fix per review.md then complete-role."
            Write-Host "→ coder (loop $loop). next-prompt"
            return
        }
        if ($st.role -eq "coder") {
            Write-Status @{
                pipeline = "coral-phase"; state = "running"; phase = $st.phase
                role = "planner"; loop = 0
                message = "Coder FAIL/plan issue → planner. Run next-prompt."
            }
            Send-Notify "[aiorchestrator] $($st.phase) · coder · FAIL → planner" "Re-plan required. See implementation.md / blockers."
            Write-Host "→ planner. next-prompt"
            return
        }
        if ($st.role -eq "summarizer") {
            Write-Status @{
                pipeline = "coral-phase"; state = "running"; phase = $st.phase
                role = "coder"; loop = [int]$st.loop
                message = "Summarizer says exit not met → coder."
            }
            Send-Notify "[aiorchestrator] $($st.phase) · summarizer · FAIL → coder" "Exit criteria not met. See summary.md"
            Write-Host "→ coder. next-prompt"
            return
        }
        Write-Host "FAIL on $($st.role) — no automatic route; set waiting_human"
        Write-Status @{
            pipeline = "coral-phase"; state = "waiting_human"; phase = $st.phase
            role = $st.role; loop = [int]$st.loop; message = "Unhandled fail."
        }
        return
    }

    # pass
    $nextRole = Get-NextRole $st.role
    if ($nextRole) {
        Write-Status @{
            pipeline = "coral-phase"; state = "running"; phase = $st.phase
            role = $nextRole; loop = [int]$st.loop
            message = "$($st.role) PASS → $nextRole. Run next-prompt."
        }
        Send-Notify "[aiorchestrator] $($st.phase) · $($st.role) · PASS → $nextRole" @"
$($st.role) passed for $($st.phase).

Next role: $nextRole
  .\tools\agent-runner\agent.ps1 next-prompt
"@
        Write-Host "→ $nextRole. next-prompt"
        return
    }

    # summarizer pass → phase done
    $nextPhase = Get-NextPhase $st.phase
    Send-Notify "[aiorchestrator] $($st.phase) · PHASE COMPLETE" @"
Phase $($st.phase) completed (summarizer pass).

Work: .agent\work\$($st.phase)\
Next phase: $(if ($nextPhase) { $nextPhase } else { '(none — pipeline done)' })
"@
    if ($nextPhase) {
        Ensure-WorkDir $nextPhase | Out-Null
        Write-Status @{
            pipeline = "coral-phase"; state = "running"; phase = $nextPhase
            role = "planner"; loop = 0
            message = "Phase $($st.phase) done → $nextPhase planner. Run next-prompt."
        }
        Write-Audit "phase-complete" "$($st.phase) -> $nextPhase"
        Write-Host "PHASE DONE → $nextPhase planner. next-prompt"
    }
    else {
        Write-Status @{
            pipeline = "coral-phase"; state = "pipeline_done"; phase = $st.phase
            role = $null; loop = 0; message = "All phases complete."
        }
        Write-Audit "pipeline-done" $st.phase
        Send-Notify "[aiorchestrator] PIPELINE DONE" "All configured phases finished."
        Write-Host "PIPELINE DONE"
    }
}

# --- command dispatch ---
switch ($Command) {
    "mail-test" {
        Send-Notify "[aiorchestrator] mail test" "If you received this, SMTP secrets work.`nRepo: $RepoRoot"
    }
    "start" {
        $p = $From
        if (-not $p) { $p = $Phase }
        if (-not $p) { $p = "phase-a" }
        Start-Pipeline $p
    }
    "status" {
        $st = Read-Status
        $st | ConvertTo-Json -Depth 5
    }
    "audit" {
        if (-not (Test-Path $AuditPath)) { Write-Host "(no audit yet)"; break }
        Get-Content $AuditPath -Tail $Tail
    }
    "stop" {
        $st = Read-Status
        Write-Status @{
            pipeline = "coral-phase"; state = "stopped"; phase = $st.phase
            role = $st.role; loop = [int]$st.loop
            message = "Stopped by operator. resume to continue."
            prior_state = $st.state
        }
        Write-Audit "stop" "operator"
        Send-Notify "[aiorchestrator] STOPPED" "Phase=$($st.phase) Role=$($st.role)`nResume: .\tools\agent-runner\agent.ps1 resume"
        Write-Host "Stopped."
    }
    "resume" {
        $st = Read-Status
        if ($st.state -ne "stopped" -and $st.state -ne "waiting_human") {
            Write-Host "Nothing to resume (state=$($st.state))."
            break
        }
        if (-not $st.phase) { throw "No phase on status." }
        $role = $st.role
        if (-not $role) { $role = "planner" }
        Write-Status @{
            pipeline = "coral-phase"; state = "running"; phase = $st.phase
            role = $role; loop = [int]$st.loop
            message = "Resumed. Run next-prompt."
        }
        Write-Audit "resume" "$($st.phase)/$role"
        Send-Notify "[aiorchestrator] RESUMED" "Phase=$($st.phase) Role=$role"
        Write-Host "Resumed → $role. next-prompt"
    }
    "restart-phase" {
        $p = $Phase
        if (-not $p) { throw "Specify -Phase phase-a" }
        $work = Join-Path $AgentDir "work\$p"
        if (Test-Path $work) {
            $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
            $archive = Join-Path $AgentDir "work\_archive\$p-$stamp"
            New-Item -ItemType Directory -Force -Path (Split-Path $archive) | Out-Null
            Move-Item $work $archive
            Write-Host "Archived to $archive"
        }
        Ensure-WorkDir $p | Out-Null
        Write-Status @{
            pipeline = "coral-phase"; state = "running"; phase = $p
            role = "planner"; loop = 0
            message = "Phase $p restarted at planner."
        }
        Write-Audit "restart-phase" $p
        Send-Notify "[aiorchestrator] $p · RESTARTED" "Back to planner. next-prompt"
        Write-Host "Restarted $p at planner."
    }
    "continue" {
        $st = Read-Status
        if ($st.state -ne "waiting_human") {
            Write-Host "Not waiting_human (state=$($st.state))."
            break
        }
        Write-Status @{
            pipeline = "coral-phase"; state = "running"; phase = $st.phase
            role = $st.role; loop = [int]$st.loop
            message = "Continued after human. Run next-prompt (or complete-role if artifact already fixed)."
        }
        Write-Audit "continue" "operator"
        Send-Notify "[aiorchestrator] CONTINUED" "Phase=$($st.phase) Role=$($st.role)"
        Write-Host "Continued. next-prompt"
    }
    "decide" {
        if (-not $Id) { throw "Provide -Id and -Answer" }
        $queue = @()
        if (Test-Path $QueuePath) {
            $queue = @(Get-Content $QueuePath -Raw | ConvertFrom-Json)
        }
        $queue += [pscustomobject]@{
            id         = $Id
            answer     = $Answer
            at         = (Get-Date).ToString("o")
            phase      = (Read-Status).phase
        }
        ($queue | ConvertTo-Json -Depth 5) | Set-Content $QueuePath -Encoding UTF8
        Write-Audit "decide" "$Id=$Answer"
        Write-Host "Recorded decision $Id. Use continue when ready."
        Send-Notify "[aiorchestrator] DECISION $Id" "Answer: $Answer"
    }
    "next-prompt" {
        $st = Read-Status
        if ($st.state -eq "stopped") { throw "Stopped. resume first." }
        if ($st.state -eq "pipeline_done") { Write-Host "Pipeline done."; break }
        if (-not $st.phase -or -not $st.role) { throw "No active role. start first." }
        $prompt = Build-Prompt $st.phase $st.role
        $out = Join-Path $AgentDir "work\$($st.phase)\NEXT_PROMPT_$($st.role).txt"
        Ensure-WorkDir $st.phase | Out-Null
        Set-Content -Path $out -Value $prompt -Encoding UTF8
        Write-Host "===== PASTE INTO NEW CURSOR AGENT CHAT ($($st.role)) ====="
        Write-Host $prompt
        Write-Host "===== ALSO SAVED: $out ====="
        Write-Audit "next-prompt" "$($st.phase)/$($st.role)"
    }
    "complete-role" {
        Complete-CurrentRole $Result
    }
    "assign-role" {
        $worker = Join-Path $PSScriptRoot "Start-RoleWorker.ps1"
        & powershell -NoProfile -File $worker -RepoRoot $RepoRoot
    }
    "monitor-start" {
        $mon = Join-Path $PSScriptRoot "Monitor.ps1"
        $pidFile = Join-Path $AgentDir "monitor.pid"
        if (Test-Path $pidFile) {
            $old = [int]((Get-Content $pidFile -Raw).Trim())
            if (Get-Process -Id $old -ErrorAction SilentlyContinue) {
                Write-Host "Monitor already running pid=$old"
                break
            }
        }
        $p = Start-Process -FilePath "powershell.exe" -ArgumentList @(
            "-NoProfile", "-ExecutionPolicy", "Bypass",
            "-File", $mon, "-IntervalSec", "60", "-RepoRoot", $RepoRoot
        ) -PassThru -WindowStyle Minimized
        Write-Host "Monitor started pid=$($p.Id) (poll every 60s)"
        Write-Audit "monitor-start" "pid=$($p.Id)"
        Send-Notify "[aiorchestrator] MONITOR STARTED" "pid=$($p.Id)`nPoll=60s`nRepo=$RepoRoot"
    }
    "monitor-stop" {
        $pidFile = Join-Path $AgentDir "monitor.pid"
        if (Test-Path $pidFile) {
            $old = [int]((Get-Content $pidFile -Raw).Trim())
            Stop-Process -Id $old -Force -ErrorAction SilentlyContinue
            Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
            Write-Host "Monitor stopped pid=$old"
            Write-Audit "monitor-stop" "pid=$old"
            Send-Notify "[aiorchestrator] MONITOR STOPPED" "pid=$old"
        }
        else {
            Write-Host "No monitor.pid"
        }
    }
    default { throw "Unknown command" }
}
