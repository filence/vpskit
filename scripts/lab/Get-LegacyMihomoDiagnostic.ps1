[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$version = 'v1.19.3'
$assetName = 'mihomo-windows-amd64-v1.19.3.zip'
$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$assetsRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot '.build\assets'))
$destination = [System.IO.Path]::GetFullPath((Join-Path $assetsRoot 'mihomo-windows-amd64-v1.19.3'))
if (-not $destination.StartsWith($assetsRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'Diagnostic destination escaped the protected build assets directory'
}

$headers = @{
    Accept = 'application/vnd.github+json'
    'User-Agent' = 'vpskit-release-verifier'
    'X-GitHub-Api-Version' = '2022-11-28'
}
$release = Invoke-RestMethod `
    -Uri "https://api.github.com/repos/MetaCubeX/mihomo/releases/tags/$version" `
    -Headers $headers
if ($release.tag_name -ne $version -or $release.draft -or $release.prerelease) {
    throw "Unexpected GitHub release metadata for $version"
}
$assets = @($release.assets | Where-Object name -EQ $assetName)
if ($assets.Count -ne 1) {
    throw "Expected exactly one diagnostic asset named $assetName"
}
$asset = $assets[0]

[System.IO.Directory]::CreateDirectory($destination) | Out-Null
$zipPath = Join-Path $destination $assetName
$partialPath = "$zipPath.partial"
try {
    Invoke-WebRequest -Uri $asset.browser_download_url -Headers $headers -OutFile $partialPath
    if ((Get-Item -LiteralPath $partialPath).Length -ne [int64]$asset.size) {
        throw 'Legacy diagnostic asset size mismatch'
    }
    Move-Item -LiteralPath $partialPath -Destination $zipPath -Force
}
finally {
    if (Test-Path -LiteralPath $partialPath) {
        Remove-Item -LiteralPath $partialPath -Force
    }
}

$extractPath = Join-Path $destination 'extracted'
if (Test-Path -LiteralPath $extractPath) {
    $resolvedExtract = [System.IO.Path]::GetFullPath($extractPath)
    if (-not $resolvedExtract.StartsWith($destination + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw 'Diagnostic extraction path escaped its fixed destination'
    }
    Remove-Item -LiteralPath $resolvedExtract -Recurse -Force
}
Expand-Archive -LiteralPath $zipPath -DestinationPath $extractPath

$executables = @(Get-ChildItem -LiteralPath $extractPath -Recurse -File -Filter '*.exe')
if ($executables.Count -ne 1) {
    throw "Expected exactly one executable in the diagnostic archive; found $($executables.Count)"
}

[pscustomobject]@{
    Release = $release.tag_name
    Asset = $asset.name
    Bytes = [int64]$asset.size
    Sha256 = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Executable = $executables[0].FullName
    Trust = 'official-github-https-size-checked-no-publisher-digest'
}
