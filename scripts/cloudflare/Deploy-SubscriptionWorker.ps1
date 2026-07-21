[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidatePattern('^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$')]
    [string]$NodeId,

    [Parameter(Mandatory)]
    [ValidatePattern('^[a-z0-9.-]+$')]
    [string]$ZoneName,

    [Parameter(Mandatory)]
    [ValidatePattern('^[a-z0-9.-]+$')]
    [string]$Hostname,

    [ValidatePattern('^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$')]
    [string]$WorkerName = 'vpskit-subscriptions-dev',

    [string]$NamespaceTitle = 'vpskit-subscriptions-dev',

    [string]$ManagementTokenFile,

    [string]$CredentialOutput,

    [switch]$Apply
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$wranglerVersion = '4.112.0'
$apiRoot = 'https://api.cloudflare.com/client/v4'
$scriptRoot = Split-Path -Parent $PSScriptRoot
$repositoryRoot = Split-Path -Parent $scriptRoot
$workerSource = Join-Path $repositoryRoot 'deploy\cloudflare\worker.mjs'

function Write-Result {
    param(
        [Parameter(Mandatory)]
        [string]$Status,
        [Parameter(Mandatory)]
        [hashtable]$Detail
    )
    [ordered]@{
        command = 'cloudflare subscription deploy'
        status  = $Status
        detail  = $Detail
    } | ConvertTo-Json -Depth 8
}

function Get-ManagementToken {
    if (-not [string]::IsNullOrWhiteSpace($env:CLOUDFLARE_API_TOKEN)) {
        return $env:CLOUDFLARE_API_TOKEN.Trim()
    }
    if ([string]::IsNullOrWhiteSpace($ManagementTokenFile)) {
        throw 'Apply mode requires CLOUDFLARE_API_TOKEN or -ManagementTokenFile.'
    }
    $resolved = (Resolve-Path -LiteralPath $ManagementTokenFile).Path
    $item = Get-Item -LiteralPath $resolved -Force
    if ($item.LinkType -or -not ($item -is [System.IO.FileInfo])) {
        throw 'Management token input must be a regular file, not a link or directory.'
    }
    $token = (Get-Content -LiteralPath $resolved -Raw).Trim()
    if ($token.Length -lt 20 -or $token -match '\s') {
        throw 'Management token input is malformed.'
    }
    return $token
}

function Invoke-CloudflareApi {
    param(
        [Parameter(Mandatory)]
        [ValidateSet('GET', 'POST', 'PUT', 'DELETE')]
        [string]$Method,
        [Parameter(Mandatory)]
        [string]$Path,
        [object]$Body
    )
    $headers = @{ Authorization = "Bearer $script:managementToken" }
    $request = @{
        Method      = $Method
        Uri         = "$apiRoot$Path"
        Headers     = $headers
        ErrorAction = 'Stop'
    }
    if ($PSBoundParameters.ContainsKey('Body')) {
        $request.ContentType = 'application/json'
        $request.Body = $Body | ConvertTo-Json -Depth 8 -Compress
    }
    $response = Invoke-RestMethod @request
    if (-not $response.success) {
        throw "Cloudflare API rejected $Method $Path."
    }
    return $response.result
}

function New-OpaqueToken {
    $bytes = [byte[]]::new(32)
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    return [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function Get-Sha256Hex {
    param([Parameter(Mandatory)][string]$Value)
    $bytes = [Text.Encoding]::UTF8.GetBytes($Value)
    $digest = [Security.Cryptography.SHA256]::HashData($bytes)
    return [Convert]::ToHexString($digest).ToLowerInvariant()
}

function Get-NodeWranglerInvocation {
    $node = (Get-Command node -ErrorAction Stop).Source
    $npxCommand = (Get-Command npx.cmd -ErrorAction Stop).Source
    $npxCli = Join-Path (Split-Path -Parent $npxCommand) 'node_modules\npm\bin\npx-cli.js'
    if (-not (Test-Path -LiteralPath $npxCli -PathType Leaf)) {
        throw "Cannot locate npx-cli.js beside $npxCommand."
    }
    return @{ Node = $node; NpxCli = $npxCli }
}

function Invoke-NativeChecked {
    param(
        [Parameter(Mandatory)][string]$FilePath,
        [Parameter(Mandatory)][string[]]$ArgumentList,
        [string]$StandardInput
    )
    $processInfo = [Diagnostics.ProcessStartInfo]::new()
    $processInfo.FileName = $FilePath
    $processInfo.UseShellExecute = $false
    $processInfo.RedirectStandardOutput = $true
    $processInfo.RedirectStandardError = $true
    if ($PSBoundParameters.ContainsKey('StandardInput')) {
        $processInfo.RedirectStandardInput = $true
    }
    foreach ($argument in $ArgumentList) {
        $processInfo.ArgumentList.Add($argument)
    }
    $process = [Diagnostics.Process]::Start($processInfo)
    if ($PSBoundParameters.ContainsKey('StandardInput')) {
        $process.StandardInput.WriteLine($StandardInput)
        $process.StandardInput.Close()
    }
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) {
        $summary = (($stderr, $stdout) -join "`n").Trim()
        if ($summary.Length -gt 1200) {
            $summary = $summary.Substring(0, 1200)
        }
        throw "Native deployment command failed with exit code $($process.ExitCode): $summary"
    }
    return $stdout
}

function Write-SensitiveJson {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][object]$Value
    )
    $absolute = [IO.Path]::GetFullPath($Path)
    if (Test-Path -LiteralPath $absolute) {
        throw "Refusing to overwrite credential output: $absolute"
    }
    $parent = Split-Path -Parent $absolute
    if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
        New-Item -ItemType Directory -Path $parent | Out-Null
    }
    $json = ($Value | ConvertTo-Json -Depth 6) + "`n"
    [IO.File]::WriteAllText($absolute, $json, [Text.UTF8Encoding]::new($false))
    if ($IsWindows) {
        $acl = [Security.AccessControl.FileSecurity]::new()
        $acl.SetAccessRuleProtection($true, $false)
        $identity = [Security.Principal.WindowsIdentity]::GetCurrent().User
        $rule = [Security.AccessControl.FileSystemAccessRule]::new(
            $identity,
            [Security.AccessControl.FileSystemRights]::FullControl,
            [Security.AccessControl.AccessControlType]::Allow
        )
        $acl.AddAccessRule($rule)
        Set-Acl -LiteralPath $absolute -AclObject $acl
    }
    else {
        & chmod 600 $absolute
        if ($LASTEXITCODE -ne 0) {
            throw 'Failed to set credential output permissions to 0600.'
        }
    }
    return $absolute
}

$ZoneName = $ZoneName.Trim('.').ToLowerInvariant()
$Hostname = $Hostname.Trim('.').ToLowerInvariant()
if ($Hostname -ne $ZoneName -and -not $Hostname.EndsWith(".$ZoneName", [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Hostname must equal the zone or be a subdomain of it.'
}
if (-not (Test-Path -LiteralPath $workerSource -PathType Leaf)) {
    throw "Worker source is missing: $workerSource"
}

if (-not $Apply) {
    Write-Result -Status 'PASS' -Detail ([ordered]@{
        mode                   = 'READ_ONLY'
        worker_name            = $WorkerName
        namespace_title        = $NamespaceTitle
        node_id                = $NodeId
        hostname               = $Hostname
        wrangler_version       = $wranglerVersion
        writes                 = @('Cloudflare KV namespace', 'Cloudflare Worker', 'Worker secrets', 'Custom Domain', 'credential output')
        management_token_shown = $false
        secret_values_shown    = $false
        apply_required         = $true
    })
    exit 0
}

if ([string]::IsNullOrWhiteSpace($CredentialOutput)) {
    throw 'Apply mode requires -CredentialOutput.'
}
if (Test-Path -LiteralPath $CredentialOutput) {
    throw 'Credential output already exists; refusing an implicit credential rotation.'
}

$script:managementToken = Get-ManagementToken
$verify = Invoke-CloudflareApi -Method GET -Path '/user/tokens/verify'
if ($verify.status -ne 'active') {
    throw 'Cloudflare management token is not active.'
}
$encodedZone = [Uri]::EscapeDataString($ZoneName)
$zones = @(Invoke-CloudflareApi -Method GET -Path "/zones?name=$encodedZone&status=active")
if ($zones.Count -ne 1) {
    throw "Expected one active Cloudflare zone named $ZoneName, found $($zones.Count)."
}
$zone = $zones[0]
$accountId = $zone.account.id
$zoneId = $zone.id

$workerScripts = @(Invoke-CloudflareApi -Method GET -Path "/accounts/$accountId/workers/scripts")
$existingWorkers = @($workerScripts | Where-Object { $_.id -eq $WorkerName })
if ($existingWorkers.Count -gt 0) {
    throw "Worker $WorkerName already exists; refusing to replace it or rotate bootstrap credentials implicitly."
}

$namespaces = @(Invoke-CloudflareApi -Method GET -Path "/accounts/$accountId/storage/kv/namespaces?per_page=100")
$matchingNamespaces = @($namespaces | Where-Object { $_.title -eq $NamespaceTitle })
if ($matchingNamespaces.Count -gt 1) {
    throw "Multiple KV namespaces use title $NamespaceTitle."
}
if ($matchingNamespaces.Count -eq 1) {
    $namespaceId = $matchingNamespaces[0].id
}
else {
    $namespace = Invoke-CloudflareApi -Method POST -Path "/accounts/$accountId/storage/kv/namespaces" -Body @{ title = $NamespaceTitle }
    $namespaceId = $namespace.id
}

$publishSecret = New-OpaqueToken
$readToken = New-OpaqueToken
$readTokenHash = Get-Sha256Hex -Value $readToken
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ("vpskit-cloudflare-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
try {
    Copy-Item -LiteralPath $workerSource -Destination (Join-Path $temporaryRoot 'worker.mjs')
    $configPath = Join-Path $temporaryRoot 'wrangler.toml'
    $config = @"
name = "$WorkerName"
main = "worker.mjs"
compatibility_date = "2026-07-21"
workers_dev = false

[[kv_namespaces]]
binding = "SUBSCRIPTIONS"
id = "$namespaceId"

[vars]
NODE_ID = "$NodeId"

[[routes]]
pattern = "$Hostname"
custom_domain = true
"@
    [IO.File]::WriteAllText($configPath, $config, [Text.UTF8Encoding]::new($false))
    $invocation = Get-NodeWranglerInvocation
    $baseArguments = @($invocation.NpxCli, '--yes', "wrangler@$wranglerVersion")
    $deployArguments = $baseArguments + @('deploy', '--config', $configPath)
    Invoke-NativeChecked -FilePath $invocation.Node -ArgumentList $deployArguments | Out-Null

    foreach ($secret in @(
        @{ Name = 'NODE_PUBLISH_SECRET'; Value = $publishSecret },
        @{ Name = 'INITIAL_READ_TOKEN_HASH'; Value = $readTokenHash }
    )) {
        $secretArguments = $baseArguments + @('secret', 'put', $secret.Name, '--config', $configPath)
        Invoke-NativeChecked -FilePath $invocation.Node -ArgumentList $secretArguments -StandardInput $secret.Value | Out-Null
    }

    $healthUri = "https://$Hostname/healthz"
    $deadline = [DateTimeOffset]::UtcNow.AddMinutes(3)
    $healthy = $false
    do {
        try {
            $health = Invoke-RestMethod -Method GET -Uri $healthUri -TimeoutSec 15
            $healthy = $health.status -eq 'PASS'
        }
        catch {
            $healthy = $false
        }
        if (-not $healthy) {
            Start-Sleep -Seconds 3
        }
    } while (-not $healthy -and [DateTimeOffset]::UtcNow -lt $deadline)
    if (-not $healthy) {
        throw "Worker deployed but custom-domain healthcheck did not become ready: $healthUri"
    }

    $credential = [ordered]@{
        schema_version      = 1
        endpoint            = "https://$Hostname"
        node_id             = $NodeId
        node_publish_secret = $publishSecret
        read_token          = $readToken
    }
    $credentialPath = Write-SensitiveJson -Path $CredentialOutput -Value $credential

    Write-Result -Status 'PASS' -Detail ([ordered]@{
        mode                   = 'APPLY'
        account_id             = $accountId
        zone_id                = $zoneId
        worker_name            = $WorkerName
        namespace_id           = $namespaceId
        namespace_title        = $NamespaceTitle
        endpoint               = "https://$Hostname"
        healthcheck            = 'PASS'
        credential_output      = $credentialPath
        wrangler_version       = $wranglerVersion
        management_token_shown = $false
        secret_values_shown    = $false
    })
}
finally {
    $script:managementToken = $null
    $publishSecret = $null
    $readToken = $null
    $readTokenHash = $null
    if (Test-Path -LiteralPath $temporaryRoot) {
        $resolvedTemporary = [IO.Path]::GetFullPath($temporaryRoot)
        $resolvedSystemTemporary = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if ($resolvedTemporary.StartsWith($resolvedSystemTemporary, [StringComparison]::OrdinalIgnoreCase) -and (Split-Path -Leaf $resolvedTemporary) -like 'vpskit-cloudflare-*') {
            Remove-Item -LiteralPath $resolvedTemporary -Recurse -Force
        }
    }
}
