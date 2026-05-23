#!/usr/bin/env bash
set -euo pipefail

libltc_version="1.3.2"
libltc_sha256="0a6d42cd6c21e925a27fa560dc45ac80057d275f23342102825909c02d3b1249"
ltc_tools_version="0.7.0"
workdir="${1:-/tmp/tcforge-ltc-build}"

rm -rf "$workdir"
mkdir -p "$workdir"
cd "$workdir"

curl -L -o "libltc-${libltc_version}.tar.gz" "https://github.com/x42/libltc/releases/download/v${libltc_version}/libltc-${libltc_version}.tar.gz"
echo "${libltc_sha256}  libltc-${libltc_version}.tar.gz" | sha256sum -c -
tar -xzf "libltc-${libltc_version}.tar.gz"

pushd "libltc-${libltc_version}"
./configure --prefix=/mingw64 --enable-shared --disable-static
make -j"$(nproc)"
make install
popd

curl -L -o "ltc-tools-${ltc_tools_version}.tar.gz" "https://github.com/x42/ltc-tools/archive/refs/tags/v${ltc_tools_version}.tar.gz"
tar -xzf "ltc-tools-${ltc_tools_version}.tar.gz"

pushd "ltc-tools-${ltc_tools_version}"
make ltcdump \
  CFLAGS="-Wall -O2 $(pkg-config --cflags ltc sndfile) -DVERSION=\\\"${ltc_tools_version}\\\"" \
  LOADLIBES="$(pkg-config --libs ltc sndfile) -lm -lpthread"
if [[ -f ltcdump.exe ]]; then
  install -m755 ltcdump.exe /mingw64/bin/ltcdump.exe
else
  install -m755 ltcdump /mingw64/bin/ltcdump.exe
fi
popd

ltcdump.exe -h >/dev/null || true
