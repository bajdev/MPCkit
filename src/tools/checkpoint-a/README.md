# Device checkpoint A

Projects written by the Go writer (`internal/xpj`, `internal/project`, `internal/export`), for build plan section 7, checkpoint A. The builder is a Go program, so the same command works in cmd, PowerShell and sh. It needs only Go.

From the repo root (in cmd, PowerShell or sh):

```
go run ./tools/checkpoint-a
```

That writes `mpckit-checkpoint-a.zip` in the current folder. To also write the projects unzipped, for example straight onto the device's card (here drive `E:`):

```
go run ./tools/checkpoint-a -folder E:\
```

Options: `-zip FILE` (zip path), `-folder DIR` (also write the projects there), `-force` (replace projects already in that folder). Without `-force`, projects already in the folder are left alone and nothing is written there.

Samples come from `testdata/device/DemoFile_[ProjectData]`. The test tones are generated in `tone.go` and are the same tone as the WAV test pack: 0.8 s at 440 Hz, then a 0.2 s beep an octave higher as an end marker. The plain 44.1 kHz 16-bit mono tone is byte-identical to that pack's control file.

Each risky item is in its own project, so one that fails to load does not hide the others.

## Projects and what to check

### MpckitKit: kit, tuning extremes, choke, lengths (A1, A2, A3, A5)

| Pad | Sample | Setting |
|---|---|---|
| A1 | kick | |
| A2 | snare | |
| A3 | hat 1 | mute group 1 |
| A4 | hat 2 | mute group 1 |
| B1 | bass (C) | none: the reference pitch |
| B2 | bass | -24 semitones |
| B3 | bass | +24 semitones |
| B4 | bass | -90 cents |
| B5 | bass | +90 cents |
| B6 | bass | -24 semitones, -90 cents |
| B7 | bass | +24 semitones, +90 cents |

Sequences: 1 "One Bar" (120 BPM, 1 bar), 2 "Long" (120 BPM, 64 bars).

1. The project loads; A1-A4 and B1 play.
2. B2-B7 show the tuning in the table and sound shifted against B1 (two octaves down/up for B2/B3, just under a semitone for B4/B5).
3. A3 and A4 cut each other off. Note the highest mute group number the device's pad settings offer.
4. Sequence 1 shows 1 bar and sequence 2 shows 64 bars; both loop at their length.

### MpckitMute: high mute group numbers (A3)

A1 and A2 (hats) in group 16; A3 and A4 in group 1.

1. Does the project load?
2. What group does each pad show? Do A1/A2 choke each other, and A3/A4?

The 2026-11-10 run used groups 16 and 32, built from commit `f36cda3`. Since the device's limit turned out to be 16, MPCkit now refuses 32 and this project uses 1 and 16.

### MpckitTempo: BPM extremes (A5)

Kick on A1. Sequences: 1 "Slow" 30 BPM, 2 "Fast" 300 BPM, 3 "Very Slow" 20 BPM, 4 "Very Fast" 999 BPM (2 bars each).

1. Does the project load?
2. What BPM does each sequence show? (20 and 999 are probably outside the device's range: note whether it clamps them, and to what. Also note the device's own BPM range from its tempo control.)

### MpckitWav: extensible WAV headers (section 7, "Not yet tested")

| Pad | File |
|---|---|
| A1 | 44.1 kHz 16-bit mono, plain header (control) |
| A2 | 48 kHz 24-bit stereo, extensible header |
| A3 | 44.1 kHz 16-bit stereo, extensible header |
| A4 | 44.1 kHz 16-bit mono, extensible header |

1. The project loads, and A2-A4 match A1 in pitch and length, with the end beep heard.

## Results

Run on the MPC Sample on 2026-11-10 by bajdev (pack built from commit `f36cda3`). All four projects loaded and passed. Also recorded in the format doc (section 7) and the build plan (section 7).

| Project | Loads | Result |
|---|---|---|
| MpckitKit | yes | All pads, tuning extremes, the group-1 choke and the 1 and 64-bar sequences work as expected. |
| MpckitMute | yes | A1/A2 (group 16) choke each other; A3/A4 (written as group 32) choke each other. Editing A3 or A4's group on the device starts it at 16. |
| MpckitTempo | yes | All four sequences work, including 20 and 999 BPM. |
| MpckitWav | yes | All four pads work and sound the same: extensible headers are accepted. |

Device mute group range: none (0) to 16.
Device BPM range: 20 to 999 (the tempo control's limits).
