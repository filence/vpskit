[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$XrayPath,

    [Parameter(Mandatory)]
    [string]$DeploymentDirectory,

    [string]$EnvironmentDocument = (Join-Path $PSScriptRoot '..\..\前期环境须知.md'),

    [string]$EndpointOverride,

    [int]$RealityEndpointPort
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Wait-LocalPort {
    param(
        [int]$Port,
        [System.Diagnostics.Process]$Process
    )

    for ($attempt = 0; $attempt -lt 80; $attempt++) {
        if ($Process.HasExited) {
            throw "Xray exited before local port $Port became ready"
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
    throw "Xray local port $Port did not become ready"
}

$XrayPath = (Resolve-Path -LiteralPath $XrayPath).Path
$DeploymentDirectory = (Resolve-Path -LiteralPath $DeploymentDirectory).Path
$sourcePath = Join-Path $DeploymentDirectory 'sing-box-reality.json'
$source = Get-Content -LiteralPath $sourcePath -Raw -Encoding UTF8 | ConvertFrom-Json
$outbounds = @($source.outbounds | Where-Object type -EQ 'vless')
if ($outbounds.Count -ne 1) {
    throw "Expected exactly one VLESS outbound; found $($outbounds.Count)"
}
$outbound = $outbounds[0]

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
$endpointPort = [int]$outbound.server_port
if ($RealityEndpointPort -gt 0) {
    if ($RealityEndpointPort -gt 65535) {
        throw 'RealityEndpointPort is outside the valid TCP range'
    }
    $endpointPort = $RealityEndpointPort
}
$localPort = 17892

$configuration = [ordered]@{
    log = [ordered]@{
        loglevel = 'warning'
    }
    inbounds = @(
        [ordered]@{
            listen = '127.0.0.1'
            port = $localPort
            protocol = 'socks'
            settings = [ordered]@{
                auth = 'noauth'
                udp = $true
            }
            tag = 'socks-in'
        }
    )
    outbounds = @(
        [ordered]@{
            protocol = 'vless'
            settings = [ordered]@{
                vnext = @(
                    [ordered]@{
                        address = $endpointIP
                        port = $endpointPort
                        users = @(
                            [ordered]@{
                                id = $outbound.uuid
                                encryption = 'none'
                                flow = $outbound.flow
                            }
                        )
                    }
                )
            }
            streamSettings = [ordered]@{
                method = 'raw'
                security = 'reality'
                realitySettings = [ordered]@{
                    serverName = $outbound.tls.server_name
                    fingerprint = 'chrome'
                    password = $outbound.tls.reality.public_key
                    shortId = $outbound.tls.reality.short_id
                    spiderX = '/'
                }
            }
            tag = 'reality-out'
        }
    )
}

$configPath = Join-Path $DeploymentDirectory 'xray-reality.endpoint-ip.test.json'
$json = $configuration | ConvertTo-Json -Depth 100
[System.IO.File]::WriteAllText($configPath, $json, [System.Text.UTF8Encoding]::new($false))

& $XrayPath run -test -config $configPath *> $null
if ($LASTEXITCODE -ne 0) {
    throw 'Xray Reality client configuration validation failed'
}
Write-Output 'XRAY_CLIENT_CONFIG=PASS version=v26.3.27'

$runId = [guid]::NewGuid().ToString('N')
$stdoutPath = Join-Path $DeploymentDirectory "xray-reality-$runId.stdout.log"
$stderrPath = Join-Path $DeploymentDirectory "xray-reality-$runId.stderr.log"
$process = Start-Process -FilePath $XrayPath `
    -ArgumentList @('run', '-config', $configPath) `
    -WindowStyle Hidden `
    -RedirectStandardOutput $stdoutPath `
    -RedirectStandardError $stderrPath `
    -PassThru
try {
    Wait-LocalPort -Port $localPort -Process $process
    $curlOutput = @(& curl.exe `
            --silent `
            --show-error `
            --fail `
            --max-time 45 `
            --socks5-hostname "127.0.0.1:$localPort" `
            'https://api.ipify.org')
    if ($LASTEXITCODE -ne 0) {
        Start-Sleep -Seconds 2
        throw "Xray Reality proxy request failed with curl exit code $LASTEXITCODE"
    }
    $actualIP = ($curlOutput -join "`n").Trim()
    if ($actualIP -ne $expectedIP) {
        throw 'Xray Reality returned an unexpected exit IP'
    }
    Write-Output 'XRAY_CLIENT_HANDSHAKE=PASS profile=reality exit_ip_matches_vps=true'
}
finally {
    if (-not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit(5000) | Out-Null
    }
}
