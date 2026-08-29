<#
.SYNOPSIS
  Coral agentic pipeline runner (phase build + product validation).
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet(
        "start", "status", "audit", "stop", "resume", "restart-phase",
        "continue", "decide", "next-prompt", "complete-role", "mail-test",
        "monitor-start", "monitor-stop", "assign-role", "next-feature", "archive-round"
    )]
    [string]$Command = "status",

    [string]$From,
    [string]$Phase,
    [string]$Pipeline = "",
    [string]$Id,
    [string]$Answer,
    [ValidateSet("pass", "fail", "blocker")]
    [string]$Result = "pass",
    [int]$Tail = 40,
    [string]$RepoRoot = ""
)

$ErrorActionPreference = "Stop"
if (-not $RepoRoot) {
    if ($PSScriptRoot) {
        $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
    }
    else {
        $RepoRoot = (Resolve-Path (Join-Path $PWD ".\")).Path
        if (-not (Test-Path (Join-Path $RepoRoot ".agent"))) {
            $RepoRoot = "C:\Users\user\Documents\GitHub\com.coraltele.aiorchestrator"
        }
    }
}
$AgentDir = Join-Path $RepoRoot ".agent"
$StatusPath = Join-Path $AgentDir "status.json"
$AuditPath = Join-Path $AgentDir "audit.jsonl"
$QueuePath = Join-Path $AgentDir "queue.json"
$ConfigPath = Join-Path $AgentDir "config.yaml"
$PipelinesDir = Join-Path $AgentDir "pipelines"
$MailScript = Join-Path $RepoRoot "tools\notify\Send-AgentMail.ps1"

function ConvertTo-StringMap($obj) {
    $h = @{}
    if ($null -eq $obj) { return $h }
    if ($obj -is [hashtable]) { return $obj }
    $obj.PSObject.Properties | ForEach-Object { $h[$_.Name] = [string]$_.Value }
    return $h
}

function ConvertTo-IntMap($obj) {
    $h = @{}
    if ($null -eq $obj) { return $h }
    if ($obj -is [hashtable]) {
        foreach ($k in $obj.Keys) { $h[$k] = [int]$obj[$k] }
        return $h
    }
    $obj.PSObject.Properties | ForEach-Object { $h[$_.Name] = [int]$_.Value }
    return $h
}

function Get-DefaultPipelineName {
    if (-not (Test-Path $ConfigPath)) { return "coral-phase" }
    foreach ($line in Get-Content $ConfigPath) {
        if ($line -match '^\s*pipeline:\s*(\S+)') { return $Matches[1].Trim() }
    }
    return "coral-phase"
}

function Load-PipelineDef([string]$Name) {
    if (-not $Name) { $Name = Get-DefaultPipelineName }
    $path = Join-Path $PipelinesDir ($Name + ".json")
    if (-not (Test-Path $path)) {
        throw ("Unknown pipeline '" + $Name + "' (missing " + $path + ")")
    }
    $raw = Get-Content $path -Raw -Encoding UTF8 | ConvertFrom-Json
    return [pscustomobject]@{
        name           = [string]$raw.name
        first_role     = [string]$raw.first_role
        roles          = @($raw.roles)
        skills         = ConvertTo-StringMap $raw.skills
        artifacts      = ConvertTo-StringMap $raw.artifacts
        fail_return    = ConvertTo-StringMap $raw.fail_return
        max_fail_loops = ConvertTo-IntMap $raw.max_fail_loops
        phases         = @($raw.phases)
        prompt_kind    = $(if ($raw.prompt_kind) { [string]$raw.prompt_kind } else { "phase" })
        manifest       = $(if ($raw.manifest) { [string]$raw.manifest } else { $null })
        pause_after_phase = $(
            if ($null -ne $raw.pause_after_phase) { [bool]$raw.pause_after_phase }
            elseif ([string]$raw.name -eq "product-validation") { $true }
            else { $false }
        )
        evidence_worktree = $(if ($raw.evidence_worktree) { [string]$raw.evidence_worktree } else { $null })
    }
}

function Get-ActivePipelineName {
    if (Test-Path $StatusPath) {
        $st = Get-Content $StatusPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($st.pipeline) { return [string]$st.pipeline }
    }
    return Get-DefaultPipelineName
}

function Get-ActivePipeline {
    return Load-PipelineDef (Get-ActivePipelineName)
}

function Read-Status {
    if (-not (Test-Path $StatusPath)) {
        return [pscustomobject]@{
            pipeline   = Get-DefaultPipelineName
            state      = "idle"
            phase      = $null
            role       = $null
            loop       = 0
            updated_at = $null
            message    = "No status yet. Run Install.ps1 then start."
        }
    }
    return Get-Content $StatusPath -Raw -Encoding UTF8 | ConvertFrom-Json
}

function Write-Status([hashtable]$Hash) {
    $Hash["updated_at"] = (Get-Date).ToString("o")
    ($Hash | ConvertTo-Json -Depth 8) | Set-Content $StatusPath -Encoding UTF8
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
    $line = ($row | ConvertTo-Json -Compress)
    for ($i = 0; $i -lt 5; $i++) {
        try {
            Add-Content -Path $AuditPath -Value $line -Encoding UTF8 -ErrorAction Stop
            return
        }
        catch {
            Start-Sleep -Milliseconds 200
        }
    }
    Write-Warning "audit write skipped (file locked)"
}

function Send-Notify([string]$Subject, [string]$Body) {
    if ($env:AGENT_NO_MAIL -eq "1") {
        Write-Host ("(AGENT_NO_MAIL=1) skip mail: " + $Subject)
        return
    }
    if (-not (Test-Path $MailScript)) {
        Write-Warning "Mail script missing"
        return
    }
    $secrets = Join-Path $AgentDir "secrets.local.json"
    if (-not (Test-Path $secrets)) {
        Write-Warning "No secrets.local.json - skip mail. Run Install.ps1"
        return
    }
    try {
        & $MailScript -Subject $Subject -Body $Body -RepoRoot $RepoRoot
    }
    catch {
        Write-Warning ("Mail failed: " + $_)
    }
}

function Ensure-WorkDir([string]$PhaseId) {
    $dir = Join-Path $AgentDir ("work\" + $PhaseId)
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    return $dir
}

function Get-NextPhase([string]$Current, $Pipe) {
    $phases = @($Pipe.phases)
    $idx = [array]::IndexOf($phases, $Current)
    if ($idx -lt 0 -or $idx -ge $phases.Count - 1) { return $null }
    return $phases[$idx + 1]
}

function Get-NextRole([string]$Role, $Pipe) {
    $roles = @($Pipe.roles)
    $idx = [array]::IndexOf($roles, $Role)
    if ($idx -lt 0 -or $idx -ge $roles.Count - 1) { return $null }
    return $roles[$idx + 1]
}

function Get-ApprovalPath([string]$PhaseId, [string]$Role, $Pipe) {
    $file = $Pipe.artifacts[$Role]
    if (-not $file) { throw ("No artifact mapped for role=" + $Role + " pipeline=" + $Pipe.name) }
    return Join-Path (Join-Path $AgentDir ("work\" + $PhaseId)) $file
}

function Parse-Approval([string]$Path) {
    if (-not (Test-Path $Path)) { return $null }
    $raw = Get-Content $Path -Raw -Encoding UTF8
    if ($raw -notmatch '(?s)#\s*agent-approval\s*\r?\n(.*)$') { return $null }
    $block = $Matches[1]
    $result = $null
    if ($block -match '(?m)^\s*result:\s*(\w+)') { $result = $Matches[1].Trim() }
    return [pscustomobject]@{ result = $result; block = $block }
}

function Build-Prompt([string]$PhaseId, [string]$Role, $Pipe) {
    $skill = $Pipe.skills[$Role]
    $phaseFile = Join-Path $AgentDir ("phases\" + $PhaseId + ".yaml")
    $work = Join-Path $AgentDir ("work\" + $PhaseId)
    $kind = $Pipe.prompt_kind
    $extra = ""
    if ($kind -eq "validation") {
        $man = $Pipe.manifest
        if (-not $man) { $man = "tests/agent/MANIFEST.yaml" }
        $extra = @"

Validation contract (ONE FEATURE this round):
* PHASE/WAVE id is the feature id: $PhaseId — test ONLY this feature; do not expand to other F-* ids
* Catalog: tests/agent/FEATURES.md + tests/agent/features/catalog.yaml
* Scenario file: tests/agent/scenarios/$PhaseId.yaml (refine if needed)
* Universal layout: $man (blocker class spec if missing)
* Docs: docs/VALIDATION_PIPELINE.md
* Skip fs_edge / live_vendor unless this feature requires them and they are configured
* Test-summarizer: re-run this feature verify; commit validation trail only if green (never secrets)
* Trail: all role artifacts under WORK DIR for this feature only
"@
    }
    else {
        $extra = @"

Phase build rules:
* Architecture locks: docs/architecture/PLATFORM_FIRST.md and phase docs
* Planner: review git log of prior phases first
* Summarizer: re-run verify then git commit only if green
"@
    }
    $prompt = @"
You are running the Coral agentic pipeline ($($Pipe.name)).

ROLE: $Role
SKILL: use skill $skill (follow it strictly)
PHASE/WAVE: $PhaseId
PHASE FILE: $phaseFile
WORK DIR: $work
REPO: $RepoRoot
$extra
Rules:
* Separate session; do not trust other roles chat.
* Read artifacts and git; write only your role outputs under WORK DIR.
* End your artifact with # agent-approval (see .agent/templates/approval.md).

When finished, ensure # agent-approval is written. Monitor will advance automatically.
"@
    return $prompt
}

function Clear-WorkerFromStatus {
    $st = Read-Status
    $h = [ordered]@{}
    foreach ($p in $st.PSObject.Properties) {
        if ($p.Name -eq "worker") { continue }
        $h[$p.Name] = $p.Value
    }
    $h["updated_at"] = (Get-Date).ToString("o")
    ($h | ConvertTo-Json -Depth 8) | Set-Content $StatusPath -Encoding UTF8
}

function Start-Pipeline([string]$StartPhase, [string]$PipeName) {
    if (-not $PipeName) { $PipeName = Get-DefaultPipelineName }
    $pipe = Load-PipelineDef $PipeName
    if ($pipe.phases -notcontains $StartPhase) {
        throw ("Unknown phase '" + $StartPhase + "' for pipeline " + $PipeName + "; allowed: " + ($pipe.phases -join ", "))
    }
    Ensure-WorkDir $StartPhase | Out-Null
    $first = $pipe.first_role
    Write-Status @{
        pipeline         = $pipe.name
        state            = "running"
        phase            = $StartPhase
        role             = $first
        loop             = 0
        completed_phases = @()
        message          = ($first + " ready. Monitor will assign worker.")
        metrics          = @{
            phase_started_at = (Get-Date).ToString("o")
            roles            = @{}
        }
    }
    Write-Audit "start" ("pipeline=" + $pipe.name + " from=" + $StartPhase + " role=" + $first)
    Send-Notify ("[aiorchestrator] " + $StartPhase + " " + $first + " STARTED") ("pipeline=" + $pipe.name + " Repo: " + $RepoRoot)
    Write-Host ("STARTED pipeline=" + $pipe.name + " " + $StartPhase + " role=" + $first)
}

function Complete-CurrentRole([string]$ForcedResult) {
    $st = Read-Status
    $pipe = Load-PipelineDef $(if ($st.pipeline) { [string]$st.pipeline } else { Get-DefaultPipelineName })
    $pipeName = $pipe.name

    if ($st.state -eq "stopped") { throw "Pipeline stopped. resume first." }
    if ($st.state -eq "waiting_human") { throw "waiting_human - use decide / continue" }
    if (-not $st.phase -or -not $st.role) { throw "No active phase/role. start first." }

    $path = Get-ApprovalPath $st.phase $st.role $pipe
    $parsed = Parse-Approval $path
    if (-not $parsed -or -not $parsed.result) {
        throw ("complete-role refused: missing # agent-approval result in " + $path)
    }
    $res = [string]$parsed.result
    if ($ForcedResult -and $ForcedResult -ne $res) {
        Write-Audit "complete-role" ("forced=" + $ForcedResult + " ignored; artifact=" + $res) "warn"
    }

    Write-Audit "complete-role" ("file=" + $path) $res

    $roleKey = [string]$st.role
    $metrics = @{}
    if ($st.metrics) {
        $st.metrics.PSObject.Properties | ForEach-Object { $metrics[$_.Name] = $_.Value }
    }
    $rolesMet = @{}
    if ($metrics["roles"]) {
        $rm = $metrics["roles"]
        if ($rm -is [hashtable]) { $rolesMet = $rm }
        else { $rm.PSObject.Properties | ForEach-Object { $rolesMet[$_.Name] = $_.Value } }
    }
    $started = $null
    if ($st.worker -and $st.worker.started_at) { $started = [datetime]$st.worker.started_at }
    $ended = Get-Date
    $dur = $null
    if ($started) { $dur = [math]::Round(($ended - $started).TotalSeconds, 1) }
    $rolesMet[$roleKey] = @{
        result     = $res
        pid        = $(if ($st.worker) { $st.worker.pid } else { $null })
        session_id = $(if ($st.worker) { $st.worker.session_id } else { $null })
        started_at = $(if ($st.worker) { $st.worker.started_at } else { $null })
        ended_at   = $ended.ToString("o")
        duration_s = $dur
    }
    $metrics["roles"] = $rolesMet

    if ($res -eq "blocker") {
        Write-Status @{
            pipeline         = $pipeName
            state            = "waiting_human"
            phase            = $st.phase
            role             = $st.role
            loop             = [int]$st.loop
            completed_phases = @($st.completed_phases)
            metrics          = $metrics
            message          = "Blocker. See blockers.md"
        }
        Send-Notify ("[aiorchestrator] " + $st.phase + " " + $st.role + " BLOCKER") "See blockers.md"
        Write-Host "BLOCKER waiting_human"
        return
    }

    if ($res -eq "fail") {
        $loop = [int]$st.loop
        $returnTo = $pipe.fail_return[$st.role]
        $maxLoops = 0
        if ($pipe.max_fail_loops.ContainsKey($st.role)) {
            $maxLoops = [int]$pipe.max_fail_loops[$st.role]
        }
        if ($returnTo -and $maxLoops -gt 0) {
            $loop++
            if ($loop -gt $maxLoops) {
                Write-Status @{
                    pipeline         = $pipeName
                    state            = "waiting_human"
                    phase            = $st.phase
                    role             = $st.role
                    loop             = $loop
                    metrics          = $metrics
                    completed_phases = @($st.completed_phases)
                    message          = ("Max fail loops exceeded for " + $st.role)
                }
                Write-Host "MAX LOOPS waiting_human"
                return
            }
            Write-Status @{
                pipeline         = $pipeName
                state            = "running"
                phase            = $st.phase
                role             = $returnTo
                loop             = $loop
                metrics          = $metrics
                completed_phases = @($st.completed_phases)
                message          = ($st.role + " FAIL -> " + $returnTo + " loop " + $loop)
            }
            Write-Host ("ADVANCE -> " + $returnTo + " loop " + $loop)
            return
        }
        if ($returnTo) {
            Write-Status @{
                pipeline         = $pipeName
                state            = "running"
                phase            = $st.phase
                role             = $returnTo
                loop             = $loop
                metrics          = $metrics
                completed_phases = @($st.completed_phases)
                message          = ($st.role + " FAIL -> " + $returnTo)
            }
            Write-Host ("ADVANCE -> " + $returnTo)
            return
        }
        Write-Status @{
            pipeline         = $pipeName
            state            = "waiting_human"
            phase            = $st.phase
            role             = $st.role
            loop             = $loop
            metrics          = $metrics
            completed_phases = @($st.completed_phases)
            message          = "Unhandled fail."
        }
        return
    }

    $nextRole = Get-NextRole $st.role $pipe
    if ($nextRole) {
        Write-Status @{
            pipeline         = $pipeName
            state            = "running"
            phase            = $st.phase
            role             = $nextRole
            loop             = [int]$st.loop
            metrics          = $metrics
            completed_phases = @($st.completed_phases)
            message          = ($st.role + " PASS -> " + $nextRole)
        }
        Send-Notify ("[aiorchestrator] " + $st.phase + " " + $st.role + " PASS") ("Next: " + $nextRole)
        Write-Host ("ADVANCE -> " + $nextRole)
        return
    }

    $nextPhase = Get-NextPhase $st.phase $pipe
    $done = @()
    if ($st.completed_phases) { $done = @($st.completed_phases) }
    $done += $st.phase
    Send-Notify ("[aiorchestrator] " + $st.phase + " PHASE COMPLETE") ("Next: " + $nextPhase)

    # Archive validation trails outside the app tree (product-validation).
    if ($pipeName -eq "product-validation") {
        $arch = Join-Path $PSScriptRoot "Archive-FeatureRound.ps1"
        if (Test-Path $arch) {
            try {
                & $arch -RepoRoot $RepoRoot -FeatureId ([string]$st.phase) -Result "pass" -AppCommit (git -C $RepoRoot rev-parse HEAD)
            }
            catch {
                Write-Warning ("Archive-FeatureRound failed: " + $_)
            }
        }
    }

    $pauseAfter = $false
    if ($pipe.PSObject.Properties.Name -contains "pause_after_phase") {
        $pauseAfter = [bool]$pipe.pause_after_phase
    }
    elseif ($pipeName -eq "product-validation") {
        $pauseAfter = $true
    }

    if ($pauseAfter) {
        Write-Status @{
            pipeline         = $pipeName
            state            = "waiting_human"
            phase            = $st.phase
            role             = $null
            loop             = 0
            completed_phases = $done
            metrics          = $metrics
            next_phase       = $nextPhase
            message          = ($st.phase + " PASS. Paused. Evidence archived. Start next with: agent.ps1 next-feature" + $(if ($nextPhase) { " (next=$nextPhase)" } else { " (none)" }))
        }
        Write-Audit "phase-complete-paused" ($st.phase + " next=" + $nextPhase)
        Write-Host ("PHASE DONE (paused) " + $st.phase + " next=" + $nextPhase)
        return
    }

    if ($nextPhase) {
        Ensure-WorkDir $nextPhase | Out-Null
        Write-Status @{
            pipeline         = $pipeName
            state            = "running"
            phase            = $nextPhase
            role             = $pipe.first_role
            loop             = 0
            completed_phases = $done
            metrics          = @{
                phase_started_at = (Get-Date).ToString("o")
                roles            = @{}
                prior_phase      = $st.phase
                prior_metrics    = $metrics
            }
            message          = ($st.phase + " done -> " + $nextPhase + " " + $pipe.first_role)
        }
        Write-Audit "phase-complete" ($st.phase + " -> " + $nextPhase)
        Write-Host ("PHASE DONE -> " + $nextPhase)
    }
    else {
        Write-Status @{
            pipeline         = $pipeName
            state            = "pipeline_done"
            phase            = $st.phase
            role             = $null
            loop             = 0
            completed_phases = $done
            metrics          = $metrics
            message          = "All phases complete."
        }
        Write-Audit "pipeline-done" $st.phase
        Write-Host "PIPELINE DONE"
    }
}

switch ($Command) {
    "mail-test" {
        Send-Notify "[aiorchestrator] mail test" ("Repo: " + $RepoRoot)
    }
    "start" {
        $p = $From
        if (-not $p) { $p = $Phase }
        $pipeName = $Pipeline
        if (-not $pipeName) { $pipeName = Get-DefaultPipelineName }
        $pipe = Load-PipelineDef $pipeName
        if (-not $p) { $p = $pipe.phases[0] }
        Start-Pipeline $p $pipeName
    }
    "status" {
        Read-Status | ConvertTo-Json -Depth 8
    }
    "audit" {
        if (-not (Test-Path $AuditPath)) { Write-Host "(no audit yet)"; break }
        Get-Content $AuditPath -Tail $Tail -Encoding UTF8
    }
    "stop" {
        $st = Read-Status
        $pipeName = $(if ($st.pipeline) { [string]$st.pipeline } else { Get-DefaultPipelineName })
        Write-Status @{
            pipeline         = $pipeName
            state            = "stopped"
            phase            = $st.phase
            role             = $st.role
            loop             = [int]$st.loop
            completed_phases = @($st.completed_phases)
            metrics          = $st.metrics
            message          = "Stopped by operator."
            prior_state      = $st.state
        }
        Write-Audit "stop" "operator"
        Write-Host "Stopped."
    }
    "resume" {
        $st = Read-Status
        if ($st.state -ne "stopped" -and $st.state -ne "waiting_human") {
            Write-Host ("Nothing to resume state=" + $st.state)
            break
        }
        $pipe = Load-PipelineDef $(if ($st.pipeline) { [string]$st.pipeline } else { Get-DefaultPipelineName })
        $role = $st.role
        if (-not $role) { $role = $pipe.first_role }
        Write-Status @{
            pipeline         = $pipe.name
            state            = "running"
            phase            = $st.phase
            role             = $role
            loop             = [int]$st.loop
            completed_phases = @($st.completed_phases)
            metrics          = $st.metrics
            message          = "Resumed. Monitor will assign."
        }
        Write-Audit "resume" ($st.phase + "/" + $role)
        Write-Host ("Resumed -> " + $role)
    }
    "restart-phase" {
        $p = $Phase
        if (-not $p) { throw "Specify -Phase phase-a (or validation-v1)" }
        $st = Read-Status
        $pipeName = $Pipeline
        if (-not $pipeName) {
            $pipeName = $(if ($st.pipeline) { [string]$st.pipeline } else { Get-DefaultPipelineName })
        }
        $pipe = Load-PipelineDef $pipeName
        if ($pipe.phases -notcontains $p) {
            throw ("Phase " + $p + " not in pipeline " + $pipeName)
        }
        $work = Join-Path $AgentDir ("work\" + $p)
        if (Test-Path $work) {
            $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
            $archive = Join-Path $AgentDir ("work\_archive\" + $p + "-" + $stamp)
            New-Item -ItemType Directory -Force -Path (Split-Path $archive) | Out-Null
            Move-Item $work $archive
            Write-Host ("Archived " + $archive)
        }
        Ensure-WorkDir $p | Out-Null
        Write-Status @{
            pipeline         = $pipe.name
            state            = "running"
            phase            = $p
            role             = $pipe.first_role
            loop             = 0
            completed_phases = @()
            metrics          = @{ phase_started_at = (Get-Date).ToString("o"); roles = @{} }
            message          = ("Phase " + $p + " restarted at " + $pipe.first_role + ".")
        }
        Write-Audit "restart-phase" ($pipe.name + "/" + $p)
        Write-Host ("Restarted " + $p + " pipeline=" + $pipe.name)
    }
    "continue" {
        $st = Read-Status
        if ($st.state -ne "waiting_human") {
            Write-Host ("Not waiting_human state=" + $st.state)
            break
        }
        $pipeName = $(if ($st.pipeline) { [string]$st.pipeline } else { Get-DefaultPipelineName })
        Write-Status @{
            pipeline         = $pipeName
            state            = "running"
            phase            = $st.phase
            role             = $st.role
            loop             = [int]$st.loop
            completed_phases = @($st.completed_phases)
            metrics          = $st.metrics
            message          = "Continued after human."
        }
        Write-Audit "continue" "operator"
        Write-Host "Continued."
    }
    "decide" {
        if (-not $Id) { throw "Provide -Id and -Answer" }
        $queue = @()
        if (Test-Path $QueuePath) {
            $queue = @(Get-Content $QueuePath -Raw -Encoding UTF8 | ConvertFrom-Json)
        }
        $queue += [pscustomobject]@{
            id     = $Id
            answer = $Answer
            at     = (Get-Date).ToString("o")
            phase  = (Read-Status).phase
        }
        ($queue | ConvertTo-Json -Depth 5) | Set-Content $QueuePath -Encoding UTF8
        Write-Audit "decide" ($Id + "=" + $Answer)
        Write-Host ("Recorded decision " + $Id)
    }
    "next-prompt" {
        $st = Read-Status
        if ($st.state -eq "stopped") { throw "Stopped. resume first." }
        if ($st.state -eq "pipeline_done") { Write-Host "Pipeline done."; break }
        if (-not $st.phase -or -not $st.role) { throw "No active role. start first." }
        $pipe = Load-PipelineDef $(if ($st.pipeline) { [string]$st.pipeline } else { Get-DefaultPipelineName })
        $prompt = Build-Prompt $st.phase $st.role $pipe
        $out = Join-Path $AgentDir ("work\" + $st.phase + "\NEXT_PROMPT_" + $st.role + ".txt")
        Ensure-WorkDir $st.phase | Out-Null
        Set-Content -Path $out -Value $prompt -Encoding UTF8
        Write-Host ("PROMPT_FILE=" + $out)
        Write-Host $prompt
        Write-Audit "next-prompt" ($st.phase + "/" + $st.role)
    }
    "complete-role" {
        Complete-CurrentRole $Result
    }
    "assign-role" {
        $worker = Join-Path $PSScriptRoot "Start-RoleWorker.ps1"
        & $worker -RepoRoot $RepoRoot
    }
    "next-feature" {
        $st = Read-Status
        $pipe = Load-PipelineDef $(if ($st.pipeline) { [string]$st.pipeline } else { "product-validation" })
        if ($st.state -notin @("waiting_human", "stopped", "idle")) {
            throw ("next-feature only when paused/stopped; state=" + $st.state)
        }
        $next = $null
        if ($st.next_phase) { $next = [string]$st.next_phase }
        if (-not $next -and $From) { $next = $From }
        if (-not $next -and $Phase) { $next = $Phase }
        if (-not $next) {
            $last = $null
            if ($st.completed_phases -and @($st.completed_phases).Count -gt 0) {
                $last = [string](@($st.completed_phases)[-1])
            }
            elseif ($st.phase) { $last = [string]$st.phase }
            if ($last) { $next = Get-NextPhase $last $pipe }
        }
        if (-not $next) { throw "No next feature. Pipeline complete or specify -From F-..." }
        Start-Pipeline $next $pipe.name
        Write-Host ("NEXT FEATURE STARTED " + $next)
    }
    "archive-round" {
        $st = Read-Status
        $fid = $Phase
        if (-not $fid) { $fid = $st.phase }
        if (-not $fid) { throw "Specify -Phase F-..." }
        $arch = Join-Path $PSScriptRoot "Archive-FeatureRound.ps1"
        & $arch -RepoRoot $RepoRoot -FeatureId $fid -Result "pass" -AppCommit (git -C $RepoRoot rev-parse HEAD)
    }
    "monitor-start" {
        $mon = Join-Path $PSScriptRoot "Monitor.ps1"
        $pidFile = Join-Path $AgentDir "monitor.pid"
        if (Test-Path $pidFile) {
            $old = [int]((Get-Content $pidFile -Raw).Trim())
            if (Get-Process -Id $old -ErrorAction SilentlyContinue) {
                Write-Host ("Monitor already running pid=" + $old)
                break
            }
        }
        $argList = @(
            "-NoProfile", "-ExecutionPolicy", "Bypass",
            "-File", $mon, "-IntervalSec", "60", "-RepoRoot", $RepoRoot
        )
        $p = Start-Process -FilePath "powershell.exe" -ArgumentList $argList -PassThru -WindowStyle Hidden
        Start-Sleep -Seconds 2
        $mp = if (Test-Path $pidFile) { (Get-Content $pidFile -Raw).Trim() } else { [string]$p.Id }
        Write-Host ("Monitor started pid=" + $mp + " (Hidden, poll 60s, dispatch-only)")
        Write-Audit "monitor-start" ("pid=" + $mp + " window=Hidden")
        Send-Notify "[aiorchestrator] MONITOR STARTED" ("pid=" + $mp)
    }
    "monitor-stop" {
        $pidFile = Join-Path $AgentDir "monitor.pid"
        if (Test-Path $pidFile) {
            $old = [int]((Get-Content $pidFile -Raw).Trim())
            Stop-Process -Id $old -Force -ErrorAction SilentlyContinue
            Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
            Write-Host ("Monitor stopped pid=" + $old)
            Write-Audit "monitor-stop" ("pid=" + $old)
        }
        else {
            Write-Host "No monitor.pid"
        }
    }
    default { throw "Unknown command" }
}
