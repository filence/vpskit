[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$MihomoPath,

    [Parameter(Mandatory)]
    [string]$DeploymentDirectory,

    [string]$SingBoxPath,

    [string]$XrayPath,

    [int]$LocalForwardPort = 17943,

    [int]$RemotePort = 14443
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$environmentDocument = Join-Path $projectRoot '前期环境须知.md'
$askPassExecutable = Join-Path $projectRoot '.build\tools\ssh-askpass.exe'
$knownHosts = Join-Path $projectRoot '.build\ssh_known_hosts'
$mihomoTest = Join-Path $PSScriptRoot 'Test-MihomoClients.ps1'
$singBoxTest = Join-Path $PSScriptRoot 'Test-WindowsClients.ps1'
$xrayTest = Join-Path $PSScriptRoot 'Test-XrayRealityClient.ps1'
foreach ($path in @($environmentDocument, $askPassExecutable, $knownHosts, $mihomoTest, $MihomoPath)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Required compatibility-test file is missing: $path"
    }
}
if (-not [string]::IsNullOrWhiteSpace($SingBoxPath) -and -not (Test-Path -LiteralPath $SingBoxPath -PathType Leaf)) {
    throw "Required sing-box compatibility-test binary is missing: $SingBoxPath"
}
if (-not [string]::IsNullOrWhiteSpace($XrayPath) -and -not (Test-Path -LiteralPath $XrayPath -PathType Leaf)) {
    throw "Required Xray compatibility-test binary is missing: $XrayPath"
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
$forward = "127.0.0.1:${LocalForwardPort}:127.0.0.1:${RemotePort}"
$stdoutPath = Join-Path $DeploymentDirectory 'xray-compat-ssh.stdout.log'
$stderrPath = Join-Path $DeploymentDirectory 'xray-compat-ssh.stderr.log'
$sshArguments = @(
    '-N'
    '-o', "UserKnownHostsFile=$knownHosts"
    '-o', 'StrictHostKeyChecking=yes'
    '-o', 'PreferredAuthentications=password'
    '-o', 'PubkeyAuthentication=no'
    '-o', 'ConnectTimeout=15'
    '-o', 'ExitOnForwardFailure=yes'
    '-L', $forward
    '-p', $targetPort
    "$targetUser@$targetHost"
)

$env:VPSKIT_SSH_PASSWORD = $targetPassword
$env:SSH_ASKPASS = $askPassExecutable
$env:SSH_ASKPASS_REQUIRE = 'force'
$env:DISPLAY = 'vpskit-ssh-compat'
$process = $null
try {
    $process = Start-Process -FilePath 'ssh.exe' `
        -ArgumentList $sshArguments `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdoutPath `
        -RedirectStandardError $stderrPath `
        -PassThru
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        if ($process.HasExited) {
            throw 'SSH compatibility tunnel exited before becoming ready'
        }
        $client = [System.Net.Sockets.TcpClient]::new()
        try {
            $pending = $client.ConnectAsync('127.0.0.1', $LocalForwardPort)
            if ($pending.Wait(250) -and $client.Connected) {
                break
            }
        }
        catch {
        }
        finally {
            $client.Dispose()
        }
        Start-Sleep -Milliseconds 250
    }
    if ($attempt -ge 60) {
        throw 'SSH compatibility tunnel did not become ready'
    }

    & pwsh -NoProfile -File $mihomoTest `
        -MihomoPath $MihomoPath `
        -DeploymentDirectory $DeploymentDirectory `
        -ProfileMode Reality `
        -EndpointOverride '127.0.0.1' `
        -RealityEndpointPort $LocalForwardPort
    if ($LASTEXITCODE -ne 0) {
        throw "Mihomo-to-Xray compatibility test failed with exit code $LASTEXITCODE"
    }
    Write-Output 'MIHOMO_XRAY_SERVER_COMPATIBILITY=PASS transport=ssh-loopback'
    if (-not [string]::IsNullOrWhiteSpace($SingBoxPath)) {
        & pwsh -NoProfile -File $singBoxTest `
            -SingBoxPath $SingBoxPath `
            -DeploymentDirectory $DeploymentDirectory `
            -EndpointOverride '127.0.0.1' `
            -RealityEndpointPort $LocalForwardPort `
            -ProfileMode Reality
        if ($LASTEXITCODE -ne 0) {
            throw "sing-box-to-Xray compatibility test failed with exit code $LASTEXITCODE"
        }
        Write-Output 'SINGBOX_XRAY_SERVER_COMPATIBILITY=PASS transport=ssh-loopback'
    }
    if (-not [string]::IsNullOrWhiteSpace($XrayPath)) {
        & pwsh -NoProfile -File $xrayTest `
            -XrayPath $XrayPath `
            -DeploymentDirectory $DeploymentDirectory `
            -EndpointOverride '127.0.0.1' `
            -RealityEndpointPort $LocalForwardPort
        if ($LASTEXITCODE -ne 0) {
            throw "Xray-to-Xray compatibility test failed with exit code $LASTEXITCODE"
        }
        Write-Output 'XRAY_XRAY_SERVER_COMPATIBILITY=PASS transport=ssh-loopback'
    }
}
finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit(5000) | Out-Null
    }
    Remove-Item Env:VPSKIT_SSH_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:SSH_ASKPASS -ErrorAction SilentlyContinue
    Remove-Item Env:SSH_ASKPASS_REQUIRE -ErrorAction SilentlyContinue
    Remove-Item Env:DISPLAY -ErrorAction SilentlyContinue
    $targetPassword = $null
}
