# Convert a PNG to a multi-resolution Windows ICO file (PNG-embedded entries).
# Usage: .\scripts\png_to_ico.ps1 -Png <pngPath> -Ico <icoPath>

param(
    [Parameter(Mandatory=$true)][string]$Png,
    [Parameter(Mandatory=$true)][string]$Ico
)

$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Drawing

$sizes = @(256, 128, 64, 48, 32, 16)

# Load source PNG
$src = [System.Drawing.Image]::FromFile((Resolve-Path $Png).Path)

# Encode each size as PNG bytes
$pngBytesList = @()
foreach ($s in $sizes) {
    $bmp = New-Object System.Drawing.Bitmap $s, $s
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.DrawImage($src, 0, 0, $s, $s)
    $g.Dispose()

    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
    $pngBytesList += ,$ms.ToArray()
    $ms.Dispose()
}
$src.Dispose()

# Assemble ICO
$msOut = New-Object System.IO.MemoryStream
$bw = New-Object System.IO.BinaryWriter $msOut

# ICONDIR header
$bw.Write([UInt16]0)          # reserved
$bw.Write([UInt16]1)          # type = ICON
$bw.Write([UInt16]$sizes.Count) # count

# Compute offsets: header(6) + entries(16*N)
$dataOffset = 6 + 16 * $sizes.Count
$currentOffset = $dataOffset

# ICONDIRENTRY for each
for ($i = 0; $i -lt $sizes.Count; $i++) {
    $s = $sizes[$i]
    $bytes = $pngBytesList[$i]
    $w = if ($s -eq 256) { [Byte]0 } else { [Byte]$s }
    $h = $w
    $bw.Write([Byte]$w)                 # width
    $bw.Write([Byte]$h)                 # height
    $bw.Write([Byte]0)                  # color count (0 = >=256 colors)
    $bw.Write([Byte]0)                  # reserved
    $bw.Write([UInt16]1)                # planes
    $bw.Write([UInt16]32)               # bit count
    $bw.Write([UInt32]$bytes.Length)    # bytes in res
    $bw.Write([UInt32]$currentOffset)   # offset
    $currentOffset += $bytes.Length
}

# Image data
for ($i = 0; $i -lt $sizes.Count; $i++) {
    $bw.Write($pngBytesList[$i])
}

$bw.Flush()
$outDir = Split-Path -Parent $Ico
$outName = Split-Path -Leaf $Ico
$outPath = Join-Path $outDir $outName
[System.IO.File]::WriteAllBytes($outPath, $msOut.ToArray())
$bw.Dispose()
$msOut.Dispose()

Write-Host "==> ICO written: $outPath ($($sizes.Count) sizes: $($sizes -join ','))"
