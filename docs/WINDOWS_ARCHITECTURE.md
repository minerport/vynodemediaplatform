# Windows architecture

## Phase 15 starting point

Before Phase 15 the repository supplied one Windows artifact: the headless Go
media-server cross-build produced by `deploy/windows/build.ps1` and CI. The same
server entry point used on Linux handled console shutdown signals and already had
Windows-specific process and disk helpers. There was no native client, service
host integration, installer, Windows secure credential implementation, desktop
playback engine, server manager, update client, or Windows UI test project.

The authoritative API descriptions remain `packages/api-schema/openapi.yaml` and
`connect/openapi.yaml`. Phase 14 Connect and local assertion bootstrap are reused;
Windows does not define another account or discovery protocol.

## Product boundaries

- **VyNode Desktop** is a native consumer client. It does not own the service.
- **VyNode Media Server** is the existing Go server running as a least-privilege
  Windows service or, for diagnostics, as a console process.
- **VyNode Server Manager** is a small optional local management surface. It may
  inspect service status and open Web Admin but must not bypass server APIs or
  expose unauthenticated privileged IPC.

Client-only and server-only installation remain independent. Removing the client
must never remove the service or server data.

## Native client decision

VyNode Desktop uses C#/.NET 10 and WinUI 3 from Windows App SDK 2.4. It is a real
native Windows XAML application, not a WebView shell. WinUI provides per-monitor
DPI, UI Automation, keyboard access, high-contrast integration, modern windowing,
and `MediaPlayerElement` backed by Windows Media Foundation. The client targets
Windows 10 1809 or later and uses a self-contained unpackaged deployment inside a
traditional installer.

Unpackaged deployment is intentional: the service and firewall configuration need
per-machine installer privileges that MSIX does not model cleanly as one product.
The client remains independently installable per machine. A future Store package
can be produced separately without changing the application architecture.

## Security and state

The application-generated device UUID is random and stable; no hardware serial or
fingerprint is collected. Connect refresh credentials and each server's refresh
credential are stored as separate Windows Credential Locker entries. Access tokens
remain in memory. Settings contain only non-secret preferences and server identity
metadata. Global logout deletes every credential and account-namespaced local state.

An endpoint is never trusted as identity. Before a cached credential is sent, the
client must compare the endpoint's server identity with the linked server ID. Each
server receives an independent assertion bootstrap and refresh credential.

## Playback

`MediaPlayerElement`/`MediaPlayer`, backed by Windows Media Foundation, is the
playback host. The Windows capability profile advertises only the supported-Windows
baseline proven in runtime acceptance and is sent to the existing server
playback-decision endpoint. Phase 15 runtime acceptance must prove H.264/AAC
MP4, HLS, server text subtitles, seeking, tracks, fullscreen, markers, quality, and
Up Next. Unsupported codecs are not advertised merely because another Windows
installation might have an optional codec extension.

## Service and data

The Go executable will implement native Service Control Manager callbacks while
preserving console mode. The default service account is `NT AUTHORITY\LocalService`:
it has low local privilege and anonymous network credentials. Server data lives in
`%ProgramData%\VyNode\Media Server`; logs live below `logs`. Program binaries live
under `%ProgramFiles%\VyNode`. UNC libraries require an explicitly configured
service identity with share and NTFS permissions; VyNode never stores share
passwords in plaintext.

## Installation and updates

WiX v5 produces separate Desktop and Media Server MSIs plus an optional bundle.
The server MSI registers the service and an executable-scoped firewall rule only
when LAN access is selected. Upgrade and repair preserve ProgramData, the SQLite
database, and the Ed25519 identity. Uninstall removes binaries/service/rules but
retains data unless the user explicitly selects data removal.

Update metadata is detached-signature verified against a pinned release public key.
Stable and Beta channels share the same schema. Update checks are bounded and never
block account or playback. Production packages require Authenticode; CI accepts
signing material only from protected secret storage and never commits private keys.
