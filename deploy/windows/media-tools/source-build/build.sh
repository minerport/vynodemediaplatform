#!/bin/sh
set -eu
mkdir -p /build /prefix /out
cd /sources
echo 'e26d41e2f496c1598f418726b871ce252ce9f18f8dbe3ad199349a42ed2cb02f  dav1d.tar.gz
8a830a34bfaf98514b5d45cf6c01b1fe78b38d5e4c10eab0de2531b783c15f90  ffmpeg.tar.gz
d053c9d86988d6bc78237ca5205865c5ddf99c98ef4cd9927eec8f6d388f6dd9  x264.tar.gz' | sha256sum -c -
for name in x264 dav1d ffmpeg; do
  mkdir /build/$name
  tar -xf /sources/$name.tar.gz -C /build/$name --strip-components=1
done
export PKG_CONFIG_PATH=/prefix/lib/pkgconfig
cd /build/x264
./configure --prefix=/prefix --host=x86_64-w64-mingw32 --cross-prefix=x86_64-w64-mingw32- --enable-static --disable-cli --disable-opencl
make -j4
make install
cd /build/dav1d
meson setup build --cross-file=/recipe/cross.ini --prefix=/prefix --libdir=lib --default-library=static -Denable_tools=false -Denable_tests=false
ninja -C build -j4
ninja -C build install
cd /build/ffmpeg
./configure --prefix=/out --target-os=mingw32 --arch=x86_64 --enable-cross-compile --cross-prefix=x86_64-w64-mingw32- --pkg-config=pkg-config --pkg-config-flags=--static --disable-autodetect --enable-gpl --enable-version3 --enable-static --disable-shared --disable-doc --disable-debug --disable-ffplay --enable-libx264 --enable-libdav1d --enable-schannel --extra-cflags=-I/prefix/include --extra-ldflags='-L/prefix/lib -static' --extra-version=vynode-source-preview
make -j4
make install
cp /build/ffmpeg/config.h /build/ffmpeg/ffbuild/config.mak /out/
x86_64-w64-mingw32-objdump -p /out/bin/ffmpeg.exe | grep 'DLL Name' > /out/dll-imports.txt
sha256sum /out/bin/*.exe > /out/binary-sha256.txt
