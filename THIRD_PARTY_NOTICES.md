# Third-party notices

VyNode Media includes or depends on third-party software. This notice is an
inventory aid and does not replace the license text distributed with each
component.

## FFmpeg 9.0 source-built preview for Windows

The 16.0.3 preview Windows Media Server installer contains the VyNode source-built
64-bit static `9.0-vynode-source-preview` FFmpeg and FFprobe pair. Earlier
engineering builds used Gyan; those binaries are not the public preview payload.

- FFmpeg copyright: FFmpeg developers and contributors, 2000–2026.
- Configured FFmpeg binary license: GPL-3.0-or-later.
- Build configuration: `--enable-gpl --enable-version3 --enable-static`, with the
  complete configuration and external-library inventory in the packaged
  `README.txt`.
- Source revision: <https://github.com/FFmpeg/FFmpeg/tree/d32b387f2b>
- Exact sources and build recipe: `deploy/windows/media-tools/source-build`.
- External media libraries: x264 (GPL-2.0-or-later), dav1d (BSD-2-Clause).
- Corresponding source downloads accompany the binary preview at
  <https://github.com/minerport/vynodemediaplatform/releases/tag/preview-16.0.3.1>.

`LICENSE`, `README.txt`, and `notices/` are installed beside the executables.
They include the x264/dav1d notices, MinGW copyright/license inventory, GCC runtime
exception, and LGPL texts. Matching upstream source archives and complete Debian
toolchain sources/build rules are provided alongside the release. VyNode invokes
the programs as separate processes. No patent clearance is represented.

## Debian Linux container

The preview uses Debian FFmpeg 7:5.1.9-0+deb12u1. Its Debian package/source
inventory, full matching Debian sources, build rules, and `/usr/share/doc`
copyright notices are provided in the Linux distribution source archive. These
are separate from the Windows FFmpeg payload. Individual component terms apply.

## Microsoft Windows application dependencies

VyNode Desktop uses Microsoft Windows App SDK and Windows SDK Build Tools. VyNode
Server Manager uses `System.ServiceProcess.ServiceController`. Package versions are
recorded by the release SBOM generated from the project files. Their respective
Microsoft package license terms apply.

## Go and Web dependencies

The Media Server, Connect service, and Web application use modules recorded in
`go.mod` and `package-lock.json`. The release SBOM records the resolved dependency
identities. Individual upstream license terms apply.
