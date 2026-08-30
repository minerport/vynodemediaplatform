# Source-built Windows media-tools candidate

This is a candidate, not the approved MSI payload. The existing managed manifest
still identifies the Gyan build. Do not publish that installer based on this
candidate's source evidence: these are different binaries.

Build context: this directory's Dockerfile, build.sh, cross.ini, and a `sources/`
directory with the three archives named `<name>.tar.gz` from sources.lock.json.
The build verifies all archive hashes before compiling. No upstream sources are
patched. Output is `/out` in the image. Export only generated binaries, configs,
and evidence, not developer credentials or repository state.

The original candidate built on Debian bookworm with GCC MinGW POSIX
12.2.0-14+deb12u1+25.2+b1, mingw-w64 10.0.0-3, NASM 2.16.01-1,
Meson 1.0.1-5, Ninja 1.11.1-2~deb12u1, and build-essential 12.9.
The base image is digest-pinned. Apt package versions are not frozen by this
recipe; byte-for-byte reproducibility is NOT established.

Authenticity boundary: upstream HTTPS source archives and recorded SHA-256,
not an upstream verified signature. Archives contain upstream copyright/license
notices. FFmpeg is configured with GPL and version3 enabled; this is not an LGPL
build. Complete source distribution must also account for applicable static
toolchain/runtime notices and dependencies. Do not infer legal clearance from
the absence of third-party DLL imports.

The first candidate passed host synthetic probe, remux, AAC audio transcode,
libx264 HLS, seek/decode, and embedded subtitle-to-WebVTT extraction using
`../Test-SourceCandidate.ps1`. It imports only Windows system DLLs. These checks
are not clean-machine or installed-service acceptance.

Deliberate external libraries: x264 and dav1d. This does not preserve every
optional Gyan codec/filter/hardware integration. Before substitution, review
capability differences, required subtitle/font/image paths, runtime licensing,
and rerun affected installed-server tests. Do not silently regress accepted
product capabilities.

Public distribution remains pending: complete corresponding-source archive and
notices, source/binary manifest and SBOM alignment, installed validation, and
release artifact verification. The Linux Debian FFmpeg payload has a separate
dependency/source inventory and is not covered by this Windows recipe.
