[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$MihomoPath,

    [Parameter(Mandatory)]
    [string]$DeploymentDirectory,

    [string]$EnvironmentDocument = (Join-Path $PSScriptRoot '..\..\前期环境须知.md'),

    [string]$EndpointOverride,

    [int]$RealityEndpointPort,

    [ValidateSet('All', 'Reality', 'Hysteria2')]
    [string]$ProfileMode = 'All',

    [switch]$EnableRealityHybridKeyExchange,

    [switch]$DisableRealityHybridKeyExchange
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
            throw "Mihomo exited before local port $Port became ready"
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
    throw "Mihomo local port $Port did not become ready"
}

function New-TestConfiguration {
    param(
        [string]$SourcePath,
        [string]$DestinationPath,
        [string]$EndpointIP,
        [int]$MixedPort,
        [int]$RealityPort,
        [ValidateSet('Reality', 'Hysteria2')]
        [string]$Profile
    )

    $configuration = Get-Content -LiteralPath $SourcePath -Raw -Encoding UTF8

    $namePattern = '(?m)^\s+-\s+name:\s*(?<name>[^\r\n]+?)\s*$'
    $proxyNames = @([regex]::Matches($configuration, $namePattern) | ForEach-Object { $_.Groups['name'].Value.Trim('"', "'") })
    $realityNames = @($proxyNames | Where-Object { $_ -like '*-Reality' })
    $hysteria2Names = @($proxyNames | Where-Object { $_ -like '*-Hysteria2' })
    if ($realityNames.Count -ne 1 -or $hysteria2Names.Count -ne 1) {
        throw 'Expected one metadata-derived Reality name and one Hysteria2 name'
    }
    $realityName = $realityNames[0]
    $hysteria2Name = $hysteria2Names[0]

    $portPattern = '(?m)^mixed-port:\s*\d+\s*$'
    if ([regex]::Matches($configuration, $portPattern).Count -ne 1) {
        throw 'Expected exactly one mixed-port setting in the Mihomo export'
    }
    $configuration = [regex]::Replace($configuration, $portPattern, "mixed-port: $MixedPort")

    $serverPattern = '(?m)^(?<indent>\s+)server:\s*[^\r\n]+\s*$'
    if ([regex]::Matches($configuration, $serverPattern).Count -ne 2) {
        throw 'Expected exactly two server endpoints in the Mihomo export'
    }
    $configuration = [regex]::Replace(
        $configuration,
        $serverPattern,
        { param($match) "$($match.Groups['indent'].Value)server: $EndpointIP" }
    )

    if ($Profile -eq 'Reality' -and $RealityPort -gt 0) {
        if ($RealityPort -gt 65535) {
            throw 'Reality endpoint port is outside the valid TCP range'
        }
        $proxyPortPattern = '(?m)^(?<indent>\s+)port:\s*\d+\s*$'
        if ([regex]::Matches($configuration, $proxyPortPattern).Count -ne 2) {
            throw 'Expected exactly two proxy port settings in the Mihomo export'
        }
        $portOccurrence = 0
        $configuration = [regex]::Replace(
            $configuration,
            $proxyPortPattern,
            {
                param($match)
                $portOccurrence++
                if ($portOccurrence -eq 1) {
                    return "$($match.Groups['indent'].Value)port: $RealityPort"
                }
                return $match.Value
            }
        )
    }

    if ($Profile -eq 'Hysteria2') {
        $escapedRealityName = [regex]::Escape($realityName)
        $escapedHysteria2Name = [regex]::Escape($hysteria2Name)
        $inlineSelectionPattern = "(?m)^(?<indent>\s*)proxies:\s*\[\s*$escapedRealityName\s*,\s*$escapedHysteria2Name\s*,\s*DIRECT\s*\]\s*$"
        $blockSelectionPattern = "(?m)^(?<indent>\s*)-\s+$escapedRealityName\s*\r?\n\k<indent>-\s+$escapedHysteria2Name\s*$"
        $inlineMatches = [regex]::Matches($configuration, $inlineSelectionPattern)
        $blockMatches = [regex]::Matches($configuration, $blockSelectionPattern)
        if (($inlineMatches.Count + $blockMatches.Count) -ne 1) {
            throw 'Expected one Reality/Hysteria2 proxy selection sequence'
        }
        if ($inlineMatches.Count -eq 1) {
            $configuration = [regex]::Replace(
                $configuration,
                $inlineSelectionPattern,
                { param($match) "$($match.Groups['indent'].Value)proxies: [$hysteria2Name, $realityName, DIRECT]" }
            )
        }
        else {
            $configuration = [regex]::Replace(
                $configuration,
                $blockSelectionPattern,
                { param($match) "$($match.Groups['indent'].Value)- $hysteria2Name`n$($match.Groups['indent'].Value)- $realityName" }
            )
        }
    }
    elseif ($EnableRealityHybridKeyExchange -or $DisableRealityHybridKeyExchange) {
        $realityShortIdPattern = '(?m)^(?<indent>\s+)short-id:\s*[^\r\n]+\s*$'
        if ([regex]::Matches($configuration, $realityShortIdPattern).Count -ne 1) {
            throw 'Expected exactly one Reality short-id setting'
        }
        $hybridValue = if ($EnableRealityHybridKeyExchange) { 'true' } else { 'false' }
        $configuration = [regex]::Replace(
            $configuration,
            $realityShortIdPattern,
            { param($match) "$($match.Value.TrimEnd())`n$($match.Groups['indent'].Value)support-x25519mlkem768: $hybridValue" }
        )
    }

    [System.IO.File]::WriteAllText(
        $DestinationPath,
        $configuration,
        [System.Text.UTF8Encoding]::new($false)
    )
}

function Test-MihomoProfile {
    param(
        [string]$Name,
        [string]$ConfigPath,
        [string]$WorkDirectory,
        [int]$Port,
        [string]$ExpectedIP
    )

    [System.IO.Directory]::CreateDirectory($WorkDirectory) | Out-Null
    & $MihomoPath -t -d $WorkDirectory -f $ConfigPath *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "$Name Mihomo configuration validation failed"
    }

    $runId = [guid]::NewGuid().ToString('N')
    $stdoutPath = Join-Path $DeploymentDirectory "mihomo-$Name-$runId.stdout.log"
    $stderrPath = Join-Path $DeploymentDirectory "mihomo-$Name-$runId.stderr.log"
    $process = Start-Process -FilePath $MihomoPath `
        -ArgumentList @('-d', $WorkDirectory, '-f', $ConfigPath) `
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
                --max-time 45 `
                --socks5-hostname "127.0.0.1:$Port" `
                'https://api.ipify.org')
        if ($LASTEXITCODE -ne 0) {
            Start-Sleep -Seconds 2
            throw "$Name proxy request failed with curl exit code $LASTEXITCODE"
        }
        $actualIP = ($curlOutput -join "`n").Trim()
        if ($actualIP -ne $ExpectedIP) {
            throw "$Name returned an unexpected exit IP"
        }
        Write-Output "MIHOMO_CLIENT_HANDSHAKE=PASS profile=$Name exit_ip_matches_vps=true"
    }
    finally {
        if (-not $process.HasExited) {
            Stop-Process -Id $process.Id -Force
            $process.WaitForExit(5000) | Out-Null
        }
    }
}

$MihomoPath = (Resolve-Path -LiteralPath $MihomoPath).Path
if ($EnableRealityHybridKeyExchange -and $DisableRealityHybridKeyExchange) {
    throw 'EnableRealityHybridKeyExchange and DisableRealityHybridKeyExchange are mutually exclusive'
}
$DeploymentDirectory = (Resolve-Path -LiteralPath $DeploymentDirectory).Path
$originalConfig = Join-Path $DeploymentDirectory 'mihomo.yaml'
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

$parseWorkDirectory = Join-Path $DeploymentDirectory 'mihomo-export-parse-work'
[System.IO.Directory]::CreateDirectory($parseWorkDirectory) | Out-Null
& $MihomoPath -t -d $parseWorkDirectory -f $originalConfig *> $null
if ($LASTEXITCODE -ne 0) {
    throw 'Original Mihomo export failed native parser validation'
}
$versionText = (& $MihomoPath -v | Select-Object -First 1).Trim()
Write-Output "MIHOMO_EXPORT_PARSE=PASS version=$versionText"

$realityConfig = Join-Path $DeploymentDirectory 'mihomo-reality.endpoint-ip.test.yaml'
$hysteria2Config = Join-Path $DeploymentDirectory 'mihomo-hysteria2.endpoint-ip.test.yaml'
New-TestConfiguration -SourcePath $originalConfig -DestinationPath $realityConfig -EndpointIP $endpointIP -MixedPort 17890 -RealityPort $RealityEndpointPort -Profile Reality
New-TestConfiguration -SourcePath $originalConfig -DestinationPath $hysteria2Config -EndpointIP $endpointIP -MixedPort 17891 -Profile Hysteria2

if ($ProfileMode -in @('All', 'Reality')) {
    Test-MihomoProfile `
        -Name 'reality' `
        -ConfigPath $realityConfig `
        -WorkDirectory (Join-Path $DeploymentDirectory 'mihomo-reality-work') `
        -Port 17890 `
        -ExpectedIP $expectedIP
}
if ($ProfileMode -in @('All', 'Hysteria2')) {
    Test-MihomoProfile `
        -Name 'hysteria2' `
        -ConfigPath $hysteria2Config `
        -WorkDirectory (Join-Path $DeploymentDirectory 'mihomo-hysteria2-work') `
        -Port 17891 `
        -ExpectedIP $expectedIP
}
