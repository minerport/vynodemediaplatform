#!/bin/sh
# Run in a disposable container, with /evidence mounted to an ignored directory.
# Capture matching source archives for every installed Debian binary package.
set -eu
mkdir -p /evidence/sources /evidence/notices
sed 's/^Types: deb$/Types: deb-src/' /etc/apt/sources.list.d/debian.sources > /etc/apt/sources.list.d/vynode-source.sources
apt-get update
dpkg-query -W -f='${binary:Package}\t${Version}\t${source:Package}\t${source:Version}\n' > /evidence/debian-packages.tsv
dpkg-query -W -f='${source:Package}=${source:Version}\n' | sort -u > /evidence/debian-sources.txt
cp -a /usr/share/doc /evidence/notices/
cp -a /usr/share/common-licenses /evidence/notices/
cd /evidence/sources
while IFS= read -r source; do
    apt-get source --download-only "$source"
done < /evidence/debian-sources.txt
sha256sum ./* > /evidence/source-sha256.txt
