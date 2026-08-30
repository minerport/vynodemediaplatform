param(
    [string]$Destination,
    [switch]$UseCache,
    [string]$DefinitionPath,
    [string]$ArchivePath
)
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
if (-not $DefinitionPath) { $DefinitionPath = Join-Path $PSScriptRoot 'manifest.json' }
$definition = Get-Content -LiteralPath $DefinitionPath -Raw | ConvertFrom-Json
if (-not $Destination) { $Destination = Join-Path $root 'artifacts\windows\media-tools\payload' }
$cache = Join-Path $root "artifacts\windows\media-tools\$($definition.archive)"

function Assert-Hash([string]$Path, [string]$Expected, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "$Label is missing: $Path" }
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected.ToLowerInvariant()) { throw "$Label SHA-256 mismatch. Expected $Expected; got $actual." }
}

New-Item -ItemType Directory -Force -Path (Split-Path $cache) | Out-Null
if ($ArchivePath) { $cache = (Resolve-Path -LiteralPath $ArchivePath).Path }
elseif (-not $UseCache -or -not (Test-Path -LiteralPath $cache)) {
    Invoke-WebRequest -UseBasicParsing -Uri $definition.archiveUrl -OutFile $cache
}
Assert-Hash $cache $definition.archiveSha256 'Media-tools archive'

$extract = Join-Path $root ('artifacts\windows\media-tools\extract-' + [guid]::NewGuid().ToString('N'))
Expand-Archive -LiteralPath $cache -DestinationPath $extract
$ffmpegFiles = @(Get-ChildItem -LiteralPath $extract -Recurse -Filter ffmpeg.exe)
$ffprobeFiles = @(Get-ChildItem -LiteralPath $extract -Recurse -Filter ffprobe.exe)
$readmeFiles = @(Get-ChildItem -LiteralPath $extract -Recurse -Filter README.txt)
$licenseFiles = @(Get-ChildItem -LiteralPath $extract -Recurse -Filter LICENSE)
if ($ffmpegFiles.Count -ne 1 -or $ffprobeFiles.Count -ne 1 -or $readmeFiles.Count -ne 1 -or $licenseFiles.Count -ne 1) {
    throw 'Approved media-tools archive does not have the expected single FFmpeg/FFprobe/license layout.'
}
$ffmpeg, $ffprobe, $readme, $license = $ffmpegFiles[0], $ffprobeFiles[0], $readmeFiles[0], $licenseFiles[0]
Assert-Hash $ffmpeg.FullName $definition.ffmpegSha256 'FFmpeg'
Assert-Hash $ffprobe.FullName $definition.ffprobeSha256 'FFprobe'
$ffmpegVersion = (& $ffmpeg.FullName -hide_banner -version | Select-Object -First 1)
$ffprobeVersion = (& $ffprobe.FullName -hide_banner -version | Select-Object -First 1)
if ($ffmpegVersion -notmatch [regex]::Escape($definition.version) -or $ffprobeVersion -notmatch [regex]::Escape($definition.version)) {
    throw "FFmpeg and FFprobe do not match approved version $($definition.version)."
}

New-Item -ItemType Directory -Force -Path $Destination | Out-Null
Copy-Item -LiteralPath $ffmpeg.FullName,$ffprobe.FullName,$readme.FullName,$license.FullName -Destination $Destination -Force
Copy-Item -LiteralPath $DefinitionPath -Destination (Join-Path $Destination 'manifest.json') -Force
$notices = Join-Path $readme.DirectoryName 'notices'
if (Test-Path -LiteralPath $notices -PathType Container) { Copy-Item -LiteralPath $notices -Destination $Destination -Recurse -Force }
Write-Host "Prepared verified $($definition.version) media tools at $Destination"
