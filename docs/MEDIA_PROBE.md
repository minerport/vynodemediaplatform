# FFprobe media inspection

`MediaProbe` isolates the scanner from execution and supports deterministic fakes. Docker installs Debian's FFmpeg package. The Windows Server MSI installs a pinned, hash-verified FFprobe beside its matching FFmpeg under the protected VyNode installation directory and the service uses that absolute path; no global `PATH` or network download is required. Administrators can persist an explicit custom path under `HKLM\SOFTWARE\VyNode\Media Server`.

Execution uses `exec.CommandContext`, never a shell. The filename is the `-i` argument, including dash-prefixed and Unicode names. Timeout is 45 seconds; cancellation kills the child; stdout is bounded to 8 MiB and stderr to 32 KiB; only structured JSON is parsed.

Normalized data covers container/duration/bitrate plus video, audio, subtitle, attachment/data stream properties. PQ is conservatively `HDR10_OR_PQ`, HLG is `HLG`, otherwise `SDR`; Dolby Vision and HDR10+ are not asserted without reliable structured evidence. Optional/version-dependent fields are tolerated and the authorized capability endpoint reports detected version.
