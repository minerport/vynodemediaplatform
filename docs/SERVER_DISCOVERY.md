# Server Discovery

The reserved DNS-SD service type is `_vynode-media._tcp`. A future advertisement contains only server name, stable installation ID, port, and protocol/API version. Discovery only locates a server and never grants authorization.

The runtime advertises `_vynode-media._tcp` while LAN discovery is enabled. Its TXT record contains only the stable server instance ID, API generation, and secure-transport capability; it contains no user, library, path, or credential data. Advertisement startup and shutdown follow the server lifecycle, failures are non-fatal and persisted for the admin UI and health screen, and manual connection remains available and cloud-independent.

Real multicast discovery is validated with host networking. Docker bridge networks do not reliably carry multicast, so bridge-mode discovery is not claimed; Unraid and other deployments must expose the host LAN appropriately.

Docker bridge multicast is environment-dependent. Host networking is usually required for straightforward multicast discovery on Unraid; neither the image nor template promises otherwise.
