# Release hardening

Phase 16 starts from the committed Phase 15 Windows client, native service,
Server Manager, independent WiX installers, signed update-metadata verifier, UAC
installer handoff, and conditional Authenticode CI hooks. Consumer and playback
features remain frozen.

## Live Connect endpoint correction

The clean Windows client exposed an unresolved placeholder Connect hostname.
Windows and Android/TV now default to `https://connect.vynodehub.com`.
Windows `VYNODE_CONNECT_URL` and Android's `vynodeConnectUrl` Gradle override
remain supported; clean-machine acceptance must use the packaged default, not a
developer environment override. Local development Compose endpoints are unchanged.
This does not change the separate update-feed or release-signing configuration.

The existing Connect service is deployed on Unraid behind Nginx Proxy Manager
with HTTPS enforcement, secure refresh-cookie flags, and request access logging
disabled for this hostname. Its database and private identities are persistent
runtime data, never repository artifacts. Existing local-server users and accounts
from another Connect installation are not automatically migrated. Global account
registration and owner-authorized server linking are still required before
zero-address server discovery can pass. A healthy HTTPS endpoint alone does not
prove client login, server discovery, or clean-machine acceptance.

## Verified starting state

- Windows server builds produced only `vynode-server.exe`; FFmpeg and FFprobe were
  resolved from explicit environment variables or `PATH`.
- The server MSI contained the server and Server Manager but no media tools.
- Installed-service media acceptance therefore used administrator-provided paths.
- Structured process arguments and `exec.CommandContext` already prevented shell
  concatenation, but an empty configured path still allowed ambient `PATH` lookup.
- SCM installation uses delayed automatic startup as LocalService with bounded
  restart recovery. Phase 15 did not execute its real reboot gate.
- Update metadata uses a pinned ECDSA public key, detached signatures, package
  SHA-256 verification, and explicit UAC handoff. Installation did not return the
  user to VyNode automatically.
- CI contained conditional Authenticode commands, but the Windows job indentation
  was invalid and it did not separate development and production release lanes.
- Artifact inventory recorded hashes and unsigned status, but there was no complete
  channel-aware release manifest, checksum document, Windows SBOM, or bundled-tool
  identity.
- Production credentials and private keys are not present in the repository.

## Phase 16 boundaries

This phase supplies a verified, offline-capable Windows media runtime and hardens
release production and acceptance. It does not add Apple source, Windows downloads,
new playback features, telemetry, general UI changes, Web optimization, or broad
API documentation cleanup.

## Approved Windows media-tools payload

VyNode pins the Gyan Doshi FFmpeg 9.0 essentials Windows x64 static ZIP from the
immutable GitHub release URL recorded in
`deploy/windows/media-tools/manifest.json`. The archive digest is verified before
extraction, and the individual FFmpeg and FFprobe executable digests are verified
before installer construction.

The included `README.txt` identifies the build as
`9.0-essentials_build-www.gyan.dev`, source commit `d32b387f2b`, and GPL version 3.
Its recorded configuration includes `--enable-gpl`, `--enable-version3`, static
linking, libx264, libx265, and other external libraries. VyNode therefore describes
this exact payload as GPLv3 and does not characterize it as an LGPL-only build.
The original `LICENSE`, full build-configuration README, source URL, archive hash,
and executable hashes are shipped with the tools.

Normal installation never downloads media tools and never changes global `PATH`.
The service uses absolute files below
`%ProgramFiles%\VyNode\Media Server\tools\ffmpeg`. Explicit administrator
administrator overrides stored as `ExternalFFmpegPath` and `ExternalFFprobePath`
under `HKLM\SOFTWARE\VyNode\Media Server` take precedence and survive MSI major
upgrades. Process environment variables remain a development/operations override
with the highest precedence. Without overrides, a missing or hash-mismatched managed tool
causes service startup to fail closed rather than execute modified code.

## Corresponding source and notices

The selected source revision is available at
`https://github.com/FFmpeg/FFmpeg/tree/d32b387f2b`. The packaged build also contains
third-party libraries with their own terms. Release engineering must retain the
upstream payload's license and build evidence and review the complete notices file
before publication. This record documents evidence and distribution actions; it is
not legal advice.

## Installed diagnostics and servicing

The authenticated admin dashboard reports the absolute executable path, managed
or custom source, and bounded first version line for FFmpeg and FFprobe. It does
not expose generated command lines. A normal MSI upgrade replaces the protected
managed payload but never resets the persistent administrator override. Updating
the pinned manifest and payload in a later release services media tools without
changing ProgramData, the installation ID, or the Ed25519 server identity.

## Update channels, completion, and rollback

Stable and Beta are separate, compile-time client identities with their own HTTPS
metadata and signature endpoints. Signed metadata also carries its channel; a
client ignores a correctly signed manifest for the other channel. Candidates must
be strictly newer, so neither same-version repair nor silent downgrade is offered
by the updater. An administrator-required rollback is a documented manual
uninstall/install operation after safeguarding ProgramData and client state.

The Desktop updater downloads into its controlled update directory, verifies the
detached ECDSA signature and MSI SHA-256, and rehashes immediately before structured
`msiexec /i` handoff. VyNode registers with Windows Restart Manager at startup, so
Windows Installer can close it during replacement and reopen the installed client
in the original user's non-elevated session. If installation completes without
closing the client, the UI also exposes a **Relaunch VyNode** action. Neither path
carries credentials or installer-only arguments.

## Signing and production labeling

Pull-request and development builds are intentionally unsigned and their release
manifest says `UNSIGNED DEVELOPMENT BUILD`. A tagged production lane requires the
Authenticode certificate/password and update public verification-key inputs,
uses a configured RFC 3161 timestamp endpoint, and verifies every first-party EXE
and MSI after signing. Missing inputs fail that lane. Only public update-key
material is embedded; private update and Authenticode material is never committed.

## Determinism and release inventory

The release bundle contains the two MSIs, three primary executables, authoritative
JSON manifest, SHA-256 checksum list, CycloneDX SBOM, and third-party notices. The
SBOM inventories Windows NuGet/WiX dependencies, Go modules, resolved Web lockfile
packages, and the exact FFmpeg/FFprobe payload. A caller-supplied `GeneratedAt`
makes metadata generation repeatable. Go `-trimpath` output is expected to be
stable for an equivalent toolchain and inputs; self-contained .NET publishing and
WiX MSIs may contain build identifiers or timestamps, and Authenticode necessarily
changes signed bytes. Release engineering compares two clean builds and records
actual equality rather than claiming byte-for-byte reproducibility universally.

## Clean-VM client theme defect (2026-08-30)

The clean Windows VM exposed unreadable sidebar icons and mismatched light
controls against the client's fixed dark surfaces. `App.xaml` did not request a
theme, so controls inherited Windows' light theme. The bounded correction requests
the existing dark theme application-wide; no navigation or product features were
redesigned. Desktop acceptance build 16.0.2 is unsigned. The 18 Windows unit tests
and self-contained client/MSI build pass; installed-VM visual verification remains
pending. Earlier playback confirmation does not prove the corrected build's UI.

## Clean-VM server Web Admin defect (2026-08-30)

The offline 16.0.0 server installed and ran, but Server Manager's `/admin` link
returned JSON `not_found`. The MSI omitted the Web payload and the Windows service
had no installed Web directory default. The packaging correction builds and
includes the existing Web app under `Media Server\web`; the service resolves it
relative to its executable when `VYNODE_WEB_DIR` is not explicitly configured.
Missing bundled `index.html` fails service startup rather than presenting a
healthy but unusable default administration path. Explicit Web-directory overrides
remain supported. Unsigned server 16.0.1 builds successfully; Web lint, five Web
tests, HTTP tests, and command vet pass. Offline installed-runtime acceptance of
this correction remains pending. The original run must not be marked PASS.

## Empty Movies catalog regression

The offline VM showed a blank Movies page before catalog identification. The
server's empty movie slice serializes as `movies: null`; the Web page dereferenced
its length. A regression test reproduced the null-list failure. The Web API
adapter now normalizes empty movie, library, and library-file responses to `[]`,
retaining the existing empty-state UI. The user also reproduced the Libraries
blank page; regression tests demonstrated both empty-library and pre-scan
file-list failures before correction. All eight Web tests and lint pass. Server
16.0.2 includes these bounded fixes; installed offline page verification remains
pending.

## Supply-chain threat review

### Public preview preparation (2026-08-30; not Phase 16 PASS)

The user requested Windows, Unraid, and Android installation/testing downloads
using `https://connect.vynodehub.com`. No GitHub release has been published by
this preparation. Production Authenticode remains a separate external gate.

- Android `16.0.3-preview.1` (version code 160003) debug APK build, debug unit
  tests, and debug lint passed in a host build directory outside OneDrive.
  This is not a production-signed APK or fresh-device playback acceptance.
- Linux container `16.0.3-preview.1` built with server vet/tests, Linux build,
  Windows cross-build, and embedded Web build. It has not been pushed to GHCR
  or installed on Unraid as part of this preview preparation.
- The existing Windows Gyan media payload's complete static dependency/source
  distribution is unresolved. The installer manifest was deliberately not
  replaced merely because a different source-built candidate passes a smoke test.
- `deploy/windows/media-tools/source-build` records the exact three source
  revisions, archive hashes, and build recipe for a Windows FFmpeg 9.0 candidate.
  Synthetic probe, remux, AAC transcode, libx264 HLS, seek/decode, and embedded
  subtitle extraction passed on the development host. Optional capability
  differences and toolchain/runtime notices still need review before substitution.
- The Linux Debian FFmpeg source/license inventory is separate and remains to
  be completed before publishing that container payload.

Client clean-VM 16.0.2 evidence was subsequently retained: self-contained launch,
global sign-in, linked Home, Search, Movie Detail, Direct Play/fullscreen, and
uninstall passed, with user-observed UI steps. The user also confirmed the theme
and Manual Connect Back correction. Offline Server 16.0.2 setup and physical scan
were observed; offline playback/transcode and final server uninstall are not
proven. Do not infer full clean-Windows PASS from the client result.

- Compromised or substituted media archives are bounded by an immutable source URL
  and pinned archive plus executable hashes; installed managed tools are checked
  before execution.
- A stale/vulnerable media build is visible by exact version, source revision, and
  hashes in diagnostics, manifests, checksums, and SBOM so it can be assessed and
  serviced through a state-preserving MSI upgrade.
- Installer substitution is bounded by Authenticode, timestamp verification,
  signed update metadata, and a second package hash check at launch.
- Update-channel confusion is bounded by separate endpoints plus signed channel
  identity and strict-newer version policy.
- Release/update key compromise requires revoking and replacing the corresponding
  public trust material in a new client release; credentials remain CI secrets and
  are never artifact inputs or command-line data.
- SBOM incompleteness is reviewed against NuGet/WiX projects, `go.mod`, the resolved
  Web lockfile, and the separately packaged media-tool manifest at each release.
