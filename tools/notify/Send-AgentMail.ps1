<#
.SYNOPSIS
  Send pipeline notification mail via Coral SMTP (SSL 465).
.NOTES
  Reads .agent/secrets.local.json only. Never commit that file.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Subject,
    [Parameter(Mandatory = $true)][string]$Body,
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
)

$ErrorActionPreference = "Stop"
$secretsPath = Join-Path $RepoRoot ".agent\secrets.local.json"
if (-not (Test-Path $secretsPath)) {
    throw "Missing $secretsPath — run tools\agent-runner\Install.ps1"
}

$secrets = Get-Content $secretsPath -Raw | ConvertFrom-Json
$smtp = $secrets.smtp
if (-not $smtp) { throw "secrets.local.json missing smtp section" }

$from = [string]$smtp.from
$toList = @($smtp.to)
if (-not $toList -or $toList.Count -eq 0) { $toList = @($smtp.username) }

# System.Net.Mail SmtpClient: port 465 + EnableSsl = implicit SSL (common for hosted mail)
$msg = New-Object System.Net.Mail.MailMessage
$msg.From = $from
foreach ($addr in $toList) { [void]$msg.To.Add([string]$addr) }
$msg.Subject = $Subject
$msg.Body = $Body
$msg.IsBodyHtml = $false

$client = New-Object System.Net.Mail.SmtpClient($smtp.host, [int]$smtp.port)
$client.EnableSsl = [bool]$smtp.ssl
$client.Credentials = New-Object System.Net.NetworkCredential([string]$smtp.username, [string]$smtp.password)
$client.Timeout = 30000

try {
    $client.Send($msg)
    Write-Host "Mail sent: $Subject"
}
finally {
    $msg.Dispose()
    $client.Dispose()
}
