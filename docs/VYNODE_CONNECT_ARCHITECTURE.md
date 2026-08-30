# VyNode Connect architecture

VyNode Connect is implemented as an optional global identity, invitation, and server-discovery control plane. It is never required for direct local login, playback, administration, local pairing, or manual remote access.

Clients order known current endpoints, LAN/manual fallbacks, and Connect registration while comparing the stable server installation ID before sending saved credentials to an endpoint that unexpectedly identifies as another server.

Connect stores account identity, server ID, endpoints, invitations, relationships, and device authorization state. It does not receive library contents, media metadata, watch history, or media traffic. Relay remains unimplemented.
