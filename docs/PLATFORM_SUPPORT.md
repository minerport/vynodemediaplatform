# Platform support

Logical metadata and cached artwork use paths below the configured application-data directory on Windows and `/config` in containers/Unraid. Provider outages do not prevent startup, scanning, authentication, or local browsing.

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
