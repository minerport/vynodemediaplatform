# Android client

Phase 13 lives in `apps/android` and is a native Kotlin/Jetpack Compose application (`com.vynode.media`), not a WebView. One application supports touch and television form factors; television mode is selected with `UiModeManager`, never screen size. Domain state is server-scoped in Room so credentials, cursors, downloads, and queued progress cannot cross servers.

The client uses the versioned REST contract, validates the stable VyNode installation ID before credential submission, discovers `_vynode-media._tcp` with Android DNS-SD, and supports explicit endpoints. Normal platform TLS validation is mandatory. The initial build intentionally rejects cleartext HTTP globally; adding local HTTP requires a narrowly designed user-consent policy rather than a trust-all TLS manager.

Versions introduced: AGP 8.12.0, Kotlin 2.2.20, compile/target SDK 36, Compose June 2026 BOM, Room 2.8.4, Media3 1.10.1, and WorkManager 2.11.2. This mutually compatible stable set avoids Compose's API-37 requirement while API 37 is unavailable in the installed stable SDK channel.

No analytics, advertising, provider SDK, or Google Play Services dependency is included.
