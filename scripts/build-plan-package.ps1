[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$documentRoot = Join-Path $projectRoot 'vpskit-plan-20260717'
$version = 'v1.1'
$dateStamp = '20260717'
$combinedName = "VPSKit-完整方案-$version-$dateStamp.md"
$archiveName = "VPSKit-完整方案-$version-$dateStamp.zip"

$sourceFiles = @(
    '01-完整方案与产品边界.md'
    '02-架构与扩展接口设计.md'
    '03-低配资源预算与运行策略.md'
    '04-开发路线与验收标准.md'
    '05-参考项目与社区调研.md'
    '06-源码审核与修订意见.md'
)

foreach ($name in $sourceFiles) {
    $path = Join-Path $documentRoot $name
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Missing source document: $path"
    }
}

$header = @'
# VPSKit 自研一键搭建脚本完整方案

版本：v1.1 源码审核修订版

日期：2026-07-17

低配基准：1 vCPU、1 GB RAM、约 10 GB 磁盘

本文件由 01–06 拆分文档按编号顺序生成。若内容不一致，以同版本拆分文档和 README 为准。
'@

$sections = [System.Collections.Generic.List[string]]::new()
$sections.Add($header.TrimEnd())

foreach ($name in $sourceFiles) {
    $path = Join-Path $documentRoot $name
    $content = [System.IO.File]::ReadAllText($path, [System.Text.Encoding]::UTF8).Trim()
    $sections.Add($content)
}

$combinedPath = Join-Path $documentRoot $combinedName
$combinedText = ($sections -join "`r`n`r`n---`r`n`r`n") + "`r`n"
[System.IO.File]::WriteAllText($combinedPath, $combinedText, [System.Text.UTF8Encoding]::new($false))

$buildRoot = Join-Path $projectRoot '.build'
$stageRoot = Join-Path $buildRoot "VPSKit-完整方案-$version-$dateStamp"
$resolvedProjectRoot = [System.IO.Path]::GetFullPath($projectRoot)
$resolvedBuildRoot = [System.IO.Path]::GetFullPath($buildRoot)
$resolvedStageRoot = [System.IO.Path]::GetFullPath($stageRoot)

if (-not $resolvedBuildRoot.StartsWith($resolvedProjectRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Build root escaped project root: $resolvedBuildRoot"
}
if (-not $resolvedStageRoot.StartsWith($resolvedBuildRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Stage root escaped build root: $resolvedStageRoot"
}

if (Test-Path -LiteralPath $stageRoot) {
    Remove-Item -LiteralPath $stageRoot -Recurse -Force
}

New-Item -ItemType Directory -Path $stageRoot | Out-Null
New-Item -ItemType Directory -Path (Join-Path $stageRoot 'source-references') | Out-Null

$packageDocuments = @('README.md') + $sourceFiles + @($combinedName)
foreach ($name in $packageDocuments) {
    Copy-Item -LiteralPath (Join-Path $documentRoot $name) -Destination (Join-Path $stageRoot $name)
}

$sourceManifest = Join-Path $projectRoot 'source-references\README.md'
Copy-Item -LiteralPath $sourceManifest -Destination (Join-Path $stageRoot 'source-references\README.md')

$archivePath = Join-Path $projectRoot $archiveName
Compress-Archive -Path (Join-Path $stageRoot '*') -DestinationPath $archivePath -CompressionLevel Optimal -Force

Remove-Item -LiteralPath $stageRoot -Recurse -Force

Write-Output "COMBINED=$combinedPath"
Write-Output "ARCHIVE=$archivePath"
