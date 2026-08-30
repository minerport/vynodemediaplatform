param([Parameter(Mandatory)][string]$BinaryDirectory, [Parameter(Mandatory)][string]$EvidenceDirectory)
$ErrorActionPreference = 'Stop'
$ffmpeg = (Resolve-Path (Join-Path $BinaryDirectory 'ffmpeg.exe')).Path
$ffprobe = (Resolve-Path (Join-Path $BinaryDirectory 'ffprobe.exe')).Path
New-Item -ItemType Directory -Force -Path $EvidenceDirectory | Out-Null
$output = (Resolve-Path $EvidenceDirectory).Path
function Run-Media([string[]]$Arguments) {
    & $ffmpeg -hide_banner -loglevel error -nostdin -y @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Synthetic media operation failed: $LASTEXITCODE" }
}
$inputFile = Join-Path $output 'synthetic input [spaces].mp4'
Run-Media @('-f','lavfi','-i','testsrc2=size=640x360:rate=24','-f','lavfi','-i','sine=frequency=440:sample_rate=48000','-t','3','-c:v','libx264','-pix_fmt','yuv420p','-c:a','aac',$inputFile)
$probeJson = & $ffprobe -v error -show_format -show_streams -of json $inputFile
if ($LASTEXITCODE -ne 0) { throw 'FFprobe failed' }
$probe = ($probeJson -join "`n") | ConvertFrom-Json
if (@($probe.streams | Where-Object { $_.codec_name -eq 'h264' -and $_.width -eq 640 }).Count -ne 1 -or @($probe.streams | Where-Object codec_name -eq 'aac').Count -ne 1) { throw 'Incorrect probe metadata' }
Run-Media @('-i',$inputFile,'-map','0','-c','copy', (Join-Path $output 'remux.mkv'))
Run-Media @('-i',$inputFile,'-c:v','copy','-c:a','aac','-b:a','96k',(Join-Path $output 'audio-transcode.mp4'))
Run-Media @('-i',$inputFile,'-vf','scale=320:180','-c:v','libx264','-c:a','aac','-f','hls','-hls_time','1','-hls_list_size','0',(Join-Path $output 'video.m3u8'))
Run-Media @('-ss','1','-i',$inputFile,'-t','1','-f','null','-')
$subtitle = Join-Path $PSScriptRoot 'synthetic-subtitle.srt'
Run-Media @('-i',$inputFile,'-i',$subtitle,'-map','0','-map','1','-c','copy',(Join-Path $output 'embedded-subtitle.mkv'))
Run-Media @('-i',(Join-Path $output 'embedded-subtitle.mkv'),'-map','0:s:0','-c:s','webvtt',(Join-Path $output 'subtitle.vtt'))
if ((Get-Content (Join-Path $output 'subtitle.vtt') -Raw) -notmatch 'Synthetic subtitle') { throw 'Subtitle extraction failed' }
$result = [ordered]@{
    scope = 'Host synthetic source-candidate smoke; not installed or clean-machine acceptance'
    ffmpegSha256 = (Get-FileHash $ffmpeg).Hash
    ffprobeSha256 = (Get-FileHash $ffprobe).Hash
    probe = $true; remux = $true; audioTranscode = $true; videoHls = $true; seekDecode = $true; subtitleExtraction = $true
    completedUtc = [DateTime]::UtcNow.ToString('o')
}
$result | ConvertTo-Json | Set-Content (Join-Path $output 'result.json') -Encoding UTF8
$result | ConvertTo-Json
