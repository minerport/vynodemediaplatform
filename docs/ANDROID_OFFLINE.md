# Android offline behavior

Offline assignments use app-private storage. A background worker streams into an owned partial file, resumes from its exact length with authenticated Range, verifies manifest size and SHA-256, and atomically renames only a valid result. Media is never buffered wholly in memory. Transient image cache is separate from downloaded manifest artwork.

Room persists server-scoped assignments, sync cursors, storage reports, and idempotent progress events. Offline playback never requires token refresh. Reconnection pushes durable progress/inventory before pulling bounded changes and applies server removal instructions. Wi-Fi-only policy is a native WorkManager constraint, not a server claim.
