# Media optimization

Optimization creates explicit derivatives and never changes or replaces an original. Outputs live below `VYNODE_OPTIMIZED_DIR` (default: `<config>/optimized`) and are atomically renamed from a partial file only after FFmpeg succeeds. Interrupted partial output is removed and is never exposed as a playable version.

Profiles are Mobile 480p, Mobile 720p, Remote 1080p, and Compatible H.264. They reuse FFmpeg with a structured argument list, H.264 video, AAC audio, yuv420p, and fast-start MP4. The database records source file, derivative file, profile, job, measured byte size, SHA-256, and status. Original deletion is not part of this subsystem.

Completed derivatives are ordinary physical versions labeled `OPTIMIZED`. Existing playback selection therefore prefers a compatible derivative over a real-time video transcode whenever it satisfies the requested limits. Optimization uses one background worker and never consumes a playback HLS transcode slot.
