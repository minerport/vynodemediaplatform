# Unraid public-preview installation

For preview 16.0.3.1, download the Docker image archive from the GitHub prerelease,
verify its SHA-256 against SHA256SUMS.txt, and import it on Unraid:

```sh
docker load -i vynode-media-16.0.3-preview.1-docker.tar
```

The imported image is `vynode-media:16.0.3-preview.1`. It is NOT a Docker Hub
repository; do not use Pull/Force Update. This preview uses manual image imports,
not an advertised GHCR feed. Import a later release before changing the image tag.

The included XML is a private Unraid template, not a Community Applications listing.
Set its Repository to the exact imported release image.
Map `/config` persistently, `/media` read-only, and `/transcode`, `/optimized`, and
`/downloads` to separate writable directories. The container runs as UID/GID 65532;
those writable directories must permit that identity. Do not grant broad write
access to your media library or reuse another application's database directory.

Open `http://<unraid-ip>:<mapped-port>/` to create the local owner and libraries.
Library paths refer to container paths such as `/media/movies`, not Unraid paths.
FFmpeg and FFprobe are installed inside the image; no host PATH setup is required.

For global sign-in, create an account at `https://connect.vynodehub.com`, configure
a trusted HTTPS reverse proxy for this media server, then run
`deploy/test/Link-PreviewServer.ps1` on a Windows administration machine using that
HTTPS origin for both URL arguments. This helper calls the existing owner-linking
APIs; it never substitutes your Connect account for the local server owner.
Connecting to the account service does not make an unreachable media endpoint
reachable. Do not advertise localhost or a container-only hostname to remote clients.

Preview status is not Phase 16 production acceptance. Back up `/config` before
upgrades. Never downgrade against an upgraded database without a compatible backup.
