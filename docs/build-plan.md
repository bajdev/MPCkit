# MPCkit 0.5.0 build plan

0.5.0 is a beta release for testing. The milestones for 1.0 (including beta testers using pre-1.0 versions) will be added later.

Written 2026-11-10. Companion to `mpckit/xpj-format-and-scope.md` (format reference, verified edit recipes, agreed 0.5.0 scope) and the "MPCkit main screen mockup" design artifact. Read the format doc first; this plan does not repeat the field paths.

## 1. What 0.5.0 is

A desktop app (Linux, Windows, Mac) for building MPC Sample kits on a computer and exporting them to the device.

- Project: new, load, name, export to folder or zip.
- Pads, banks A-H x 16: assign a WAV, replace, clear, move, copy, swap.
- Per pad: tuning (semitones -24 to +24, cents -90 to +90) and mute group.
- Click a pad to hear it, with tuning and mute-group choking applied.
- Sequences, 128 slots: add, remove, name, BPM, bar length. No note editing.
- WAV only.

Not in 0.5.0: start/end trim and waveform, level and pan, loop and trigger modes, envelopes, filters, effects, pad colours, other audio formats, songs, note editing.

## 2. Stack

- Go for all file work.
- Wails v3 (v3.1.0, the latest stable release when M4 started; it needs Go 1.26) for the window, dialogs, file drop and the Go-to-frontend bridge.
- Svelte + TypeScript + Vite for the UI (plain Svelte, not SvelteKit).
- No database, no network access, no settings file in 0.5.0.

Wails pieces this plan relies on (confirm against the installed version at setup):
- Services: Go structs registered with the app; TypeScript bindings generated with `wails3 generate bindings`.
- Dialogs: `app.Dialog.OpenFile()` and `SaveFile()`; a cancelled dialog returns an empty string and no error.
- File drop: `EnableFileDrop: true` on the window, `data-file-drop-target` on each drop element, and a `WindowFilesDropped` event carrying real file paths plus the target element's id. HTML drag-and-drop inside the page keeps working alongside it.
- Setup: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`, then `wails3 doctor`. On a Linux machine without the GTK packages, build the CLI with `CGO_ENABLED=0`.

## 3. Repository layout

```
mpckit/
  main.go                    Wails app entry: window, services, embedded frontend
  internal/
    xpj/                     container + ordered JSON tree (no domain knowledge)
    wav/                     WAV header reader (format, channels, rate, bits, frame count)
    project/                 domain model and every edit operation
    export/                  folder and zip writers
    app/                     Wails services, DTOs, audio route, file-drop handling
  frontend/                  Svelte + TS + Vite
    src/lib/                 PadGrid, BankBar, PadPanel, SequencePanel, AboutModal, stores, audio
  testdata/
    device/                  DemoBlank, Demo02, DemoFile with their data folders (saved on the device)
    accepted/                MpckitT1, T2, T3 with their data folders (written by the prototype, loaded on the device)
  (demo-projects/ moved to testdata/device in M1)
  docs/                      copies of the format doc and this plan
  Taskfile.yml               build tasks from the Wails template
```

Rule: `internal/xpj`, `wav`, `project` and `export` import nothing from Wails and are fully testable with `go test`. Only `internal/app` and `main.go` touch Wails.

## 4. Design decisions

### 4.1 Ordered JSON tree (`internal/xpj`)

The device accepts files only if we stay close to what it writes, so the tree must round-trip byte-for-byte.

- Container: gunzip, split off the five header lines, parse the rest. On write: header + body, gzip with modification time 0 and OS byte 3.
- Parse with `json.Decoder` token by token and `UseNumber()`, building our own tree: objects keep key order (slice of key/value pairs), numbers keep their original text.
- Write in the device's layout: one key or value per line, no indentation, `": "` after keys, no trailing newline. Do not escape `<`, `>`, `&` or `/`.
- New numeric values: integer fields as integers (`4`); float fields always with a decimal point (`96.0`, `20.0`, `5.0`, never `96`). Tuning `pitch` is a 32-bit float widened to 64-bit and printed in shortest form (12.4 is written `12.399999618530273`; the device wrote the same float64 with 17 digits, `12.399999618530274`. Both read back identically, so the shortest form is kept.)
- Float fields written in 0.5.0: `seq.bpm`, `pad.tempo`, `layer.pitch`, and `metadata.tempo` / `metadata.tune` on new pool entries. Everything else we write is an integer or a string.
- Helpers: get/set by path, deep clone of a subtree, insert and remove array elements.

Why not `map[string]any` or a path-patching library: maps lose key order, and we need to insert whole objects (pool entries, sequences) in the device's layout.

### 4.2 Template

`DemoBlank.xpj` is embedded in the binary with `go:embed`. It is the source for:
- New project (the whole file).
- Clear pad (any of its 128 identical pads).
- Add sequence (its single empty sequence).

### 4.3 Project model (`internal/project`)

A `Project` wraps one tree plus things the file does not hold:
- `Name` (file and folder name).
- `SourceDir`: the data folder of a loaded project, empty for a new one.
- `Sources`: for each pool file name, the absolute path it will be copied from at export (a file in `SourceDir`, or a WAV the user picked).
- `Dirty` flag.

Every edit is a method that changes the tree using the recipes in the format doc:

| Operation | Notes |
|---|---|
| `AssignSample(slot, path)` | Reads the WAV header; adds or reuses a pool entry; sets layer name/file, `sliceInfo.End`, pad tempo 20.0, `LoopCrossfadeLength` -1. Replacing an occupied pad clears it first. |
| `ClearPad(slot)` | Replaces the pad with the blank pad; drops the pool entry if no pad still uses it. |
| `CopyPad(from, to)` / `MovePad` / `SwapPads` | Whole-pad copies. Copies share the pool entry (device-tested). |
| `PadHasNotes(slot)` | True if any sequence has a note or automation event on note 36 + slot. Drives the move warning. |
| `SetTune(slot, coarse, fine)` | Writes `coarseTune`, `fineTune`, `pitch` on the layer. Clamps to the 0.5.0 ranges. |
| `SetMuteGroup(slot, n)` | `whichMuteGroup`; 0 is off. |
| `AddSequence(slot)` | Clones the template sequence; name `Sequence NN` (slot + 1); BPM copied from the most recently edited sequence, else 120; 2 bars. |
| `RemoveSequence(slot)` | Refused when it is the last sequence. If it was `currentSequence`, point that at the lowest remaining slot. |
| `SetSequence(slot, name, bpm, bars)` | Length change writes all 26 fields. |

Pool rules:
- Pool name = file name without extension; `path` = file name. The entry goes in both `samples[]` and `tracks[0].samples[]`.
- Same source file on several pads: one entry, one file.
- Different source files with the same file name: add a `001`, `002` suffix, as the device does.
- Entries that came with a loaded project keep their metadata untouched. New entries use `loadImpl` 0, tempo 0.0, root note 60, tune 0.0, key "C Natural Minor" (the values the device wrote for the non-factory sample).
- Limit: 128 samples. Refuse the 129th with a clear message.

Validation at the edge of the model: WAV must be uncompressed (integer PCM or 32-bit float), mono or stereo, at most 20 minutes; `sliceInfo.End` is the file's own frame count minus 1 at its own sample rate; project and sequence names non-empty and free of characters that are illegal in file names on Windows or FAT; bars at least 1; BPM 20-999 (the device's range); mute group 0-16.

### 4.4 Export (`internal/export`)

- Folder: the user picks a destination; write `<Name>.xpj` and `<Name>_[ProjectData]/` with every pool file. Create the data folder even when empty. If either already exists, ask before replacing. Write to a temporary name in the same directory and rename at the end.
- Zip: the same two entries at the zip root.
- Before writing, check every source file still exists; report all missing files in one message.
- There is no separate Save in 0.5.0: Load reads a project, Export writes one.

### 4.5 Playback preview

- Go serves the selected pad's source WAV to the frontend at a local route (the app's asset handler, or the built-in file-server service).
- The frontend decodes it with Web Audio and plays it through a buffer source with `detune` = coarse x 100 + fine (cents). Pitch and speed change together, as on the device.
- For loaded pads that already carry a start/end trim, play only that region so the preview matches the device, even though trim is not editable in 0.5.0.
- Mute-group choking: the frontend tracks which pads are sounding and their mute group. Starting a pad first stops every other sounding pad in the same non-zero group, with a 5-10 ms fade so the cut does not click. Group 0 (off) never chokes. The fade length is our choice, not measured from the device.
- A new click on the same pad restarts it (pads are monophonic on the device).
- No effects.

### 4.6 Frontend state

- Go is the single source of truth. Each service call returns a fresh state object and the Svelte store replaces its contents.
- State object: project name, dirty flag, 128 pads (slot, label, file name, coarse, fine, mute group, has notes), sequences (slot, name, BPM, bars, has notes), sample and sequence counts.
- UI-only state stays in Svelte: active tab, selected bank and slot per tab, About open.
- Unsaved-changes prompt on New, Load and window close when dirty.

## 5. Service API (`internal/app`)

```
New() State
Load() (State, error)                       // open dialog, .xpj filter
SetName(name) (State, error)

AssignSampleDialog(slot) (State, error)     // open dialog, .wav filter
AssignSamplePath(slot, path) (State, error) // used by file drop
ClearPad(slot) State
MovePad(from, to) State
CopyPad(from, to) State
SwapPads(a, b) State
SetTune(slot, coarse, fine) State
SetMuteGroup(slot, group) State

AddSequence(slot) State
RemoveSequence(slot) (State, error)
SetSequence(slot, name, bpm, bars) (State, error)

ExportFolder() (string, error)              // returns the path written
ExportZip() (string, error)
Version() string
```

File drop: each pad button carries `data-file-drop-target` and an id such as `pad-17`; the Go handler for `WindowFilesDropped` takes the first `.wav` path and calls the same code as `AssignSamplePath`, then emits a state-changed event.

## 6. Milestones

Each ends with something testable. Order matters; nothing in the UI starts until the file layer is proven.

**M1. Container and tree.**
Done when: all three device projects load and re-write byte-identical after decompression, in `go test`.

**M2. Edit operations.**
Done when:
- Building from the blank template with the same edits as the prototype produces output identical to `MpckitT1.xpj` and `MpckitT3.xpj` in `testdata/accepted` (these exact files loaded on the device).
- Rebuilding `Demo02` from the blank template matches the device's file except the known noise (`tracks[0].colour`, `assignableEnvelopeFollowerAssignments.currentControlIndex`) and recorded events.
- Unit tests cover clear, copy, move, swap, pool reuse, name collision, the 128 limit, sequence add/remove/length, and `PadHasNotes` against `DemoFile`.

**M3. WAV reader, export, and a small CLI.**
A throwaway command (`go run ./cmd/mpckit-cli`) that builds a project from flags, for device tests without the UI.
Done when: folder and zip exports are correct on disk, and Device checkpoint A passes.

**M4. App shell.**
Wails project, window, embedded Svelte app, the mockup's layout and styling with static data, About modal, generated bindings wired to a store.
Done when: `wails3 dev` shows the two tabs driven by a `New()` state from Go.

**M5. Pads tab.**
Assign by dialog and by file drop, replace, clear, tuning sliders, mute group, playback preview with tuning and mute-group choking, counts in the footer.
Done when: a kit built entirely in the UI exports and matches what the CLI produces for the same edits, and two pads in one mute group cut each other off in the preview.

**M6. Move, copy, swap.**
HTML drag between pads (move; swap onto a filled pad; Alt for copy), dragging over a bank button switches bank, the "Move to pad" and "Copy to pad" buttons as the non-drag route, and the recorded-notes warning.
Done when: all three work across banks and the warning appears only for pads with notes.

**M7. Sequences tab.**
Slot grid, add, remove, name, BPM, bars.
Done when: edits match the M2 golden outputs through the UI.

**M8. Load and guard rails.**
Load an existing project, missing-file badges, unsaved-changes prompts, overwrite confirmation, error messages for bad WAVs and limits.
Done when: Device checkpoint B passes.

**M9. Packaging and release.**
Icons, version stamping, builds for the three systems, README with install notes, licence. The About modal and README state that MPCkit is not affiliated with Akai.
Done when: a tagged v0.5.0 build runs on each system.

## 7. Device checkpoints

Run by hand on the MPC Sample. Record results in the format doc.

**A (after M3), files from the Go writer:** built by `go run ./tools/checkpoint-a`; projects, checklist and results table in `tools/checkpoint-a/README.md`.
1. A kit with samples on A1-A4 and B1 loads and plays.
2. Tuning at the extremes: -24, +24 semitones and -90, +90 cents load and show those values.
3. Mute groups: confirm the highest group number the device offers; two pads in one group choke each other.
4. WAV formats: covered by the separate WAV test pack below, which can be run before any Go code exists.
5. A sequence at 1 bar and one at a long length (for example 64 bars); BPM at the device's lowest and highest.

Results (2026-11-10, on the device): all passed. Tuning limits confirmed; mute groups run from none to 16; BPM from 20 to 999; 1 and 64-bar sequences work; extensible-header WAVs play. Details in the format doc, section 7, and `tools/checkpoint-a/README.md`. M3 is done.

**WAV test pack (`mpckit-wav-tests.zip`, built 2026-11-10 with the prototype writer):**

Akai's published spec for the device: file import at 16 or 24-bit and 44.1, 48 or 96 kHz; internal processing at 44.1 kHz, 32-bit float. That describes importing through the device's own browser. MPCkit places files straight into the project folder, so the test checks whether the device still converts them when it loads a project.

Every file is the same one-second sound: 0.8 s at 440 Hz, then 0.2 s an octave higher as an end marker. Each pad should match A1 in pitch and length, and the high beep must be heard at the end.

| Project | Pad | File |
|---|---|---|
| `MpckitW1` | A1 | 44.1 kHz 16-bit mono (control) |
| | A2 | 44.1 kHz 16-bit stereo |
| | A3 | 44.1 kHz 24-bit stereo |
| | A4 | 48 kHz 16-bit stereo |
| | A5 | 48 kHz 24-bit stereo |
| | A6 | 96 kHz 24-bit stereo |
| | A7 | 22.05 kHz 16-bit mono (outside the spec) |
| `MpckitW2` | A1 | 44.1 kHz 16-bit mono (control) |
| | A2 | 44.1 kHz 8-bit mono (outside the spec) |
| | A3 | 44.1 kHz 32-bit integer stereo (outside the spec) |
| | A4 | 44.1 kHz 32-bit float stereo (outside the spec) |

How to read the results:
- Lower pitch and longer than A1: the device played the file as if it were 44.1 kHz. MPCkit must then convert such files at export, or refuse them.
- End beep missing: the slice end is being read in 44.1 kHz frames. MPCkit must then scale `sliceInfo.End` for other sample rates.
- Project fails to load, or a pad is silent: that format is refused; MPCkit rejects it at assign time.

Results (2026-11-10, on the device): both projects loaded and every pad in W1 and W2 played at the correct pitch and through to the end beep, including the four formats outside Akai's spec.

Conclusions:
- The device reads each WAV's own header when it loads a project and converts as needed. MPCkit does not have to convert audio.
- `sliceInfo.End` is counted in the file's own frames at its own sample rate. (The 22.05 kHz pad settles this: read as 44.1 kHz frames it would have stopped halfway, before the beep.)
- Accepted so far: 8, 16, 24 and 32-bit integer and 32-bit float; mono and stereo; 22.05, 44.1, 48 and 96 kHz.

0.5.0 rule for assigning a file: accept any uncompressed WAV (integer PCM or 32-bit float), mono or stereo. Refuse compressed WAVs (ADPCM, MP3-in-WAV), more than two channels, and anything over 20 minutes.

Not yet tested: sample rates above 96 kHz or below 22.05 kHz; 64-bit float (refused in 0.5.0). The extensible header that many audio tools write was confirmed in checkpoint A (PCM inside, 16 and 24-bit, mono and stereo); MPCkit accepts it when it declares PCM or float inside.

**B (after M8), round trips through the app:** step-by-step projects and a results table in `docs/checkpoint-b.md`.
1. Load `DemoFile` (recorded notes), change nothing, export: sequences still play the same on the device.
2. Load `DemoFile`, move A1 to A5 after the warning: A5 plays the kick, the notes still point at A1.
3. Copy a pad across banks; swap two pads.
4. Remove a sequence; add one in a different bank.
5. Shorten a sequence that has notes: note what the device does with notes past the end.

Results (2026-11-11, on the device): all five passed. Notes past a shortened end are kept and play again when the sequence is lengthened. B4 worked but did not settle whether `currentSequence` is a slot or a list position (format doc, open question 7). Details in `docs/checkpoint-b.md`. M8 is done.

## 8. Builds

- Wails apps are built on each target system. Use one machine or CI runner per system; the Linux VPS alone cannot produce the Mac build.
- Linux needs the GTK and WebKitGTK development packages that `wails3 setup` reports; Windows needs the WebView2 runtime (present on current Windows); macOS builds are unsigned unless a developer certificate is added, so the README explains the first-run prompt.
- Version comes from the git tag and shows in the About modal.

## 9. Risks and open questions

| Item | Handling |
|---|---|
| Firmware update changes the format | The header version (`1.3.0.12`) and `data.version` (28) are checked on load; warn when they differ from the tested values. |
| WAV variants the device rejects or plays wrongly | Largely retired by the WAV test pack (section 7): all eleven formats played correctly. Remaining unknowns are listed there. Akai's spec caps a sample at 20 minutes; refuse longer files. |
| The app's preview cannot decode a WAV the device accepts | Each system's webview has its own audio decoder, and 8-bit, 32-bit integer or extensible files may fail in some. Try the webview's decoder first; if it fails, fall back to a Go route that decodes the PCM itself and returns 32-bit float samples for the frontend to play. `internal/wav` therefore needs a small decoder as well as the header reader. |
| Mute group count | Resolved (checkpoint A): none and 1-16. The UI list offers Off, 1-16. |
| Shortening sequences with notes | Resolved (checkpoint B5): the device keeps notes past the end and plays them again when the sequence is lengthened. The app still asks before shortening and leaves the notes in the file. |
| Tuning limits taken from a community toolkit | Resolved (checkpoint A): ±24 semitones and ±90 cents confirmed on the device. |
| Very long or unusual file names | Keep names as they are; reject only characters illegal on Windows/FAT. Revisit if the device truncates. |
| File drop quirks per system | Dialog-based assign is always available as the fallback. |

## 10. Before the first coding session

1. Commit the contents of `mpckit-device-tests.zip` to `testdata/accepted/` and move `demo-projects/` to `testdata/device/`.
2. Copy this plan and the format doc into `docs/`.
3. Confirm the repo's branch rules (direct pushes to main, or pull requests only).
4. Confirm the coding session can reach git.monoexp.com.
5. Licence: MIT, copyright 2026 bajdev (done). Decide the three build machines or runners.

## Appendix: reference writer

The Python prototype that produced the three device-accepted test projects. It is the behaviour the Go code must reproduce (M2). `F` keeps each decimal's original text.

```python
import gzip, json, copy, struct, re

class F(str): pass

def read_xpj(path):
    raw = gzip.decompress(open(path, 'rb').read()).decode('utf-8')
    lines = raw.split('\n', 5)
    return lines[:5], json.loads(lines[5], parse_float=lambda s: F('\x00F' + s))

def dump_body(doc):
    s = json.dumps(doc, indent=0, separators=(',', ': '), ensure_ascii=False)
    return re.sub(r'"\\u0000F([^"]+)"', r'\1', s)

def write_xpj(path, header, doc):
    raw = ('\n'.join(header) + '\n' + dump_body(doc)).encode('utf-8')
    gz = bytearray(gzip.compress(raw, compresslevel=6, mtime=0))
    gz[8] = 0; gz[9] = 3            # XFL 0, OS = Unix
    open(path, 'wb').write(bytes(gz))

def f32(x):
    return struct.unpack('f', struct.pack('f', x))[0]

def sample(name):
    return {"version": 1, "name": name, "path": name + ".wav", "loadImpl": 0,
            "metadata": {"tempo": 0.0, "rootNote": 60, "tune": 0.0, "key": "C Natural Minor"}}

def add_sample(d, name):
    d['samples'].append(sample(name))
    d['tracks'][0]['samples'].insert(0, sample(name))

def assign(d, slot, name, frames, coarse=0, fine=0):
    pad = d['tracks'][0]['program']['drum']['instruments'][slot]
    layer = pad['layersv'][0]
    pad['tempo'] = 20.0
    layer['sampleName'] = name
    layer['sampleFile'] = name + '.wav'
    layer['sliceInfo']['End'] = frames - 1
    layer['sliceInfo']['LoopCrossfadeLength'] = -1
    if coarse or fine:
        layer['coarseTune'] = coarse
        layer['fineTune'] = fine
        layer['pitch'] = f32(coarse + fine / 100)

def set_len(seq, bars):
    pulses = bars * 3840
    seq['lengthBars'] = bars; seq['loopEndBar'] = bars
    seq['lengthPulses'] = pulses; seq['loopEndPulses'] = pulses
    for clip in seq['trackClipMaps'][0]:
        clip['value']['endPulses'] = pulses
        clip['value']['loopEndPulses'] = pulses

# MpckitT1: built from DemoBlank
header, doc = read_xpj('DemoBlank.xpj'); d = doc['data']
add_sample(d, 'Linn 9000 kick'); add_sample(d, 'Linn 9000 kick001')
assign(d, 0, 'Linn 9000 kick', 8213)
assign(d, 16, 'Linn 9000 kick001', 8213, coarse=5)
s0 = d['sequences'][0]
s0['value']['name'] = 'Verse'; s0['value']['bpm'] = 96.0; set_len(s0['value'], 8)
s1 = copy.deepcopy(s0); s1['key'] = 1; s1['value']['name'] = 'Chorus'; set_len(s1['value'], 4)
d['sequences'] = [s1, s0]
write_xpj('MpckitT1.xpj', header, doc)
# MpckitT3 is the same with one pool entry ('Linn 9000 kick') used by both pads.
# MpckitT2 is T1 with the WAV files reduced to fmt + data chunks.
```
