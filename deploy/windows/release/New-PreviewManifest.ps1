param([Parameter(Mandatory)][string]$Directory,[Parameter(Mandatory)][string]$SourceCommit)
$ErrorActionPreference='Stop'
if($SourceCommit -notmatch '^[a-f0-9]{40}$'){throw 'Use the complete source commit ID'}
$files=@(Get-ChildItem -LiteralPath $Directory -File | Where-Object Name -notin 'release-manifest.json','SHA256SUMS.txt')
foreach($extension in '.msi','.apk','.tar'){if(-not($files | Where-Object Extension -eq $extension)){throw "Missing primary artifact type $extension"}}
$entries=foreach($file in $files){
    if($file.Extension -in '.db','.pfx','.p12','.jks','.keystore','.log'){throw 'Prohibited artifact type'}
    [ordered]@{name=$file.Name;size=$file.Length;sha256=(Get-FileHash -LiteralPath $file.FullName).Hash.ToLowerInvariant();signing=if($file.Extension -eq '.msi'){(Get-AuthenticodeSignature -LiteralPath $file.FullName).Status.ToString()}elseif($file.Extension -eq '.apk'){'Android debug certificate; verified separately'}else{'not Authenticode signed'}}
}
$manifest=[ordered]@{
    schemaVersion=1;version='16.0.3-preview.1';releaseTag='preview-16.0.3.1';releaseType='TESTING PRERELEASE - NOT PRODUCTION CERTIFIED'
    sourceCommit=$SourceCommit;connectUrl='https://connect.vynodehub.com';windowsVersion='16.0.3';androidVersionCode=160003
    containerImage='vynode-media:16.0.3-preview.1';containerDistribution='docker load archive; no registry auto-update'
    mediaTools=@{windows='9.0-vynode-source-preview';linuxDebian='7:5.1.9-0+deb12u1'}
    limitations=@('No production Authenticode','Android debug signing','Manual updates','New servers require owner setup and Connect linking','Not a full Phase 16 acceptance PASS')
    artifacts=$entries
}
$manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $Directory 'release-manifest.json') -Encoding utf8
Get-ChildItem -LiteralPath $Directory -File | Where-Object Name -ne 'SHA256SUMS.txt' | Sort-Object Name | ForEach-Object {
    "$( (Get-FileHash -LiteralPath $_.FullName).Hash.ToLowerInvariant())  $($_.Name)"
} | Set-Content -LiteralPath (Join-Path $Directory 'SHA256SUMS.txt') -Encoding ascii
