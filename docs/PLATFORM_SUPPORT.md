# Platform support

Remote access through a standards-compliant reverse proxy is platform-neutral. IPv4/IPv6 proxy and local-network policy is supported. Phase 11 ships host-LAN mDNS advertisement and an opt-in UPnP IGD client; it does not ship built-in TLS, NAT-PMP, or PCP. Docker bridge multicast cannot be assumed, so discovery validation distinguishes host networking and Unraid behavior.

Logical metadata and cached artwork use paths below the configured application-data directory on Windows and `/config` in containers/Unraid. Provider outages do not prevent startup, scanning, authentication, or local browsing.

Phase 3 was runtime-validated in the Linux Docker image and cross-built for Windows. Native Windows Phase 3 execution remains unavailable in the validation environment because a host Go toolchain is not installed; this does not alter the already validated read-only Windows scanning design.

Direct Play uses the same Go HTTP delivery implementation in native and container builds. Unraid uses the identical OCI image; mounted media should remain read-only, and host disk/network throughput directly bounds original-byte delivery. Phase 4 Windows validation is cross-build-only unless noted in its completion report.

**SUPPORTED** is implemented and validated; **PLANNED** has an explicit roadmap;
**FUTURE** is intentionally deferred and subject to platform evaluation.

| Target | Status | Phase 0 reality |
| --- | --- | --- |
| Docker (Linux amd64) | SUPPORTED | Image builds/runs with persistent config |
| Docker (Linux arm64) | PLANNED | Architecture-compatible; not validated here |
| Unraid | PLANNED | Same-image CA template; not tested on Unraid |
| Linux native | PLANNED | Go target/CI build; host startup not validated here |
| Windows native | SUPPORTED | Native cross-build/startup; no Service/installer |
| Web | SUPPORTED | Authenticated library administration and technical inventory |
| Android phone/tablet | PLANNED | Native Kotlin/Compose strategy only |
| Android TV / Google TV / Fire TV / NVIDIA Shield | PLANNED | Compose/Media3 TV strategy only |
| iOS / iPadOS / tvOS / macOS | FUTURE | SwiftUI/AVFoundation strategy only |
| Roku | FUTURE | No implementation |
| Samsung Tizen | FUTURE | No implementation |
| LG webOS | FUTURE | No implementation |
