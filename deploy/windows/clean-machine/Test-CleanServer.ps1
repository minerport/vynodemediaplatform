param([string]$BundleRoot=$PSScriptRoot,[string]$ResultPath=(Join-Path $PSScriptRoot 'server-result.json'),[switch]$OfflineIsolationConfirmed)
. (Join-Path $PSScriptRoot 'CleanMachine.Common.ps1')
$preflight=Get-CleanMachinePreflight
$result=[ordered]@{environment=[ordered]@{windows=(Get-CimInstance Win32_OperatingSystem | Select-Object Caption,Version,BuildNumber,OSArchitecture);preflight=$preflight};server=[ordered]@{};offline=[ordered]@{operatorConfirmed=[bool]$OfflineIsolationConfirmed};dependencies=[ordered]@{};artifacts=[ordered]@{}}
try {
    if(-not $preflight.pass){throw 'Developer-tool preflight failed; this is not a literal clean environment.'}
    if(-not $OfflineIsolationConfirmed){throw 'Disconnect Internet or place the VM on an isolated network, then rerun with -OfflineIsolationConfirmed.'}
    $msi=Assert-BundleArtifact $BundleRoot 'VyNode-Media-Server.msi'
    $media=Assert-BundleArtifact $BundleRoot 'vynode-clean-media.mp4'
    $result.artifacts.serverMsiSha256=(Get-FileHash $msi -Algorithm SHA256).Hash
    $result.artifacts.mediaSha256=(Get-FileHash $media -Algorithm SHA256).Hash
    $result.server.installExitCode=Invoke-Msi Install $msi (Join-Path $PSScriptRoot 'server-install.log')
    $service=Get-CimInstance Win32_Service -Filter "Name='VyNodeMediaServer'"
    $result.server.service=[ordered]@{present=$null-ne $service;state=$service.State;startMode=$service.StartMode;account=$service.StartName}
    $tools=Join-Path $env:ProgramFiles 'VyNode\Media Server\tools\ffmpeg'
    foreach($name in 'ffmpeg.exe','ffprobe.exe'){
        $path=Join-Path $tools $name
        if(-not(Test-Path -LiteralPath $path)){throw "Bundled tool missing: $path"}
        $result.server[$name]=[ordered]@{path=$path;sha256=(Get-FileHash $path -Algorithm SHA256).Hash;version=(& $path -version 2>&1 | Select-Object -First 1)}
    }
    $result.dependencies=[ordered]@{coreclrBundled=(Test-Path (Join-Path $env:ProgramFiles 'VyNode\Media Server\coreclr.dll'));ffmpegFromPath=[bool](Get-Command ffmpeg.exe -ErrorAction SilentlyContinue);ffprobeFromPath=[bool](Get-Command ffprobe.exe -ErrorAction SilentlyContinue);repositoryAbsent=-not(Test-Path (Join-Path $env:USERPROFILE 'VidStack'))}
    $result.server.programData=Test-Path (Join-Path $env:ProgramData 'VyNode\Media Server')
    $rule=Get-NetFirewallRule -DisplayName 'VyNode Media Server (Local network)' -ErrorAction SilentlyContinue
    $port=if($rule){$rule | Get-NetFirewallPortFilter}
    $result.server.firewall=[ordered]@{present=$null-ne $rule;direction=$rule.Direction;action=$rule.Action;protocol=$port.Protocol;localPort=$port.LocalPort}
    $result.server.ready=try{(Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:8096/ready' -TimeoutSec 15).StatusCode -eq 200}catch{$false}
    Write-Host "Synthetic fixture: $media. Configure a temporary acceptance library through Server Manager/Web Admin. Never enter credentials into this script."
    $result.server.serverManager=Confirm-Gate 'Server Manager reports the installed running service and opens Admin'
    $result.server.scan=Confirm-Gate 'The fixture scan reports container, codecs, streams, duration, resolution, and audio data via bundled FFprobe'
    $result.server.directPlay=Confirm-Gate 'Representative Direct Play succeeds'
    $result.server.transcode=Confirm-Gate 'A forced Direct Stream or transcode succeeds using bundled FFmpeg'
    $data=Join-Path $env:ProgramData 'VyNode\Media Server'
    $result.server.uninstallExitCode=Invoke-Msi Uninstall $msi (Join-Path $PSScriptRoot 'server-uninstall.log')
    $result.server.serviceRemoved=$null-eq(Get-Service VyNodeMediaServer -ErrorAction SilentlyContinue)
    $result.server.firewallRemoved=$null-eq(Get-NetFirewallRule -DisplayName 'VyNode Media Server (Local network)' -ErrorAction SilentlyContinue)
    $result.server.programDataRetained=Test-Path -LiteralPath $data
    $result.offline.pass=($result.server.installExitCode -in 0,3010) -and [bool]$result.server.'ffmpeg.exe' -and [bool]$result.server.'ffprobe.exe'
    $result.server.pass=$result.server.service.present -and $result.server.programData -and $result.server.firewall.present -and $result.server.ready -and $result.server.serverManager -and $result.server.scan -and $result.server.directPlay -and $result.server.transcode -and $result.server.serviceRemoved -and $result.server.firewallRemoved -and $result.server.programDataRetained -and $result.offline.pass
} catch { $result.server.pass=$false;$result.server.error=$_.Exception.Message;Save-FailureDiagnostics $PSScriptRoot 'server' }
Save-CleanResult $result $ResultPath
if(-not $result.server.pass){exit 1}
