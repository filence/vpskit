[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$cleanupRoot = (Resolve-Path -LiteralPath (Join-Path $projectRoot '.build\client-configs')).Path.TrimEnd([IO.Path]::DirectorySeparatorChar)
$accepted = (Resolve-Path -LiteralPath (Join-Path $cleanupRoot 'accepted-lab31-20260720')).Path
$targetNames = @(
    'lab24-20260718-172610'
    'lab29-20260719-105200'
    'lab31-20260720-090122'
    'lab31-final-20260720-091433'
    'manual-test-20260720-091958'
)

$resolvedTargets = @()
[int64]$totalBytes = 0
foreach ($name in $targetNames) {
    $expected = Join-Path $cleanupRoot $name
    $resolved = (Resolve-Path -LiteralPath $expected).Path
    if ((Split-Path -Parent $resolved) -ne $cleanupRoot) {
        throw "Target escaped cleanup root: $resolved"
    }
    if ((Split-Path -Leaf $resolved) -cne $name) {
        throw "Unexpected target name: $resolved"
    }
    if ($resolved -eq $accepted) {
        throw 'Accepted config directory must not be deleted'
    }
    $files = Get-ChildItem -LiteralPath $resolved -Recurse -File
    $totalBytes += [int64](($files | Measure-Object Length -Sum).Sum)
    $resolvedTargets += $resolved
}

foreach ($target in $resolvedTargets) {
    Remove-Item -LiteralPath $target -Recurse -Force
}
foreach ($target in $resolvedTargets) {
    if (Test-Path -LiteralPath $target) {
        throw "Cleanup target still exists: $target"
    }
}

if (-not (Test-Path -LiteralPath $accepted -PathType Container)) {
    throw 'Accepted config directory is missing'
}
$acceptedNames = @(Get-ChildItem -LiteralPath $accepted -File | Sort-Object Name | Select-Object -ExpandProperty Name)
$expectedAccepted = @(
    'mihomo.yaml'
    'share-links.txt'
    'sing-box-hysteria2.json'
    'sing-box-reality.json'
) | Sort-Object
$differences = Compare-Object -ReferenceObject $expectedAccepted -DifferenceObject $acceptedNames
if ($differences) {
    throw 'Accepted config file set changed'
}

Write-Output "LOCAL_CLEANUP=PASS directories=5 bytes=$totalBytes"
Write-Output "ACCEPTED_CONFIGS=PRESERVED path=$accepted"
