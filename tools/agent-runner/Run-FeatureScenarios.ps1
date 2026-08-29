<#
.SYNOPSIS
  Run tests/agent scenarios for must_test features (go_test / file_exists / skip).
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$ResultsDir = "",
    [switch]$IncludeOptionalLive,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
if (-not $RepoRoot) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}

$AgentTests = Join-Path $RepoRoot "tests\agent"
$Scenarios = Join-Path $AgentTests "scenarios"
if (-not $ResultsDir) {
    $ResultsDir = Join-Path $AgentTests ("results\run-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
}
New-Item -ItemType Directory -Force -Path $ResultsDir | Out-Null

$logPath = Join-Path $ResultsDir "run-log.md"
$lines = @("# Feature scenario run", "", "Repo: $RepoRoot", "At: $(Get-Date -Format o)", "")

$pass = 0; $fail = 0; $skip = 0

function Get-YamlScalar([string]$Raw, [string]$Key) {
    if ($Raw -match "(?m)^\s*$([regex]::Escape($Key)):\s*(.+)\s*$") {
        return $Matches[1].Trim().Trim('"').Trim("'")
    }
    return $null
}

$files = Get-ChildItem -Path $Scenarios -Filter "F-*.yaml" | Sort-Object Name
foreach ($f in $files) {
    $raw = Get-Content $f.FullName -Raw -Encoding UTF8
    $id = Get-YamlScalar $raw "id"
    $status = Get-YamlScalar $raw "status"
    if (-not $id) { $id = $f.BaseName }

    $lines += "## $id"
    $lines += "- file: $($f.Name)"
    $lines += "- status: $status"

    if ($status -eq "deferred") {
        $skip++; $lines += "- result: **skip** (deferred)"; $lines += ""; continue
    }
    if ($status -eq "optional_live" -and -not $IncludeOptionalLive) {
        $skip++; $lines += "- result: **skip** (optional_live; pass -IncludeOptionalLive)"; $lines += ""; continue
    }
    if ($status -eq "out_of_scope_v1") {
        $skip++; $lines += "- result: **skip** (out_of_scope_v1)"; $lines += ""; continue
    }

    $ok = $true
    $notes = @()

    # file_exists steps
    if ($raw -match '(?m)^\s*-\s*action:\s*file_exists\s*$' -or $raw -match 'action:\s*file_exists') {
        if ($raw -match '(?m)^\s*path:\s*(\S+)') {
            $rel = $Matches[1].Trim()
            $p = Join-Path $RepoRoot $rel
            if (-not (Test-Path $p)) {
                $ok = $false
                $notes += "missing file $rel"
            }
            else { $notes += "file_exists $rel" }
        }
    }

    # go_test packages: naive extract of ./internal/... tokens under packages or steps
    $pkgs = [regex]::Matches($raw, '\./internal/[a-zA-Z0-9_/\-]+') | ForEach-Object { $_.Value } | Select-Object -Unique
    if ($status -eq "must_test" -or ($status -eq "optional_live" -and $IncludeOptionalLive)) {
        foreach ($pkg in $pkgs) {
            if ($DryRun) {
                $notes += "dry-run go test $pkg"
                continue
            }
            Push-Location $RepoRoot
            try {
                $out = & go test $pkg -count=1 2>&1 | Out-String
                $code = $LASTEXITCODE
                $notes += ("go test {0} exit={1}" -f $pkg, $code)
                if ($code -ne 0) {
                    $ok = $false
                    $pkgLog = Join-Path $ResultsDir (($id -replace '[^\w\-]', '_') + "_" + ($pkg -replace '[^\w]', '_') + ".txt")
                    Set-Content -Path $pkgLog -Value $out -Encoding UTF8
                    $notes += "log: $pkgLog"
                }
            }
            finally { Pop-Location }
        }
    }

    # review / git_scan: mark as needs_human_or_agent if no go_test
    if ($raw -match 'action:\s*review' -or $raw -match 'action:\s*git_scan') {
        if ($pkgs.Count -eq 0 -and -not ($raw -match 'file_exists')) {
            $notes += "review/git_scan: agent must confirm (runner recorded check pending)"
        }
    }

    if ($ok) {
        $pass++
        $lines += "- result: **pass**"
    }
    else {
        $fail++
        $lines += "- result: **fail**"
    }
    foreach ($n in $notes) { $lines += "  - $n" }
    $lines += ""
}

$lines += "# Totals"
$lines += "- pass: $pass"
$lines += "- fail: $fail"
$lines += "- skip: $skip"

Set-Content -Path $logPath -Value ($lines -join "`n") -Encoding UTF8
Write-Host ("RESULTS " + $logPath)
Write-Host ("pass=$pass fail=$fail skip=$skip")
if ($fail -gt 0) { exit 1 }
exit 0
