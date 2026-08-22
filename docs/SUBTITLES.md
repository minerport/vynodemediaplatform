# Subtitles

VyNode read-only discovers `.srt`, `.vtt`, `.ass`, and `.ssa` files adjacent to inventoried videos using equal or `basename.language` naming. Associations are stored separately and paths never enter consumer APIs.

Phase 5 converts SRT to sanitized WebVTT, sanitizes VTT, and can extract embedded text subtitles through bounded FFmpeg output. ASS/SSA styling may be lost. Sidecars are limited to 2 MiB, require UTF-8, and HTML-significant cue text is escaped. Responses are `text/vtt` with no-store caching.

PGS, VobSub, and other image subtitles truthfully return `SUBTITLE_REQUIRES_VIDEO_TRANSCODE`; there is no OCR or burn-in. Only usable tracks appear in the web selector. Subtitle delivery uses the same user/login/playback-session/version-bound authorization as media.
