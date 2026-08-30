param(
    [Parameter(Mandatory)][string]$ServerUrl,
    [Parameter(Mandatory)][string]$PublicServerUrl
)
$ErrorActionPreference='Stop'
$connect='https://connect.vynodehub.com'
$ServerUrl=$ServerUrl.TrimEnd('/')
$PublicServerUrl=$PublicServerUrl.TrimEnd('/')
foreach($target in @($ServerUrl,$PublicServerUrl)) {
    $uri=[uri]$target
    if(-not $uri.IsAbsoluteUri -or $uri.UserInfo -or $uri.Query -or $uri.Fragment -or $uri.AbsolutePath -ne '/') { throw 'Use an origin URL without credentials, path, query, or fragment.' }
    if($uri.Scheme -ne 'https' -and -not($uri.Scheme -eq 'http' -and $uri.IsLoopback -and $target -eq $ServerUrl)) { throw 'HTTPS is required except for local loopback administration.' }
}
if(([uri]$PublicServerUrl).Scheme -ne 'https'){throw 'The advertised endpoint must use trusted HTTPS.'}
$localToken=$null; $globalToken=$null; $stage='identity'
function Request($Base,$Path,$Method='GET',$Body=$null,$Token=$null) {
    $headers=@{'X-VyNode-Client'='native'}
    if($Token){$headers.Authorization='Bearer '+$Token}
    $requestOptions=@{Uri=($Base+$Path);Method=$Method;Headers=$headers;MaximumRedirection=0;TimeoutSec=30;ErrorAction='Stop'}
    if($null-ne $Body){$requestOptions.Body=ConvertTo-Json -InputObject $Body -Depth 10 -Compress;$requestOptions.ContentType='application/json'}
    Invoke-RestMethod @requestOptions
}
function Login($Base,$Global) {
    $name=Read-Host $(if($Global){'VyNode Connect username'}else{'Local server OWNER username'})
    $password=Read-Host 'Password (hidden)' -AsSecureString
    $credential=[PSCredential]::new($name,$password)
    $body=@{username=$name;password=$credential.GetNetworkCredential().Password}
    $device=@{name='VyNode preview enrollment';platform='WINDOWS';clientName='VyNode enrollment';clientVersion='16.0.3-preview.1'}
    if($Global){$body.deviceName=$device.name;$body.platform=$device.platform;$body.clientName=$device.clientName;$body.clientVersion=$device.clientVersion}else{$body.device=$device}
    try { Request $Base $(if($Global){'/api/v1/account/login'}else{'/api/v1/auth/login'}) 'POST' $body }
    finally {$body.password=$null;$credential=$null;$password.Dispose()}
}
try {
    $identity=Request $ServerUrl '/api/v1/system/info'
    $publicIdentity=Request $PublicServerUrl '/api/v1/system/info'
    $id=[string]$identity.instanceId
    if(-not $id -or $publicIdentity.instanceId -ne $id){throw 'Endpoints identify different servers.'}
    Write-Host "Server: $($identity.serverName); installation ID: $id"
    Write-Host 'Passwords and tokens are held only for these requests. Do not run with PowerShell transcription or debug tracing.'
    $stage='local-owner-login';$local=Login $ServerUrl $false;$localToken=$local.accessToken
    if($local.user.role -ne 'OWNER'){throw 'Local OWNER required.'}
    $stage='existing-trust';$settings=Request $ServerUrl '/api/v1/connect/settings' 'GET' $null $localToken
    if($settings.enabled -and $settings.connectUrl.TrimEnd('/') -ne $connect){throw 'A different Connect installation is configured; migration is not performed by this helper.'}
    $stage='global-login';$global=Login $connect $true;$globalToken=$global.accessToken
    Write-Host "Global owner account: $($global.account.username)"
    if((Read-Host 'Type LINK to authorize linking this server to that account') -cne 'LINK'){throw 'Cancelled.'}
    $stage='configure-trust';$keys=Request $connect '/.well-known/vynode-connect-keys'
    if(@($keys.keys).Count -eq 0){throw 'No public trust keys.'}
    Request $ServerUrl '/api/v1/connect/settings' 'PUT' @{enabled=$true;connectUrl=$connect;issuer=$connect;signingKeys=$keys} $localToken | Out-Null
    $stage='claim';$claim=Request $ServerUrl '/api/v1/connect/server/claim' 'POST' @{} $localToken
    $claimed=Request $connect '/api/v1/servers/claim/complete' 'POST' @{challenge=$claim.challenge} $globalToken
    if($claimed.id -ne $id){throw 'Claim identity mismatch.'}
    $stage='owner-link';$request=Request $ServerUrl '/api/v1/connect/link/request' 'POST' @{} $localToken
    $grant=Request $connect "/api/v1/servers/$id/link-grant" 'POST' @{state=$request.state} $globalToken
    Request $ServerUrl '/api/v1/connect/link/complete' 'POST' @{state=$request.state;grant=$grant.grant} $localToken | Out-Null
    $stage='advertise-endpoint';Request $ServerUrl '/api/v1/connect/server/endpoint' 'PUT' @{url=$PublicServerUrl;kind='secure'} $localToken | Out-Null
    $stage='verify-discovery';$servers=Request $connect '/api/v1/servers' 'GET' $null $globalToken
    $matched=@($servers | Where-Object id -eq $id)
    if($matched.Count -ne 1 -or $matched[0].relationship -ne 'OWNER' -or $PublicServerUrl -notin $matched[0].endpoints.url){throw 'Discovery verification failed.'}
    Write-Host 'Enrollment and owner discovery passed. Sign in on Windows or Android to test playback.'
} catch {
    Write-Host "Stopped at $stage. Response bodies and credentials are not printed. Earlier linking steps may have succeeded; inspect state before retrying."
    exit 1
} finally {
    if($localToken){try{Request $ServerUrl '/api/v1/auth/logout' 'POST' @{} $localToken | Out-Null}catch{}}
    if($globalToken){try{Request $connect '/api/v1/account/logout' 'POST' @{} $globalToken | Out-Null}catch{}}
    $localToken=$null;$globalToken=$null;$local=$null;$global=$null;$claim=$null;$grant=$null;$request=$null
}
