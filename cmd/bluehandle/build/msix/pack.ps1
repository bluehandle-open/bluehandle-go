# Pack bluehandle.exe into an unsigned MSIX for the Microsoft Store, which
# signs Store packages itself. Needs the Windows 10/11 SDK (makeappx.exe).
#
#   pwsh cmd/bluehandle/build/msix/pack.ps1 -Version 1.0.1 -Exe bin/bluehandle.exe -Out dist/bluehandle.msix
#
# Identity values default to the Store product (Partner Center > Product
# identity); the environment variables MSIX_IDENTITY_NAME, MSIX_PUBLISHER and
# MSIX_PUBLISHER_DISPLAY_NAME override them.
param(
    [Parameter(Mandatory)] [string] $Version,
    [Parameter(Mandatory)] [string] $Exe,
    [Parameter(Mandatory)] [string] $Out,
    [string] $IdentityName = $env:MSIX_IDENTITY_NAME,
    [string] $Publisher = $env:MSIX_PUBLISHER,
    [string] $PublisherDisplayName = $env:MSIX_PUBLISHER_DISPLAY_NAME
)
$ErrorActionPreference = 'Stop'

if (-not $IdentityName) { $IdentityName = 'whyun.bluehandle' }
if (-not $Publisher) { $Publisher = 'CN=48325D51-0D86-4D4C-811F-9CFEA8D3F947' }
if (-not $PublisherDisplayName) { $PublisherDisplayName = 'whyun' }

# The Store wants four numeric parts with the last one 0, e.g. 1.0.1.0.
$parts = @(($Version -replace '^v', '' -replace '[-+].*$', '').Split('.') | Where-Object { $_ -ne '' })
if ($parts.Count -lt 1 -or $parts.Count -gt 3 -or ($parts | Where-Object { $_ -notmatch '^\d+$' })) {
    throw "Version '$Version' is not of the form 1, 1.2 or 1.2.3"
}
while ($parts.Count -lt 3) { $parts += '0' }
$msixVersion = ($parts + '0') -join '.'

$makeappx = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\*\x64\makeappx.exe" -ErrorAction SilentlyContinue |
    Sort-Object { [version]$_.Directory.Parent.Name } -Descending | Select-Object -First 1
if (-not $makeappx) { throw 'makeappx.exe not found; install the Windows SDK' }

$here = $PSScriptRoot
$stage = Join-Path ([IO.Path]::GetTempPath()) ("bluehandle-msix-" + [guid]::NewGuid())
New-Item -ItemType Directory $stage | Out-Null
try {
    Copy-Item (Join-Path $here 'Assets') $stage -Recurse
    Copy-Item $Exe (Join-Path $stage 'bluehandle.exe')

    $esc = { param($s) [Security.SecurityElement]::Escape($s) }
    $manifest = (Get-Content (Join-Path $here 'AppxManifest.xml') -Raw -Encoding utf8).
        Replace('{{IDENTITY_NAME}}', (& $esc $IdentityName)).
        Replace('{{PUBLISHER}}', (& $esc $Publisher)).
        Replace('{{PUBLISHER_DISPLAY_NAME}}', (& $esc $PublisherDisplayName)).
        Replace('{{VERSION}}', $msixVersion)
    [IO.File]::WriteAllText((Join-Path $stage 'AppxManifest.xml'), $manifest, [Text.UTF8Encoding]::new($false))

    $outDir = Split-Path -Parent $Out
    if ($outDir) { New-Item -ItemType Directory $outDir -Force | Out-Null }
    & $makeappx.FullName pack /o /d $stage /p $Out
    if ($LASTEXITCODE -ne 0) { throw "makeappx failed with exit code $LASTEXITCODE" }
    Write-Host "Packed $Out (version $msixVersion, $IdentityName, $Publisher)"
} finally {
    Remove-Item $stage -Recurse -Force
}
