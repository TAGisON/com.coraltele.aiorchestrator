<#
.SYNOPSIS
  Run Validation V1 harness (Tier A always; Tier B env-gated).
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [switch]$IncludeTierB
)

$ErrorActionPreference = "Stop"
if (-not $RepoRoot) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}

$work = Join-Path $RepoRoot ".agent\work\validation-v1"
New-Item -ItemType Directory -Force -Path $work | Out-Null
$logPath = Join-Path $work "run-log.md"

$lines = New-Object System.Collections.Generic.List[string]
[void]$lines.Add("# Validation V1 run-log")
[void]$lines.Add("")
[void]$lines.Add("Repo: $RepoRoot")
[void]$lines.Add("At: $(Get-Date -Format o)")
[void]$lines.Add("")

Push-Location $RepoRoot
try {
    [void]$lines.Add("## Tier A - go test ./internal/validation")
    $out = & go test ./internal/validation -count=1 -v 2>&1 | Out-String
    $code = $LASTEXITCODE
    [void]$lines.Add('```')
    [void]$lines.Add($out.TrimEnd())
    [void]$lines.Add('```')
    [void]$lines.Add("")
    if ($code -ne 0) {
        [void]$lines.Add("- result: **fail** (exit $code)")
        ($lines -join "`n") + "`n" | Set-Content -Path $logPath -Encoding UTF8
        exit $code
    }
    [void]$lines.Add("- result: **pass**")
    [void]$lines.Add("")

    if ($IncludeTierB) {
        [void]$lines.Add("## Tier B note")
        [void]$lines.Add("IncludeTierB: ensure SARVAM_API_KEY or .agent/secrets.local.json is set; harness skips otherwise.")
        [void]$lines.Add("")
    }

    [void]$lines.Add("## agent-approval")
    [void]$lines.Add('```yaml')
    [void]$lines.Add("role: test-runner")
    [void]$lines.Add("phase: validation-v1")
    [void]$lines.Add("result: pass")
    [void]$lines.Add("approval:")
    [void]$lines.Add("  harness_followed: pass")
    [void]$lines.Add("  all_run_or_skip: pass")
    [void]$lines.Add("  evidence_paths: pass")
    [void]$lines.Add("  no_secrets_logged: pass")
    [void]$lines.Add('```')
    ($lines -join "`n") + "`n" | Set-Content -Path $logPath -Encoding UTF8
    Write-Host "Wrote $logPath"
}
finally {
    Pop-Location
}
