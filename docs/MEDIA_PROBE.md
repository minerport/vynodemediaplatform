# FFprobe media inspection

`MediaProbe` isolates the scanner from execution and supports deterministic fakes. FFprobe uses explicit `VYNODE_FFPROBE_PATH` or PATH discovery. Docker installs Debian's FFmpeg package; Windows uses PATH/configuration and does not yet bundle it.

Execution uses `exec.CommandContext`, never a shell. The filename is the `-i` argument, including dash-prefixed and Unicode names. Timeout is 45 seconds; cancellation kills the child; stdout is bounded to 8 MiB and stderr to 32 KiB; only structured JSON is parsed.

Normalized data covers container/duration/bitrate plus video, audio, subtitle, attachment/data stream properties. PQ is conservatively `HDR10_OR_PQ`, HLG is `HLG`, otherwise `SDR`; Dolby Vision and HDR10+ are not asserted without reliable structured evidence. Optional/version-dependent fields are tolerated and the authorized capability endpoint reports detected version.
