<#
.SYNOPSIS
  Assign current role to an independent worker session: write prompt, open new Cursor window, track PID.
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
)

$ErrorActionPreference = "Stop"
$AgentDir = Join-Path $RepoRoot ".agent"
$StatusPath = Join-Path $AgentDir "status.json"
$Runner = Join-Path $RepoRoot "tools\agent-runner\agent.ps1"

if (-not (Test-Path $StatusPath)) { throw "No status.json — start pipeline first" }
$st = Get-Content $StatusPath -Raw | ConvertFrom-Json
if ($st.state -ne "running") { throw "state=$($st.state); only assign when running" }
if (-not $st.phase -or -not $st.role) { throw "missing phase/role" }

# Generate next-prompt file
& powershell -NoProfile -File $Runner -Command next-prompt -RepoRoot $RepoRoot | Out-Null
$promptFile = Join-Path $AgentDir "work\$($st.phase)\NEXT_PROMPT_$($st.role).txt"
if (-not (Test-Path $promptFile)) { throw "prompt file missing: $promptFile" }

# Clear consumed marker so monitor can pick up a new approval
$marker = Join-Path $AgentDir "work\$($st.phase)\.$($st.role).consumed"
if (Test-Path $marker) { Remove-Item $marker -Force }

$sessionId = [guid]::NewGuid().ToString("N").Substring(0, 12)
$workDir = Join-Path $AgentDir "work\$($st.phase)"
$handshake = Join-Path $workDir "WORKER_$($st.role).json"

# Open a NEW Cursor window on the prompt (independent UI session)
$cursorCmd = (Get-Command cursor -ErrorAction SilentlyContinue)
if (-not $cursorCmd) {
    throw "cursor CLI not on PATH"
}
$proc = Start-Process -FilePath $cursorCmd.Source -ArgumentList @("-n", $promptFile) -PassThru -WindowStyle Normal

# Update status with worker pid + session
$h = [ordered]@{}
$st.PSObject.Properties | ForEach-Object { $h[$_.Name] = $_.Value }
$h["worker"] = [ordered]@{
    pid         = $proc.Id
    session_id  = $sessionId
    role        = $st.role
    phase       = $st.phase
    prompt_file = $promptFile
    started_at  = (Get-Date).ToString("o")
    status      = "assigned"
}
$h["message"] = "Worker assigned role=$($st.role) pid=$($proc.Id) session=$sessionId. Complete role in THAT Cursor window only."
$h["updated_at"] = (Get-Date).ToString("o")
($h | ConvertTo-Json -Depth 8) | Set-Content $StatusPath -Encoding UTF8

@{
    session_id = $sessionId
    pid        = $proc.Id
    role       = $st.role
    phase      = $st.phase
    assigned   = (Get-Date).ToString("o")
} | ConvertTo-Json | Set-Content $handshake -Encoding UTF8

Write-Host "Assigned $($st.role) -> Cursor pid=$($proc.Id) session=$sessionId"
Write-Host "Prompt: $promptFile"
Write-Host "When the role finishes (approval block written), Monitor will advance within ~1 minute,"
Write-Host "or run: .\tools\agent-runner\agent.ps1 complete-role -Result pass"
