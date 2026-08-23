# Security

Playback preferences are scoped to the authenticated user. Marker reads require playback access; marker mutation requires metadata administration and emits audited create, update, or delete events. APIs and diagnostics never expose paths or FFmpeg commands.

Automatic analysis is offline and uses structured FFmpeg arguments; it never exposes source paths or uploads fingerprints. Automation accepts no SQL, scripts, arbitrary paths, moves, or original-media deletion. Optimized output is confined to a dedicated controlled root and finalized atomically. Manual marker decisions always outrank generated evidence.

Offline downloads require an active paired session, matching user and device assignment, and an explicit current DOWNLOAD grant. IDs are not credentials. Generated paths use server IDs under an ownership-marked root; client filenames are sanitized for control characters, separators, Windows reserved names, and length. Original media stays read-only. Revocation denies future/resumed requests but cannot erase bytes already held by a disconnected device; VyNode does not claim DRM.

HLS playlists and every fMP4 resource use the existing HttpOnly playback credential and session ownership checks. Generated URLs never contain source paths, traversal is rejected, and responses are private `no-store`.

Generated playback uses structured FFmpeg arguments and never a shell. Clients may select stored track IDs and a bounded timestamp but cannot supply filters or flags. Diagnostics are bounded and redact paths; media and subtitle credentials are route-scoped, HttpOnly, SameSite, and bound to an active login session.

Metadata credentials are accepted only by capability-gated owner/admin routes, stored outside ordinary settings responses, and never logged or redisplayed. Provider clients bound time and response size and constrain redirects. Artwork writes are confined to the application-controlled config cache; media roots remain read-only inputs.

Passwords are Argon2id hashes. Tokens, passwords, hashes, and authorization headers are excluded from API errors, audit metadata, and request logs. Refresh digests and append-oriented audit rows live in SQLite; the signing key is a restrictive file under `/config`. No secrets are baked into images.

Observability APIs require OWNER/ADMIN capabilities; ordinary users can access only their principal-derived playback summary. Physical storage paths remain admin-only. Webhook URLs are re-resolved at configuration and delivery, public HTTPS is the default, redirects are disabled, dangerous IP classes remain blocked, response reads are bounded, and optional HMAC secrets are encrypted locally and never returned. VyNode sends no cloud telemetry.

Browser refresh uses an HttpOnly SameSite=Strict cookie, Secure on HTTPS, with a narrow auth path. Access credentials remain in memory. Unsafe requests with an Origin must match the request origin or exact configured `VYNODE_ALLOWED_ORIGIN`; wildcards and malformed origins are rejected. Preflight responses are restricted and credentialed wildcard CORS is never emitted. This combines SameSite with Origin-based CSRF defense.

The socket peer address drives throttling and audit context. `Forwarded`, `X-Forwarded-For`, and `X-Real-IP` are ignored; trusted proxy ranges are not implemented. Authentication endpoints allow ten attempts per peer per rolling minute and recover automatically.

Responses set CSP, frame denial, MIME sniffing protection, no-referrer policy, restricted browser permissions, and same-origin resource policy. Stable API errors omit database and filesystem details. Audit data contains only safe event, actor/target, request, outcome, and limited client context.

Direct Play never accepts a client path or physical file ID. Session creation resolves an authorized logical item to a recorded association. A one-hour random, non-refreshable media credential is delivered only in a SameSite, HttpOnly, path-restricted cookie and only its digest is stored; it is restricted to one playback session and its still-valid user/login session. Media URLs contain no credential. The CSP permits media only from the same origin, and permissive CORS is not enabled.

## Physical media safety

Physical-library administration is OWNER/ADMIN-only. Source validation requires
absolute readable server-visible directories and rejects configuration/transcode
overlap. Scanning skips symlinks and never writes user media. FFprobe uses structured
arguments without a shell, timeout/cancellation, and bounded output. An unavailable
root preserves prior inventory rather than causing mass missing-file updates.

## Not yet implemented

MFA, passkeys, cloud/console recovery, owner recovery or transfer, external identity providers, and VyNode Connect are not implemented. Account deletion remains deferred.

## Sharing and remote access

Invitation and pairing raw secrets are never persisted: SHA-256 digests support equality checks while Argon2id remains the account password KDF. Pairing requires both short-code approval and possession of a separate high-entropy challenge, then issues normal rotating refresh credentials. Forwarded headers are ignored unless the socket peer matches an explicit trusted-proxy CIDR. Library grants are enforced server-side and revocation terminates affected playback.

## Curation privacy

Curation ownership comes from the authenticated principal, never a caller-supplied user ID. Users cannot enumerate another account's playlists, watchlist, favorites, private collections, or Home layout. Shared collection administration requires OWNER/ADMIN policy checks. Smart rules accept only bounded typed nodes compiled through parameterized allowlists—never SQL or scripts.
