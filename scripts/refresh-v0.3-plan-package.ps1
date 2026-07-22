[CmdletBinding()]
param(
[string]$Revision = 'R28',
    [string]$DateStamp = '20260722'
)

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$documentRoot = Join-Path $projectRoot 'VPSKit-功能演进完整方案-v0.3-20260720'
$combinedName = 'VPSKit-完整方案-v0.3-20260720.md'
$manifestName = 'FILE-MANIFEST.md'
$archiveName = "VPSKit-功能演进完整方案-v0.3-$Revision-$DateStamp.zip"
$archivePath = Join-Path $projectRoot $archiveName
$sourceNames = @(0..12 | ForEach-Object { '{0:D2}-*.md' -f $_ })
$sourceFiles = @()
foreach ($pattern in $sourceNames) {
    $match = @(Get-ChildItem -LiteralPath $documentRoot -File -Filter $pattern)
    if ($match.Count -ne 1) { throw "Expected one plan source matching $pattern, found $($match.Count)." }
    $sourceFiles += $match[0].Name
}

$code = [char]96
$baseline = "- 当前产品基线：VPSKit ${code}v0.2.21-lab.1${code}；方案 A r0008、方案 B r0014、Salamander r0015、schema 9 白名单/自定义规则 r0012、${code}doctor --fix${code}、扩展 ${code}system inspect${code}、Fail2ban SSH 白名单、规则刷新失败保护、Hysteria2 能力矩阵、性能基线、受管 UDP buffer、Adapter Registry 渐进接入与服务等待收口、只读带宽建议、端口跳跃 redirect/客户端导出/重启恢复与系统更新候选检查已完成对应实机验收"
$reference = "本文件由同目录 00–12 分卷按顺序机械合并。出现歧义时，以分卷、${code}${manifestName}${code} 和当前源码为准。"
$header = @"
# VPSKit 功能演进完整方案 v0.3-$Revision

> 标题：VPSKit 功能演进完整方案
>
> 生成时间：$((Get-Date).ToString('yyyy-MM-dd HH:mm'))
>
> 生成者：Codex
>
> 版本：v0.3-$Revision
>
> 用途：用户筛选后的 VPSKit 后续功能实施依据

- 原编制日期：2026-07-20
- 精简修订日期：2026-07-21
$baseline
- 证据原则：仅保留功能进入实施路线；暂停功能不安排版本号

$reference
"@.Trim()
$sections = @($header)
foreach ($name in $sourceFiles) {
    $sections += [IO.File]::ReadAllText((Join-Path $documentRoot $name), [Text.Encoding]::UTF8).Trim()
}
$combinedPath = Join-Path $documentRoot $combinedName
[IO.File]::WriteAllText($combinedPath, (($sections -join "`r`n`r`n---`r`n`r`n") + "`r`n"), [Text.UTF8Encoding]::new($false))

$manifestFiles = @($sourceFiles + 'README.md' + $combinedName | Sort-Object)
$rows = foreach ($name in $manifestFiles) {
    $path = Join-Path $documentRoot $name
    $item = Get-Item -LiteralPath $path
    $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
    "| ``$name`` | $($item.Length) | ``$hash`` |"
}
$manifest = @(
    '# VPSKit 方案包文件清单',
    '',
    "- 方案版本：v0.3-$Revision",
    "- 清单生成日期：$((Get-Date).ToString('yyyy-MM-dd'))",
    '- 工作区基线：v0.2.21-lab.1，方案 A r0008、方案 B r0014、Salamander r0015、schema 9 r0012、Fail2ban SSH 白名单、规则刷新失败保护、Hysteria2 能力矩阵、性能基线、受管 UDP buffer、Adapter Registry 渐进接入、服务等待收口、只读带宽建议与端口跳跃 redirect/客户端导出/重启恢复实机验收；schema 10 通用实例兼容层已完成当前 VPS 部署验收',
    '- 说明：为避免自引用，清单不记录自身哈希；ZIP 仍包含本清单。',
    '',
    '| 文件 | 字节数 | SHA-256 |',
    '| --- | ---: | --- |'
) + $rows + @(
    '',
    '## 包内白名单',
    '',
    '本包仅包含本方案目录内的 Markdown 文档和本清单；不包含 VPS 密码、Cloudflare Token、私钥、客户端订阅链接或任何其他凭据。',
    ''
)
[IO.File]::WriteAllText((Join-Path $documentRoot $manifestName), ($manifest -join "`r`n"), [Text.UTF8Encoding]::new($false))

if (Test-Path -LiteralPath $archivePath) { throw "Refusing to overwrite existing plan archive: $archivePath" }
$stageRoot = Join-Path $projectRoot ('.build\vpskit-plan-{0}-package' -f $Revision)
if (Test-Path -LiteralPath $stageRoot) { Remove-Item -LiteralPath $stageRoot -Recurse -Force }
New-Item -ItemType Directory -Path $stageRoot | Out-Null
try {
    foreach ($name in @($manifestFiles + $manifestName)) {
        Copy-Item -LiteralPath (Join-Path $documentRoot $name) -Destination (Join-Path $stageRoot $name)
    }
    Compress-Archive -LiteralPath (Get-ChildItem -LiteralPath $stageRoot -File | Select-Object -ExpandProperty FullName) -DestinationPath $archivePath -CompressionLevel Optimal
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [IO.Compression.ZipFile]::OpenRead($archivePath)
    try {
        $expected = @($manifestFiles + $manifestName | Sort-Object)
        $actual = @($zip.Entries | ForEach-Object { $_.FullName } | Sort-Object)
        if (($expected -join "`n") -ne ($actual -join "`n")) { throw 'Plan ZIP entry whitelist mismatch.' }
        foreach ($entry in $zip.Entries) {
            $stream = $entry.Open()
            try { $zipHash = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($stream)).ToLowerInvariant() }
            finally { $stream.Dispose() }
            $localHash = (Get-FileHash -LiteralPath (Join-Path $documentRoot $entry.FullName) -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($zipHash -ne $localHash) { throw "Plan ZIP content hash mismatch: $($entry.FullName)" }
        }
    }
    finally { $zip.Dispose() }
}
finally {
    if (Test-Path -LiteralPath $stageRoot) { Remove-Item -LiteralPath $stageRoot -Recurse -Force }
}

Write-Output "COMBINED=$combinedPath"
Write-Output "ARCHIVE=$archivePath"
Write-Output 'VERIFY=PASS'
