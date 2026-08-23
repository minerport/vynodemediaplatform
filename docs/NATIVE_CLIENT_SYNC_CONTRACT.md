# Native client sync contract

This is the Phase 13 client contract shared by Android/Media3, Apple URLSession/AVFoundation, and Windows clients:

1. Discover or configure the server, pair, and rotate normal native credentials.
2. Pull the device cursor and bounded changes; perform a bounded full resync when instructed.
3. Request a device-bound download and poll preparation state.
4. Fetch the versioned manifest and artwork from VyNode's authenticated cache.
5. Download with HTTP Range into app-private temporary storage, resume as needed, verify exact size and SHA-256, then atomically expose the local copy.
6. Report download inventory and storage headroom without exposing local paths.
7. Play locally with no server connection and queue coalesced progress events with an installation epoch, device sequence, and idempotency ID.
8. On reconnect, push progress/inventory, process removal instructions, acknowledge deletions, then pull the next cursor page.

Android should use app-private storage, WorkManager/Media3-style background work, and Keystore credentials. Apple clients should use sandbox storage, background URLSession, and Keychain. Windows clients should use an app-controlled directory and platform credential storage. The protocol assumes none of those platforms and does not implement custom media encryption or DRM.
