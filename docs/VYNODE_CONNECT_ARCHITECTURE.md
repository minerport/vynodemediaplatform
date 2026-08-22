# VyNode Connect Architecture (Future)

VyNode Connect is not implemented and is never required for local login, playback, administration, invitation acceptance, pairing, or manual remote access.

A future `ServerDiscoveryProvider` boundary may order known current endpoints, LAN discovery, manual remote endpoints, and optional Connect registration. Clients must compare the stable server installation ID before sending saved credentials to an endpoint that unexpectedly identifies as another server.

An optional registration service may know account identity, server ID, endpoints, and device authorization state. It should not receive library contents, media metadata, or watch history. Relay is a reserved future connection type only; no relay traffic or fake Connect calls exist.
