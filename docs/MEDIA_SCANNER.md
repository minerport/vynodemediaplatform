# Physical media scanner

Scans are persistent background jobs with `QUEUED`, `RUNNING`, `COMPLETED`, `COMPLETED_WITH_ERRORS`, `FAILED`, and `CANCELED` states. Duplicate active scans are rejected. Progress uses polling in Phase 2; WebSocket transport remains pending.

The scanner streams `filepath.WalkDir`, skips symlinks, trash/system directories, and unsupported extensions. Candidates are `.avi`, `.m2ts`, `.m4v`, `.mkv`, `.mov`, `.mp4`, `.mpeg`, `.mpg`, `.ts`, and `.webm`; FFprobe, not the extension, establishes valid media.

Unchanged relative path, 64-bit size, and nanosecond mtime reuse probe data. Changes re-probe transactionally. Failed reprobes retain last-known-good technical fields with an error. Missing files remain as history; offline roots never mass-mark inventory. Move detection is remove-plus-add. Full files are never hashed.

Probe concurrency defaults to 2 and is configurable from 1–8. Cancellation reaches active FFprobe processes. Per-file errors are bounded. Scanning is read-only. Separate tentative filename hints recognize year, `S01E02`, `S01E02E03`, and `1x01`; they are not authoritative metadata.
