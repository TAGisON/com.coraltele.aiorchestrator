<#
.SYNOPSIS
  Create the aiorchestrator Postgres database on the shared Coral lab instance (like telemetry).
#>
[CmdletBinding()]
param(
    [string]$PgHost = "127.0.0.1",
    [int]$Port = 5432,
    [string]$User = "postgres",
    [string]$Database = "aiorchestrator",
    [string]$Psql = ""
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
if ($exists -eq "1") {
    Write-Host "Database already exists: $Database"
} else {
    & $Psql -U $User -h $PgHost -p $Port -d postgres -c "CREATE DATABASE $Database OWNER $User;"
    Write-Host "Created database: $Database"
}

Write-Host ""
Write-Host "Set in .env:"
Write-Host "DATABASE_URL=postgres://${User}@${PgHost}:${Port}/${Database}?sslmode=disable"
Write-Host "REQUIRE_DATABASE=1"
Write-Host ""
Write-Host "Then: go run ./cmd/aiorchestrator"
Write-Host "Lab UI: http://127.0.0.1:8080/lab/"
