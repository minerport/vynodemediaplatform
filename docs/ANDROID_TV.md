# Android TV and NVIDIA Shield

The same APK declares both normal and Leanback launcher categories, does not require a touchscreen, and detects television mode through `UiModeManager`. Home uses vertical server-configured rows with horizontal media lists and explicit focus borders. D-pad traversal, predictable Back behavior, pairing without password typing, and readable errors take priority over animation and decoration.

NVIDIA Shield is a target, not a codec assumption. Capability reporting must query the device. Until a physical Shield installation is executed, status is **APK TARGETED — REAL DEVICE NOT VALIDATED**. Fire TV remains a compatibility target; the core has no Play Services requirement, but it is not yet runtime validated.

## Connected-test transport status

On the Windows Phase 13 validation host, both the API 36 Android TV emulator and the API 36 phone emulator complete the device-side instrumentation suite successfully. Direct `adb shell am instrument -w` also exits successfully. The Gradle connected-test task nevertheless exits non-zero after the tests finish with `Failed to receive the UTP test results`; the UTP host log records `io.grpc.StatusRuntimeException: UNAVAILABLE: io exception`. The generated UTP result records both tests as passed and both emulators remain connected.

This is classified as a host-side AGP Unified Test Platform gRPC result-transport defect after test completion, not an application or TV-emulator test failure. CI intentionally does not suppress connected-test failures with `|| true`; JVM tests, lint, and APK builds remain required, while connected instrumentation must run in a separate environment where UTP result transport is reliable or report this exact known failure explicitly.

Validated tooling for this observation:

- Gradle 8.13
- Android Gradle Plugin 8.12.0
- Android platform/TV emulator API 36
- adb 37.0.1

## Runtime validation boundary

The production-server TV matrix has runtime evidence for Direct Play, Direct Stream, Audio Transcode, and Video Transcode/HLS. Pairing, credential restoration after process death, authenticated Home, session revocation, server-identity mismatch protection, D-pad Home/detail traversal, Skip Intro, and automatic Episode 1 to Episode 2 playback have also been exercised. Phase 13 is not considered complete until the remaining interactive acceptance checks—especially native search, quality change continuity, alternate-audio switching, rendered subtitle cues, autoplay cancellation, and exact credits seeking—are recorded against the final APK.
