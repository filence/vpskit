[CmdletBinding()]
param(
    [ValidatePattern('^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$')]
    [string]$WorkerName = 'vpskit-subscriptions',

    [string]$ClientSubscriptionFile = (Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) '.build\subscription-deployment\prod-node-main-client-current.json'),

    [string]$EnvironmentDocument = (Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) '.前期环境须知.md'),

    [switch]$Apply
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repositoryRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$workerSource = Join-Path $repositoryRoot 'deploy\cloudflare\worker.mjs'
$wranglerVersion = '4.112.0'

function Get-WorkersDeployToken {
    param([Parameter(Mandatory)][string]$Path)
    $text = [IO.File]::ReadAllText((Resolve-Path -LiteralPath $Path), [Text.Encoding]::UTF8)
    $match = [regex]::Match($text, '(?ms)^\s*3、VPSKit-Workers-Deploy API key\s*\r?\n\s*API Token[：:]\s*([^\s`]+)')
    if (-not $match.Success) { throw 'VPSKit-Workers-Deploy API Token was not found in the environment document.' }
    return $match.Groups[1].Value.Trim()
}

function Invoke-CFRead {
    param([Parameter(Mandatory)][string]$Path)
    $response = Invoke-RestMethod -Method Get -Uri "https://api.cloudflare.com/client/v4$Path" -Headers @{ Authorization = "Bearer $script:token" } -ErrorAction Stop
    if (-not $response.success) { throw "Cloudflare rejected read request $Path." }
    return $response.result
}

function Invoke-NativeChecked {
    param([Parameter(Mandatory)][string]$FilePath, [Parameter(Mandatory)][string[]]$ArgumentList)
    $processInfo = [Diagnostics.ProcessStartInfo]::new()
    $processInfo.FileName = $FilePath
    $processInfo.UseShellExecute = $false
    $processInfo.RedirectStandardOutput = $true
    $processInfo.RedirectStandardError = $true
    foreach ($argument in $ArgumentList) { $processInfo.ArgumentList.Add($argument) }
    $process = [Diagnostics.Process]::Start($processInfo)
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) {
        $summary = (($stderr, $stdout) -join "`n").Trim()
        if ($summary.Length -gt 1200) { $summary = $summary.Substring(0, 1200) }
        throw "Wrangler deployment failed with exit code $($process.ExitCode): $summary"
    }
}

if (-not (Test-Path -LiteralPath $workerSource -PathType Leaf)) { throw "Worker source is missing: $workerSource" }
if (-not (Test-Path -LiteralPath $ClientSubscriptionFile -PathType Leaf)) { throw "Client subscription file is missing: $ClientSubscriptionFile" }
$subscription = Get-Content -LiteralPath $ClientSubscriptionFile -Raw | ConvertFrom-Json
$endpoint = [Uri]$subscription.endpoint
if ($endpoint.Scheme -ne 'https' -or [string]::IsNullOrWhiteSpace($endpoint.DnsSafeHost)) { throw 'Client subscription endpoint is invalid.' }
$hostname = $endpoint.DnsSafeHost

$script:token = Get-WorkersDeployToken -Path $EnvironmentDocument
$previousToken = $env:CLOUDFLARE_API_TOKEN
$hadToken = Test-Path Env:CLOUDFLARE_API_TOKEN
$temporaryRoot = $null
try {
    $verify = Invoke-CFRead -Path '/user/tokens/verify'
    if ($verify.status -ne 'active') { throw 'Cloudflare management token is not active.' }
    $accounts = @(Invoke-CFRead -Path '/accounts?per_page=50')
    if ($accounts.Count -ne 1) { throw "Expected exactly one readable account, found $($accounts.Count)." }
    $accountId = $accounts[0].id
    $scripts = @(Invoke-CFRead -Path "/accounts/$accountId/workers/scripts") | Where-Object { $_.id -eq $WorkerName }
    if ($scripts.Count -ne 1) { throw "Expected existing Worker $WorkerName exactly once, found $($scripts.Count)." }
    $settings = Invoke-CFRead -Path "/accounts/$accountId/workers/scripts/$WorkerName/settings"
    $binding = @($settings.bindings | Where-Object { $_.name -eq 'SUBSCRIPTIONS' -and $_.type -eq 'kv_namespace' })
    if ($binding.Count -ne 1 -or [string]::IsNullOrWhiteSpace($binding[0].namespace_id)) { throw 'Existing Worker does not expose exactly one SUBSCRIPTIONS KV binding.' }

    if (-not $Apply) {
        [ordered]@{
            command = 'cloudflare subscription update'
            status = 'PASS'
            detail = [ordered]@{
                mode = 'READ_ONLY'
                worker_name = $WorkerName
                endpoint = "https://$hostname"
                existing_worker = $true
                existing_kv_binding = $true
                credentials_shown = $false
                apply_required = $true
            }
        } | ConvertTo-Json -Depth 5
        exit 0
    }

    $temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('vpskit-worker-update-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
    Copy-Item -LiteralPath $workerSource -Destination (Join-Path $temporaryRoot 'worker.mjs')
    $configPath = Join-Path $temporaryRoot 'wrangler.toml'
    $config = @"
name = "$WorkerName"
main = "worker.mjs"
compatibility_date = "2026-07-21"
workers_dev = false

[[kv_namespaces]]
binding = "SUBSCRIPTIONS"
id = "$($binding[0].namespace_id)"

[vars]
NODE_ID = "$($settings.bindings | Where-Object { $_.name -eq 'NODE_ID' } | Select-Object -ExpandProperty text -First 1)"

[[routes]]
pattern = "$hostname"
custom_domain = true
"@
    [IO.File]::WriteAllText($configPath, $config, [Text.UTF8Encoding]::new($false))
    $node = (Get-Command node -ErrorAction Stop).Source
    $npxCommand = (Get-Command npx.cmd -ErrorAction Stop).Source
    $npxCli = Join-Path (Split-Path -Parent $npxCommand) 'node_modules\npm\bin\npx-cli.js'
    if (-not (Test-Path -LiteralPath $npxCli -PathType Leaf)) { throw 'npx-cli.js is unavailable.' }
    $env:CLOUDFLARE_API_TOKEN = $script:token
    Invoke-NativeChecked -FilePath $node -ArgumentList @($npxCli, '--yes', "wrangler@$wranglerVersion", 'deploy', '--config', $configPath)
    $health = Invoke-RestMethod -Method Get -Uri "https://$hostname/healthz" -TimeoutSec 30
    if ($health.status -ne 'PASS') { throw 'Updated Worker healthcheck did not pass.' }
    [ordered]@{
        command = 'cloudflare subscription update'
        status = 'PASS'
        detail = [ordered]@{
            mode = 'APPLY'
            worker_name = $WorkerName
            endpoint = "https://$hostname"
            kv_binding_preserved = $true
            secrets_rotated = $false
            healthcheck = 'PASS'
            credentials_shown = $false
        }
    } | ConvertTo-Json -Depth 5
}
finally {
    if ($hadToken) { $env:CLOUDFLARE_API_TOKEN = $previousToken } else { Remove-Item Env:CLOUDFLARE_API_TOKEN -ErrorAction SilentlyContinue }
    $previousToken = $null
    $script:token = $null
    if ($temporaryRoot -and (Test-Path -LiteralPath $temporaryRoot)) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
