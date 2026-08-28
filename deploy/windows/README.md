# Native Windows foundation

VyNode Server is a normal Go executable and does not require Docker. With Go 1.24+:

```powershell
./deploy/windows/build.ps1
$env:VYNODE_CONFIG_DIR = "$env:ProgramData\VyNode Media"
./artifacts/windows/vynode-server.exe
```

The process handles Ctrl+C in console mode and native Service Control Manager
stop/shutdown requests in service mode through the same graceful shutdown path.
The service installer defaults to `NT AUTHORITY\LocalService`, stores persistent
state below `%ProgramData%\VyNode\Media Server`, and retains that state on normal
uninstall and upgrade.

The server MSI also installs VyNode Server Manager. It reports the installed
version and SCM state, starts/stops/restarts through normal Windows elevation,
opens the local Web Admin, and opens the documented data and log folders. It does
not duplicate Web administration or access server credentials/private keys.

Build the unsigned client and server installers independently:

```powershell
./deploy/windows/build-client.ps1
./deploy/windows/build-server-installer.ps1
```

Separate MSIs are intentional: a viewer can install VyNode Desktop without the
server, and removing the desktop client cannot remove a local media server.
Production publishing requires Authenticode signing; no development signing key
is stored in this repository.
