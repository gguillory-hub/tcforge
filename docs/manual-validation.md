# Manual Validation Checklist

Use short clips first. Keep originals read-only and write outputs to a separate folder.

## Fixture Set

- 3 Sony A7IV clips with LTC on audio.
- 1 Sony A7CII clip with LTC on audio, if available.
- 1 Sony FX6 clip with embedded metadata timecode as a reference.

## Toolchain

```powershell
ffmpeg -version
ffprobe -version
ltcdump --help
ltc2meta probe .\A7IV_TEST.MP4
```

## Single Clip

```powershell
ltc2meta write .\A7IV_TEST.MP4 --channel right --fps 23.976 --output .\synced\A7IV_TEST.mov --verbose
```

Expected:

- Original file is unchanged.
- Output file is created without video re-encode.
- Log shows decoded LTC start timecode.
- Resolve reads the output start timecode.
- Premiere reads the output start timecode.

## Batch

```powershell
ltc2meta batch .\clips --channel right --fps 23.976 --output-dir .\synced --verbose
```

Expected:

- Each supported media file gets a `.mov` output.
- Failed files are reported without hiding successful files.

## Failure Cases

```powershell
ltc2meta write .\A7IV_TEST.MP4 --channel left --fps 23.976 --output .\synced\wrong-channel.mov
```

Expected:

- Wrong channel or no LTC fails clearly.
- Existing output fails unless `--overwrite` is provided.
- Missing `ffmpeg`, `ffprobe`, or `ltcdump` reports the missing tool name.
