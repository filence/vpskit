[CmdletBinding()]
param(
    [string]$Version = 'v0.1.0-lab.1'
)

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$goRoot = Join-Path $projectRoot '.build\tools\go1.26.5\go\bin'
$go = Join-Path $goRoot 'go.exe'
$gofmt = Join-Path $goRoot 'gofmt.exe'
$buildRoot = Join-Path $projectRoot '.build\release'
$bundleRoot = Join-Path $buildRoot $Version
$keyRoot = Join-Path $projectRoot '.build\keys'
$privateKey = Join-Path $keyRoot 'release-ed25519-private.key'
$publicKey = Join-Path $keyRoot 'release-ed25519-public.key'
$nextPrivateKey = Join-Path $keyRoot 'release-ed25519-next-private.key'
$nextPublicKey = Join-Path $keyRoot 'release-ed25519-next-public.key'
$releaseTool = Join-Path $buildRoot 'vpskit-release.exe'
$windowsVPSKit = Join-Path $buildRoot 'vpskit.exe'
$archivePath = Join-Path $buildRoot "$Version-linux-amd64.tar.gz"
$syft = Join-Path $projectRoot '.build\tools\syft-1.44.0\syft.exe'

if (-not (Test-Path -LiteralPath $go -PathType Leaf)) {
    throw "Go toolchain is missing: $go"
}
if (-not (Test-Path -LiteralPath $syft -PathType Leaf)) {
    throw "Pinned Syft 1.44.0 is missing: $syft"
}

New-Item -ItemType Directory -Path $buildRoot,$keyRoot -Force | Out-Null
$resolvedBuildRoot = [System.IO.Path]::GetFullPath($buildRoot).TrimEnd([System.IO.Path]::DirectorySeparatorChar)
$resolvedBundleRoot = [System.IO.Path]::GetFullPath($bundleRoot)
if (-not $resolvedBundleRoot.StartsWith($resolvedBuildRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Bundle directory escaped the release build root: $resolvedBundleRoot"
}
if (Test-Path -LiteralPath $resolvedBundleRoot) {
    Remove-Item -LiteralPath $resolvedBundleRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $resolvedBundleRoot -Force | Out-Null

$goFiles = @(
    Get-ChildItem -LiteralPath $projectRoot -File -Recurse -Filter '*.go'
    | Where-Object {
        $_.FullName -notlike "$projectRoot\source-references\*" -and
        $_.FullName -notlike "$projectRoot\.build\*"
    }
    | ForEach-Object { $_.FullName }
)
& $gofmt -w @goFiles
if ($LASTEXITCODE -ne 0) {
    throw "gofmt failed with exit code $LASTEXITCODE"
}

& $go test ./...
if ($LASTEXITCODE -ne 0) {
    throw "go test failed with exit code $LASTEXITCODE"
}

$releaseBuildArgs = @('build', '-trimpath', '-o', $releaseTool, './cmd/vpskit-release')
& $go @releaseBuildArgs
if ($LASTEXITCODE -ne 0) {
    throw "release tool build failed with exit code $LASTEXITCODE"
}

if (-not (Test-Path -LiteralPath $privateKey -PathType Leaf)) {
    $keyArgs = @('keygen', '--private', $privateKey, '--public', $publicKey)
    & $releaseTool @keyArgs
    if ($LASTEXITCODE -ne 0) {
        throw "release key generation failed with exit code $LASTEXITCODE"
    }
}
if (-not (Test-Path -LiteralPath $nextPrivateKey -PathType Leaf)) {
    $nextKeyArgs = @('keygen', '--private', $nextPrivateKey, '--public', $nextPublicKey)
    & $releaseTool @nextKeyArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Next release key generation failed with exit code $LASTEXITCODE"
    }
}

$publicKeyValue = [System.IO.File]::ReadAllText($publicKey, [System.Text.Encoding]::UTF8).Trim()
$nextPublicKeyValue = [System.IO.File]::ReadAllText($nextPublicKey, [System.Text.Encoding]::UTF8).Trim()
if ([string]::IsNullOrWhiteSpace($publicKeyValue)) {
    throw 'Release public key is empty.'
}
$currentKeyID = (& $releaseTool keyid --public $publicKey).Trim()
$nextKeyID = (& $releaseTool keyid --public $nextPublicKey).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($currentKeyID) -or [string]::IsNullOrWhiteSpace($nextKeyID)) {
    throw 'Release key ID generation failed.'
}
$trustPolicy = [ordered]@{
    schema_version = 1
    keys = @(
        [ordered]@{ id = $currentKeyID; public_key = $publicKeyValue },
        [ordered]@{ id = $nextKeyID; public_key = $nextPublicKeyValue }
    )
    revoked_key_ids = @()
}
$trustPolicyJson = $trustPolicy | ConvertTo-Json -Depth 5 -Compress
$trustPolicyBase64 = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($trustPolicyJson)).TrimEnd('=')
$linkerFlags = "-s -w -X main.version=$Version -X main.releasePublicKeyBase64=$trustPolicyBase64"

$savedGOOS = $env:GOOS
$savedGOARCH = $env:GOARCH
$savedCGO = $env:CGO_ENABLED
try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    $linuxBuildArgs = @('build', '-trimpath', '-ldflags', $linkerFlags, '-o', (Join-Path $bundleRoot 'vpskit'), './cmd/vpskit')
    & $go @linuxBuildArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Linux VPSKit build failed with exit code $LASTEXITCODE"
    }

    $env:GOOS = 'windows'
    $windowsBuildArgs = @('build', '-trimpath', '-ldflags', $linkerFlags, '-o', $windowsVPSKit, './cmd/vpskit')
    & $go @windowsBuildArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Windows VPSKit build failed with exit code $LASTEXITCODE"
    }
}
finally {
    $env:GOOS = $savedGOOS
    $env:GOARCH = $savedGOARCH
    $env:CGO_ENABLED = $savedCGO
}

Copy-Item -LiteralPath (Join-Path $projectRoot '.build\assets\sing-box-linux-amd64\sing-box') -Destination (Join-Path $bundleRoot 'sing-box') -Force
Copy-Item -LiteralPath (Join-Path $projectRoot '.build\assets\xray-linux-amd64-v26.3.27\extracted\xray') -Destination (Join-Path $bundleRoot 'xray') -Force
Copy-Item -LiteralPath (Join-Path $projectRoot '.build\assets\lego-linux-amd64\lego') -Destination (Join-Path $bundleRoot 'lego') -Force

$licensesRoot = Join-Path $bundleRoot 'licenses'
New-Item -ItemType Directory -Path $licensesRoot -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $projectRoot 'LICENSE') -Destination (Join-Path $bundleRoot 'LICENSE') -Force
Copy-Item -LiteralPath (Join-Path $projectRoot 'NOTICE.md') -Destination (Join-Path $bundleRoot 'NOTICE.md') -Force
Copy-Item -LiteralPath (Join-Path $projectRoot 'THIRD_PARTY_LICENSES.md') -Destination (Join-Path $bundleRoot 'THIRD_PARTY_LICENSES.md') -Force
Copy-Item -LiteralPath (Join-Path $projectRoot '.build\assets\sing-box-linux-amd64\LICENSE') -Destination (Join-Path $licensesRoot 'sing-box.LICENSE') -Force
Copy-Item -LiteralPath (Join-Path $projectRoot '.build\assets\xray-linux-amd64-v26.3.27\extracted\LICENSE') -Destination (Join-Path $licensesRoot 'xray.LICENSE') -Force
Copy-Item -LiteralPath (Join-Path $projectRoot '.build\assets\lego-linux-amd64\LICENSE') -Destination (Join-Path $licensesRoot 'lego.LICENSE') -Force
Copy-Item -LiteralPath (Join-Path $projectRoot 'licenses\go-qrcode.LICENSE') -Destination (Join-Path $licensesRoot 'go-qrcode.LICENSE') -Force

& $syft "file:$(Join-Path $bundleRoot 'vpskit')" -o "spdx-json=$(Join-Path $bundleRoot 'vpskit.spdx.json')"
if ($LASTEXITCODE -ne 0) {
    throw "SBOM generation failed with exit code $LASTEXITCODE"
}

$singBoxBinaryHash = (Get-FileHash -LiteralPath (Join-Path $bundleRoot 'sing-box') -Algorithm SHA256).Hash.ToLowerInvariant()
$xrayBinaryHash = (Get-FileHash -LiteralPath (Join-Path $bundleRoot 'xray') -Algorithm SHA256).Hash.ToLowerInvariant()
$legoBinaryHash = (Get-FileHash -LiteralPath (Join-Path $bundleRoot 'lego') -Algorithm SHA256).Hash.ToLowerInvariant()
$versionsLock = [ordered]@{
    schema_version = 1
    generated_at = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    template_revision = 2
    assets = @(
        [ordered]@{
            id = 'sing-box'
            version = '1.13.14'
            channel = 'stable'
            source_repo = 'SagerNet/sing-box'
            source_ref = 'v1.13.14'
            source_commit = '25a600db24f7680ad9806ce5427bd0ab8afe1114'
            source_url = 'https://github.com/SagerNet/sing-box/releases/tag/v1.13.14'
            source_archive = [ordered]@{
                name = 'sing-box-1.13.14-linux-amd64.tar.gz'
                size = 23832905
                sha256 = 'f48703461a15476951ac4967cdad339d986f4b8096b4eb3ff0829a500502d697'
            }
            installed_file = 'sing-box'
            binary_sha256 = $singBoxBinaryHash
            verified_at = '2026-07-18'
            state_schema_min = 1
        },
        [ordered]@{
            id = 'xray'
            version = '26.3.27'
            channel = 'stable'
            source_repo = 'XTLS/Xray-core'
            source_ref = 'v26.3.27'
            source_commit = 'd2758a023cd7f4174a5a5fa4ff66e487d4342ba0'
            source_url = 'https://github.com/XTLS/Xray-core/releases/tag/v26.3.27'
            source_archive = [ordered]@{
                name = 'Xray-linux-64.zip'
                size = 21136402
                sha256 = '23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae'
            }
            installed_file = 'xray'
            binary_sha256 = $xrayBinaryHash
            verified_at = '2026-07-19'
            state_schema_min = 4
        },
        [ordered]@{
            id = 'lego'
            version = '5.2.2'
            channel = 'stable'
            source_repo = 'go-acme/lego'
            source_ref = 'v5.2.2'
            source_commit = '3d5a6695e027d625bd34334d516d77f578d43f11'
            source_url = 'https://github.com/go-acme/lego/releases/tag/v5.2.2'
            source_archive = [ordered]@{
                name = 'lego_v5.2.2_linux_amd64.tar.gz'
                size = 21076129
                sha256 = '018de6d3f2da09630caa2fbbe8c6aa459323ad0ac0a053d0e808268914b38a8b'
            }
            installed_file = 'lego'
            binary_sha256 = $legoBinaryHash
            verified_at = '2026-07-18'
            state_schema_min = 1
        }
    )
}
$versionsLockText = $versionsLock | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText((Join-Path $bundleRoot 'versions.lock'), "$versionsLockText`n", [System.Text.UTF8Encoding]::new($false))

$manifestPath = Join-Path $bundleRoot 'release-manifest.json'
$signaturePath = Join-Path $bundleRoot 'release-manifest.sig'
$manifestArgs = @('manifest', '--dir', $bundleRoot, '--output', $manifestPath, '--release-id', $Version, '--signing-key-id', $currentKeyID)
& $releaseTool @manifestArgs
if ($LASTEXITCODE -ne 0) {
    throw "manifest generation failed with exit code $LASTEXITCODE"
}
$signArgs = @('sign', '--manifest', $manifestPath, '--private', $privateKey, '--output', $signaturePath)
& $releaseTool @signArgs
if ($LASTEXITCODE -ne 0) {
    throw "manifest signing failed with exit code $LASTEXITCODE"
}
$verifyArgs = @('verify', '--dir', $bundleRoot, '--public', $publicKey)
& $releaseTool @verifyArgs
if ($LASTEXITCODE -ne 0) {
    throw "release verification failed with exit code $LASTEXITCODE"
}
$bundleVerifyArgs = @('bundle', 'verify', '--dir', $bundleRoot)
& $windowsVPSKit @bundleVerifyArgs
if ($LASTEXITCODE -ne 0) {
    throw "embedded-key bundle verification failed with exit code $LASTEXITCODE"
}

if (Test-Path -LiteralPath $archivePath) {
    Remove-Item -LiteralPath $archivePath -Force
}
$packArgs = @('pack', '--dir', $bundleRoot, '--output', $archivePath)
& $releaseTool @packArgs
if ($LASTEXITCODE -ne 0) {
    throw "release archive creation failed with exit code $LASTEXITCODE"
}
$archiveHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
[System.IO.File]::WriteAllText("$archivePath.sha256", "$archiveHash  $([System.IO.Path]::GetFileName($archivePath))`n", [System.Text.UTF8Encoding]::new($false))

Write-Output "RELEASE=$Version"
Write-Output "BUNDLE=$bundleRoot"
Write-Output "ARCHIVE=$archivePath"
Write-Output "ARCHIVE_SHA256=$archiveHash"
