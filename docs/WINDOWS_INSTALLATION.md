# Windows installation

VyNode uses two independent WiX v7 MSIs:

- `VyNode-Desktop-unsigned.msi` installs the native viewer and Start menu entry.
- `VyNode-Media-Server-unsigned.msi` installs the Go server service and the pinned
  FFmpeg/FFprobe payload for offline operation.

This separation provides real client-only and server-only installations and prevents
desktop uninstall from changing the service. A future optional bundle may offer
Desktop, Server, or Both while retaining the same independent package identities.

The server MSI grants LocalService access only to its ProgramData root and creates
an executable/port-scoped TCP 8096 local-subnet firewall exception. It does not
open broad Internet scope. Normal uninstall removes binaries, service registration,
and the installer-owned firewall rule but retains server data. Explicit data removal
must be a separately confirmed action.

Both packages are per-machine installers and therefore require normal Windows UAC
approval. VyNode never disables or bypasses UAC. Desktop installation does not
register the server service or create an inbound firewall rule. Desktop uninstall
removes installed binaries and shortcuts; per-user cache/credential cleanup follows
the documented account privacy policy and is verified separately from server data.

An in-place MSI major upgrade preserves the independent desktop profile and the
server ProgramData directory. Server database, configuration, installation identity,
Connect link, libraries, and grants are acceptance invariants. Repair or same-version
maintenance must not erase persistent data.

Artifacts are unsigned development builds. SmartScreen warnings must not be bypassed.
Production releases require Authenticode signing by CI using protected credentials.
Unsigned files are always labeled development artifacts by the release manifest.
