<#
.SYNOPSIS
  Copy one feature round trail from app .agent/work into the validation-evidence worktree and commit.
.NOTES
  product-validation.json pipeline removed in P1.9 — set -EvidenceRoot explicitly; pipelines pending new L3 catalog.
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [Parameter(Mandatory = $true)]
    [string]$FeatureId,
    [string]$Result = "pass",
    [string]$AppCommit = "",
    [string]$EvidenceRoot = ""
)

$ErrorActionPreference = "Stop"
if (-not $RepoRoot) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}
if (-not $EvidenceRoot) {
    $pipePath = Join-Path $RepoRoot ".agent\pipelines\product-validation.json"
    if (Test-Path $pipePath) {
        $p = Get-Content $pipePath -Raw | ConvertFrom-Json
        if ($p.evidence_worktree) { $EvidenceRoot = [string]$p.evidence_worktree }
    }
}
if (-not $EvidenceRoot) {
    $EvidenceRoot = Join-Path (Split-Path $RepoRoot -Parent) "com.coraltele.aiorchestrator-validation-evidence"
}
if (-not (Test-Path $EvidenceRoot)) {
    throw ("Evidence worktree missing: " + $EvidenceRoot + " - create with git worktree add")
}

$src = Join-Path $RepoRoot (".agent\work\" + $FeatureId)
if (-not (Test-Path $src)) { throw ("No work dir: " + $src) }

$dst = Join-Path $EvidenceRoot ("rounds\" + $FeatureId)
New-Item -ItemType Directory -Force -Path $dst | Out-Null

$copied = @()
foreach ($name in @("scenarios.md", "fixtures.md", "run-log.md", "audit-report.md", "review.md", "summary.md", "blockers.md")) {
    $p = Join-Path $src $name
    if (Test-Path $p) {
        Copy-Item $p (Join-Path $dst $name) -Force
        $copied += $name
    }
}

if (-not $AppCommit) {
    $AppCommit = (git -C $RepoRoot rev-parse HEAD).Trim()
}
$short = $AppCommit
if ($short.Length -gt 7) { $short = $short.Substring(0, 7) }

$meta = [ordered]@{
    feature_id  = $FeatureId
    result      = $Result
    app_repo    = "com.coraltele.aiorchestrator"
    app_commit  = $AppCommit
    archived_at = (Get-Date).ToString("o")
    artifacts   = $copied
}
($meta | ConvertTo-Json -Depth 5) | Set-Content (Join-Path $dst "meta.json") -Encoding UTF8

$roundsDir = Join-Path $EvidenceRoot "rounds"
$rows = @()
Get-ChildItem $roundsDir -Directory -ErrorAction SilentlyContinue | ForEach-Object {
    $mp = Join-Path $_.FullName "meta.json"
    if (Test-Path $mp) {
        $rows += (Get-Content $mp -Raw | ConvertFrom-Json)
    }
}
$rows = @($rows | Sort-Object { $_.archived_at } -Descending)

$nl = "`n"
$index = "# Validation INDEX" + $nl + $nl
$index += "Living dashboard for com.coraltele.aiorchestrator feature rounds." + $nl + $nl
$index += "## Recent" + $nl + $nl
$index += "| Feature | Result | App commit | Archived | Summary |" + $nl
$index += "|---|---|---|---|---|" + $nl
$i = 0
foreach ($m in $rows) {
    if ($i -ge 10) { break }
    $c = [string]$m.app_commit
    if ($c.Length -gt 7) { $c = $c.Substring(0, 7) }
    $sum = "rounds/" + $m.feature_id + "/summary.md"
    $index += "| " + $m.feature_id + " | **" + $m.result + "** | " + $c + " | " + $m.archived_at + " | [" + $m.feature_id + "](" + $sum + ") |" + $nl
    $i++
}
$index += $nl + "## All rounds" + $nl + $nl
$index += "| # | Feature | Result | App commit | Path |" + $nl
$index += "|---|---|---|---|---|" + $nl
$asc = @($rows | Sort-Object { $_.archived_at })
$num = 1
foreach ($m in $asc) {
    $c = [string]$m.app_commit
    if ($c.Length -gt 7) { $c = $c.Substring(0, 7) }
    $index += "| " + $num + " | " + $m.feature_id + " | " + $m.result + " | " + $c + " | [rounds/" + $m.feature_id + "](rounds/" + $m.feature_id + "/) |" + $nl
    $num++
}
$index += $nl + "## Notes" + $nl + $nl
$index += "- App contract: tests/agent/ in the application repo." + $nl
$index += "- Pipeline pauses after each feature; continue with agent.ps1 next-feature." + $nl
$index += "- Deferred: F-edge-fs-live, F-gw-tts-engine, F-job-interpret; optional: F-sarvam-live." + $nl

Set-Content -Path (Join-Path $EvidenceRoot "INDEX.md") -Value $index -Encoding UTF8

Push-Location $EvidenceRoot
try {
    $ErrorActionPreference = "Continue"
    git add INDEX.md ("rounds/" + $FeatureId) 2>&1 | Out-Null
    $pending = git status --porcelain
    if ($pending) {
        git commit -m ("archive: " + $FeatureId + " " + $Result + " (app " + $short + ")") 2>&1 | Out-Host
        if ($LASTEXITCODE -ne 0) { throw ("git commit failed exit=" + $LASTEXITCODE) }
        Write-Host ("ARCHIVED " + $FeatureId + " -> " + $EvidenceRoot)
    }
    else {
        Write-Host ("ARCHIVED " + $FeatureId + " (no git changes)")
    }
}
finally {
    $ErrorActionPreference = "Stop"
    Pop-Location
}
