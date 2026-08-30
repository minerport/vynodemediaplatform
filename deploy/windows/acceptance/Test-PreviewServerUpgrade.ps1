param([Parameter(Mandatory)][string]$Msi,[Parameter(Mandatory)][string]$ExpectedSha256,[Parameter(Mandatory)][string]$ResultPath)
$ErrorActionPreference='Stop'
$result=[ordered]@{pass=$false;scope='Installed host test-server upgrade, not clean-VM acceptance'}
try {
    if ((Get-FileHash -LiteralPath $Msi).Hash -ne $ExpectedSha256) { throw 'Installer hash mismatch' }
    $before=Invoke-RestMethod 'http://127.0.0.1:8096/api/v1/system/info'
    $result.installationIdBefore=$before.instanceId
    $result.msiSha256=$ExpectedSha256
    $install=Start-Process msiexec.exe -ArgumentList @('/i',('"'+$Msi+'"'),'/qn','/norestart') -Wait -PassThru
    $result.installExitCode=$install.ExitCode
    if($install.ExitCode -notin 0,3010){throw "Installer exit $($install.ExitCode)"}
    $deadline=[DateTime]::UtcNow.AddSeconds(60)
    do {
        try {$ready=(Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:8096/ready' -TimeoutSec 3).StatusCode -eq 200}catch{$ready=$false}
        if(-not $ready){Start-Sleep -Seconds 2}
    } until ($ready -or [DateTime]::UtcNow -gt $deadline)
    if(-not $ready){throw 'Server did not become ready'}
    $after=Invoke-RestMethod 'http://127.0.0.1:8096/api/v1/system/info'
    $result.identityPreserved=$after.instanceId -eq $before.instanceId
    $service=Get-CimInstance Win32_Service -Filter "Name='VyNodeMediaServer'"
    $result.service=@{state=$service.State;account=$service.StartName;startMode=$service.StartMode}
    $tools=Join-Path $env:ProgramFiles 'VyNode\Media Server\tools\ffmpeg'
    $definition=Get-Content (Join-Path $tools 'manifest.json') -Raw | ConvertFrom-Json
    foreach($tool in 'ffmpeg','ffprobe') {
        $path=Join-Path $tools "$tool.exe"
        $hash=(Get-FileHash -LiteralPath $path).Hash
        if($hash -ne $definition."${tool}Sha256"){throw 'Installed tool hash mismatch'}
        $result[$tool]=@{sha256=$hash;version=(& $path -version | Select-Object -First 1)}
    }
    $result.ready=$true
    $result.pass=$result.identityPreserved -and $service.State -eq 'Running' -and $service.StartName -eq 'NT AUTHORITY\LocalService'
} catch { $result.error=$_.Exception.Message }
$result | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $ResultPath -Encoding UTF8
if(-not $result.pass){exit 1}
