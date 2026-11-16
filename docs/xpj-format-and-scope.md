# MPCkit: MPC Sample project format and 0.5.0 / 2.0 scope

Sources: three projects saved on an MPC Sample, in `testdata/device/` of `bajdev/mpckit` (first committed as `demo-projects/`), analysed 2026-11-10.

- `DemoBlank.xpj`: new empty project.
- `DemoFile.xpj`: 6 factory samples on pads A1-A4 and B1-B2, two sequences with recorded notes.
- `Demo02.xpj`: one non-factory sample on pad 1, and a copy on pad 5 with tuning, start and end changed.

Everything below is observed in those files unless marked as an open question. Three projects written with these recipes have been loaded and played on the device (see section 7).

## Decisions so far

- Desktop only (Linux, Windows, Mac). No hosted website.
- UI: Wails v3 with a Svelte frontend. Go owns all file work.
- The `.xpj` reader/writer is its own Go package with no UI code.
- New projects are built from the blank project as a template; edits change only known fields.

## 0.5.0 scope (agreed 2026-11-10 as the 1.0 scope; renamed 2026-11-11: 0.5.0 is a beta release for testing)

Intended use: set up new kits on the desktop, then move to the device to create sequences and tracks.

In:
- Project name.
- Pads, banks A-H x 16: assign a WAV, clear, move and copy between pads (section 3).
- Per pad: tuning by sliders (semitones -24 to +24, cents -90 to +90) and mute group.
- Pad playback preview in the app, applying tuning and mute-group choking. A preview only: no effects.
- Sequences: add, name, BPM, bar length. No note editing.
- Export to a folder or zip: the `.xpj` plus its `_[ProjectData]` folder.
- Audio: WAV only.

UI (mockup agreed 2026-11-10, "MPCkit main screen mockup" artifact):
- Title row with an About link that opens a modal.
- Project name with New, Load, Export zip, Export to folder.
- Two tabs: Pads and Sequences. Each has bank buttons A-H, a 4x4 grid (pad 1 bottom-left) and a settings panel for the selected slot.
- Pads: click to select and play; drag to move, drag onto a filled pad to swap, Alt-drag to copy; drop a WAV to assign.

2.0 or later:
- Start and end trim, together with a waveform view (moved out of 0.5.0: not useful without the waveform). The file fields are already verified (section 3).
- Level and pan (fields not yet confirmed from a device file).
- Trigger and loop modes, envelopes, filters, effects, pad colours.
- Sequence editing (notes), including making notes follow a moved pad.
- Other audio formats, songs.

## 1. Container

| Item | Observed |
|---|---|
| File | gzip stream; about 47-52 KB on disk, about 3 MB uncompressed |
| Header | five text lines: `ACVS`, `1.3.0.12`, `SerialisableProjectData`, `json`, `Linux` |
| Body | one JSON object, `{"data": {...}}`, 66 top-level keys under `data` |
| Layout | one key or value per line, no indentation, `": "` after keys, ASCII only |
| Product | `originalCreatorProductIdentifier` and `lastSavedProductIdentifier` are both `AC50`; `data.version` is 28 |
| Audio | sibling folder `<ProjectName>_[ProjectData]/` holding the sample files; file names keep their spaces |
| Project name | not stored in the file; the name is the file name plus the folder name |
| Empty project | the device creates the data folder even when it holds no samples |

## 2. What the blank project establishes

- All 128 pads are identical, and identical to the empty pads in the other two projects. One blank pad is the template for "clear pad".
- It holds one sequence: slot 0, "Sequence 01", 128 BPM, 2 bars, no events. This is the template for a new sequence.
- Outside pads, samples and sequences, the three projects differ in only three fields:
  - `tracks[0].colour` (a different colour per project)
  - `engineMode` ("Sample" or "Sequence": the mode the device was in when saved)
  - `assignableEnvelopeFollowerAssignments.currentControlIndex` (varies with no pattern; treat as noise)
- The same 11 tracks are always present: `Drum 001` (all 128 pads), `Submix 1`-`Submix 8`, `Out 1/2`, `Out 3/4`.
- Insert effects on the input and `Out 1/2` (LP Filter, Pad-Controlled FXs, Flex Beat Player, Color Compressor) exist even in the blank project. Their settings are opaque text blobs.

## 3. Verified edit recipes

Paths are relative to `data`. `pad[n]` means `tracks[0].program.drum.instruments[n]`, where n = bank x 16 + pad - 1 (pad 5 of bank A is n = 4, confirmed). `layer` means `pad[n].layersv[0]`.

**Check:** applying these recipes to `DemoBlank` reproduces `Demo02` (ignoring recorded notes) with 4 differences in 141,561 values: the track colour, the noise field above, one float rounding (fixed by rule below), and one `LoopCrossfadeLength`.

### Rename project
Rename the `.xpj` and the `_[ProjectData]` folder together. Nothing inside the file changes.

### Assign a sample to a pad
1. Put the audio file in the data folder under the exact name used in `path`.
2. Add a pool entry to both `samples[]` and `tracks[0].samples[]`:
   `{version: 1, name, path, loadImpl: 0, metadata: {tempo: 0.0, rootNote: 60, tune: 0.0, key}}`
3. Set `layer.sampleName` and `layer.sampleFile`.
4. Set `layer.sliceInfo.End` to the frame count minus 1 (true for all 8 samples seen).
5. Set `pad[n].tempo` to 20.0 (the value the device writes when the sample has no tempo).
6. Set `layer.sliceInfo.LoopCrossfadeLength` to -1 (what the device wrote for an untouched non-factory sample).

The device gave the second pad its own copy of the file (`Linn 9000 kick001.wav`, byte-identical to the first) and its own pool entry. Sharing also works: a test project with two pads pointing at one file and one pool entry loaded and played (section 7), so MPCkit can store each file once.

### Move or copy a pad (proposed, not yet device-tested)
A pad's settings hold nothing that depends on its slot: all 128 blank pads are identical. So:
- Copy: duplicate the whole `pad[n]` object into the target slot. Both pads can point at the same file and pool entry (sharing is device-tested).
- Move: copy, then reset the source slot to the blank pad.
- Swap: exchange the two `pad[n]` objects.

Recorded notes follow the slot, not the sample (a note stores 36 + n). Moving a pad that has notes in a sequence changes what those notes play. Decided for 0.5.0: warn before moving a pad that has recorded notes, and leave the notes untouched. Rewriting notes to follow the pad belongs with sequence editing, a later feature.

### Tune a pad
Three fields on the layer, written together:
- `layer.coarseTune` (semitones, integer; demo: 12)
- `layer.fineTune` (cents, integer; demo: 40)
- `layer.pitch` = coarse + fine / 100, stored as a 32-bit float (demo: the device wrote 12.399999618530274; MPCkit writes the shortest form of the same float64, 12.399999618530273)

### Trim start and end
- `layer.sliceInfo.Start` and `layer.sliceInfo.LoopStart` = start frame (demo: 328)
- `layer.sliceInfo.End` = end frame (demo: 3777)
- Trimming does not touch the audio file. `layer.sampleStart` and `sampleEnd` stay 0.

### Rename a sequence / change BPM
One field each: `seq.name`, `seq.bpm` (`seq` = `sequences[].value`).

### Change sequence length
Exactly 26 fields, confirmed by comparing a 2-bar and a 4-bar sequence:
- `seq.lengthBars`, `seq.loopEndBar` = bars
- `seq.lengthPulses`, `seq.loopEndPulses` = bars x 3840 (4/4, 960 pulses per beat)
- `endPulses` and `loopEndPulses` on each of the 11 clips in `seq.trackClipMaps[0]`

### Add a sequence
Copy the blank sequence, set `sequences[].key` to the slot (0-based), then name, BPM and length as above. Only sequences that exist are stored, and the device does not write them in slot order.

## 4. Round-trip rules

- Edit only known fields and write everything else back as read.
- Keep key order. Go maps do not keep order.
- Keep number text. About 300 of 49,000 decimals per file would be re-spelled by a shortest-form encoder.
- New float values that the device stores as 32-bit (tuning, levels) should be rounded to 32-bit before writing.
- Verified: all three files can be parsed and re-written byte-for-byte (after decompression) when key order and number text are kept.
- Suggested Go approach: path-based edits on the raw JSON bytes, or an ordered decoder that keeps numbers as text. Avoid `map[string]any` with `json.Marshal`.

## 5. Properties by group

### Project

| Property | Path | Seen | Proposed |
|---|---|---|---|
| Project name | file name and `_[ProjectData]` folder name | DemoFile | 0.5.0 |
| Master tempo | `masterTempo`, `masterTempoEnabled` | 128.0, false in all three | 2.0 (sequences carry their own BPM) |
| Project key | `key` | "C Major" | 2.0 |
| Info tags | `info.title`, `artist`, `originalArtist`, `album`, `year`, `genre`, `copyright`, `notes` | all empty | 2.0 |
| Quantise | `quantiser.enabled`, `timeDivision`, `swingLevel` | true, 5, 0.0 | 2.0 |
| Master volume | `mixer.volume` | 0.7079 | 2.0 |
| Track colour | `tracks[0].colour` | differs per project | preserve |
| Saved mode | `engineMode` | "Sample", "Sequence" | preserve |

### Sample pool

| Property | Path | Seen | Proposed |
|---|---|---|---|
| Name | `samples[].name` | "Linn 9000 kick" | 0.5.0 |
| File | `samples[].path` | "Linn 9000 kick.wav" | 0.5.0 |
| Load mode | `samples[].loadImpl` | 0 for one-shots and the user sample; 1 for three factory loops | 0.5.0, write 0 |
| Tempo | `samples[].metadata.tempo` | 0.0 for the user sample; 20-114 for factory | 0.5.0, write 0.0 |
| Root note, tune | `samples[].metadata.rootNote`, `tune` | 60, 0.0 | 0.5.0, defaults |
| Key | `samples[].metadata.key` | "C Natural Minor" on the user sample | 0.5.0, default |

### Pad: assignment, tuning, trim (verified)

| Property | Path | Proposed |
|---|---|---|
| Sample | `layer.sampleName`, `layer.sampleFile` | 0.5.0 |
| Slice end | `layer.sliceInfo.End` | 0.5.0, computed |
| Pad tempo | `pad[n].tempo` | 0.5.0, computed |
| Tune | `layer.coarseTune`, `fineTune`, `pitch` | 0.5.0 |
| Start, end | `layer.sliceInfo.Start`, `LoopStart`, `End` | 2.0, with waveform view |

### Pad: playback

| Property | Path | Seen | Proposed |
|---|---|---|---|
| Mute group | `pad[n].whichMuteGroup` | 0 none; 1 on both hats in DemoFile; device offers 0-16 | 0.5.0 |
| Level, pan | `pad[n].mixable.volume`, `pan` | 0.7079 (unity), 0.5 centre | 2.0 (not yet seen changed) |
| Trigger mode | `pad[n].triggerMode` | 0; 3 on the looped pad | 2.0 until mapped |
| Layer loop | `layer.loopMode`, `sliceInfo.LoopMode` | 0/1 | 2.0 until mapped |
| One-shot | `pad[n].zonePlayTime` | 1 on every pad | 2.0 |
| Mono / voices | `pad[n].monophonic`, `polyphony` | true, 1 | 2.0 |
| Mute, solo | `pad[n].mixable.mute`, `solo` | false | 2.0 |

### Pad: sound shaping (all 2.0)

| Property | Path |
|---|---|
| Amp envelope | `pad[n].synthSection.ampEnvelope.Attack`, `Hold`, `Decay`, `Sustain`, `Release`, curves |
| Filter | `synthSection.filterData.value0/1`: `filterType`, `filterCutoff`, `filterResonance`, `filterEnvelopeAmount` |
| Other envelopes | `synthSection.filterEnvelope`, `pitchEnvelope`, `auxEnvelope` |
| LFOs | `synthSection.lfoData.value0/1` |
| Velocity response | `synthSection.velocitySensitivity`, `velocityToPitch`, `velocityToPan`, `velocityToStart` |
| Pad effects | `pad[n].padEffects.padEffectsData.value0-7` (`fxType`, `fxValue`) |
| Sends, routing | `pad[n].mixable.sends[4]`, `audioRoute` |
| Stretch, chop | `pad[n].bpmLock`, `warpEnable`, `stretchPercentage`, `chopProperties.*` |
| Loop points | `layer.loopStart`, `loopEnd`, `loopCrossfadeLength` |
| Velocity range, randomise | `layer.velocityStart`, `velocityEnd`, `pitchRandom`, `VolumeRandom`, `PanRandom`, `OffsetRandom` |
| Pad colours | `tracks[0].program.programPads` |

### Sequence

| Property | Path | Seen | Proposed |
|---|---|---|---|
| Slot | `sequences[].key` | 0, 1 | 0.5.0 |
| Name | `seq.name` | "Intro", "Sequence 02" | 0.5.0 |
| BPM | `seq.bpm`, `seq.tempoEnable` | 105, 110, 128; true | 0.5.0 |
| Length | `seq.lengthBars` plus the 25 dependent fields | 2, 4 | 0.5.0 |
| Time signature | `seq.timeSignatureTrack.timeSignatures[]` | 4/4 | 2.0 |
| Transpose | `seq.transposition` | 0 | 2.0 |
| Notes | `...eventList.events[]` type 3: `time`, `note`, `velocity`, `length`, `probability`, `ratchet` | up to 32 notes | preserve |
| Automation | events type 1: `automation.parameter`, `note`, `value` | parameter 131 only | preserve |

### Songs, mixer, controller state

| Property | Path | Proposed |
|---|---|---|
| Songs | `songs[0-31]`: `name`, `ignoreTempo`, `items` | 2.0 |
| Submix and output tracks | `tracks[1-10]`, `mixer.submixes`, `mixer.outputs` | preserve |
| Insert effects | `...mixable.inserts.effects[].plugin` (`description`, `state` blob) | preserve |
| Q-Link, XY, crossfader assignments | `qlink*`, `assignable*` | preserve |
| Scenes, snapshots | `scene`, `parameterSnapshotterData` | preserve |
| UI state | `tui*`, `mpcControlSurfaceBehaviour`, `padPerformSettings`, `arpeggiatorProperties` | preserve |

## 6. Audio files

- Seen: 44.1 kHz, 16-bit mono (user sample), 16-bit and 24-bit stereo (factory).
- Every file saved by the device carries extra chunks: `atem` (JSON: default slice, bar count, key), `smpl`, and on factory files `meta`. The non-factory kick has `atem` and `smpl` too, so the device adds them itself when it saves.
- The sample's key in the project (`C Natural Minor`) matches the `atem` chunk in its file.
- Device-tested 2026-11-10 with plain WAVs placed straight into the project folder: 8, 16, 24 and 32-bit integer and 32-bit float; mono and stereo; 22.05, 44.1, 48 and 96 kHz all loaded and played at the correct pitch and full length. The device converts on load, so MPCkit does not need to. `sliceInfo.End` is in the file's own frames at its own sample rate. Details and remaining unknowns are in the build plan, section 7.

## 7. Device tests

`mpckit-device-tests.zip` holds three projects built from `DemoBlank` with the recipes above. Each has the kick on A1, the kick tuned +5 on B1, sequence 1 "Verse" (96 BPM, 8 bars) and sequence 2 "Chorus" (96 BPM, 4 bars).

| Project | Difference | Answers |
|---|---|---|
| `MpckitT1` | device-saved WAVs, one file per pad | does the device load a file written by our writer? |
| `MpckitT2` | plain WAVs with the Akai chunks removed | can users bring any ordinary WAV? |
| `MpckitT3` | both pads point at one file and one pool entry | can pads share a sample? |

Results (2026-11-10, on the device): all three projects loaded, and pads A1 and B1 played in each. So the writer's output is accepted, plain WAVs without Akai chunks work, and two pads can share one file and one pool entry.

Tuning confirmed: B1 was written as +5 semitones and the device shows +5.

Sequences confirmed: the device shows "Verse" (96 BPM, 8 bars) and "Chorus" (96 BPM, 4 bars).

Project name confirmed: the device shows `MpckitT1`, taken from the file and folder name.

### Checkpoint A (2026-11-10)

Four projects written by the Go writer (`tools/checkpoint-a`, built from commit `f36cda3`). All loaded and passed on the device:

- **Kit** (`MpckitKit`): samples on A1-A4 and B1 play. Tuning at -24 and +24 semitones, -90 and +90 cents, and both combined (-24/-90, +24/+90) loads and plays as expected. Two hats in mute group 1 choke each other. Sequences of 1 and 64 bars work.
- **Mute groups** (`MpckitMute`): the device offers none and 1-16. Pads written with group 16 choke each other; so do pads written with group 32, though editing such a pad's group on the device starts it at 16. MPCkit writes 0-16 only.
- **Tempo** (`MpckitTempo`): sequences at 20, 30, 300 and 999 BPM all work. The device's tempo control runs from 20 to 999, and MPCkit uses the same range.
- **WAV headers** (`MpckitWav`): 48 kHz 24-bit stereo, 44.1 kHz 16-bit stereo and mono, all with the extensible header, play the same as a plain-header control.

Every item on the original 0.5.0 list (project name, pad sample assignment, sequence name, BPM, bar length, adding a sequence) plus pad tuning has now been written by these recipes and accepted by the device.

### Checkpoint B (2026-11-11)

Five projects made from DemoFile in the app, exported, and loaded on the device (details and results in `docs/checkpoint-b.md`). All loaded and worked:

- **Unchanged round trip** (B1): plays as DemoFile.
- **Moving a pad with notes** (B2): the sample plays from its new pad; the notes stay on the old slot, as expected.
- **Copy across banks, swap pads with notes** (B3): as expected.
- **Remove the current sequence, add one in bank B** (B4): works; Sequence 02 is selected on open (see open question 7).
- **Shorten a sequence with notes** (B5): the notes past the new end are kept and play again once the sequence is lengthened (open question 4).

## 8. Open questions

1. Device-test a moved and a copied pad.
1a. ~~Confirm the tuning limits on the device.~~ Confirmed 2026-11-10 (checkpoint A): -24 to +24 semitones and -90 to +90 cents.
2. Level and pan have not been seen changed; confirm they are written to `pad[n].mixable`.
3. `triggerMode` values (0 and 3 seen) and how they pair with `loopMode`.
4. ~~When a sequence with notes is shortened, are notes past the new end kept or removed?~~ Answered 2026-11-11 (checkpoint B5): kept. They don't play while the sequence is shorter, and play again when it is lengthened on the device.
5. Sample rates outside 22.05-96 kHz and 64-bit float are untested. (The extensible header was confirmed in checkpoint A; other WAV bit depths and sample rates are confirmed; non-WAV formats are out of 0.5.0.)
6. What key and tempo to write for a sample MPCkit imports (the device detects them; MPCkit cannot yet).
7. Is `currentSequence` a slot number or an index into `sequences[]`? Every file seen has 0, and slot 0 is stored second, so the files cannot tell. MPCkit treats it as a slot (build plan 4.3). Checkpoint B4 (removing the current sequence) worked, with Sequence 02 selected on open, but in that file slot 1 and list position 1 are the same sequence, so it does not tell them apart. Still open.
