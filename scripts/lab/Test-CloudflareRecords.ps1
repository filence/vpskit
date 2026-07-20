[CmdletBinding()]
param(
    [string]$EnvironmentDocument = (Join-Path $PSScriptRoot '..\..\前期环境须知.md')
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Read-EnvironmentField {
    param(
        [string]$Text,
        [string]$Label
    )

    $match = [regex]::Match(
        $Text,
        "(?m)^$([regex]::Escape($Label))[：:]\s*(?<value>.+?)\s*$"
    )
    if (-not $match.Success) {
        throw "Required environment field is missing: $Label"
    }
    return $match.Groups['value'].Value.Trim()
}

try {
    $stage = 'read_environment'
    $text = Get-Content -LiteralPath $EnvironmentDocument -Raw -Encoding UTF8
    $stage = 'read_record_name'
    $recordValue = Read-EnvironmentField -Text $text -Label '1、灰云域名'
    $recordMatches = @([regex]::Matches($recordValue, '(?i)(?:[a-z0-9-]+\.)+[a-z]{2,63}'))
    if ($recordMatches.Count -ne 1) {
        throw 'The Cloudflare field did not contain exactly one DNS name'
    }
    $recordName = $recordMatches[0].Value.ToLowerInvariant()
    $stage = 'read_api_token'
    $token = Read-EnvironmentField -Text $text -Label 'API Token'
    $stage = 'read_vps_address'
    $expectedIP = Read-EnvironmentField -Text $text -Label 'IPV4地址'

    $stage = 'validate_record_name'
    if ($recordName -notmatch '^[A-Za-z0-9.-]+$') {
        throw 'The Cloudflare record name is invalid'
    }
    $stage = 'validate_vps_address'
    if ($expectedIP -notmatch '^\d{1,3}(?:\.\d{1,3}){3}$') {
        throw 'The expected VPS address is invalid'
    }
    $stage = 'validate_api_token'
    if ([string]::IsNullOrWhiteSpace($token) -or $token -match '[\r\n\x00]') {
        throw 'The Cloudflare API token is invalid'
    }

    $stage = 'derive_zone'
    $labels = $recordName.TrimEnd('.').Split('.')
    if ($labels.Count -lt 2) {
        throw 'Unable to derive a Cloudflare zone name'
    }
    $zoneName = ($labels[-2..-1] -join '.')
    $headers = @{
        Authorization = "Bearer $token"
        Accept = 'application/json'
    }

    $encodedZone = [System.Net.WebUtility]::UrlEncode($zoneName)
    $stage = 'lookup_zone'
    $zoneResponse = Invoke-RestMethod `
        -Uri "https://api.cloudflare.com/client/v4/zones?name=$encodedZone" `
        -Headers $headers
    if (-not $zoneResponse.success -or @($zoneResponse.result).Count -ne 1) {
        throw 'Cloudflare zone lookup did not return exactly one zone'
    }
    $zoneID = $zoneResponse.result[0].id

    $encodedRecord = [System.Net.WebUtility]::UrlEncode($recordName)
    $stage = 'lookup_a_record'
    $aResponse = Invoke-RestMethod `
        -Uri "https://api.cloudflare.com/client/v4/zones/$zoneID/dns_records?type=A&name=$encodedRecord" `
        -Headers $headers
    $aRecords = @($aResponse.result)
    $aRecordMatches = (
        $aResponse.success -and
        $aRecords.Count -eq 1 -and
        $aRecords[0].content -eq $expectedIP -and
        $aRecords[0].proxied -eq $false
    )

    $challengeName = "_acme-challenge.$recordName"
    $encodedChallenge = [System.Net.WebUtility]::UrlEncode($challengeName)
    $stage = 'lookup_acme_txt'
    $txtResponse = Invoke-RestMethod `
        -Uri "https://api.cloudflare.com/client/v4/zones/$zoneID/dns_records?type=TXT&name=$encodedChallenge" `
        -Headers $headers
    $txtRecords = @($txtResponse.result)
    $txtClean = $txtResponse.success -and $txtRecords.Count -eq 0

    $stage = 'validate_dns_invariants'
    if (-not $aRecordMatches -or -not $txtClean) {
        throw 'Cloudflare DNS state did not match the deployment invariants'
    }
    Write-Output 'CLOUDFLARE_DNS=PASS a_record_matches_vps=true proxied=false acme_txt_records=0'
}
catch {
    throw "Cloudflare DNS verification failed at stage '$stage' without exposing credential material"
}
finally {
    $token = $null
    $headers = $null
}
