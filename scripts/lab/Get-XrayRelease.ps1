[CmdletBinding()]
param(
    [string]$Version = 'v26.3.27',
    [string]$AssetName = 'Xray-windows-64.zip',

    [string]$ExpectedExecutable = 'xray.exe',

    [Parameter(Mandatory)]
    [string]$DestinationRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$headers = @{
    Accept = 'application/vnd.github+json'
    'User-Agent' = 'vpskit-release-verifier'
    'X-GitHub-Api-Version' = '2022-11-28'
}
$release = Invoke-RestMethod `
    -Uri "https://api.github.com/repos/XTLS/Xray-core/releases/tags/$Version" `
    -Headers $headers
if ($release.tag_name -ne $Version -or $release.draft -or $release.prerelease) {
    throw "Unexpected GitHub release metadata for $Version"
}

$assets = @($release.assets | Where-Object name -EQ $AssetName)
if ($assets.Count -ne 1) {
    throw "Expected exactly one release asset named $AssetName; found $($assets.Count)"
}
$asset = $assets[0]
if ($asset.digest -notmatch '^sha256:([0-9a-f]{64})$') {
    throw 'GitHub release asset is missing a usable SHA-256 digest'
}
$expectedSha256 = $Matches[1]

$destination = [System.IO.Path]::GetFullPath($DestinationRoot)
[System.IO.Directory]::CreateDirectory($destination) | Out-Null
$zipPath = Join-Path $destination $AssetName
$partialPath = "$zipPath.partial"
try {
    Invoke-WebRequest -Uri $asset.browser_download_url -Headers $headers -OutFile $partialPath
    $actualLength = (Get-Item -LiteralPath $partialPath).Length
    if ($actualLength -ne [int64]$asset.size) {
        throw "Size mismatch: expected $($asset.size), got $actualLength"
    }
    $actualSha256 = (Get-FileHash -LiteralPath $partialPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualSha256 -ne $expectedSha256) {
        throw "SHA-256 mismatch: expected $expectedSha256, got $actualSha256"
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
    Remove-Item -LiteralPath $extractPath -Recurse -Force
}
Expand-Archive -LiteralPath $zipPath -DestinationPath $extractPath
$executable = Join-Path $extractPath $ExpectedExecutable
if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
    throw "The verified Xray archive did not contain $ExpectedExecutable"
}

[pscustomobject]@{
    Release = $release.tag_name
    Asset = $asset.name
    Bytes = [int64]$asset.size
    Sha256 = $actualSha256
    Executable = $executable
    Verification = 'PASS'
}
