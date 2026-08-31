<#
.SYNOPSIS
  Create (or recreate) the aiorchestrator Postgres database — schema applied on process boot.
  Does not insert engines, credentials, profiles, or any tenant data.
#>
[CmdletBinding()]
param(
    [string]$PgHost = "127.0.0.1",
    [int]$Port = 5432,
    [string]$User = "postgres",
    [string]$Database = "aiorchestrator",
    [string]$Psql = "",
    [switch]$Recreate
)

$ErrorActionPreference = "Stop"
if (-not $Psql) {
    $cmd = Get-Command psql -ErrorAction SilentlyContinue
    if ($cmd) { $Psql = $cmd.Source }
    else {
        foreach ($c in @(
            "C:\Program Files\PostgreSQL\16\bin\psql.exe",
            "C:\Program Files\PostgreSQL\15\bin\psql.exe"
        )) {
            if (Test-Path $c) { $Psql = $c; break }
        }
    }
}
if (-not $Psql -or -not (Test-Path $Psql)) {
    throw "psql not found. Install PostgreSQL client or pass -Psql path."
}

$exists = & $Psql -U $User -h $PgHost -p $Port -d postgres -Atc "SELECT 1 FROM pg_database WHERE datname='$Database'"
if ($Recreate -and $exists -eq "1") {
    Write-Host "Terminating connections and dropping database: $Database"
    & $Psql -U $User -h $PgHost -p $Port -d postgres -v ON_ERROR_STOP=1 -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$Database' AND pid <> pg_backend_pid();"
    & $Psql -U $User -h $PgHost -p $Port -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS $Database;"
    $exists = ""
}
if ($exists -eq "1") {
    Write-Host "Database already exists: $Database (pass -Recreate for a empty wipe)"
} else {
    & $Psql -U $User -h $PgHost -p $Port -d postgres -c "CREATE DATABASE $Database OWNER $User;"
    Write-Host "Created empty database: $Database"
}

Write-Host ""
Write-Host "Boot uses conf/aiorchestrator.properties (database.url). Migrations apply schema only on start."
Write-Host "Then: go run ./cmd/aiorchestrator"
Write-Host "Lab UI: http://127.0.0.1:8011/lab/"
Write-Host "Configure engines + credentials via Control API / lab Settings (nothing is pre-seeded)."
