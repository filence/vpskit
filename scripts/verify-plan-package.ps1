[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$documentRoot = Join-Path $projectRoot 'vpskit-plan-20260717'
$version = 'v1.1'
$dateStamp = '20260717'
$combinedName = "VPSKit-完整方案-$version-$dateStamp.md"
$archiveName = "VPSKit-完整方案-$version-$dateStamp.zip"
$archivePath = Join-Path $projectRoot $archiveName

$documentNames = @(
    'README.md'
    '01-完整方案与产品边界.md'
    '02-架构与扩展接口设计.md'
    '03-低配资源预算与运行策略.md'
    '04-开发路线与验收标准.md'
    '05-参考项目与社区调研.md'
    '06-源码审核与修订意见.md'
    $combinedName
)

foreach ($name in $documentNames) {
    $path = Join-Path $documentRoot $name
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Missing document: $path"
    }
}

$readmePath = Join-Path $documentRoot 'README.md'
$readme = [System.IO.File]::ReadAllText($readmePath, [System.Text.Encoding]::UTF8)
foreach ($name in $documentNames[1..6]) {
    if (-not $readme.Contains($name)) {
        throw "README does not reference: $name"
    }
}

$combinedPath = Join-Path $documentRoot $combinedName
$combined = [System.IO.File]::ReadAllText($combinedPath, [System.Text.Encoding]::UTF8)
foreach ($name in $documentNames[1..6]) {
    $sourcePath = Join-Path $documentRoot $name
    $source = [System.IO.File]::ReadAllText($sourcePath, [System.Text.Encoding]::UTF8).Trim()
    if (-not $combined.Contains($source)) {
        throw "Combined document is not synchronized with: $name"
    }
}

if (-not (Test-Path -LiteralPath $archivePath -PathType Leaf)) {
    throw "Missing archive: $archivePath"
}

Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [System.IO.Compression.ZipFile]::OpenRead($archivePath)
try {
    $expectedEntries = @($documentNames + 'source-references/README.md') | Sort-Object
    $actualEntries = @($zip.Entries | ForEach-Object { $_.FullName.Replace('\', '/') } | Sort-Object)

    if (($expectedEntries -join "`n") -ne ($actualEntries -join "`n")) {
        throw "ZIP entry list mismatch.`nEXPECTED:`n$($expectedEntries -join "`n")`nACTUAL:`n$($actualEntries -join "`n")"
    }

    if ($actualEntries | Where-Object { $_ -match '(^|/)\.git(/|$)' }) {
        throw 'ZIP unexpectedly contains a Git repository.'
    }

    $entryToLocalPath = @{}
    foreach ($name in $documentNames) {
        $entryToLocalPath[$name] = Join-Path $documentRoot $name
    }
    $entryToLocalPath['source-references/README.md'] = Join-Path $projectRoot 'source-references\README.md'

    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        foreach ($entry in $zip.Entries) {
            $stream = $entry.Open()
            try {
                $zipHash = [Convert]::ToHexString($sha256.ComputeHash($stream))
            }
            finally {
                $stream.Dispose()
            }

            $localHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $entryToLocalPath[$entry.FullName]).Hash
            if ($zipHash -ne $localHash) {
                throw "ZIP content hash mismatch: $($entry.FullName)"
            }
        }
    }
    finally {
        $sha256.Dispose()
    }
}
finally {
    $zip.Dispose()
}

Write-Output 'PASS=README_REFERENCES'
Write-Output 'PASS=COMBINED_SYNCHRONIZED'
Write-Output 'PASS=ZIP_ENTRY_WHITELIST'
Write-Output 'PASS=ZIP_CONTENT_HASHES'
Write-Output "COMBINED_SHA256=$((Get-FileHash -Algorithm SHA256 -LiteralPath $combinedPath).Hash)"
Write-Output "ARCHIVE_SHA256=$((Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash)"
