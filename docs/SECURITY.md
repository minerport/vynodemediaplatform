# Security

Metadata credentials are accepted only by capability-gated owner/admin routes, stored outside ordinary settings responses, and never logged or redisplayed. Provider clients bound time and response size and constrain redirects. Artwork writes are confined to the application-controlled config cache; media roots remain read-only inputs.

Passwords are Argon2id hashes. Tokens, passwords, hashes, and authorization headers are excluded from API errors, audit metadata, and request logs. Refresh digests and append-oriented audit rows live in SQLite; the signing key is a restrictive file under `/config`. No secrets are baked into images.

Browser refresh uses an HttpOnly SameSite=Strict cookie, Secure on HTTPS, with a narrow auth path. Access credentials remain in memory. Unsafe requests with an Origin must match the request origin or exact configured `VYNODE_ALLOWED_ORIGIN`; wildcards and malformed origins are rejected. Preflight responses are restricted and credentialed wildcard CORS is never emitted. This combines SameSite with Origin-based CSRF defense.

The socket peer address drives throttling and audit context. `Forwarded`, `X-Forwarded-For`, and `X-Real-IP` are ignored; trusted proxy ranges are not implemented. Authentication endpoints allow ten attempts per peer per rolling minute and recover automatically.

Responses set CSP, frame denial, MIME sniffing protection, no-referrer policy, restricted browser permissions, and same-origin resource policy. Stable API errors omit database and filesystem details. Audit data contains only safe event, actor/target, request, outcome, and limited client context.

## Physical media safety

Physical-library administration is OWNER/ADMIN-only. Source validation requires
absolute readable server-visible directories and rejects configuration/transcode
overlap. Scanning skips symlinks and never writes user media. FFprobe uses structured
arguments without a shell, timeout/cancellation, and bounded output. An unavailable
root preserves prior inventory rather than causing mass missing-file updates.

## Not yet implemented

MFA, passkeys, cloud/console recovery, owner recovery or transfer, external identity providers, trusted-proxy parsing, VyNode Connect, and device pairing are not implemented. Account deletion remains deferred.
