# Video transcoding

VyNode preserves the least-cost order: Direct Play, Direct Stream, Audio Transcode, Video Transcode, then Unsupported. Existing compatible versions are evaluated across the first three modes before a video transcode candidate is admitted.

Phase 6 software output is H.264 `yuv420p` using `libx264` with the `veryfast` preset. The bounded rate-control policy uses target bitrate, 1.25× maximum rate, and a two-maximum-rate buffer. Frame rate is preserved. Scaling preserves aspect ratio, never upscales, and produces even dimensions.

Profiles are 1080p (8 Mbps target), 720p (4 Mbps), and 480p (1.5 Mbps). Auto selects the highest profile that fits source size, client resolution, and the effective bandwidth limit. Remote bandwidth defaults to 20 Mbps and is configurable with `VYNODE_REMOTE_BITRATE`. `VYNODE_VIDEO_TRANSCODES` defaults to one; admission is immediate and returns `TRANSCODE_CAPACITY_REACHED` rather than creating a hidden queue.

HDR Direct Play behavior is preserved. HDR-to-SDR video transcode returns `HDR_TONE_MAPPING_UNAVAILABLE`; Phase 6 does not claim unvalidated tone mapping. Subtitle burn-in is not enabled in the initial core.
