# Playback pipeline

Semantic audio/subtitle resolution occurs for the selected physical version before the existing least-cost decision. Autoplay creates a fresh decision for each episode, so mode changes between episodes are supported.

The fixed priority is `DIRECT_PLAY`, `DIRECT_STREAM`, `AUDIO_TRANSCODE`, `VIDEO_TRANSCODE`, then `UNSUPPORTED`. Direct Play serves inventoried bytes with byte ranges. Phase 5 generated modes stream fragmented MP4 (`frag_keyframe+empty_moov+default_base_moof`) from FFmpeg stdout; Phase 6 video transcodes use authenticated fMP4 HLS. Progress remains time-based.

Direct Stream copies selected video and audio. Audio Transcode always copies video and encodes audio as AAC: 192 kb/s for stereo and 384 kb/s above stereo. Channels are preserved up to the client limit and six-channel Phase 5 maximum; downmixing is disclosed in the plan.

FFmpeg uses structured arguments without a shell. Client flags are never accepted. Diagnostics are capped at 32 KiB and paths are redacted. Disconnect, seek, stop, admin stop, and shutdown terminate processes and Unix process groups. The generated-pipeline limit defaults to two (`VYNODE_PLAYBACK_PIPELINES`, range 1–16); Direct Play uses no slot. Migration 7 stores independent pipeline-instance state.

Generated seeks use input `-ss`; accuracy is keyframe-dependent, not frame-perfect. HTTP output applies natural backpressure and no complete movie is buffered. The admin capability endpoint exposes a safe version, relevant codecs/muxers, and active/configured counts—not commands or paths.

The browser player attaches generated fragmented MP4 responses through Media Source Extensions. Each seek replaces the object URL, aborts the prior fetch, rejects stale SourceBuffer callbacks with a generation counter, and maps the new segment timeline to the requested logical offset. The append window is bounded by the logical duration. Visible seek controls and native media seeks use the same replacement path; Direct Play continues to use its ordinary authenticated range URL. Production CSP permits `blob:` only for media playback.

Phase 5 automated runtime validation uses the in-app Chromium build with its reported H.264/AAC support. Synthetic sources use short keyframe intervals; generated seek acceptance is within two seconds of the requested logical time. Other browsers remain capability-driven and were not claimed as Phase 5 runtime validations.
