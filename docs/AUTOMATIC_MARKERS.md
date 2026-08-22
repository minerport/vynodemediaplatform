# Automatic markers

VyNode performs marker analysis locally with FFmpeg and never uploads fingerprints. Intro analysis samples at most the first 15 minutes as mono 8 kHz PCM, hashes non-overlapping two-second chunks, and searches an offset-tolerant recurring contiguous sequence across at least three episodes. This permits different cold-open lengths. Ten seconds of recurrence is the minimum evidence. Compact hashes are cached against source size and modification identity; decoded audio is discarded.

Credits analysis samples one 32×18 grayscale frame per second from only the final three minutes and looks for a sustained low-luma interval. It is deliberately conservative and keeps post-credit markers distinct. Recap analysis has schema/job foundations but is not auto-activated because reliable local evidence is not yet strong enough.

Scores are normalized: HIGH ≥ 0.82, MEDIUM ≥ 0.60, otherwise LOW. The default server policy does not automatically activate even HIGH candidates. MEDIUM candidates require review and LOW evidence never affects playback.

Manual precedence is absolute. Creating or adjusting a marker makes it `MANUAL`, deactivates competing automatic markers, and future analysis cannot activate over it. Rejections persist by logical item, marker type, and source identity, so unchanged evidence stays suppressed. Changed source identity permits fresh review without deleting manual work.

Sources are `MANUAL`, `AUTOMATIC_AUDIO`, `AUTOMATIC_VIDEO`, `AUTOMATIC_HYBRID`, and `IMPORTED`. Accepted automatic markers enter the same player path as manual markers.
