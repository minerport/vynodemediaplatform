# Playback pipeline

Phase 5 uses the fixed priority `DIRECT_PLAY`, `DIRECT_STREAM`, `AUDIO_TRANSCODE`, then `UNSUPPORTED`. Direct Play serves inventoried bytes with byte ranges. Generated modes stream fragmented MP4 (`frag_keyframe+empty_moov+default_base_moof`) from FFmpeg stdout and accept only a validated `start` time. A seek creates a new pipeline instance in the same logical session and cancels its predecessor. Progress remains time-based.

Direct Stream copies selected video and audio. Audio Transcode always copies video and encodes audio as AAC: 192 kb/s for stereo and 384 kb/s above stereo. Channels are preserved up to the client limit and six-channel Phase 5 maximum; downmixing is disclosed in the plan.

FFmpeg uses structured arguments without a shell. Client flags are never accepted. Diagnostics are capped at 32 KiB and paths are redacted. Disconnect, seek, stop, admin stop, and shutdown terminate processes and Unix process groups. The generated-pipeline limit defaults to two (`VYNODE_PLAYBACK_PIPELINES`, range 1–16); Direct Play uses no slot. Migration 7 stores independent pipeline-instance state.

Generated seeks use input `-ss`; accuracy is keyframe-dependent, not frame-perfect. HTTP output applies natural backpressure and no complete movie is buffered. The admin capability endpoint exposes a safe version, relevant codecs/muxers, and active/configured counts—not commands or paths.
