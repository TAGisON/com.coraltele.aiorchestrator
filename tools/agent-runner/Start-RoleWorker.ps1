<#
.SYNOPSIS
  Dispatch current role: write prompt + DISPATCH artifact. Default: no UI.

.DESCRIPTION
  Does NOT open Cursor or a console window unless -OpenCursor is set
  (or .agent/config.yaml monitor.assign_opens_new_cursor_window: true).

  Parent/operator runs the role from NEXT_PROMPT_*.txt (Task agent, existing
  chat, etc.). Monitor waits for # agent-approval and does not re-dispatch.
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path,
    [switch]$OpenCursor,
    [switch]$Force
)

$ErrorActionPreference = "Stop"
$AgentDir = Join-Path $RepoRoot ".agent"
$StatusPath = Join-Path $AgentDir "status.json"
$ConfigPath = Join-Path $AgentDir "config.yaml"
$Runner = Join-Path $RepoRoot "tools\agent-runner\agent.ps1"
$AuditPath = Join-Path $AgentDir "audit.jsonl"

function Test-ConfigOpensCursor {
    if (-not (Test-Path $ConfigPath)) { return $false }
    $raw = Get-Content $ConfigPath -Raw -Encoding UTF8
    if ($raw -match '(?m)^\s*assign_opens_new_cursor_window:\s*true\s*$') { return $true }
    return $false
}

function Write-AuditRow([string]$Detail, [string]$ResultName = "ok") {
    $stLocal = Get-Content $StatusPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $row = [ordered]@{
        ts     = (Get-Date).ToString("o")
        cmd    = "assign-role"
        phase  = $stLocal.phase
        role   = $stLocal.role
        state  = $stLocal.state
        result = $ResultName
        detail = $Detail
    }
    Add-Content -Path $AuditPath -Value (($row | ConvertTo-Json -Compress)) -Encoding UTF8
}

if (-not (Test-Path $StatusPath)) { throw "No status.json" }
$st = Get-Content $StatusPath -Raw -Encoding UTF8 | ConvertFrom-Json
if ($st.state -ne "running") { throw ("state=" + $st.state) }
if (-not $st.phase -or -not $st.role) { throw "missing phase/role" }

# Idempotent: already dispatched for this role → do not flash UI again
if (-not $Force -and $st.worker -and $st.worker.role -eq $st.role -and $st.worker.phase -eq $st.phase) {
    $ws = [string]$st.worker.status
    if ($ws -in @("dispatched", "assigned", "awaiting")) {
        Write-Host ("ALREADY_DISPATCHED role=" + $st.role + " session=" + $st.worker.session_id + " (use -Force to rewrite prompt)")
        Write-AuditRow ("skip already dispatched session=" + $st.worker.session_id)
        return
    }
}

# next-prompt in-process (no nested console window)
& $Runner -Command next-prompt -RepoRoot $RepoRoot | Out-Null
$promptFile = Join-Path $AgentDir ("work\" + $st.phase + "\NEXT_PROMPT_" + $st.role + ".txt")
if (-not (Test-Path $promptFile)) { throw ("prompt missing: " + $promptFile) }

$marker = Join-Path $AgentDir ("work\" + $st.phase + "\." + $st.role + ".consumed")
if (Test-Path $marker) { Remove-Item $marker -Force }

$sessionId = [guid]::NewGuid().ToString("N").Substring(0, 12)
$wantCursor = $OpenCursor -or (Test-ConfigOpensCursor)
$mode = "dispatch"
$procId = $null
$workerStatus = "dispatched"

if ($wantCursor) {
    $cursorCmd = Get-Command cursor -ErrorAction SilentlyContinue
    if (-not $cursorCmd) { throw "cursor CLI not on PATH (needed because OpenCursor/config enabled)" }
    # Still avoid stealing focus: start without forcing Normal foreground.
    $proc = Start-Process -FilePath $cursorCmd.Source -ArgumentList @("-n", $promptFile) -PassThru -WindowStyle Minimized
    $procId = $proc.Id
    $mode = "cursor"
    $workerStatus = "assigned"
}

$h = [ordered]@{}
foreach ($p in $st.PSObject.Properties) { $h[$p.Name] = $p.Value }
$h["worker"] = [ordered]@{
    pid         = $procId
    session_id  = $sessionId
    role        = $st.role
    phase       = $st.phase
    prompt_file = $promptFile
    started_at  = (Get-Date).ToString("o")
    status      = $workerStatus
    mode        = $mode
}
$h["message"] = ("Dispatched role=" + $st.role + " mode=" + $mode + " session=" + $sessionId + " prompt=" + $promptFile)
$h["updated_at"] = (Get-Date).ToString("o")
($h | ConvertTo-Json -Depth 8) | Set-Content $StatusPath -Encoding UTF8

$dispatch = Join-Path $AgentDir ("work\" + $st.phase + "\DISPATCH_" + $st.role + ".json")
@{
    session_id  = $sessionId
    pid         = $procId
    role        = $st.role
    phase       = $st.phase
    prompt_file = $promptFile
    assigned_at = (Get-Date).ToString("o")
    skill       = $(
        $pipeName = if ($st.pipeline) { [string]$st.pipeline } else { "coral-phase" }
        $pipePath = Join-Path $AgentDir ("pipelines\" + $pipeName + ".json")
        if (Test-Path $pipePath) {
            $pdef = Get-Content $pipePath -Raw -Encoding UTF8 | ConvertFrom-Json
            $sk = $pdef.skills.PSObject.Properties | Where-Object { $_.Name -eq $st.role } | Select-Object -First 1
            if ($sk) { [string]$sk.Value } else { $pipeName + "-" + $st.role }
        }
        else { "coral-phase-" + $st.role }
    )
    mode        = $mode
} | ConvertTo-Json | Set-Content $dispatch -Encoding UTF8

Write-AuditRow ("mode=" + $mode + " session=" + $sessionId + " pid=" + $procId)

Write-Host ("DISPATCHED " + $st.role + " mode=" + $mode + " session=" + $sessionId)
Write-Host ("PROMPT " + $promptFile)
if ($mode -eq "dispatch") {
    Write-Host "No Cursor/console opened. Run the prompt from an existing agent session (or set assign_opens_new_cursor_window: true)."
}
