VyNode source-built FFmpeg / FFprobe 9.0-vynode-source-preview, Windows x64

FFmpeg copyright 2000-2026 the FFmpeg developers.
x264 copyright 2003-2023 the x264 project and contributors.
dav1d copyright 2018-2019 VideoLAN and dav1d authors.
All upstream per-file copyright notices remain in the corresponding sources.

This FFmpeg build enables GPL and version3: GPL-3.0-or-later, NOT LGPL-only.
x264 is GPL-2.0-or-later; dav1d is BSD-2-Clause. See LICENSE and notices/.
MinGW and GCC runtime notices are included; the corresponding-source download
also includes the exact Debian toolchain source packages and build rules.
VyNode uses FFmpeg as an independent subprocess, not as a linked library.
No warranty or patent clearance is represented.

Source release location:
https://github.com/minerport/vynodemediaplatform/releases
Use the matching preview's source archives, source lock, build recipe and hashes.
Do not distribute this payload without the corresponding source downloads.

Build configuration:
--prefix=/out --target-os=mingw32 --arch=x86_64 --enable-cross-compile
--cross-prefix=x86_64-w64-mingw32- --pkg-config=pkg-config
--pkg-config-flags=--static --disable-autodetect --enable-gpl --enable-version3
--enable-static --disable-shared --disable-doc --disable-debug --disable-ffplay
--enable-libx264 --enable-libdav1d --enable-schannel
--extra-cflags=-I/prefix/include --extra-ldflags='-L/prefix/lib -static'
--extra-version=vynode-source-preview

Required server software H.264/AAC, remux, HLS and subtitle extraction paths
passed host synthetic testing. Optional Gyan library inventory is not identical:
libass rendering, zscale, and optional external hardware encoders are absent.
VyNode currently does not offer validated hardware transcoding or HDR tone mapping.
Installed-server acceptance must still be recorded for each release.
