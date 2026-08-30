param(
    [ValidateSet('stable','beta')][string]$Channel = 'stable',
    [string]$Version = '16.0.0',
    [ValidateSet('development','production')][string]$ReleaseType = 'development',
    [string]$OutputDirectory,
    [string]$GeneratedAt,
    [string]$MediaToolsManifest
)
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $root 'artifacts\windows\release' }
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null
if (-not $GeneratedAt) { $GeneratedAt = (Get-Date).ToUniversalTime().ToString('o') }
if (-not $MediaToolsManifest) { $MediaToolsManifest = Join-Path $root 'deploy\windows\media-tools\manifest.json' }
$media = Get-Content -LiteralPath $MediaToolsManifest -Raw | ConvertFrom-Json
$desktopMsi=if($ReleaseType -eq 'production'){'VyNode-Desktop.msi'}else{'VyNode-Desktop-unsigned.msi'}
$serverMsi=if($ReleaseType -eq 'production'){'VyNode-Media-Server.msi'}else{'VyNode-Media-Server-unsigned.msi'}
$artifacts = @(
    @{Name=$desktopMsi; Path=(Join-Path $root "artifacts\windows\installer\$desktopMsi"); Kind='desktop-msi'},
    @{Name=$serverMsi; Path=(Join-Path $root "artifacts\windows\installer\$serverMsi"); Kind='server-msi'},
    @{Name='VyNode.exe'; Path=(Join-Path $root 'artifacts\windows\desktop\win-x64\VyNode.exe'); Kind='desktop'},
    @{Name='vynode-server.exe'; Path=(Join-Path $root 'artifacts\windows\vynode-server.exe'); Kind='server'},
    @{Name='VyNode.ServerManager.exe'; Path=(Join-Path $root 'artifacts\windows\server-manager\VyNode.ServerManager.exe'); Kind='server-manager'}
)
foreach ($artifact in $artifacts) {
    if (-not (Test-Path -LiteralPath $artifact.Path -PathType Leaf)) { throw "Missing release artifact: $($artifact.Path)" }
}

$entries = foreach ($artifact in $artifacts) {
    $signature = Get-AuthenticodeSignature -LiteralPath $artifact.Path
    $signed = $signature.Status -eq 'Valid'
    if ($ReleaseType -eq 'production' -and -not $signed) { throw "Production artifact is not Authenticode-valid: $($artifact.Name)" }
    [ordered]@{
        name=$artifact.Name; kind=$artifact.Kind; platform='windows'; architecture='x64'
        sha256=(Get-FileHash -LiteralPath $artifact.Path -Algorithm SHA256).Hash.ToLowerInvariant()
        authenticodeStatus=if($signed){'VALID'}else{'UNSIGNED DEVELOPMENT BUILD'}
        signerSubject=if($signed){$signature.SignerCertificate.Subject}else{$null}
        signerThumbprint=if($signed){$signature.SignerCertificate.Thumbprint}else{$null}
    }
}

$sbomName = "vynode-$Version-windows-x64.cdx.json"
$authenticodeReport = Join-Path $OutputDirectory 'authenticode-verification.json'
$updateManifest = Join-Path $OutputDirectory 'update\manifest.json'
$updateSignature = Join-Path $OutputDirectory 'update\manifest.sig'
if ($ReleaseType -eq 'production') {
    foreach ($required in $authenticodeReport,$updateManifest,$updateSignature) {
        if (-not (Test-Path -LiteralPath $required -PathType Leaf)) { throw "Production release metadata is missing post-signing evidence: $required" }
    }
}
$manifest = [ordered]@{
    schemaVersion=1; product='VyNode Media'; version=$Version; channel=$Channel
    releaseType=if($ReleaseType -eq 'production'){'PRODUCTION RELEASE'}else{'UNSIGNED DEVELOPMENT BUILD'}
    platform='windows'; architecture='x64'; generatedAt=$GeneratedAt
    updateSigningKeyId=if($env:WINDOWS_UPDATE_PUBLIC_KEY_ID){$env:WINDOWS_UPDATE_PUBLIC_KEY_ID}else{'UNCONFIGURED-DEVELOPMENT'}
    authenticodeVerification=if(Test-Path $authenticodeReport){[IO.Path]::GetFileName($authenticodeReport)}else{$null}
    updateManifest=if(Test-Path $updateManifest){'update/manifest.json'}else{$null}
    mediaTools=[ordered]@{version=$media.version; distribution=$media.distribution; license=$media.license; archiveSha256=$media.archiveSha256; ffmpegSha256=$media.ffmpegSha256; ffprobeSha256=$media.ffprobeSha256; relationship='CONTAINED_BY server-msi'}
    sbom=$sbomName; notices='THIRD_PARTY_NOTICES.md'; artifacts=$entries
}
$manifestPath = Join-Path $OutputDirectory 'release-manifest.json'
$manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $manifestPath -Encoding utf8NoBOM

$components = [Collections.Generic.List[object]]::new()
$componentKeys = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
function Add-Component([hashtable]$Component) {
    $key = "$($Component.type):$($Component.name):$($Component.version)"
    if ($componentKeys.Add($key)) { $components.Add($Component) }
}
Add-Component ([ordered]@{type='application';name='VyNode Desktop';version=$Version})
Add-Component ([ordered]@{type='application';name='VyNode Media Server';version=$Version})
Add-Component ([ordered]@{type='application';name='VyNode Server Manager';version=$Version})
Add-Component ([ordered]@{type='application';name='FFmpeg';version=$media.version;licenses=@(@{license=@{id='GPL-3.0-or-later'}});hashes=@(@{alg='SHA-256';content=$media.ffmpegSha256});externalReferences=@(@{type='vcs';url=$media.sourceUrl})})
Add-Component ([ordered]@{type='application';name='FFprobe';version=$media.version;licenses=@(@{license=@{id='GPL-3.0-or-later'}});hashes=@(@{alg='SHA-256';content=$media.ffprobeSha256});externalReferences=@(@{type='vcs';url=$media.sourceUrl})})
[xml]$clientProject = Get-Content (Join-Path $root 'apps\windows\VyNode.Windows\VyNode.Windows.csproj')
[xml]$managerProject = Get-Content (Join-Path $root 'apps\windows\VyNode.ServerManager\VyNode.ServerManager.csproj')
@($clientProject.Project.ItemGroup.PackageReference)+@($managerProject.Project.ItemGroup.PackageReference) | Where-Object { $_.Include } | ForEach-Object {
    Add-Component ([ordered]@{type='library';name=[string]$_.Include;version=[string]$_.Version;purl="pkg:nuget/$([uri]::EscapeDataString([string]$_.Include))@$($_.Version)"})
}
Get-ChildItem (Join-Path $root 'deploy\windows\installer') -Filter *.wixproj -Recurse | ForEach-Object {
    [xml]$project = Get-Content $_.FullName
    @($project.Project.ItemGroup.PackageReference) | Where-Object { $_.Include } | ForEach-Object {
        Add-Component ([ordered]@{type='library';name=[string]$_.Include;version=[string]$_.Version;purl="pkg:nuget/$([uri]::EscapeDataString([string]$_.Include))@$($_.Version)"})
    }
}
$goMod = Get-Content (Join-Path $root 'go.mod')
$goMod | Where-Object { $_ -match '^\s*([^\s]+)\s+(v[^\s]+)(?:\s+// indirect)?\s*$' } | ForEach-Object {
    Add-Component ([ordered]@{type='library';name=$Matches[1];version=$Matches[2];purl="pkg:golang/$([uri]::EscapeDataString($Matches[1]))@$($Matches[2])"})
}
$lock = Get-Content (Join-Path $root 'package-lock.json') -Raw | ConvertFrom-Json -AsHashtable
foreach ($entry in $lock.packages.GetEnumerator()) {
    if ($entry.Key -notlike 'node_modules/*' -or -not $entry.Value.version) { continue }
    $name = $entry.Key.Substring('node_modules/'.Length)
    Add-Component ([ordered]@{type='library';name=$name;version=[string]$entry.Value.version;purl="pkg:npm/$([uri]::EscapeDataString($name))@$($entry.Value.version)"})
}
$identityBytes = [Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes("VyNode:${Version}:${Channel}:windows:x64"))[0..15]
$sbom = [ordered]@{bomFormat='CycloneDX';specVersion='1.5';serialNumber="urn:uuid:$([guid]::new([byte[]]$identityBytes))";version=1;metadata=@{timestamp=$GeneratedAt;component=@{type='application';name='VyNode Media for Windows';version=$Version}};components=$components}
$sbom | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $OutputDirectory $sbomName) -Encoding utf8NoBOM
Copy-Item -LiteralPath (Join-Path $root 'THIRD_PARTY_NOTICES.md') -Destination $OutputDirectory -Force

$checksumTargets = @($artifacts.Path)+@($manifestPath,(Join-Path $OutputDirectory $sbomName),(Join-Path $OutputDirectory 'THIRD_PARTY_NOTICES.md'))
foreach ($optional in $authenticodeReport,$updateManifest,$updateSignature) { if(Test-Path -LiteralPath $optional){$checksumTargets += $optional} }
$outputRoot=[IO.Path]::GetFullPath($OutputDirectory)
$checksumTargets | ForEach-Object {
    $absolute=[IO.Path]::GetFullPath($_)
    $name=if($absolute.StartsWith($outputRoot+[IO.Path]::DirectorySeparatorChar)){[IO.Path]::GetRelativePath($outputRoot,$absolute).Replace('\','/')}else{[IO.Path]::GetFileName($absolute)}
    "$(Get-FileHash -LiteralPath $absolute -Algorithm SHA256 | Select-Object -ExpandProperty Hash)  $name"
} | Set-Content -LiteralPath (Join-Path $OutputDirectory 'SHA256SUMS.txt') -Encoding ascii
Write-Host "Created $($manifest.releaseType) release metadata in $OutputDirectory"
