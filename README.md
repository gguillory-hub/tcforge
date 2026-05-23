# tcforge

`tcforge` turns audio LTC recorded on a camera audio channel into real video timecode metadata that NLEs can read.

It is intended to be camera- and timecode-box-agnostic. If your camera records valid Linear Timecode as audio, `tcforge` should be able to decode it and write a new `.mov` with proper `tmcd` timecode metadata without re-encoding the video.

```powershell
tcforge fix input.MP4
```

## Status

`tcforge` is an early v0.1 CLI. The core workflow has been validated, but broader camera/timecode-box testing is still needed.

This is a community utility, not a commercial product. The goal is to make a small, reliable, open tool for a workflow that currently has too few good options.

## Tested With

Validated so far:

- Sony A7IV recording Deity TC-1 audio LTC over 3.5mm input
- Sony FX6 MXF files with embedded timecode as a reference
- Premiere Pro 2026
- DaVinci Resolve
- Windows

Not yet tested:

- Tentacle Sync, UltraSync, Atomos, Ambient, or other LTC boxes
- Canon, Panasonic, Fuji, Nikon, Blackmagic, or other camera files
- macOS or Linux
- Drop-frame edge cases beyond the current Sony/Deity samples

Reports from other workflows are very welcome.

## Requirements

Required external tools:

- `ffmpeg`
- `ffprobe`
- `ltcdump` from [`ltc-tools`](https://github.com/x42/ltc-tools)

Official release packages include these tools in a `tools` folder beside the `tcforge` executable, so normal users do not need to install them separately.

When running from source or using a bare development binary, these tools must be available on `PATH`, except on Windows where `tcforge` also checks:

```text
C:\Users\<you>\tools\ltcdump\ltcdump.exe
```

You can override tool paths with environment variables:

```powershell
$env:TCFORGE_LTCDUMP = "C:\Users\you\tools\ltcdump\ltcdump.exe"
```

## Installing Release Builds

Windows x64 releases include:

- `tcforge-windows-x64.zip`: portable package
- `tcforge-windows-x64-setup.exe`: unsigned installer that installs `tcforge` and adds it to your user `PATH`

macOS Apple Silicon releases include:

- `tcforge-macos-arm64.tar.gz`: portable package

The first release builds are unsigned. Windows may show a SmartScreen warning, and macOS may require approval in Privacy & Security before running the downloaded binary.

## Quick Start

Most users should start with:

```powershell
tcforge fix input.MP4
```

This will:

- infer the video frame rate
- auto-detect whether LTC is on the left or right audio channel
- decode the LTC start timecode
- write a clean `.mov` containing the first video stream plus generated `tmcd`
- leave the original file untouched

The output defaults to:

```text
input_tcforge.mov
```

You can also use the bare-file shortcut:

```powershell
tcforge input.MP4
```

## Common Commands

Inspect a file:

```powershell
tcforge probe input.MP4
```

Scan audio channels for LTC:

```powershell
tcforge probe input.MP4 --scan-ltc --fps 29.97
```

Verify a generated file:

```powershell
tcforge verify input_tcforge.mov
```

Fix one file:

```powershell
tcforge fix input.MP4
```

Fix several files with shared settings:

```powershell
tcforge fix C1315.MP4 C1316.MP4 --output-dir .\synced
```

Use per-file settings from a JSON manifest:

```powershell
tcforge fix --manifest jobs.json
```

Advanced/manual write:

```powershell
tcforge write input.MP4 --channel right --fps 29.97 --clean --output synced.mov
```

## Output Modes

Default `fix` mode writes a clean NLE-friendly file:

- first video stream only
- generated `tmcd` timecode metadata
- no audio LTC track
- no camera timed metadata streams

This clean output has been validated in Premiere Pro 2026 and DaVinci Resolve.

If you want to keep all original streams and only add timecode metadata:

```powershell
tcforge fix input.MP4 --preserve
```

The lower-level `write` command preserves streams by default and uses `--clean` when you want the clean output shape.

## Verification

Use `verify` after a write/fix run when you want a quick confidence check before importing into an NLE:

```powershell
tcforge verify input_tcforge.mov
```

It checks that:

- the file is readable by `ffprobe`
- a video stream is present
- timecode metadata is present
- a `tmcd`/data timecode track is present
- video fps can be detected
- audio streams are absent

Audio streams are reported as a warning, not a failure, because `--preserve` outputs are valid but may still contain the original audio LTC track. Use `--json` for machine-readable verification output.

## Batch and Manifests

For multiple files that share the same options:

```powershell
tcforge fix clip1.MP4 clip2.MP4 clip3.MP4 --output-dir .\synced
```

For per-file overrides, use a JSON manifest:

```json
[
  {
    "input": "C1315.MP4",
    "output": "synced/C1315.mov",
    "channel": "auto",
    "clean": true
  },
  {
    "input": "C1316.MP4",
    "output": "synced/C1316.mov",
    "fps": "23.976",
    "channel": "right",
    "clean": true
  }
]
```

Supported manifest fields:

- `input`
- `output`
- `fps`
- `channel`
- `clean`
- `preserve`
- `overwrite`
- `allow_fps_mismatch`

## Useful Flags

- `--channel auto|left|right|1|2`: choose the LTC audio channel. Defaults to `auto`.
- `--fps <fps>`: manually set timecode frame rate. `fix` can infer this for common rates.
- `--output <file>`: output path for a single input.
- `--output-dir <folder>`: output folder for multiple inputs.
- `--preserve`: keep original streams in `fix` mode.
- `--clean`: write clean video plus generated `tmcd` in `write` mode.
- `--overwrite`: replace existing output files.
- `--allow-fps-mismatch`: advanced escape hatch when requested timecode fps intentionally differs from detected video fps.
- `--dry-run`: print planned commands without writing media.
- `--json`: emit machine-readable output.
- `--verbose`: include external command details.

## Troubleshooting

`tcforge` tries to fail with a plain-language reason and a next step.

- `ltc_not_found`: no usable audio LTC was decoded. Run `tcforge probe input.MP4 --scan-ltc --fps <fps>` and check LTC box cabling/audio levels.
- `invalid_media`: `ffprobe` could not read the file. Confirm the file opens in an NLE and is fully copied from the camera card.
- `fps_mismatch`: requested `--fps` does not match detected video fps. Use the detected fps, or pass `--allow-fps-mismatch` only for intentional edge cases.
- `output_exists`: the output file already exists. Choose another output name or pass `--overwrite`.
- `output_not_writable`: the output folder cannot be written. Check permissions or choose another output location.

## Development

This project currently uses only the Go standard library.

```powershell
go test ./...
go run . probe input.MP4
```

Build local release packages with:

```powershell
.\release\package.ps1 -Platform windows-x64 -Version dev -DependencyDir C:\path\to\deps
```

The dependency directory must contain `ffmpeg`, `ffprobe`, and `ltcdump` for the target platform. Packaged releases include third-party notices and `SHA256SUMS.txt` for bundled files.

## Contributing

The most useful contribution right now is validation from other gear.

If you test another camera, recorder, timecode box, frame rate, NLE, or operating system, please open an issue with:

- camera model
- timecode box/model
- file format/container
- frame rate
- which audio channel had LTC
- NLE used for verification
- whether `tcforge probe --scan-ltc` and `tcforge fix` worked

Small sample files are helpful when they can be shared legally.
