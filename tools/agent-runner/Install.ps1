<#
.SYNOPSIS
  One-time setup for Coral agentic phase pipeline.
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
)

$ErrorActionPreference = "Stop"
$agentDir = Join-Path $RepoRoot ".agent"
$secretsPath = Join-Path $agentDir "secrets.local.json"
$examplePath = Join-Path $agentDir "secrets.example.json"

New-Item -ItemType Directory -Force -Path (Join-Path $agentDir "work") | Out-Null

if (-not (Test-Path $secretsPath)) {
    Write-Host "Creating secrets.local.json (gitignored). Password is NOT echoed."
    $user = Read-Host "SMTP username (e.g. ayush.garg@coraltele.com)"
    $secure = Read-Host "SMTP password" -AsSecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        $plain = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
    $hostName = Read-Host "SMTP host [mail.coraltele.com]"
    if ([string]::IsNullOrWhiteSpace($hostName)) { $hostName = "mail.coraltele.com" }
    $portIn = Read-Host "SMTP port [465]"
    if ([string]::IsNullOrWhiteSpace($portIn)) { $portIn = "465" }
    $to = Read-Host "Notify to (comma-separated) [$user]"
    if ([string]::IsNullOrWhiteSpace($to)) { $to = $user }
    $toArr = @($to.Split(",") | ForEach-Object { $_.Trim() } | Where-Object { $_ })

    $obj = [ordered]@{
        smtp = [ordered]@{
            host     = $hostName
            port     = [int]$portIn
            ssl      = $true
            username = $user
            password = $plain
            from     = $user
            to       = $toArr
        }
    }
    $json = $obj | ConvertTo-Json -Depth 5
    Set-Content -Path $secretsPath -Value $json -Encoding UTF8
    Write-Host "Wrote $secretsPath"
}
else {
    Write-Host "Secrets already exist: $secretsPath"
}

# Initial status if missing
$statusPath = Join-Path $agentDir "status.json"
if (-not (Test-Path $statusPath)) {
    $status = @{
        pipeline   = "coral-phase"
        state      = "idle"
        phase      = $null
        role       = $null
        loop       = 0
        updated_at = (Get-Date).ToString("o")
        message    = "Installed. Run: .\tools\agent-runner\agent.ps1 start -From phase-a"
    } | ConvertTo-Json
    Set-Content $statusPath -Value $status -Encoding UTF8
}

$auditPath = Join-Path $agentDir "audit.jsonl"
if (-not (Test-Path $auditPath)) {
    New-Item -ItemType File -Path $auditPath | Out-Null
}

Write-Host ""
Write-Host "Next:"
Write-Host "  .\tools\notify\Send-AgentMail.ps1 -Subject 'pipeline test' -Body 'ok'"
Write-Host "  .\tools\agent-runner\agent.ps1 start -From phase-a"
Write-Host "  .\tools\agent-runner\agent.ps1 next-prompt"
Write-Host ""
Write-Host "SECURITY: If you pasted a mail password in chat, rotate it on the mail server."
