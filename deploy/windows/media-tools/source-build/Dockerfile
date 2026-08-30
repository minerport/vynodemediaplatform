# Source-built candidate. Not the approved installer payload until acceptance completes.
FROM debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171
RUN apt-get update && apt-get install -y --no-install-recommends build-essential gcc-mingw-w64-x86-64-posix g++-mingw-w64-x86-64-posix nasm pkg-config meson ninja-build ca-certificates && rm -rf /var/lib/apt/lists/*
COPY sources /sources
COPY build.sh cross.ini /recipe/
RUN sh /recipe/build.sh
