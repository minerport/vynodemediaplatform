# Full-product testing preview 16.0.3.1

Status: full-product testing preview prepared for publication; not Phase 16 PASS.
This is not a clients-only release. Do not publish until Windows client/server,
Android APK, and the Unraid container are all available with matching evidence.

## Current artifacts

- Windows Desktop MSI 16.0.3: rebuilt; unsigned development build; 18 Windows
  tests passed. No production update trust key is configured. Manual upgrades
  only; signing warnings are expected, not evidence of production trust.
- Windows Server MSI 16.0.3: rebuilt with Web Admin and the preview source-built
  FFmpeg/FFprobe payload. Installed-host upgrade passed: ready, LocalService,
  stable installation ID and matching managed tool hashes.
- Android 16.0.3-preview.1 / version code 160003: debug-signed APK, unit tests,
  lint and build passed. Existing installations signed with a different key
  cannot be upgraded in place; do not uninstall without considering local data.
- Linux/Unraid image 16.0.3-preview.1: deployed on the user's Unraid host at port
  18096 with isolated persistent storage. Web setup and readiness return 200.

The actual server HLS manager passed remux, audio-transcode, video-transcode,
FFprobe inspection, and seek/decode checks using the installed Windows tools and
the packaged Linux tools on Unraid. These synthetic runtime tests do not claim
an interactive end-to-end playback rerun on every client or clean-VM acceptance.

Windows and Android default to `https://connect.vynodehub.com`. A media server
must be separately linked by its local owner and advertise an endpoint reachable
from the clients. A Connect account alone does not supply a media server.

## FFmpeg distribution evidence

The preview Windows tools are **not** the old Gyan payload. They are built from
the exact FFmpeg, x264 and dav1d revisions/hashes in
`deploy/windows/media-tools/source-build/sources.lock.json`. The build has GPL
and version3 enabled, no nonfree flag, and uses FFmpeg as a separate executable.
Full upstream archives, build recipe and retained per-file copyright notices are
collected for distribution beside the binaries.

The Windows payload includes GPL text, x264 and dav1d notices, MinGW copyright
and license inventory, GCC copyright including its runtime exception, and LGPL
texts. Matching Debian source packages/build rules for the entire installed
build environment were downloaded successfully, including the MinGW/GCC runtime.
The archive is deliberately broader than the linked runtime dependency set.

The Linux image uses Debian FFmpeg 7:5.1.9-0+deb12u1, not the Windows build.
Matching source packages for every Debian package in that image were downloaded,
with package/source version inventory, copyright notices and SHA-256 lists.
APT verifies repository metadata; individual source download hashes are retained.
Never replace this image with a freshly resolved image without checking that
the distributed sources still match its exact package inventory.

The original Windows candidate uses only x264 and dav1d as explicitly enabled
external media libraries. Optional Gyan filters/codecs are not all present.
Current server code uses software libx264/AAC, scale/format, MP4/HLS and WebVTT;
synthetic probe, remux, audio/video transcode, seek and subtitle extraction passed.
Hardware backends are currently diagnostic-only (`Available: false`), and HDR
tone mapping is rejected by the existing decision engine. This does not claim
hardware/HDR playback support or equivalence to all features of the Gyan build.

These are documented redistribution steps, not legal advice or patent clearance.
Do not publish binaries without the corresponding source assets and notices.

## Installation and remaining operational steps

- Create the Unraid server's local owner through its Web setup page. No default
  password was created. Configure read-only library mounts and metadata provider
  settings. Physical scans alone do not populate identified Movies/Shows.
- Link each new server to your Connect account using its local OWNER. Configure
  trusted HTTPS and advertise a client-reachable endpoint for global access.
- Android is sideloaded and debug-signed; Windows MSIs are unsigned. Neither is
  represented as a production-trusted installer or store submission.
- Unraid distribution is an image archive imported using `docker load`, not a
  GHCR/Community Applications listing or automatic update feed.

Production Authenticode and the remaining clean-VM gates remain distinct from
this testing preview. No merge, Phase 16 PASS, or Phase 17 work is authorized here.
