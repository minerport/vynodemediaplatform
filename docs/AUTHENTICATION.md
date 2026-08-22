# Authentication

VyNode accounts are local database identities. A fresh database has explicit incomplete setup state; one transactional bootstrap creates the sole `OWNER`, marks setup complete, audits the action, and creates a normal device session. Usernames are trimmed, lowercased, ASCII-only, 3–64 characters, and match `[a-z0-9][a-z0-9._-]*`; uniqueness is case-insensitive by normalization.

Passwords use Argon2id in self-describing PHC form with 64 MiB memory, 3 iterations, parallelism 2, a 16-byte random salt, and a 32-byte result. Passwords are 10–256 bytes. Malformed hashes fail safely. The encoding supports future parameter upgrades; automatic rehash is not yet performed.

Access tokens are HS256 JWTs lasting 15 minutes. Claims are user ID, session ID, role, issuer/server installation ID, issuance time, and expiry. The 256-bit signing key is generated once as `/config/auth-signing.key` with mode `0600` where supported. Already-issued access tokens remain valid until expiry after revocation; revocation blocks refresh immediately.

Refresh credentials contain a session ID plus 256 random bits. Only SHA-256 digests are stored. They last 30 days, rotate at every exchange, and retain the immediately previous digest for replay detection. Reuse of the previous credential revokes the session and creates `SESSION_REFRESH_REUSE_DETECTED`; reauthentication is required.

The browser keeps access tokens only in memory. Its refresh credential is an HttpOnly, SameSite=Strict cookie scoped to `/api/v1/auth`, marked Secure under HTTPS, and restored on reload. A shared single-flight promise ensures concurrent expired requests perform one rotation. Native clients opt into refresh-token response delivery with `X-VyNode-Client: native` and will eventually store it in Android Keystore, Apple Keychain, or Windows secure credential storage.

Unsafe cookie requests use same-origin validation; exact `VYNODE_ALLOWED_ORIGIN` supports a separate trusted UI origin. Missing Origin is accepted for native clients using bearer credentials. `/config/vynode.db` and `/config/auth-signing.key` persist together. Authentication never requires Internet access.

OWNER has `server.manage`, `users.manage`, `sessions.manage`, `security.view`, and `audit.view`; ADMIN has all except `server.manage`; USER has no administrative capabilities. All users retain self-service account/session operations. Owner creation, disablement, deletion, and transfer are not Phase 1 administrative operations.

Phase 2 adds `libraries.view`, `libraries.manage`, `libraries.scan`, and
`media.inventory.view` to OWNER and ADMIN. USER receives none of these capabilities,
so normal users cannot probe server paths or view physical filesystem inventory.
