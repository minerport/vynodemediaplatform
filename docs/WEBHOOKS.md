# Webhooks

Webhook payloads are stable JSON:

```json
{"schemaVersion":1,"eventId":"…","eventType":"SOURCE_UNAVAILABLE","occurredAt":"2026-08-22T12:00:00Z","serverId":"…","payload":{}}
```

When a signing secret exists, `X-VyNode-Signature` is `sha256=` followed by the lowercase hexadecimal HMAC-SHA256 of the exact HTTP request body. `X-VyNode-Event-ID` is also sent for receiver deduplication.

Public HTTPS is the default policy. Loopback, unspecified, multicast, link-local, and private addresses are rejected after DNS resolution both when configured and immediately before delivery. Redirects are disabled. Explicit OWNER/ADMIN settings may allow private/LAN destinations and HTTP, which the UI labels as security-sensitive; link-local and metadata-service targets remain prohibited even with private access enabled. Schemes other than HTTP/HTTPS and URLs with embedded credentials are rejected.

The Test action creates a clearly synthetic event and uses the normal queue, validation, signing, retry, and history pipeline.
