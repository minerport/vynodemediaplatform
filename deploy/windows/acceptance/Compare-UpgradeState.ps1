param([Parameter(Mandatory)][string]$Before, [Parameter(Mandatory)][string]$After)
$a = Get-Content -LiteralPath $Before -Raw | ConvertFrom-Json
$b = Get-Content -LiteralPath $After -Raw | ConvertFrom-Json
$checks = [ordered]@{
    readyAfter = [bool]$b.ready
    installationIdPreserved = $a.installationId -and ($a.installationId -eq $b.installationId)
    publicIdentityPreserved = $a.publicKeyFingerprint -and ($a.publicKeyFingerprint -eq $b.publicKeyFingerprint)
    databasePresent = [bool]$b.database.exists
    privateIdentityStillPresent = [bool]$b.privateIdentityPresent
}
$checks | ConvertTo-Json
if (@($checks.Values) -contains $false) { exit 1 }

