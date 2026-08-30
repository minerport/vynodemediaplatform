param([Parameter(Mandatory)][string]$Destination)
$ErrorActionPreference = 'Stop'
$destinationPath = [IO.Path]::GetFullPath($Destination)
if (Test-Path -LiteralPath $destinationPath) { throw 'Choose a new directory; existing contents will not be overwritten.' }
$lock = Get-Content (Join-Path $PSScriptRoot 'sources.lock.json') -Raw | ConvertFrom-Json
New-Item -ItemType Directory -Path (Join-Path $destinationPath 'sources') -Force | Out-Null
foreach ($name in @('Dockerfile','build.sh','cross.ini','sources.lock.json','README.md')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination $destinationPath
}
foreach ($source in $lock.sources) {
    $file = Join-Path $destinationPath "sources/$($source.name).tar.gz"
    Invoke-WebRequest -UseBasicParsing -Uri $source.url -OutFile $file
    if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash -ne $source.sha256) { throw "Source hash mismatch: $($source.name). Do not build this context." }
}
Write-Host "Verified source context prepared at $destinationPath. Candidate only; not release approval."
