# Windows update security

Update discovery is independent from account and media availability. Failure or
timeout never blocks application startup. Stable is the default channel; Beta uses
the same signed schema but a distinct feed identity. A client ignores even
correctly signed metadata for the other channel.

Metadata contains channel, semantic version, HTTPS package URL, SHA-256 package
digest, minimum compatible client version, and publication time. The canonical
metadata bytes are signed with an offline ECDSA P-256 release key. The verifier is
designed to receive only the pinned public key. It verifies the signature before parsing actionable fields,
then verifies the downloaded package digest in fixed time. Production MSI/EXE
artifacts additionally require Authenticode publisher verification.

A URL or a hash downloaded from the same unsigned channel is never sufficient.
Invalid metadata, a modified package, an older Stable version, or an unknown channel
is rejected. No private release key or test certificate is committed. CI receives
signing access only through protected secret storage and signs after reproducible
unsigned artifacts have been built and scanned.

The update boundary is secure detection and verification followed by an explicit
installer handoff. It does not silently install updates. Feed I/O is
bounded and offline/slow feeds return no update without blocking app launch.
Release CI accepts the Authenticode certificate and password only through protected
`WINDOWS_SIGNING_CERT_BASE64` and `WINDOWS_SIGNING_CERT_PASSWORD` secrets, removes
the temporary PFX in a `finally` block, and verifies every resulting signature.

The verification library and CI signing boundary are implemented. The installed
client receives the release public key ID and ECDSA P-256 SPKI through build-time
assembly metadata. CI variables `WINDOWS_UPDATE_PUBLIC_KEY_ID` and
`WINDOWS_UPDATE_PUBLIC_KEY_SPKI` contain public material only. A signed release
build fails if either value is absent. An unsigned build without a key fails closed:
update checking is disabled without affecting account, browsing, or playback.

Debug builds may use the explicitly named `VYNODE_UPDATE_TEST_*` environment
override. Release builds ignore that override. Acceptance feed keys are generated
only below ignored artifacts and are deleted after testing; they are not production
trust anchors.

After signed metadata and the downloaded MSI hash pass verification, the user sees
an explicit Install Update action. VyNode accepts only the exact managed MSI path,
re-hashes it immediately before launching `msiexec.exe` with structured arguments,
and lets normal Windows UAC proceed. It never silently elevates or auto-approves UAC.
Unsigned development MSIs can produce normal SmartScreen or publisher warnings.
VyNode registers with Windows Restart Manager so Windows Installer can close it for
replacement and reopen it in the original user's non-elevated session. If the
installer completes without closing VyNode, the client presents a clear Relaunch
VyNode action. Neither completion path inherits installer elevation or passes
secrets on a command line. Normal update policy requires a strictly newer version;
rollback is an explicit administrator procedure, not a silent updater downgrade.
