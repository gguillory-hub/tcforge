# ltc2meta

`ltc2meta` is a small CLI for turning audio LTC recorded by mirrorless cameras into file timecode metadata that NLEs can read.

The first target workflow is Sony A7IV / A7CII footage with LTC recorded on one audio channel:

```powershell
ltc2meta write input.MP4 --channel right --fps 23.976 --output synced.mov
```

## Status

Early MVP. The tool shells out to existing command-line utilities instead of bundling or reimplementing LTC decoding.

Required on `PATH`:

- `ffmpeg`
- `ffprobe`
- `ltcdump` from `ltc-tools`

## Commands

```powershell
ltc2meta probe input.MP4
ltc2meta write input.MP4 --channel right --fps 23.976 --output synced.mov
ltc2meta batch .\clips --channel right --fps 23.976 --output-dir .\synced
```

Useful flags:

- `--drop-ltc-audio`: omit the first audio stream from the output to avoid re-encoding. For stereo camera audio, this removes the whole stream, not only one channel.
- `--overwrite`: allow replacing an existing output file.
- `--dry-run`: print planned external commands without writing media.
- `--json`: emit machine-readable results.
- `--verbose`: include command details in human output.

## Development

This project currently uses only the Go standard library.

```powershell
go test ./...
go run . probe input.MP4
```

## Notes

The default output recommendation is `.mov`, even when the input is `.mp4`, because QuickTime-style timecode tracks are widely understood by NLEs.

`ltc2meta` does not modify the original input file.

This project shells out to `ltc-tools` and does not copy, modify, or link GPL `ltc-tools` code.
