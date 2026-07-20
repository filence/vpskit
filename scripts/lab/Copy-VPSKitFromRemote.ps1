[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$RemotePath,

    [Parameter(Mandatory)]
    [string]$LocalPath
)

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$environmentDocument = Join-Path $projectRoot '前期环境须知.md'
$askPassSource = Join-Path $projectRoot '.build\tools\ssh-askpass.cs'
$askPassExecutable = Join-Path $projectRoot '.build\tools\ssh-askpass.exe'
$knownHosts = Join-Path $projectRoot '.build\ssh_known_hosts'
$localParent = Split-Path -Parent $LocalPath

foreach ($requiredPath in @($environmentDocument, $askPassSource)) {
    if (-not (Test-Path -LiteralPath $requiredPath -PathType Leaf)) {
        throw "Required file is missing: $requiredPath"
    }
}
if (-not (Test-Path -LiteralPath $localParent -PathType Container)) {
    throw "Local destination directory is missing: $localParent"
}

$documentText = [System.IO.File]::ReadAllText($environmentDocument, [System.Text.Encoding]::UTF8)
function Get-EnvironmentField {
    param([Parameter(Mandatory)][string]$Label)
    $match = [regex]::Match($documentText, "(?m)^$([regex]::Escape($Label))[：:]\s*(.+?)\s*$")
    if (-not $match.Success) {
        throw "Required environment field is missing: $Label"
    }
    return $match.Groups[1].Value.Trim()
}

$targetHost = Get-EnvironmentField -Label 'IPV4地址'
$targetPort = Get-EnvironmentField -Label '端口'
$targetUser = Get-EnvironmentField -Label '用户名'
$targetPassword = Get-EnvironmentField -Label 'Root密码'

$sourceTimestamp = (Get-Item -LiteralPath $askPassSource).LastWriteTimeUtc
if (-not (Test-Path -LiteralPath $askPassExecutable -PathType Leaf) -or (Get-Item -LiteralPath $askPassExecutable).LastWriteTimeUtc -lt $sourceTimestamp) {
    if (Test-Path -LiteralPath $askPassExecutable) {
        Remove-Item -LiteralPath $askPassExecutable -Force
    }
    $csharpCompiler = 'C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe'
    if (-not (Test-Path -LiteralPath $csharpCompiler -PathType Leaf)) {
        throw "C# compiler is missing: $csharpCompiler"
    }
    & $csharpCompiler /nologo /target:exe "/out:$askPassExecutable" $askPassSource
    if ($LASTEXITCODE -ne 0) {
        throw "SSH askpass build failed with exit code $LASTEXITCODE"
    }
}

$scpArguments = @(
    '-o', "UserKnownHostsFile=$knownHosts"
    '-o', 'StrictHostKeyChecking=accept-new'
    '-o', 'PreferredAuthentications=password'
    '-o', 'PubkeyAuthentication=no'
    '-o', 'ConnectTimeout=15'
    '-P', $targetPort
    "${targetUser}@${targetHost}:$RemotePath"
    $LocalPath
)

$env:VPSKIT_SSH_PASSWORD = $targetPassword
$env:SSH_ASKPASS = $askPassExecutable
$env:SSH_ASKPASS_REQUIRE = 'force'
$env:DISPLAY = 'vpskit-scp'
try {
    & scp.exe @scpArguments
    if ($LASTEXITCODE -ne 0) {
        throw "SCP failed with exit code $LASTEXITCODE"
    }
}
finally {
    Remove-Item Env:VPSKIT_SSH_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:SSH_ASKPASS -ErrorAction SilentlyContinue
    Remove-Item Env:SSH_ASKPASS_REQUIRE -ErrorAction SilentlyContinue
    Remove-Item Env:DISPLAY -ErrorAction SilentlyContinue
    $targetPassword = $null
}

if (-not (Test-Path -LiteralPath $LocalPath -PathType Leaf)) {
    throw "Downloaded file is missing: $LocalPath"
}
