param([string]$OutputDirectory,[string]$GeneratedAt,[switch]$Production)
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
if(-not $OutputDirectory){$OutputDirectory=Join-Path $root 'artifacts\windows\clean-machine-bundle'}
if(-not $GeneratedAt){$GeneratedAt=[DateTime]::UtcNow.ToString('o')}
if((Test-Path -LiteralPath $OutputDirectory) -and (Get-ChildItem -LiteralPath $OutputDirectory -Force | Select-Object -First 1)){throw "Clean-machine output directory must be absent or empty: $OutputDirectory"}
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null
$desktopRelative=if($Production){'artifacts\windows\installer\VyNode-Desktop.msi'}else{'artifacts\windows\installer\VyNode-Desktop-unsigned.msi'}
$serverRelative=if($Production){'artifacts\windows\installer\VyNode-Media-Server.msi'}else{'artifacts\windows\installer\VyNode-Media-Server-unsigned.msi'}
$desktopSource=Join-Path $root $desktopRelative
$serverSource=Join-Path $root $serverRelative
if($Production){foreach($p in $desktopSource,$serverSource){if(-not(Test-Path -LiteralPath $p -PathType Leaf) -or (Get-AuthenticodeSignature $p).Status -ne 'Valid'){throw "Production clean-machine bundle requires Authenticode-valid MSI: $p"}}}
$inputs=@{
    'VyNode-Desktop.msi'=$desktopSource
    'VyNode-Media-Server.msi'=$serverSource
}
foreach($entry in $inputs.GetEnumerator()){if(-not(Test-Path $entry.Value)){throw "Missing artifact: $($entry.Value)"};Copy-Item -LiteralPath $entry.Value -Destination (Join-Path $OutputDirectory $entry.Key) -Force}
$ffmpeg=Join-Path $root 'artifacts\windows\media-tools\payload\ffmpeg.exe'
if(-not(Test-Path $ffmpeg)){throw 'Approved managed FFmpeg payload is required to generate the synthetic fixture.'}
$media=Join-Path $OutputDirectory 'vynode-clean-media.mp4'
& $ffmpeg -hide_banner -loglevel error -f lavfi -i 'testsrc2=size=640x360:rate=24' -f lavfi -i 'sine=frequency=880:sample_rate=48000' -t 8 -threads 1 -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -b:a 96k -metadata title='VyNode Synthetic Acceptance Fixture' -metadata creation_time='2026-01-01T00:00:00Z' -movflags +faststart -y $media
if($LASTEXITCODE -ne 0){throw 'Synthetic media generation failed.'}
foreach($name in 'CleanMachine.Common.ps1','Test-CleanClient.ps1','Test-CleanServer.ps1','README.md'){Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination $OutputDirectory -Force}
$artifacts=Get-ChildItem $OutputDirectory -File | Where-Object Name -notin @('bundle-manifest.json','client-result.json','server-result.json') | ForEach-Object {[ordered]@{name=$_.Name;sha256=(Get-FileHash $_.FullName -Algorithm SHA256).Hash;size=$_.Length}}
$releaseType=if($Production){'PRODUCTION SIGNED'}else{'UNSIGNED DEVELOPMENT PREPARATION'}
[ordered]@{schemaVersion=1;generatedAt=$GeneratedAt;purpose='Literal clean-Windows Phase 16 acceptance only';releaseType=$releaseType;selfContainedClient=$true;offlineServerPayload=$true;artifacts=$artifacts} | ConvertTo-Json -Depth 5 | Set-Content (Join-Path $OutputDirectory 'bundle-manifest.json') -Encoding utf8
Write-Host "Created ignored clean-machine bundle at $OutputDirectory"
