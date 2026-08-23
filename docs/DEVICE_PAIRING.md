# Device Pairing

Pairing is a seven-minute bootstrap into ordinary VyNode authentication. A native/TV client receives an unpredictable `XXXX-XXXX` display code and a separate 256-bit possession challenge. Only hashes are persisted. An authenticated user looks up the code, reviews device name/client/platform, and approves or denies it for their own account.

Approval alone does not issue credentials. The requesting device must exchange the original private challenge. A successful exchange creates the same rotating refresh-token session used by password login, marks the device as `PAIRED`, and makes it visible through session management. Revoking that session prevents refresh; no permanent API key exists.

Codes are collision-retried, one-time, rate-limited to eight lookup attempts per source per minute, and persisted through restart until expiration. Polling returns only state and timing, never the approving user. Approval, denial, and later session revocation are audited; status polls are not.
# Offline-device use

Only sessions belonging to devices authorized through pairing may create, transfer, inventory, or synchronize offline downloads. Pairing remains only the bootstrap into normal rotating sessions; downloads do not introduce permanent device API keys.
