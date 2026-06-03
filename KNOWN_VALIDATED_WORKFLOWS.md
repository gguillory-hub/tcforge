# Known Validated Workflows

tcforge is a public alpha. This page tracks workflows that have been reported or personally validated so users can judge whether their setup is close to a known-good path.

If you validate another camera, timecode box, frame rate, NLE, or operating system, please open a validation report issue.

## Validated

| Camera / source | Timecode source | Format | FPS | LTC channel | OS | NLE / verifier | Result | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Sony A7IV | Deity TC-1 over 3.5mm audio input | MP4 | 29.97 | Left | Windows | Premiere Pro 2026, DaVinci Resolve, `tcforge verify` | Worked | Clean `.mov` output with generated `tmcd` timecode validated. |
| Sony FX6 | Embedded camera timecode reference | MXF | Reported by file | n/a | Windows | `ffprobe`, `tcforge verify` reference checks | Reference only | Used as a known embedded-timecode comparison file. |

## Still Wanted

- Tentacle Sync, UltraSync, Atomos, Ambient, and other LTC boxes
- Canon, Panasonic, Fuji, Nikon, Blackmagic, RED, ARRI, and other camera files
- More frame rates, especially drop-frame and mixed-rate edge cases
- Additional NLE validation, including Avid Media Composer and Final Cut Pro
- macOS validation after signed/notarized packaging exists
