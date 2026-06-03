---
name: Validation report
about: Report whether tcforge worked with a specific camera, timecode box, frame rate, and NLE.
title: "Validation: "
labels: validation
assignees: ""
---

## Result

- [ ] Worked
- [ ] Partially worked
- [ ] Did not work

## Workflow

- Camera:
- Timecode box / generator:
- File format / container:
- Frame rate:
- LTC audio channel, if known:
- Operating system:
- NLE or verification app:

## tcforge version

Paste the output of:

```powershell
tcforge --version
```

## What you tested

- [ ] GUI scan
- [ ] GUI fix
- [ ] CLI `probe --scan-ltc`
- [ ] CLI `fix`
- [ ] CLI `verify`

## Notes

What happened? If it worked, please include the detected timecode and NLE used for validation.

## Sample file

If you can legally share a short sample file, please say so here. Do not upload private client media publicly unless you have permission.
