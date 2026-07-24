# Dev run with required Wails tags.
# Usage: .\scripts\dev.ps1

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

Write-Host "Dev mode: go run -tags production ."
Write-Host "Or use: wails dev"
go run -tags production .
