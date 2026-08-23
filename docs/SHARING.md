# Sharing

Account roles and media access are independent. OWNER and ADMIN retain unrestricted media access in the initial self-hosted policy, while USER accounts require explicit normalized `VIEW` and `PLAY` grants for each library. A logical item is visible when at least one available association belongs to an accessible library; inaccessible copies never expand access.

The server enforces grants for library listing, movie/show/detail/search/artwork, collections and smart collections, playlists, watchlists, Home rows, playback discovery, and playback creation. Revoking PLAY stops affected active sessions immediately with `LIBRARY_ACCESS_REVOKED`; subsequent media requests fail because stopped sessions are not authorized.

Invitations are local, bounded to 1 hour, 24 hours, or 7 days, revocable, and single-use. Only OWNER can invite ADMIN. The raw random token is shown once and stored only as SHA-256. Acceptance uses the normal Argon2id password path and normal device/session architecture. Lost links must be replaced, never recovered from storage. No email or cloud service is required.

Playlist/watchlist entries remain in storage after grant loss but are hidden from responses until access returns, avoiding both metadata leakage and destructive edits to personal organization.
# Download permission

`DOWNLOAD` is independent from `VIEW` and `PLAY`. Grant replacement immediately revokes device download assignments and subscriptions that no longer have a matching accessible library. Native clients receive removal instructions on their next sync; already-offline bytes cannot be recalled.
