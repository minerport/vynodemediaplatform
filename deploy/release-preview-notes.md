# VyNode Media 16.0.3.1 — full-product testing preview

Windows Desktop + Windows Media Server + Android + Unraid/Linux container.
This is a testing prerelease, **not Phase 16 production PASS**.

## Install

- **Windows:** install the Desktop MSI for playback. Install the Server MSI on
  the machine hosting your media. It includes Web Admin, Server Manager, FFmpeg
  and FFprobe; no SDK, PATH editing, or separate media-tool download is needed.
- **Android:** sideload `VyNode-Android-16.0.3-preview.1-debug.apk` on Android 8+
  phone/TV. This is debug-signed. A differently signed existing installation
  cannot be upgraded in place; preserve your data before uninstalling anything.
- **Unraid:** download the Docker archive and run
  `docker load -i vynode-media-16.0.3-preview.1-docker.tar`, then use the supplied
  `vynode-media.xml` template and README. The image is local-imported, not on Docker
  Hub; do not use Pull/Force Update. Persistent directories need UID/GID 65532
  write access; keep media mounts read-only.

Verify downloads using SHA256SUMS.txt. Windows MSIs are **unsigned development
builds**, not Authenticode-trusted releases. Production update trust is not
configured; use these versioned downloads for manual upgrades.

## Sign in and add media

Clients default to **https://connect.vynodehub.com**. Create/sign in with your
VyNode account, not your Microsoft account. A new media server needs local owner
setup and owner-authorized Connect linking before it appears automatically.
Use a trusted HTTPS endpoint reachable by your clients; Connect is not a media
relay. Local/manual access remains available.

Configure libraries and metadata provider settings in Web Admin. A physical-file
scan is distinct from identifying a Movies/Shows catalog. An empty global account
or a newly installed empty server does not include a sample media library.

## Verified / boundaries

- Windows unit tests: 18 passed; Windows client/server MSI builds passed.
- Installed Windows server upgrade: ready, LocalService, installation ID retained,
  managed executable hashes verified.
- Server media engine: real remux, audio transcode, video/HLS, probe and seek/decode
  passed using the packaged tools on Windows and Unraid.
- Android debug unit tests, lint, build and APK signature verification passed.
- Unraid container deployed with healthy readiness, working Web setup and bundled
  tools. Local owner setup and linking are operator actions, not preconfigured
  credentials. New interactive client/server pairing remains part of user testing.

Known limits: unsigned Windows distribution, debug-signed Android, manual updates,
no Windows offline downloads, no Apple client, no validated hardware transcoding
or HDR tone mapping. Full clean-VM and production-signing gates are not claimed.

## Source and notices

Windows FFmpeg is `9.0-vynode-source-preview` (GPL-3.0-or-later), with x264 and
dav1d. Linux uses Debian FFmpeg 7:5.1.9-0+deb12u1. These are separate payloads.
Their matching source archives, complete Debian dependency/toolchain sources,
build recipes, package inventories, checksums and notices are included below.
Keep these source downloads available alongside any redistributed binary.
No legal advice or patent clearance is represented.
