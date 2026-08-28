# Windows troubleshooting

This guide applies to the VyNode Desktop client, VyNode Media Server Windows
service, and VyNode Server Manager. Current development packages are unsigned.

## Service does not start

1. Open VyNode Server Manager and confirm the installed version, service status,
   data path, and log path.
2. Use Restart once. If Windows requests elevation, approve it only from an
   interactive trusted session.
3. Inspect the server log under `%ProgramData%\VyNode\Media Server\logs` and the
   Windows Service Control Manager event log. Do not post logs publicly before
   checking them for account or media-path information.
4. Confirm TCP port 8096 is not already owned by another process and that the
   configured media/database paths are available to the service account.

The service runs independently of the Desktop client and does not require an
interactive user to be signed in. Removing or signing out of the client must not
be used as a service-control mechanism.

## LocalService and media permissions

The default service identity is Windows `LocalService`, selected to avoid
unnecessary administrative privilege. It does not automatically inherit the
interactive user's access to every file.

For local NTFS media, grant the service identity read and traverse access to the
specific library directories. Keep write access limited to locations that VyNode
must update. Do not make an entire drive or the server identity/configuration
world-writable to solve an access problem.

UNC and network-share access depends on both share permissions and the Windows
identity presented to the remote host. `LocalService` normally has no reusable
remote-user credentials. Phase 15 does not store network-share passwords. Use an
appropriately managed service account and explicit share/NTFS permissions where
UNC access is required; local NTFS libraries are the validated default.

## Firewall and TCP 8096

The server MSI owns an inbound TCP rule for port 8096, scoped to the installed
server executable and the intended local-network profiles/subnet. The Desktop
client MSI does not create this rule. Server uninstall removes only the rule
owned by that MSI.

If LAN clients cannot connect, confirm the server is healthy locally before
checking the installed rule. Do not replace it with an unrestricted Any/Any rule.

## Connect unavailable

VyNode Connect outage is not a logout. A previously linked client can use its
cached server profile and server-local credential after the expected server
identity is verified. Local Web Admin, local libraries, and local playback remain
available when the Media Server itself is reachable.

If no cached server relationship exists, use Account or Advanced connection
recovery. Manual address entry is a recovery/local-only path, not the normal
global-account flow.

## Credential and session recovery

Global and per-server refresh credentials are stored separately in Windows
Credential Locker. Never copy them into configuration files or support logs.

If a session can no longer refresh, sign in again through the appropriate account
flow. Explicit global sign-out applies the Phase 14 privacy policy and clears
account-linked server credentials; it does not stop the local Media Server
service. An identity mismatch must be resolved rather than bypassed.

## MSI installation logs

For diagnostic installation logging from an interactive elevated session, use a
specific log path, for example:

    msiexec.exe /i "VyNode-Desktop-unsigned.msi" /L*v "%TEMP%\vynode-desktop-msi.log"
    msiexec.exe /i "VyNode-Media-Server-unsigned.msi" /L*v "%TEMP%\vynode-server-msi.log"

Uninstall and maintenance operations can use the same `/L*v` option. MSI logs
may contain filesystem paths and user or machine names; review them before
sharing. Do not put credentials on an `msiexec` command line.

## SmartScreen and unsigned development builds

Current Phase 15 acceptance artifacts are labeled UNSIGNED DEVELOPMENT BUILD.
Windows may display SmartScreen or unknown-publisher warnings. Do not disable
SmartScreen, UAC, Defender, or signature policy to hide these warnings.

Production releases require externally supplied Authenticode signing material
and publisher reputation. Private certificates and passwords are never stored in
this repository.

## Data, configuration, and logs

Server persistent state is under `%ProgramData%\VyNode\Media Server`, including
the database, configuration, server identity, and logs. It is intentionally not
stored under Program Files. Normal server uninstall retains this directory and
never deletes media files; retained data can restore the server identity after a
reinstall.

Application binaries live in their installer-selected protected locations.
Server Manager reports the effective installed version and opens the configured
data and log folders so support does not need to guess paths.
