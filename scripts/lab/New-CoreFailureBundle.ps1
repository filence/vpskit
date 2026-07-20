[CmdletBinding()]
param(
    [string]$ReleaseVersion = 'v0.1.0-lab.20',
    [string]$FixtureVersion = '1.13.15'
)

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$releaseRoot = Join-Path $projectRoot '.build\release'
$sourceBundle = Join-Path $releaseRoot $ReleaseVersion
$fixtureID = "$ReleaseVersion-core-failure"
$fixtureRoot = Join-Path $releaseRoot $fixtureID
$releaseTool = Join-Path $releaseRoot 'vpskit-release.exe'
$privateKey = Join-Path $projectRoot '.build\keys\release-ed25519-private.key'
$publicKey = Join-Path $projectRoot '.build\keys\release-ed25519-public.key'
$archivePath = Join-Path $releaseRoot "$fixtureID-linux-amd64.tar.gz"

foreach ($required in @($sourceBundle, $releaseTool, $privateKey, $publicKey)) {
    if (-not (Test-Path -LiteralPath $required)) {
        throw "Required release input is missing: $required"
    }
}

if (Test-Path -LiteralPath $fixtureRoot) {
    Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $fixtureRoot -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $sourceBundle 'vpskit') -Destination (Join-Path $fixtureRoot 'vpskit')
Copy-Item -LiteralPath (Join-Path $sourceBundle 'lego') -Destination (Join-Path $fixtureRoot 'lego')
Copy-Item -LiteralPath (Join-Path $sourceBundle 'versions.lock') -Destination (Join-Path $fixtureRoot 'versions.lock')

$fixtureCore = @"
#!/bin/sh
case "`$1" in
  version)
    printf 'sing-box version $FixtureVersion\n'
    exit 0
    ;;
  check)
    exit 0
    ;;
  *)
    exit 86
    ;;
esac
"@
[System.IO.File]::WriteAllText((Join-Path $fixtureRoot 'sing-box'), $fixtureCore, [System.Text.UTF8Encoding]::new($false))

$fixtureCoreHash = (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'sing-box') -Algorithm SHA256).Hash.ToLowerInvariant()
$versionsLockPath = Join-Path $fixtureRoot 'versions.lock'
$versionsLock = Get-Content -LiteralPath $versionsLockPath -Raw | ConvertFrom-Json
$lockedCore = $versionsLock.assets | Where-Object id -eq 'sing-box'
$lockedCore.version = $FixtureVersion
$lockedCore.source_ref = "v$FixtureVersion"
$lockedCore.source_commit = '0000000000000000000000000000000000000001'
$lockedCore.source_url = "https://github.com/SagerNet/sing-box/releases/tag/v$FixtureVersion"
$lockedCore.source_archive.name = "vpskit-controlled-failure-$FixtureVersion"
$lockedCore.source_archive.size = (Get-Item -LiteralPath (Join-Path $fixtureRoot 'sing-box')).Length
$lockedCore.source_archive.sha256 = $fixtureCoreHash
$lockedCore.binary_sha256 = $fixtureCoreHash
$versionsLockText = $versionsLock | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText($versionsLockPath, "$versionsLockText`n", [System.Text.UTF8Encoding]::new($false))

$manifestPath = Join-Path $fixtureRoot 'release-manifest.json'
$signaturePath = Join-Path $fixtureRoot 'release-manifest.sig'
$currentKeyID = (& $releaseTool keyid --public $publicKey).Trim()
& $releaseTool manifest --dir $fixtureRoot --output $manifestPath --release-id $fixtureID --signing-key-id $currentKeyID
if ($LASTEXITCODE -ne 0) {
    throw 'Fixture manifest generation failed.'
}
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$coreAsset = $manifest.assets | Where-Object id -eq 'sing-box'
$coreAsset.version = $FixtureVersion
$coreAsset.channel = 'stable'
$coreAsset.source_url = "https://github.com/SagerNet/sing-box/releases/tag/v$FixtureVersion"
$coreAsset.source_repo = 'SagerNet/sing-box'
$coreAsset.source_ref = "v$FixtureVersion"
$coreAsset.source_commit = '0000000000000000000000000000000000000001'
$coreAsset.state_schema_min = 1
$manifestText = $manifest | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText($manifestPath, "$manifestText`n", [System.Text.UTF8Encoding]::new($false))

& $releaseTool sign --manifest $manifestPath --private $privateKey --output $signaturePath
if ($LASTEXITCODE -ne 0) {
    throw 'Fixture manifest signing failed.'
}
& $releaseTool verify --dir $fixtureRoot --public $publicKey
if ($LASTEXITCODE -ne 0) {
    throw 'Fixture bundle verification failed.'
}

if (Test-Path -LiteralPath $archivePath) {
    Remove-Item -LiteralPath $archivePath -Force
}
tar.exe -czf $archivePath -C $releaseRoot $fixtureID
if ($LASTEXITCODE -ne 0) {
    throw 'Fixture archive creation failed.'
}
$archiveHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Output "FIXTURE=$fixtureRoot"
Write-Output "ARCHIVE=$archivePath"
Write-Output "ARCHIVE_SHA256=$archiveHash"
