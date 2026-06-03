# Release Signing Notes

tcforge Windows alpha builds can be shipped unsigned while broader validation is gathered. Signed Windows and macOS releases should be treated as separate release-readiness milestones.

## Windows

Goal:

- Sign `tcforge.exe`
- Sign `tcforge-gui.exe`
- Sign the installer EXE
- Sign bundled executable tools only when the license/source and redistribution policy allow it and signing does not obscure their provenance

Recommended behavior:

- Use SHA256 file digest and timestamp digest options with SignTool.
- Store certificates, keys, and passwords only in CI secrets or a secure local certificate store.
- Never commit `.pfx`, private keys, passwords, or signing logs that expose secrets.
- Keep unsigned ZIP artifacts available for technical users who prefer portable tools.

Inno Setup supports invoking a configured sign tool during installer creation. The release workflow should only enable signing when signing secrets are present.

## macOS

Do not publish a public macOS package until it is signed and notarized.

Required track:

- Apple Developer Program enrollment
- Developer ID certificate
- Sign `tcforge`, `tcforge-gui`, `ffmpeg`, `ffprobe`, `ltcdump`, and bundled dylibs
- Submit for notarization with `notarytool`
- Staple the notarization ticket where applicable
- Package as a notarized `.dmg` or polished `.zip`

Unsigned macOS builds previously created quarantine and killed-binary friction. The public macOS release should avoid that path.
