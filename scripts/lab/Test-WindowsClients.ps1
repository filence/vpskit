[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$SingBoxPath,

    [Parameter(Mandatory)]
    [string]$DeploymentDirectory,

    [string]$EnvironmentDocument = (Join-Path $PSScriptRoot '..\..\前期环境须知.md'),

    [string]$EndpointOverride,

    [int]$RealityEndpointPort,

    [ValidateSet('All', 'Reality', 'Hysteria2')]
    [string]$ProfileMode = 'All'
)

$ErrorActionPreference = 'Stop'

function Wait-LocalPort {
    param(
        [int]$Port,
        [System.Diagnostics.Process]$Process
    )

    for ($attempt = 0; $attempt -lt 40; $attempt++) {
        if ($Process.HasExited) {
            throw "Client process exited before port $Port became ready"
        }
        $client = [System.Net.Sockets.TcpClient]::new()
        try {
            $pending = $client.ConnectAsync('127.0.0.1', $Port)
            if ($pending.Wait(250) -and $client.Connected) {
                return
            }
        }
        catch {
        }
        finally {
            $client.Dispose()
        }
        Start-Sleep -Milliseconds 250
    }
    throw "Local SOCKS port $Port did not become ready"
}

function Get-FreeTcpPort {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    try {
        $listener.Start()
        return ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
    }
    finally {
        $listener.Stop()
    }
}

function Test-VPSKitProfile {
    param(
        [string]$Name,
        [string]$ConfigName,
        [int]$Port,
        [string]$ExpectedIP
    )

    $configPath = Join-Path $DeploymentDirectory $ConfigName
    & $SingBoxPath check -c $configPath
    if ($LASTEXITCODE -ne 0) {
        throw "$Name config validation failed"
    }

    $runId = [guid]::NewGuid().ToString('N')
    $stdoutPath = Join-Path $DeploymentDirectory "$Name-$runId.stdout.log"
    $stderrPath = Join-Path $DeploymentDirectory "$Name-$runId.stderr.log"
    $process = Start-Process -FilePath $SingBoxPath `
        -ArgumentList @('run', '-c', $configPath) `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdoutPath `
        -RedirectStandardError $stderrPath `
        -PassThru
    try {
        Wait-LocalPort -Port $Port -Process $process
        $curlOutput = @(& curl.exe `
                --silent `
                --show-error `
                --fail `
                --max-time 30 `
                --socks5-hostname "127.0.0.1:$Port" `
                'https://api.ipify.org')
        if ($LASTEXITCODE -ne 0) {
            Start-Sleep -Seconds 2
            throw "$Name proxy request failed: $LASTEXITCODE"
        }
        $actualIP = ($curlOutput -join "`n").Trim()
        if ($actualIP -ne $ExpectedIP) {
            throw "$Name returned an unexpected exit IP"
        }
        Write-Output "CLIENT_HANDSHAKE=PASS profile=$Name exit_ip_matches_vps=true"
    }
    finally {
        if (-not $process.HasExited) {
            Stop-Process -Id $process.Id -Force
            $process.WaitForExit(5000) | Out-Null
        }
    }
}

function New-EndpointOverrideConfig {
    param(
        [string]$SourceName,
        [string]$DestinationName,
        [string]$OutboundType,
        [string]$EndpointIP,
        [int]$EndpointPort,
        [int]$LocalPort
    )

    $sourcePath = Join-Path $DeploymentDirectory $SourceName
    $destinationPath = Join-Path $DeploymentDirectory $DestinationName
    $configuration = Get-Content -LiteralPath $sourcePath -Raw -Encoding UTF8 | ConvertFrom-Json
    $matchingOutbounds = @($configuration.outbounds | Where-Object { $_.type -eq $OutboundType })
    if ($matchingOutbounds.Count -ne 1) {
        throw "Expected exactly one $OutboundType outbound in $SourceName"
    }
    $matchingOutbounds[0].server = $EndpointIP
    if ($EndpointPort -gt 0) {
        if ($EndpointPort -gt 65535) {
            throw 'Endpoint port is outside the valid range'
        }
        $matchingOutbounds[0].server_port = $EndpointPort
    }
    $configuration.inbounds[0].listen_port = $LocalPort
    $configuration.log.level = 'debug'
    $json = $configuration | ConvertTo-Json -Depth 100
    [System.IO.File]::WriteAllText($destinationPath, $json, [System.Text.UTF8Encoding]::new($false))
}

$resolvedSingBox = (Resolve-Path -LiteralPath $SingBoxPath).Path
$resolvedDeployment = (Resolve-Path -LiteralPath $DeploymentDirectory).Path
$environmentText = Get-Content -LiteralPath $EnvironmentDocument -Raw -Encoding UTF8
$ipMatch = [regex]::Match($environmentText, '(?m)^IPV4地址[：:]\s*(?<ip>\d{1,3}(?:\.\d{1,3}){3})\s*$')
if (-not $ipMatch.Success) {
    throw 'VPS IPv4 field is missing from the environment document'
}
$expectedIP = $ipMatch.Groups['ip'].Value
$endpointIP = $expectedIP
if (-not [string]::IsNullOrWhiteSpace($EndpointOverride)) {
    $parsedEndpoint = $null
    if (-not [System.Net.IPAddress]::TryParse($EndpointOverride, [ref]$parsedEndpoint)) {
        throw 'EndpointOverride must be an IP address'
    }
    $endpointIP = $EndpointOverride
}

$SingBoxPath = $resolvedSingBox
$DeploymentDirectory = $resolvedDeployment
$realityLocalPort = Get-FreeTcpPort
$hysteria2LocalPort = Get-FreeTcpPort
while ($hysteria2LocalPort -eq $realityLocalPort) {
    $hysteria2LocalPort = Get-FreeTcpPort
}
New-EndpointOverrideConfig `
    -SourceName 'sing-box-reality.json' `
    -DestinationName 'sing-box-reality.endpoint-ip.test.json' `
    -OutboundType 'vless' `
    -EndpointIP $endpointIP `
    -EndpointPort $RealityEndpointPort `
    -LocalPort $realityLocalPort
New-EndpointOverrideConfig `
    -SourceName 'sing-box-hysteria2.json' `
    -DestinationName 'sing-box-hysteria2.endpoint-ip.test.json' `
    -OutboundType 'hysteria2' `
    -EndpointIP $endpointIP `
    -LocalPort $hysteria2LocalPort

if ($ProfileMode -in @('All', 'Reality')) {
    Test-VPSKitProfile -Name 'reality' -ConfigName 'sing-box-reality.endpoint-ip.test.json' -Port $realityLocalPort -ExpectedIP $expectedIP
}
if ($ProfileMode -in @('All', 'Hysteria2')) {
    Test-VPSKitProfile -Name 'hysteria2' -ConfigName 'sing-box-hysteria2.endpoint-ip.test.json' -Port $hysteria2LocalPort -ExpectedIP $expectedIP
}
