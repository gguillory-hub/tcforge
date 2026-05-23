# Third-Party Notices

tcforge release bundles may include third-party command-line tools in the `tools/` directory so users do not need to install Go, FFmpeg, or ltc-tools separately.

## FFmpeg

- Project: https://ffmpeg.org/
- Legal information: https://ffmpeg.org/legal.html
- Source: https://git.ffmpeg.org/ffmpeg.git
- License: FFmpeg is commonly distributed under LGPL, but builds that enable GPL components are distributed under GPL. Check the bundled build metadata and `SHA256SUMS.txt` for the exact binary included in this release.

## ltc-tools

- Project: https://github.com/x42/ltc-tools
- Source: https://github.com/x42/ltc-tools
- License: GPL-2.0-only
- Bundled command used by tcforge: `ltcdump`

## libltc

- Project: https://github.com/x42/libltc
- Source: https://github.com/x42/libltc
- License: LGPL-3.0-only
- Included only when the bundled `ltcdump` build needs a separate runtime library.

## Binary Checksums

Every release package includes `SHA256SUMS.txt` listing the checksums of the files shipped in that package.
