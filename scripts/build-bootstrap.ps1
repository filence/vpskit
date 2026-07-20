[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidatePattern('^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$')]
    [string]$Version,

    [Parameter(Mandatory)]
    [ValidatePattern('^[0-9A-Za-z_.-]+/[0-9A-Za-z_.-]+$')]
    [string]$Repository,

    [Parameter(Mandatory)]
    [string]$ArchivePath,

    [string]$OutputPath
)

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$templatePath = Join-Path $PSScriptRoot 'install.sh.in'
if (-not (Test-Path -LiteralPath $templatePath -PathType Leaf)) {
    throw "Bootstrap template is missing: $templatePath"
}

$resolvedArchive = [System.IO.Path]::GetFullPath($ArchivePath)
if (-not (Test-Path -LiteralPath $resolvedArchive -PathType Leaf)) {
    throw "Release archive is missing: $resolvedArchive"
}
$archiveInfo = Get-Item -LiteralPath $resolvedArchive -Force
if ($archiveInfo.Attributes -band [System.IO.FileAttributes]::ReparsePoint) {
    throw 'Release archive must not be a symbolic link or reparse point.'
}

if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    $OutputPath = Join-Path (Split-Path -Parent $resolvedArchive) 'install.sh'
}
$resolvedOutput = [System.IO.Path]::GetFullPath($OutputPath)
$outputParent = Split-Path -Parent $resolvedOutput
if (-not (Test-Path -LiteralPath $outputParent -PathType Container)) {
    throw "Bootstrap output directory is missing: $outputParent"
}
if ($resolvedOutput -eq $resolvedArchive -or $resolvedOutput -eq [System.IO.Path]::GetFullPath($templatePath)) {
    throw 'Bootstrap output path would overwrite an input file.'
}

$archiveHash = (Get-FileHash -LiteralPath $resolvedArchive -Algorithm SHA256).Hash.ToLowerInvariant()
$template = [System.IO.File]::ReadAllText($templatePath, [System.Text.Encoding]::UTF8)
$bootstrap = $template.Replace('@VPSKIT_VERSION@', $Version).Replace('@VPSKIT_REPOSITORY@', $Repository).Replace('@VPSKIT_ARCHIVE_SHA256@', $archiveHash)
if ($bootstrap.Contains('@VPSKIT_')) {
    throw 'Bootstrap template still contains unresolved placeholders.'
}
$bootstrap = $bootstrap.Replace("`r`n", "`n").Replace("`r", "`n")
[System.IO.File]::WriteAllText($resolvedOutput, $bootstrap, [System.Text.UTF8Encoding]::new($false))

Write-Output "BOOTSTRAP=$resolvedOutput"
Write-Output "VERSION=$Version"
Write-Output "REPOSITORY=$Repository"
Write-Output "ARCHIVE_SHA256=$archiveHash"
