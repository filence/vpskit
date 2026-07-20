[CmdletBinding()]
param([string]$ReleaseVersion = 'v0.1.0-lab.21')

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$releaseRoot = Join-Path $projectRoot '.build\release'
$sourceBundle = Join-Path $releaseRoot $ReleaseVersion
$fixtureID = "$ReleaseVersion-next-key"
$fixtureRoot = Join-Path $releaseRoot $fixtureID
$releaseTool = Join-Path $releaseRoot 'vpskit-release.exe'
$nextPrivateKey = Join-Path $projectRoot '.build\keys\release-ed25519-next-private.key'
$nextPublicKey = Join-Path $projectRoot '.build\keys\release-ed25519-next-public.key'
$archivePath = Join-Path $releaseRoot "$fixtureID-linux-amd64.tar.gz"

foreach ($required in @($sourceBundle, $releaseTool, $nextPrivateKey, $nextPublicKey)) {
    if (-not (Test-Path -LiteralPath $required)) {
        throw "Required rotation input is missing: $required"
    }
}
if (Test-Path -LiteralPath $fixtureRoot) {
    Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
}
Copy-Item -LiteralPath $sourceBundle -Destination $fixtureRoot -Recurse

$manifestPath = Join-Path $fixtureRoot 'release-manifest.json'
$signaturePath = Join-Path $fixtureRoot 'release-manifest.sig'
$nextKeyID = (& $releaseTool keyid --public $nextPublicKey).Trim()
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$manifest.release_id = $fixtureID
$manifest.signing_key_id = $nextKeyID
$manifestText = $manifest | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText($manifestPath, "$manifestText`n", [System.Text.UTF8Encoding]::new($false))
& $releaseTool sign --manifest $manifestPath --private $nextPrivateKey --output $signaturePath
if ($LASTEXITCODE -ne 0) {
    throw 'Next-key signing failed.'
}
& $releaseTool verify --dir $fixtureRoot --public $nextPublicKey
if ($LASTEXITCODE -ne 0) {
    throw 'Next-key direct verification failed.'
}
& (Join-Path $sourceBundle 'vpskit') bundle verify --dir $fixtureRoot
if ($LASTEXITCODE -ne 0) {
    throw 'Embedded rotation policy rejected the next key.'
}

if (Test-Path -LiteralPath $archivePath) {
    Remove-Item -LiteralPath $archivePath -Force
}
tar.exe -czf $archivePath -C $releaseRoot $fixtureID
if ($LASTEXITCODE -ne 0) {
    throw 'Rotation fixture archive creation failed.'
}
$archiveHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Output "FIXTURE=$fixtureRoot"
Write-Output "ARCHIVE=$archivePath"
Write-Output "ARCHIVE_SHA256=$archiveHash"
