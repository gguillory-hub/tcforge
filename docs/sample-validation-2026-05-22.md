# Sample Validation - 2026-05-22

## Files

- `A013C003_260522DM.MXF`: Sony FX6, BNC LTC input.
- `C1315.MP4`: Sony A7IV, Deity TC-1 audio LTC over 3.5mm input.

## Probe Results

- FX6 MXF:
  - Video: 3840x2160 H.264, 29.97 fps.
  - Audio: four mono 24-bit PCM streams.
  - Data: SMPTE-436M ANC stream.
  - Format timecode: `00:06:04;05`.
- A7IV MP4:
  - Video: 3840x2160 H.264 10-bit 4:2:2, 29.97 fps.
  - Audio: stereo 16-bit PCM.
  - Data: Sony timed metadata stream with timecode tag `22:02:07:25`.

## Audio LTC Decode

Extracted A7IV audio channels with ffmpeg and decoded using `ltcdump`.

- Left channel: valid LTC starting at `00:05:22:22`.
- Right channel: invalid/noisy LTC decode.

Working decode command:

```powershell
ltcdump --fps 30000/1001 "$env:TEMP\c1315_left_ltc.wav"
```

## Metadata Write

Created `C1315_tcforge.mov` with stream copy:

```powershell
ffmpeg -y -i C1315.MP4 -map 0 -c copy -timecode 00:05:22:22 -metadata timecode=00:05:22:22 -write_tmcd on C1315_tcforge.mov
```

Output probe showed:

- Video stream timecode tag: `00:05:22:22`.
- New `tmcd` data stream timecode tag: `00:05:22:22`.
- Original Sony timed metadata stream was preserved.

## CLI Validation

Ran the actual CLI command successfully:

```powershell
go run . write C1315.MP4 --channel left --fps 29.97 --output C1315_tcforge_cli.mov --verbose
```

CLI result:

- Status: `ok`.
- Decoded start timecode: `00:05:22:22`.
- Output: `C1315_tcforge_cli.mov`.
- ffprobe confirmed video stream timecode tag: `00:05:22:22`.
- ffprobe confirmed generated `tmcd` stream timecode tag: `00:05:22:22`.

Validated `--clean` mode:

```powershell
go run . write C1315.MP4 --channel left --fps 29.97 --clean --output C1315_clean_cli.mov --verbose
```

CLI result:

- Status: `ok`.
- Mode: `clean`.
- Decoded start timecode: `00:05:22:22`.
- Output: `C1315_clean_cli.mov`.
- ffprobe confirmed exactly one video stream and one generated `tmcd` data stream.
- Both streams carry timecode tag `00:05:22:22`.

Validated automatic channel detection:

```powershell
go run . write C1315.MP4 --fps 29.97 --clean --output C1315_auto_clean.mov --verbose
```

CLI result:

- Status: `ok`.
- Selected channel: `left`.
- Decoded start timecode: `00:05:22:22`.
- Output: `C1315_auto_clean.mov`.
- ffprobe confirmed exactly one video stream and one generated `tmcd` data stream.
- Both streams carry timecode tag `00:05:22:22`.

Validated improved probe with LTC scan:

```powershell
go run . probe C1315.MP4 --scan-ltc --fps 29.97
```

Probe result:

- Existing Sony timed metadata timecode: `22:02:07:25`.
- Audio layout: one stereo PCM stream.
- LTC scan found left channel at `00:05:22:22`.
- LTC scan did not find usable LTC on the right channel.
- Probe recommended `left`.
- Probe warned that decoded LTC differs from existing Sony metadata.

Validated beginner commands:

```powershell
go run . fix C1315.MP4 --fps 29.97 --output C1315_fix_cli.mov --verbose
go run . C1315.MP4 --fps 29.97 --output C1315_shortcut_cli.mov --verbose
go run . fix C1315.MP4 --output C1315_fix_inferred.mov --verbose
go run . C1315.MP4 --output C1315_shortcut_inferred.mov --verbose
```

Beginner commands:

- Inferred `29.97` fps from the primary video stream when `--fps` was omitted.
- Selected the left channel automatically.
- Wrote clean output.
- Decoded start timecode `00:05:22:22`.
- Produced one video stream plus one generated `tmcd` data stream.

Validated user-facing error handling:

```powershell
go run . fix C1315.MP4 --fps 25 --output C1315_bad_fps.mov --json
go run . fix C1315.MP4 --fps 29.97 --channel right --output C1315_wrong_channel.mov --verbose
```

Results:

- FPS mismatch fails before writing with `error_code=fps_mismatch`.
- JSON failures include `error_code`, `error`, and `suggestion`.
- Wrong manual channel fails with a suggestion to use `--channel auto` or run `probe --scan-ltc`.

Validated multi-file shared-options fix:

```powershell
go run . fix C1315.MP4 A013C003_260522DM.MXF --output-dir batch-test --verbose
go run . fix C1315.MP4 A013C003_260522DM.MXF --output-dir batch-json --json
```

Results:

- Multi-file runs continue after individual failures.
- A7IV sample succeeded with `00:05:22:22`.
- FX6 reference failed gracefully with `error_code=ltc_not_found` because it does not contain audio LTC in the expected stereo workflow.
- Summary output reports success/failure counts.
- JSON output returns one `WriteResult` per input.
- Duplicate output paths are rejected within the same run.

Validated JSON manifest support:

```powershell
go run . fix --manifest docs\sample-jobs.json --verbose
go run . fix --manifest docs\sample-jobs.json --json --overwrite
```

Results:

- Manifest jobs can provide per-file `input`, `output`, `fps`, `channel`, and `clean`.
- Omitted `fps` is inferred per job.
- A7IV manifest job succeeded with `00:05:22:22`.
- FX6 manifest job failed gracefully with `error_code=ltc_not_found`.
- JSON output returns one `WriteResult` per manifest job.

## Premiere Pro 2026 Check

`C1315_tcforge_cli.mov` showed `00;05;22;22` in Premiere's Modify Clip > Timecode dialog when Linear Timecode (LTC) was selected.

When Original Timecode was selected, Premiere showed `00;00;00;00`. This means that dialog is not sufficient proof that Premiere accepts the generated `tmcd` metadata as original/source timecode.

Additional test files created:

- `C1315_no_ltc_audio.mov`: removes the audio stream, preserves Sony timed metadata, adds generated `tmcd`.
- `C1315_clean_tmcd.mov`: video-only, removes audio LTC and Sony timed metadata, adds generated `tmcd`.

Both files probe correctly with ffprobe at `00:05:22:22`.

Premiere Pro 2026 successfully displayed `00:05:22:22` in the Source Monitor for `C1315_clean_tmcd.mov` after changing the Source Monitor display to Timecode. Because this file has no audio LTC and no Sony timed metadata stream, this confirms Premiere can read the generated `tmcd` metadata.

## DaVinci Resolve Check

DaVinci Resolve showed the correct start timecode directly in the Media Storage import view:

- `C1315_tcforge_cli.mov`: `00:05:22:22`
- `C1315_tcforge.mov`: `00:05:22:22`
- Original `C1315.MP4`: `22:02:07;25`
- FX6 reference `A013C003_260522DM.MXF`: `00:06:04;05`

This confirms Resolve reads the generated MOV timecode metadata from the CLI output.

## Follow-Up

- Decide whether v0.1-alpha is ready to tag after a README polish pass.
