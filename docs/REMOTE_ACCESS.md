# Remote Access

VyNode remains fully local-first. Manual endpoints and reverse proxies do not require VyNode Connect or any external service. The owner may configure one HTTPS manual endpoint, explicit trusted proxy CIDRs, and local-network CIDRs. HTTP requires a separate insecure opt-in and remains visibly unsafe.

Forwarded client IP and scheme are honored only when the socket peer belongs to an explicitly configured trusted proxy CIDR. Spoofed forwarding headers from direct clients are ignored. Local classification affects playback quality and warnings only; it never bypasses authentication or role/grant checks. IPv4 and IPv6 CIDRs are normalized.

Reverse-proxy TLS is the supported TLS topology in Phase 11. Built-in certificate loading, ACME, and URL-base hosting are not implemented. The endpoint remains `UNVERIFIED_EXTERNALLY`; a server-side hairpin check cannot prove Internet reachability. Host/router firewalls are never modified.

Automatic port mapping is off by default. When explicitly enabled, VyNode discovers a UPnP IGD, maps only its configured TCP listener for a bounded lease, renews before expiry, and deletes only its own mapping on disable or clean shutdown. Discovery, renewal, and deletion state is persisted and visible to administrators. A successful router mapping is not reported as proof of Internet reachability. NAT-PMP and PCP remain future work and are not claimed in this checkpoint.
