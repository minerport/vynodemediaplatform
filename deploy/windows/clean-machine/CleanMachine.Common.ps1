$ErrorActionPreference = 'Stop'
function Get-BundleManifest([string]$Root) {
    $path=Join-Path $Root 'bundle-manifest.json'
    if(-not(Test-Path -LiteralPath $path)){throw "Missing bundle manifest: $path"}
    Get-Content -LiteralPath $path -Raw | ConvertFrom-Json
}
function Assert-BundleArtifact([string]$Root,[string]$Name) {
    $manifest=Get-BundleManifest $Root
    $entry=$manifest.artifacts | Where-Object name -eq $Name
    if(-not $entry){throw "Artifact is absent from bundle manifest: $Name"}
    $path=Join-Path $Root $Name
    if(-not(Test-Path -LiteralPath $path -PathType Leaf)){throw "Missing artifact: $path"}
    $actual=(Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash
    if($actual -ne $entry.sha256){throw "SHA-256 mismatch for $Name"}
    $path
}
function Get-CleanMachinePreflight {
    $commands=[ordered]@{}
    foreach($name in 'go.exe','node.exe','npm.cmd','wix.exe','ffmpeg.exe','ffprobe.exe','devenv.exe','adb.exe','studio64.exe'){
        $commands[$name]=[bool](Get-Command $name -ErrorAction SilentlyContinue)
    }
    $sdks=@()
    if(Get-Command dotnet.exe -ErrorAction SilentlyContinue){$sdks=@(& dotnet --list-sdks 2>$null)}
    $visualStudio=[bool](Get-ChildItem 'HKLM:\SOFTWARE\Microsoft\VisualStudio\SxS\VS7','HKLM:\SOFTWARE\WOW6432Node\Microsoft\VisualStudio\SxS\VS7' -ErrorAction SilentlyContinue)
    [ordered]@{commands=$commands;dotnetSdks=$sdks;visualStudio=$visualStudio;pass=(-not $visualStudio -and $sdks.Count -eq 0 -and -not($commands.Values -contains $true))}
}
function Invoke-Msi([ValidateSet('Install','Uninstall')][string]$Action,[string]$Target,[string]$LogPath) {
    $verb=if($Action -eq 'Install'){'/i'}else{'/x'}
    $process=Start-Process msiexec.exe -Verb RunAs -ArgumentList @($verb,('"'+$Target+'"'),'/qn','/norestart','/l*v',('"'+$LogPath+'"')) -Wait -PassThru
    if($process.ExitCode -notin 0,3010){throw "MSI $Action failed with exit code $($process.ExitCode). See $LogPath"}
    $process.ExitCode
}
function Confirm-Gate([string]$Prompt) { (Read-Host "$Prompt Type PASS only after direct observation") -ceq 'PASS' }
function Save-CleanResult([hashtable]$Result,[string]$Path) {
    $Result.completedAt=[DateTime]::UtcNow.ToString('o')
    $Result | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $Path -Encoding utf8
    Write-Host "Result written to $Path"
}
function Save-FailureDiagnostics([string]$Root,[string]$Component) {
    $since=(Get-Date).AddHours(-2)
    Get-WinEvent -FilterHashtable @{LogName=@('Application','System');StartTime=$since} -ErrorAction SilentlyContinue |
        Where-Object {$_.ProviderName -match 'MsiInstaller|Service Control Manager|VyNode'} |
        Select-Object TimeCreated,LogName,ProviderName,Id,LevelDisplayName,Message |
        ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $Root "$Component-event-log.json") -Encoding utf8
    if($Component -eq 'server'){
        $logs=Join-Path $env:ProgramData 'VyNode\Media Server\logs'
        if(Test-Path $logs){
            $destination=Join-Path $Root 'server-runtime-logs'
            New-Item -ItemType Directory -Force $destination | Out-Null
            Get-ChildItem $logs -File -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending | Select-Object -First 3 | Copy-Item -Destination $destination
        }
    }
}
