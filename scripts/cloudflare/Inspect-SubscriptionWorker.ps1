[CmdletBinding()]
param(
    [string]$EnvironmentDocument = (Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) '.前期环境须知.md')
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

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

$script:token = Get-WorkersDeployToken -Path $EnvironmentDocument
try {
    $verify = Invoke-CFRead -Path '/user/tokens/verify'
    $accounts = @(Invoke-CFRead -Path '/accounts?per_page=50')
    if ($accounts.Count -ne 1) { throw "Expected exactly one readable account, found $($accounts.Count)." }
    $accountId = $accounts[0].id
    $workers = @(Invoke-CFRead -Path "/accounts/$accountId/workers/scripts") | Where-Object { $_.id -like 'vpskit-subscriptions*' }
    $namespaces = @(Invoke-CFRead -Path "/accounts/$accountId/storage/kv/namespaces?per_page=100") | Where-Object { $_.title -like 'vpskit-subscriptions*' }
    [ordered]@{
        command = 'cloudflare subscription inspect'
        status = 'PASS'
        detail = [ordered]@{
            token_active = ($verify.status -eq 'active')
            worker_names = @($workers | ForEach-Object { $_.id })
            kv_namespace_titles = @($namespaces | ForEach-Object { $_.title })
            credentials_shown = $false
        }
    } | ConvertTo-Json -Depth 5
}
finally {
    $script:token = $null
}
