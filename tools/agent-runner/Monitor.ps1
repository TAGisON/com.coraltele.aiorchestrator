<#
.SYNOPSIS
  Poll status every N seconds; advance roles on approval; dispatch quietly.
  ASCII-only. No foreground windows. Does not re-open Cursor by default.
#>
[CmdletBinding()]
param(
    [int]$IntervalSec = 60,
    [string]$RepoRoot = "",
    [switch]$Once
)

$ErrorActionPreference = "Stop"
if (-not $RepoRoot) {
    if ($PSScriptRoot) {
        $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
    }
    else {
        $RepoRoot = "C:\Users\user\Documents\GitHub\com.coraltele.aiorchestrator"
    }
}

$AgentDir = Join-Path $RepoRoot ".agent"
$StatusPath = Join-Path $AgentDir "status.json"
$AuditPath = Join-Path $AgentDir "audit.jsonl"
$PidPath = Join-Path $AgentDir "monitor.pid"
$Runner = Join-Path $RepoRoot "tools\agent-runner\agent.ps1"
$Worker = Join-Path $RepoRoot "tools\agent-runner\Start-RoleWorker.ps1"

function Write-Audit([string]$Detail) {
    $row = [ordered]@{
        ts     = (Get-Date).ToString("o")
        cmd    = "monitor"
        detail = $Detail
    }
    Add-Content -Path $AuditPath -Value (($row | ConvertTo-Json -Compress)) -Encoding UTF8
}

function Invoke-RunnerQuiet([string[]]$ArgList) {
    # Same process space when possible; Hidden child if needed — never Normal/Minimized flash.
    $all = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $Runner) + $ArgList + @("-RepoRoot", $RepoRoot)
    $p = Start-Process -FilePath "powershell.exe" -ArgumentList $all -Wait -PassThru -WindowStyle Hidden
    if ($p.ExitCode -ne 0) {
        throw ("runner exit " + $p.ExitCode + " args=" + ($ArgList -join " "))
    }
}

function Invoke-WorkerQuiet {
    $all = @(
        "-NoProfile", "-ExecutionPolicy", "Bypass",
        "-File", $Worker, "-RepoRoot", $RepoRoot
    )
    $p = Start-Process -FilePath "powershell.exe" -ArgumentList $all -Wait -PassThru -WindowStyle Hidden
    if ($p.ExitCode -ne 0) {
        throw ("worker exit " + $p.ExitCode)
    }
}

function Read-Status {
    if (-not (Test-Path $StatusPath)) { return $null }
    return Get-Content $StatusPath -Raw -Encoding UTF8 | ConvertFrom-Json
}

function Get-PipelineArtifactMap($st) {
    $name = "coral-phase"
    if ($st -and $st.pipeline) { $name = [string]$st.pipeline }
    $path = Join-Path $AgentDir ("pipelines\" + $name + ".json")
    if (-not (Test-Path $path)) {
        return @{
            planner = "plan.md"; coder = "implementation.md"
            reviewer = "review.md"; summarizer = "summary.md"
        }
    }
    $raw = Get-Content $path -Raw -Encoding UTF8 | ConvertFrom-Json
    $map = @{}
    $raw.artifacts.PSObject.Properties | ForEach-Object { $map[$_.Name] = [string]$_.Value }
    return $map
}

function Get-ApprovalResult([string]$Phase, [string]$Role) {
    $st = Read-Status
    $map = Get-PipelineArtifactMap $st
    if (-not $map.ContainsKey($Role)) { return $null }
    $path = Join-Path $AgentDir ("work\" + $Phase + "\" + $map[$Role])
    if (-not (Test-Path $path)) { return $null }
    $raw = Get-Content $path -Raw -Encoding UTF8
    if ($raw -match '(?s)#\s*agent-approval\s*\r?\n.*?^\s*result:\s*(\w+)') {
        return $Matches[1].Trim()
    }
    if ($raw -match '(?m)^\s*result:\s*(pass|fail|blocker)\s*$') {
        return $Matches[1].Trim()
    }
    return $null
}

function Test-CursorWorkerAlive($st) {
    if (-not $st.worker) { return $false }
    if ([string]$st.worker.mode -ne "cursor") { return $false }
    if (-not $st.worker.pid) { return $false }
    try {
        $null = Get-Process -Id ([int]$st.worker.pid) -ErrorAction Stop
        return $true
    }
    catch { return $false }
}

function Test-DispatchPending($st) {
    if (-not $st.worker) { return $false }
    if ($st.worker.role -ne $st.role -or $st.worker.phase -ne $st.phase) { return $false }
    $ws = [string]$st.worker.status
    return ($ws -in @("dispatched", "assigned", "awaiting"))
}

function Invoke-Tick {
    $st = Read-Status
    if (-not $st) { Write-Host "No status.json"; return }

    $wpid = "-"
    if ($st.worker -and $st.worker.pid) { $wpid = $st.worker.pid }
    $wmode = if ($st.worker) { $st.worker.mode } else { "-" }
    Write-Host ("[{0}] state={1} phase={2} role={3} worker_mode={4} pid={5}" -f (Get-Date -Format o), $st.state, $st.phase, $st.role, $wmode, $wpid)

    if ($st.state -in @("stopped", "pipeline_done", "idle")) { return }
    if ($st.state -eq "waiting_human") { Write-Audit "heartbeat waiting_human"; return }
    if ($st.state -ne "running") { return }
    if (-not $st.phase -or -not $st.role) { return }

    $approval = Get-ApprovalResult $st.phase $st.role
    $marker = Join-Path $AgentDir ("work\" + $st.phase + "\." + $st.role + ".consumed")

    Write-Audit ("tick role=" + $st.role + " approval=" + $approval + " pending=" + (Test-DispatchPending $st))

    if ($approval -in @("pass", "fail", "blocker")) {
        if (-not (Test-Path $marker)) {
            Write-Host ("Detected approval=" + $approval + " for " + $st.role + " -> complete-role")
            Write-Audit ("auto complete-role " + $st.role + " " + $approval)
            Invoke-RunnerQuiet @("complete-role", "-Result", $approval)
            New-Item -ItemType File -Force -Path $marker | Out-Null
            $st2 = Read-Status
            $h = [ordered]@{}
            foreach ($p in $st2.PSObject.Properties) {
                if ($p.Name -eq "worker") { continue }
                $h[$p.Name] = $p.Value
            }
            $h["updated_at"] = (Get-Date).ToString("o")
            ($h | ConvertTo-Json -Depth 8) | Set-Content $StatusPath -Encoding UTF8
            return
        }
        else {
            Write-Audit ("approval already consumed for " + $st.role)
        }
        return
    }

    # Waiting on an already-dispatched role: never re-open Cursor / re-dispatch.
    if (Test-DispatchPending $st) {
        if ([string]$st.worker.mode -eq "cursor" -and -not (Test-CursorWorkerAlive $st)) {
            Write-Audit ("cursor pid dead; still waiting for artifact (no re-open) role=" + $st.role)
        }
        else {
            Write-Audit ("heartbeat awaiting_artifact role=" + $st.role)
        }
        return
    }

    # No worker for this role yet → quiet dispatch (files only by default).
    Write-Host ("No dispatch for " + $st.role + " - assign quietly")
    Write-Audit ("assign-role " + $st.role)
    Invoke-WorkerQuiet
}

$PID | Set-Content $PidPath -Encoding ASCII
Write-Audit ("monitor start interval=" + $IntervalSec + " pid=" + $PID)
Write-Host ("Monitor running pid=" + $PID + " interval=" + $IntervalSec + "s (Hidden; dispatch-only by default)")

try {
    while ($true) {
        try { Invoke-Tick } catch { Write-Warning $_; Write-Audit ("tick-error " + $_) }
        if ($Once) { break }
        Start-Sleep -Seconds $IntervalSec
    }
}
finally {
    if (Test-Path $PidPath) {
        $cur = (Get-Content $PidPath -Raw).Trim()
        if ($cur -eq "$PID") { Remove-Item $PidPath -Force -ErrorAction SilentlyContinue }
    }
    Write-Audit ("monitor stop pid=" + $PID)
}
