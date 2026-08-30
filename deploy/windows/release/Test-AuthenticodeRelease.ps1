param(
    [string]$ExpectedSignerSubject = $env:WINDOWS_SIGNING_EXPECTED_SUBJECT,
    [string]$ExpectedSignerThumbprint = $env:WINDOWS_SIGNING_EXPECTED_THUMBPRINT,
    [string]$OutputPath,
    [switch]$UseUnsignedDevelopmentTargets
)
$ErrorActionPreference = 'Stop'
if (-not $ExpectedSignerSubject) { throw 'ExpectedSignerSubject is required.' }
if (-not $ExpectedSignerThumbprint) { throw 'ExpectedSignerThumbprint is required.' }
$expectedThumbprint = ($ExpectedSignerThumbprint -replace '\s','').ToUpperInvariant()
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
if (-not $OutputPath) { $OutputPath = Join-Path $root 'artifacts\windows\release\authenticode-verification.json' }
$desktopMsi=if($UseUnsignedDevelopmentTargets){'VyNode-Desktop-unsigned.msi'}else{'VyNode-Desktop.msi'}
$serverMsi=if($UseUnsignedDevelopmentTargets){'VyNode-Media-Server-unsigned.msi'}else{'VyNode-Media-Server.msi'}
$targets = @(
    (Join-Path $root 'artifacts\windows\desktop\win-x64\VyNode.exe'),
    (Join-Path $root 'artifacts\windows\vynode-server.exe'),
    (Join-Path $root 'artifacts\windows\server-manager\VyNode.ServerManager.exe'),
    (Join-Path $root "artifacts\windows\installer\$desktopMsi"),
    (Join-Path $root "artifacts\windows\installer\$serverMsi")
)
function Resolve-SignTool {
    $command=Get-Command signtool.exe -ErrorAction SilentlyContinue
    if($command){return $command.Source}
    $kits=Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
    $candidate=Get-ChildItem $kits -Filter signtool.exe -File -Recurse -ErrorAction SilentlyContinue | Where-Object FullName -match '\\x64\\signtool\.exe$' | Sort-Object FullName -Descending | Select-Object -First 1
    if(-not $candidate){
        $nuget=Join-Path $env:USERPROFILE '.nuget\packages\microsoft.windows.sdk.buildtools'
        $candidate=Get-ChildItem $nuget -Filter signtool.exe -File -Recurse -ErrorAction SilentlyContinue | Where-Object FullName -match '\\x64\\signtool\.exe$' | Sort-Object FullName -Descending | Select-Object -First 1
    }
    if(-not $candidate){throw 'Windows SDK signtool.exe was not found.'}
    $candidate.FullName
}
$signtool = Resolve-SignTool
$results = foreach ($target in $targets) {
    if (-not (Test-Path -LiteralPath $target -PathType Leaf)) { throw "Missing signing target: $target" }
    $signature = Get-AuthenticodeSignature -LiteralPath $target
    $nativeOutput = (& $signtool verify /pa /all /v $target 2>&1 | Out-String)
    $nativeExit = $LASTEXITCODE
    $digest = if ($nativeOutput -match '(?im)Hash of file \(([^)]+)\)') { $Matches[1].ToUpperInvariant() } else { $null }
    $timestamp = $signature.TimeStamperCertificate
    $item = [ordered]@{
        file=[IO.Path]::GetFileName($target)
        sha256=(Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash
        signaturePresent=$null -ne $signature.SignerCertificate
        signatureStatus=[string]$signature.Status
        signerSubject=$signature.SignerCertificate.Subject
        signerThumbprint=$signature.SignerCertificate.Thumbprint
        digestAlgorithm=$digest
        timestampPresent=$null -ne $timestamp
        timestampSubject=$timestamp.Subject
        timestampThumbprint=$timestamp.Thumbprint
        timestampNotBefore=if($timestamp){$timestamp.NotBefore.ToUniversalTime().ToString('o')}else{$null}
        timestampNotAfter=if($timestamp){$timestamp.NotAfter.ToUniversalTime().ToString('o')}else{$null}
        windowsTrustExitCode=$nativeExit
        windowsTrustValid=$nativeExit -eq 0
    }
    if ($signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid) { throw "Authenticode is not valid for ${target}: $($signature.Status)" }
    if ($signature.SignerCertificate.Subject -notlike "*$ExpectedSignerSubject*") { throw "Unexpected signer subject for $target." }
    if (($signature.SignerCertificate.Thumbprint -replace '\s','').ToUpperInvariant() -ne $expectedThumbprint) { throw "Unexpected signer thumbprint for $target." }
    if ($digest -ne 'SHA256') { throw "Expected SHA256 file digest for $target; found '$digest'." }
    if (-not $timestamp) { throw "Trusted timestamp is missing for $target." }
    if ($nativeExit -ne 0) { throw "Windows trust verification failed for $target.`n$nativeOutput" }
    [pscustomobject]$item
}
$document = [ordered]@{generatedAt=[DateTime]::UtcNow.ToString('o');policy='Windows Authenticode /pa, SHA256 digest, trusted RFC3161 timestamp';artifacts=$results}
New-Item -ItemType Directory -Force (Split-Path $OutputPath) | Out-Null
$document | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $OutputPath -Encoding utf8NoBOM
$document | ConvertTo-Json -Depth 6
