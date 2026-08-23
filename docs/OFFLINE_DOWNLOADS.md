# Offline downloads

Offline media is a managed, device-bound assignment—not a permanent anonymous URL. `DOWNLOAD` is independent from `VIEW` and `PLAY`. A paired native session requests a typed profile and receives a plan choosing `ORIGINAL_COPY`, `EXISTING_OPTIMIZED_VERSION`, or `GENERATED_OFFLINE_VERSION`, in that order where applicable.

Profiles are versioned: Original preserves source bytes; High is at most 1920×1080 H.264 at 6 Mbps plus AAC 192 kbps; Medium is 1280×720 at 2.5 Mbps plus AAC 128 kbps; Low is 854×480 at 1.2 Mbps plus AAC 128 kbps. Generated output is MP4/H.264/yuv420p/AAC stereo with fast-start metadata. One semantic primary audio track is selected; commentary avoidance follows the existing preference architecture. Text subtitles are separate manifest sidecars; image subtitles and HDR-to-SDR offline conversion are not claimed.

Generated files live beneath the configured `VYNODE_DOWNLOADS_DIR` (`/downloads` in Docker). The root contains an ownership marker and assets use server IDs. Partial files are atomically renamed and only recognized owned partials are cleaned. Original media is opened read-only and never copied merely for server caching.

READY assets expose exact 64-bit size and SHA-256. `GET|HEAD /api/v1/downloads/{id}/file` requires the assignment's current user, paired device, active session, and current DOWNLOAD grant. Standard single-range HTTP supports resume and returns 416 for invalid/multiple ranges. ETag is checksum-derived and Content-Disposition uses a Windows-safe sanitized title.

Identical source revision/profile requests reuse one asset while device assignments remain separate. Generated cache bytes are deleted only when an owned asset has no active assignment; quota eviction never targets originals, optimized media, or active assignments. Custom encryption, DRM, bandwidth throttling, and server-side Wi-Fi detection are intentionally absent.
