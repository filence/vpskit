[CmdletBinding()]
param(
    [string]$ReleaseVersion = 'v0.1.0-lab.22',
    [string]$FixtureVersion = '1.13.16'
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$releaseRoot = Join-Path $projectRoot '.build\release'
$sourceBundle = Join-Path $releaseRoot $ReleaseVersion
$fixtureID = "$ReleaseVersion-core-crash"
$fixtureRoot = Join-Path $releaseRoot $fixtureID
$releaseTool = Join-Path $releaseRoot 'vpskit-release.exe'
$privateKey = Join-Path $projectRoot '.build\keys\release-ed25519-private.key'
$publicKey = Join-Path $projectRoot '.build\keys\release-ed25519-public.key'
$archivePath = Join-Path $releaseRoot "$fixtureID-linux-amd64.tar.gz"

foreach ($required in @($sourceBundle, $releaseTool, $privateKey, $publicKey)) {
    if (-not (Test-Path -LiteralPath $required)) {
        throw "Required crash fixture input is missing: $required"
    }
}
if (Test-Path -LiteralPath $fixtureRoot) {
    Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $fixtureRoot -Force | Out-Null
foreach ($name in @('vpskit', 'lego', 'versions.lock')) {
    Copy-Item -LiteralPath (Join-Path $sourceBundle $name) -Destination (Join-Path $fixtureRoot $name)
}

$fixtureCore = @"
#!/bin/sh
case "`$1" in
  version)
    printf 'sing-box version $FixtureVersion\n'
    exit 0
    ;;
  check)
    case "`$0" in
      /usr/local/lib/vpskit/bin/sing-box) sleep 120 ;;
    esac
    exit 0
    ;;
  *)
    exit 86
    ;;
esac
"@
$corePath = Join-Path $fixtureRoot 'sing-box'
[System.IO.File]::WriteAllText($corePath, $fixtureCore, [System.Text.UTF8Encoding]::new($false))
$coreHash = (Get-FileHash -LiteralPath $corePath -Algorithm SHA256).Hash.ToLowerInvariant()

$versionsLockPath = Join-Path $fixtureRoot 'versions.lock'
$versionsLock = Get-Content -LiteralPath $versionsLockPath -Raw | ConvertFrom-Json
$lockedCore = $versionsLock.assets | Where-Object id -eq 'sing-box'
$lockedCore.version = $FixtureVersion
$lockedCore.source_ref = "v$FixtureVersion"
$lockedCore.source_commit = '0000000000000000000000000000000000000002'
$lockedCore.source_url = "https://github.com/SagerNet/sing-box/releases/tag/v$FixtureVersion"
$lockedCore.source_archive.name = "vpskit-controlled-crash-$FixtureVersion"
$lockedCore.source_archive.size = (Get-Item -LiteralPath $corePath).Length
$lockedCore.source_archive.sha256 = $coreHash
$lockedCore.binary_sha256 = $coreHash
$versionsLockText = $versionsLock | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText($versionsLockPath, "$versionsLockText`n", [System.Text.UTF8Encoding]::new($false))

$manifestPath = Join-Path $fixtureRoot 'release-manifest.json'
$signaturePath = Join-Path $fixtureRoot 'release-manifest.sig'
$currentKeyID = (& $releaseTool keyid --public $publicKey).Trim()
& $releaseTool manifest --dir $fixtureRoot --output $manifestPath --release-id $fixtureID --signing-key-id $currentKeyID
if ($LASTEXITCODE -ne 0) {
    throw 'Crash fixture manifest generation failed.'
}
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$coreAsset = $manifest.assets | Where-Object id -eq 'sing-box'
$coreAsset.version = $FixtureVersion
$coreAsset.channel = 'stable'
$coreAsset.source_url = "https://github.com/SagerNet/sing-box/releases/tag/v$FixtureVersion"
$coreAsset.source_repo = 'SagerNet/sing-box'
$coreAsset.source_ref = "v$FixtureVersion"
$coreAsset.source_commit = '0000000000000000000000000000000000000002'
$coreAsset.state_schema_min = 1
$manifestText = $manifest | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText($manifestPath, "$manifestText`n", [System.Text.UTF8Encoding]::new($false))
& $releaseTool sign --manifest $manifestPath --private $privateKey --output $signaturePath
if ($LASTEXITCODE -ne 0) {
    throw 'Crash fixture signing failed.'
}
& $releaseTool verify --dir $fixtureRoot --public $publicKey
if ($LASTEXITCODE -ne 0) {
    throw 'Crash fixture verification failed.'
}

if (Test-Path -LiteralPath $archivePath) {
    Remove-Item -LiteralPath $archivePath -Force
}
tar.exe -czf $archivePath -C $releaseRoot $fixtureID
if ($LASTEXITCODE -ne 0) {
    throw 'Crash fixture archive creation failed.'
}
$archiveHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Output "FIXTURE=$fixtureRoot"
Write-Output "ARCHIVE=$archivePath"
Write-Output "ARCHIVE_SHA256=$archiveHash"
