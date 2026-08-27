<#
.SYNOPSIS
  Poll .agent/status.json every N seconds; advance roles; track worker PID.
#>
[CmdletBinding()]
param(
    [int]$IntervalSec = 60,
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path,
    [switch]$Once
)

$ErrorActionPreference = "Stop"
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

function Read-Status {
    if (-not (Test-Path $StatusPath)) { return $null }
    return Get-Content $StatusPath -Raw | ConvertFrom-Json
}

function Get-ApprovalResult([string]$Phase, [string]$Role) {
    $map = @{
        planner    = "plan.md"
        coder      = "implementation.md"
        reviewer   = "review.md"
        summarizer = "summary.md"
    }
    $path = Join-Path $AgentDir "work\$Phase\$($map[$Role])"
    if (-not (Test-Path $path)) { return $null }
    $raw = Get-Content $path -Raw
    if ($raw -match '(?m)^\s*result:\s*(\w+)') { return $Matches[1].Trim() }
    return $null
}

function Test-WorkerAlive($st) {
    if (-not $st.worker -or -not $st.worker.pid) { return $false }
    try {
        $p = Get-Process -Id ([int]$st.worker.pid) -ErrorAction Stop
        return $null -ne $p
    }
    catch { return $false }
}

function Invoke-Tick {
    $st = Read-Status
    if (-not $st) {
        Write-Host "No status.json"
        return
    }
    Write-Host ("[{0}] state={1} phase={2} role={3} worker_pid={4}" -f (Get-Date -Format o), $st.state, $st.phase, $st.role, $(if ($st.worker) { $st.worker.pid } else { "-" }))

    if ($st.state -eq "stopped" -or $st.state -eq "pipeline_done" -or $st.state -eq "idle") {
        return
    }
    if ($st.state -eq "waiting_human") {
        Write-Audit "heartbeat waiting_human"
        return
    }
    if ($st.state -ne "running") { return }
    if (-not $st.phase -or -not $st.role) { return }

    $alive = Test-WorkerAlive $st
    $approval = Get-ApprovalResult $st.phase $st.role

    # Role finished writing approval while worker may still be open
    if ($approval -in @("pass", "fail", "blocker")) {
        $marker = Join-Path $AgentDir "work\$($st.phase)\.$($st.role).consumed"
        if (-not (Test-Path $marker)) {
            Write-Host "Detected approval=$approval for $($st.role) — complete-role"
            Write-Audit "auto complete-role $($st.role) $approval"
            & powershell -NoProfile -File $Runner -Command complete-role -Result $approval -RepoRoot $RepoRoot
            New-Item -ItemType File -Force -Path $marker | Out-Null
            # clear worker after advance
            $st2 = Read-Status
            if ($st2.PSObject.Properties.Name -contains "worker") {
                $h = @{}
                $st2.PSObject.Properties | ForEach-Object { $h[$_.Name] = $_.Value }
                $h.Remove("worker")
                $h["updated_at"] = (Get-Date).ToString("o")
                ($h | ConvertTo-Json -Depth 8) | Set-Content $StatusPath -Encoding UTF8
            }
            return
        }
    }

    if (-not $alive) {
        # Need a worker for current role if approval not yet written
        if (-not $approval) {
            Write-Host "No live worker and no approval — assign $($st.role)"
            Write-Audit "assign-role $($st.role)"
            & powershell -NoProfile -File $Worker -RepoRoot $RepoRoot
        }
        else {
            Write-Audit "heartbeat worker dead approval=$approval waiting consume"
        }
    }
    else {
        Write-Audit "heartbeat worker_alive pid=$($st.worker.pid)"
    }
}

# Record monitor pid
$PID | Set-Content $PidPath -Encoding ASCII
Write-Audit "monitor start interval=$IntervalSec pid=$PID"
Write-Host "Monitor running pid=$PID interval=${IntervalSec}s (Ctrl+C to stop). AGENT_NO_MAIL=$env:AGENT_NO_MAIL"

try {
    while ($true) {
        try { Invoke-Tick } catch { Write-Warning $_; Write-Audit "tick-error $_" }
        if ($Once) { break }
        Start-Sleep -Seconds $IntervalSec
    }
}
finally {
    if (Test-Path $PidPath) {
        $cur = Get-Content $PidPath -Raw
        if ($cur.Trim() -eq "$PID") { Remove-Item $PidPath -Force -ErrorAction SilentlyContinue }
    }
    Write-Audit "monitor stop pid=$PID"
}
