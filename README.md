# mpckit

Build kits for the Akai MPC Sample on your computer, then copy them to the device.

[![Build](https://github.com/bajdev/MPCkit/actions/workflows/build.yml/badge.svg)](https://github.com/bajdev/MPCkit/actions/workflows/build.yml)

> **Beta.** mpckit works on the projects it has been tested with, but keep a copy of any project you care about before opening it here.

mpckit is a desktop app for Windows, macOS and Linux. It reads and writes MPC Sample project files (`.xpj`) and their sample folders, so you can lay out pads with a mouse and keyboard and leave the device for making beats.

## What it does

- Start a new project or load one saved on the device.
- Assign WAV files to pads, banks A–H, 16 pads each. Drop files straight from your file manager.
- Move, copy and swap pads, across banks too.
- Tune pads (semitones and cents) and set mute groups.
- Click a pad to hear it, with its tuning and mute-group choking.
- Add sequences and set their name, BPM and length. Recorded notes are kept but not edited.
- Export a project to a folder or a zip, ready to copy to the device.

Not in this version: trimming with a waveform view, level and pan, loop and trigger modes, effects and note editing.

## Download

Get the latest build from [Releases](https://github.com/bajdev/MPCkit/releases).

| System | File |
|---|---|
| Windows 10 / 11 (64-bit) | `mpckit-<version>-windows-amd64.zip` |
| macOS (Apple silicon and Intel) | `mpckit-<version>-macos-universal.zip` |
| Linux (64-bit) | `mpckit-<version>-linux-amd64.tar.gz` |

### First run

The builds are not signed yet, so each system asks once before opening the app.

- **Windows:** if SmartScreen appears, choose **More info**, then **Run anyway**. mpckit uses the Microsoft WebView2 runtime, which comes with Windows 11 and current Windows 10.
- **macOS:** unzip and move `mpckit.app` to Applications. Open it once; when macOS blocks it, go to **System Settings → Privacy & Security** and choose **Open Anyway**.
- **Linux:** extract and run `./mpckit`. It needs GTK 3 and WebKitGTK 4.1 (on Debian and Ubuntu: `sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0`).

## Getting a project onto the device

Export writes two things side by side:

```
MyKit.xpj
MyKit_[ProjectData]/      the samples used by the project
```

Copy both, together, to the device's storage and load `MyKit` on the MPC Sample. The project's name comes from these two names, so rename them only inside mpckit.

## Building from source

You need Go (the version in `src/go.mod`), Node.js 22 or newer, and the Wails v3 CLI at the version in `src/go.mod`. On Linux you also need the GTK and WebKitGTK development packages; `wails3 doctor` lists what is missing.

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0   # match src/go.mod
cd src
wails3 dev          # run with live reload
wails3 build        # build for this system into src/bin/
go test ./...       # run the tests
```

## Repository layout

```
src/      the app: Go code, Svelte frontend, Wails build files, test projects
docs/     project file format notes and the build plan
.github/  CI: builds for all three systems on every pull request and tag
```

## Contributing

Bug reports, device test results and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE).

mpckit is an independent project, not made or endorsed by Akai Professional or inMusic. MPC and MPC Sample are trademarks of their owners.
