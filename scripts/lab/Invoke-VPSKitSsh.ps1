[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$RemoteScriptPath
)

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$environmentDocument = @(
    (Join-Path $projectRoot '.前期环境须知.md'),
    (Join-Path $projectRoot '前期环境须知.md')
) | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
$askPassSource = Join-Path $projectRoot '.build\tools\ssh-askpass.cs'
$askPassExecutable = Join-Path $projectRoot '.build\tools\ssh-askpass.exe'
$knownHosts = Join-Path $projectRoot '.build\ssh_known_hosts'

foreach ($requiredPath in @($environmentDocument, $askPassSource, $RemoteScriptPath)) {
    if (-not (Test-Path -LiteralPath $requiredPath -PathType Leaf)) {
        throw "Required file is missing: $requiredPath"
    }
}

$documentText = [System.IO.File]::ReadAllText($environmentDocument, [System.Text.Encoding]::UTF8)

function Get-EnvironmentField {
    param(
        [Parameter(Mandatory)]
        [string]$Label
    )

    $escapedLabel = [regex]::Escape($Label)
    $match = [regex]::Match($documentText, "(?m)^${escapedLabel}[：:]\s*(.+?)\s*$")
    if (-not $match.Success) {
        throw "Required environment field is missing: $Label"
    }

    return $match.Groups[1].Value.Trim()
}

$targetHost = Get-EnvironmentField -Label 'IPV4地址'
$targetPort = Get-EnvironmentField -Label '端口'
$targetUser = Get-EnvironmentField -Label '用户名'
$targetPassword = Get-EnvironmentField -Label 'Root密码'
$labDomainField = Get-EnvironmentField -Label '1、灰云域名'
$labDomain = ($labDomainField -split '[，,]', 2)[0].Trim()

if ($targetHost -notmatch '^\d{1,3}(?:\.\d{1,3}){3}$') {
    throw 'The SSH host is not an IPv4 address.'
}
if ($targetPort -notmatch '^\d{1,5}$') {
    throw 'The SSH port is invalid.'
}
if ($labDomain -notmatch '^[A-Za-z0-9](?:[A-Za-z0-9.-]{1,251}[A-Za-z0-9])$' -or $labDomain -notmatch '\.') {
    throw 'The lab domain is invalid.'
}
$labLabels = $labDomain.Split('.')
if ($labLabels.Count -lt 3) {
    throw 'The lab domain must include a host label and zone.'
}
$labZone = $labLabels[1..($labLabels.Count - 1)] -join '.'

$sourceTimestamp = (Get-Item -LiteralPath $askPassSource).LastWriteTimeUtc
$shouldBuild = -not (Test-Path -LiteralPath $askPassExecutable -PathType Leaf)
if (-not $shouldBuild) {
    $shouldBuild = (Get-Item -LiteralPath $askPassExecutable).LastWriteTimeUtc -lt $sourceTimestamp
}

if ($shouldBuild) {
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

$sshArguments = @(
    '-o', "UserKnownHostsFile=$knownHosts"
    '-o', 'StrictHostKeyChecking=accept-new'
    '-o', 'PreferredAuthentications=password'
    '-o', 'PubkeyAuthentication=no'
    '-o', 'ConnectTimeout=15'
    '-p', $targetPort
    "$targetUser@$targetHost"
    'bash -s'
)

$env:VPSKIT_SSH_PASSWORD = $targetPassword
$env:SSH_ASKPASS = $askPassExecutable
$env:SSH_ASKPASS_REQUIRE = 'force'
$env:DISPLAY = 'vpskit-ssh'

try {
    $remoteScript = [System.IO.File]::ReadAllText($RemoteScriptPath, [System.Text.Encoding]::UTF8).Replace("`r`n", "`n").Replace("`r", "`n")
    $labEnvironment = @(
        "export VPSKIT_LAB_DOMAIN='$labDomain'"
        "export VPSKIT_LAB_ZONE='$labZone'"
        "export VPSKIT_LAB_IPV4='$targetHost'"
    ) -join "`n"
    $remoteScript = $labEnvironment + "`n" + $remoteScript.TrimEnd("`n") + "`nexit 0`n"
    $remoteScript | & ssh.exe @sshArguments
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
        throw "SSH command failed with exit code $exitCode"
    }
}
finally {
    Remove-Item Env:VPSKIT_SSH_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:SSH_ASKPASS -ErrorAction SilentlyContinue
    Remove-Item Env:SSH_ASKPASS_REQUIRE -ErrorAction SilentlyContinue
    Remove-Item Env:DISPLAY -ErrorAction SilentlyContinue
    $targetPassword = $null
}
