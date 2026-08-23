# Episode download subscriptions

A subscription belongs to one user, paired device, show, profile, and desired unwatched count (1–20). The server selects available non-special episodes in canonical season/episode order and creates normal device download assignments until the window is full. Movie downloads remain manual.

When remove-watched is enabled, a confirmed synchronized watched event produces a removal instruction; local deletion is not assumed until acknowledged. The next fill selects the following unwatched episode. DOWNLOAD access revocation disables the subscription and revokes its assignments.

Automatic filling respects the device's reported minimum-free-space headroom. When headroom is exhausted the subscription becomes `STORAGE_LIMITED` and no new preparation is queued. Wi-Fi-only is stored for future native-client enforcement because the server cannot reliably know a phone's active network.
