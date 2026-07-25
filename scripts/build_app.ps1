# Build K-Autokey.exe with required Wails production tags (GUI, no console window).
# Output: build\bin\K-Autokey.exe
#
# Usage:
#   .\scripts\build_app.ps1              # release + UPX (if available)
#   .\scripts\build_app.ps1 -NoUPX       # skip compression
#   .\scripts\build_app.ps1 -FetchUPX    # download portable UPX when missing

param(
    [switch]$NoUPX,
    [switch]$FetchUPX
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

$OutDir = Join-Path $Root "build\bin"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$Out = Join-Path $OutDir "K-Autokey.exe"

function Format-Size([long]$bytes) {
    return ("{0:N2} MB" -f ($bytes / 1MB))
}

function Find-Tool([string]$name) {
    $cmd = Get-Command $name -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $gobin = $env:GOPATH
    if (-not $gobin) { $gobin = Join-Path $env:USERPROFILE "go" }
    $candidate = Join-Path $gobin "bin\$name.exe"
    if (Test-Path $candidate) { return $candidate }
    return $null
}

function Ensure-RsrcSyso {
    # Regenerate rsrc.syso (icon + manifest) if rsrc.exe is available,
    # so the exe embeds the logo and admin manifest. Falls back to committed syso.
    $rsrc = Find-Tool "rsrc"
    if (-not $rsrc) { return }
    $ico = Join-Path $Root "app.ico"
    $man = Join-Path $Root "embedded\app.manifest"
    if (-not (Test-Path $ico) -or -not (Test-Path $man)) { return }
    & $rsrc -manifest $man -ico $ico -o (Join-Path $Root "rsrc.syso")
    if ($LASTEXITCODE -eq 0) {
        Write-Host "==> rsrc: regenerated rsrc.syso (icon + manifest)"
    }
}

function Find-UPX {
    $cmd = Get-Command upx -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $local = Join-Path $Root "tools\upx\upx.exe"
    if (Test-Path $local) { return $local }
    return $null
}

function Ensure-UPX {
    $existing = Find-UPX
    if ($existing) { return $existing }
    if (-not $FetchUPX) {
        # Auto-download UPX if missing so compression is on by default
        $FetchUPX = $true
    }

    $tools = Join-Path $Root "tools\upx"
    New-Item -ItemType Directory -Force -Path $tools | Out-Null
    $zip = Join-Path $tools "upx.zip"
    Write-Host "==> downloading UPX..."
    Invoke-WebRequest -Uri "https://github.com/upx/upx/releases/download/v4.2.4/upx-4.2.4-win64.zip" -OutFile $zip -UseBasicParsing
    Expand-Archive -Path $zip -DestinationPath $tools -Force
    $found = Get-ChildItem -Path $tools -Recurse -Filter upx.exe | Select-Object -First 1
    if (-not $found) { throw "UPX download failed" }
    Copy-Item $found.FullName (Join-Path $tools "upx.exe") -Force
    return (Join-Path $tools "upx.exe")
}

Ensure-RsrcSyso

Write-Host "==> go build -tags production,desktop -trimpath (GUI, no console)"
# Use -ldflags= form so PowerShell does not strip -H=windowsgui
$ldflags = "-H=windowsgui -s -w"
go build -tags "production,desktop" -trimpath "-ldflags=$ldflags" -o $Out .
if ($LASTEXITCODE -ne 0) {
    throw "go build failed"
}
if (-not (Test-Path -LiteralPath $Out)) {
    throw "Build failed: $Out not found"
}

# PE subsystem must be 2 (IMAGE_SUBSYSTEM_WINDOWS_GUI)
$fullOut = (Resolve-Path -LiteralPath $Out).Path
$peBytes = [System.IO.File]::ReadAllBytes($fullOut)
$peOffset = [BitConverter]::ToInt32($peBytes, 0x3C)
$subsystem = [BitConverter]::ToUInt16($peBytes, $peOffset + 0x5C)
if ($subsystem -ne 2) {
    throw "Build is console subsystem ($subsystem); expected GUI (2). Check -ldflags."
}

$rawSize = (Get-Item -LiteralPath $Out).Length
Write-Host ("Built: {0} ({1}, subsystem=GUI)" -f $Out, (Format-Size $rawSize))

if (-not $NoUPX) {
    $upx = Ensure-UPX
    if ($upx) {
        Write-Host "==> UPX compress (--best --lzma)"
        & $upx --best --lzma $Out
        if ($LASTEXITCODE -ne 0) {
            Write-Host "UPX failed; keeping uncompressed binary."
        }
        else {
            $packed = (Get-Item -LiteralPath $Out).Length
            Write-Host ("Compressed: {0} -> {1}" -f (Format-Size $rawSize), (Format-Size $packed))
        }
    }
    else {
        Write-Host "UPX not found. Install UPX or re-run with -FetchUPX for ~3MB output."
    }
}

Write-Host "Done. Run this exe. Do not use plain go run/go build without -tags and -H=windowsgui."
