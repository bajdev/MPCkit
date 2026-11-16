# MPCkit waveform and start/end editor: work plan

Written 2026-11-14. **Going ahead for 0.6.0** (bajdev, 2026-11-16). PR W1 is done; PRs W2-W5 follow in order, each after the one before it is merged.

The feature: see a pad's sample as a waveform, set where it starts and ends, and zoom in to place both points exactly. The scope document lists it under "2.0 or later" as "Start and end trim, together with a waveform view".

Five PRs. PR W1 fixed a preview bug; it was done as PR 13 of the review plan and released in 0.5.5. PRs W2-W4 build the feature. PR W5 releases it after a check on the device.

## What it adds, and what it costs

Read this before deciding to build it.

| | Today (`main`, 2026-11-14) | Added by PRs W1-W4 (estimate) |
| --- | --- | --- |
| Go, without tests | about 4,200 lines | about 150 lines |
| Go tests | about 3,000 lines | about 200 lines |
| Frontend, with styles | about 2,000 lines | about 550 lines, in 3 new files |
| Bound methods | 21 | 22 (`SetTrim`) |
| Go modules, npm packages | | none |

- The frontend grows by more than a quarter. Nearly all of it is in three new files (`TrimEditor.svelte`, `Waveform.svelte`, `waveform.ts`) that nothing else depends on. Removing the feature later means deleting them and one button.
- The drawing and dragging code cannot be tested on the server. The frontend has no test runner, and adding one needs new packages. Every change to the editor needs a walk-through on the laptop.
- The file format side is low risk: the fields are verified from a device file, the model already reads them, and the preview already plays only the region between them.
- Exit points: stop after W1 (the bug fix, review plan PR 13), or after W2 (start and end can be set from `mpckit-cli`, no editor).

## What is already there

- **File fields, verified** (format doc, "Trim start and end"): `layer.sliceInfo.Start` and `layer.sliceInfo.LoopStart` hold the start frame, `layer.sliceInfo.End` the end frame. Frames are the file's own, at its own sample rate. The audio file is not touched, and `layer.sampleStart` and `sampleEnd` stay 0. In `testdata/device/Demo02.xpj`, pad A5 has start 328 and end 3777; pad A1 is untouched (0 and 8212, the frame count minus 1).
- **Model:** `project.Pad` has `Start` and `End`, and the app's state sends them to the frontend as `start` and `end`.
- **Preview:** `play()` in `frontend/src/lib/audio.ts` plays only the region from `start` to `end`.
- **Decoded audio:** `/audio/<slot>?pcm=1` serves every frame as 32-bit floats, used when the webview cannot decode a format.
- **Copy, move, swap** carry the whole pad, so a pad's start and end travel with it. **Replace** builds the pad from the template, so it resets them. Both are right as they are.
- **Dialogs:** `Modal.svelte` (review plan, PR 12, merged) gives the backdrop, Escape and focus handling.
- **`TestRebuildDemo02`** sets the three slice fields by hand, with a comment that trim is a 2.0 feature. PR W2 replaces that with the real call.

## Problems found while planning

### The preview cuts samples short when sample rates differ (bug in 0.5.3)

`play()` treats `start` and `end` as positions in the decoded buffer. They are frames of the file. The webview decodes every file to the sample rate of the sound output, so the two only match when file and output run at the same rate.

Measured on the VPS (WebKitGTK, 2026-11-14) with the one-second files in `testdata/wav-formats`:

| File | `end` in the project | Decoded at 44.1 kHz | Decoded at 48 kHz |
| --- | --- | --- | --- |
| 22 kHz | 22049 | 44099 frames | 47998 frames |
| 44.1 kHz | 44099 | 44100 | 47999 |
| 48 kHz | 47999 | 44100 | 48000 |
| 96 kHz | 95999 | 44100 | 48000 |

So with no trim set at all:

- On a 44.1 kHz output, a 22 kHz sample plays its first half only.
- On a 48 kHz output (common on Windows laptops), every 44.1 kHz sample loses its last 8%, and a 22 kHz sample loses more than half.
- A file with a higher rate than the output plays in full, but a trimmed region of it starts and ends in the wrong place.

Only the preview is wrong. Exports are not affected. PR W1 (review plan PR 13) fixes it.

### The frontend does not know how long a sample is

The state carries `start` and `end` but not the file's frame count, so nothing in the window can tell whether `end` is the last frame. The editor needs the frame count and the sample rate. PR W1 sends both with the audio, which also gives `play()` what it needs.

### The pad settings panel has no room

Measured with Demo02's pad A1 selected: the panel has 139 px to spare in a 1280x720 window and 2 px in a 1024x600 one. A waveform in the panel would make it scroll in smaller windows. Hence the dialog (decision 1).

### One field the device changed is not understood

On the pad it trimmed, the device wrote `sliceInfo.LoopCrossfadeLength` 0. MPCkit writes -1 on every pad it assigns, which is what the device wrote on the untouched pad. Whether a trimmed pad with -1 behaves the same on the device is not known. PR W5's device check answers it.

## Decisions

Settled with bajdev:

| # | Topic | Decision | Settled |
| --- | --- | --- | --- |
| 1 | Where the waveform goes | In a dialog built on `Modal.svelte`, opened from a button in Pad settings. Not in the panel itself. | 2026-11-14 |
| 2 | Where this plan lives | This file. | 2026-11-14 |
| 3 | Version | The feature ships as 0.6.0 (still beta). | 2026-11-16 |
| 4 | When an edit is saved | The dialog works on its own copy. **Apply** saves start and end in one step; Cancel and Escape drop the changes. | 2026-11-16 |
| 5 | Shortest region | End must be after start: at least 2 frames. | 2026-11-16 |
| 6 | `LoopCrossfadeLength` | Leave the field as it is when start and end change. Revisit only if the device check (W5) shows a problem. | 2026-11-16 |
| 7 | Units | Start and end are shown and typed as frames, with the time in milliseconds beside each. | 2026-11-16 |
| 8 | Stereo | One waveform: at each point, the loudest of the channels. | 2026-11-16 |
| 9 | Trimmed marker on the pad grid | None in this version. | 2026-11-16 |
| 10 | Command line | Add `-trim PAD=START:END` to `mpckit-cli`, so start and end can be tested on the server and used in scripts. | 2026-11-16 |

Out of scope: loop points and loop modes, slicing a sample across pads, snapping to zero crossings, changing the audio file, fades, and undo.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and PRs, PR records, the checks before every push, the code rules and the release steps. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the PR record's Notes.
- Frames are whole numbers in the file's own sample rate, everywhere: in Go, in the bindings, in the state and in the editor. Convert to seconds only at the two places that need it: `play()` and the time shown beside a field.
- The edit recipe in the format doc is not to change. PR W5 changes only that document's scope lists and tables.
- No new npm packages: the waveform is drawn on a `<canvas>` with the 2D context, and dragging uses pointer events.
- Nothing may add height to the Pad settings panel. The window must still fit without scrolling at 1024x600.
- The dialog must fit a 960x540 window, the smallest the layout supports.
- A loaded project can hold a start or end outside the file (an edited or damaged file). The editor clamps what it shows; it never fails to open.

## PR order

```
PR W1 preview-timing  →  PR W2 trim-model  →  PR W3 waveform-view  →  PR W4 trim-editor  →  PR W5 release-0-6-0 (tag v0.6.0)
```

| PR | Branch | What | Needs |
| --- | --- | --- | --- |
| W1 | `preview-timing` | Fix the preview's start and end; send frame count and sample rate with the audio. **Done** as review plan PR 13, released in 0.5.5 | nothing |
| W2 | `trim-model` | `SetTrim` in the model, the app service and `mpckit-cli` | decisions 3-10 confirmed |
| W3 | `waveform-view` | The dialog with the waveform, zoom, scroll and play | W1, and review plan PR 12 merged |
| W4 | `trim-editor` | Markers, fields, Apply and Reset | W2 and W3 |
| W5 | `release-0-6-0` | Device check, documents, version | W4 |

Each PR starts from `main` after the one before it is merged.

## PRs and tasks

### PR W1: `preview-timing`

**Done.** Moved on 2026-11-14 to `docs/review-fixes-plan.md` as **PR 13**, because bajdev would fix the bug whether or not the feature was built. Its tasks are there. It was merged and released in 0.5.5, so PR W3 can use it.

### PR W2: `trim-model`

No window changes.

- [ ] `func (p *Project) SetTrim(slot int, start, end int64) error` in `internal/project/pads.go`:
  - The pad must hold a sample, and its source file must be readable: the frame count comes from `wav.ReadFile` on the source. A missing source is an error that names the file.
  - Require `0 <= start`, `start < end`, `end <= frames - 1`. Error messages give the allowed range in plain words.
  - Write `sliceInfo.Start`, `sliceInfo.LoopStart` (both `start`) and `sliceInfo.End` with the `setter`, so a failure changes nothing (as `SetTune` does). Touch no other field (decision 6).
  - Mark the project dirty.
- [ ] `func (s *Service) SetTrim(slot int, start, end int64) (State, error)` in `internal/app/app.go`, through `apply`. Add it to the list in `TestBoundMethods`, regenerate the bindings and commit them.
- [ ] `mpckit-cli`: `-trim PAD=START:END`, parsed by its own function like the other edit flags. `-h` gains exactly this one entry.
- [ ] Tests:
  - `TestRebuildDemo02` calls `SetTrim(4, 328, 3777)` in place of the three `l.Set` lines, and its list of known differences stays the same.
  - A table test for the limits: negative start, start equal to end, end past the last frame, empty pad, missing source.
  - A case in `TestFailedEditsChangeNothing`.
  - Copy, move and swap keep start and end; Replace resets them to 0 and the new file's last frame.
  - A project trimmed, exported and loaded again has the same start and end.

**Done when** the tests above pass, the bindings hold `SetTrim`, and this works on the server:

```
mpckit-cli -name TrimTest -out /tmp/trim -pad A1=<a WAV> -trim A1=100:2000
mpckit-cli -load /tmp/trim/TrimTest.xpj -name TrimTest2 -out /tmp/trim2
```

with `sliceInfo.Start`, `LoopStart` and `End` of pad A1 equal to 100, 100 and 2000 in both exports.

### PR W3: `waveform-view`

The dialog, read-only: it shows the sample and the current region, zooms, scrolls and plays. No markers to drag yet.

- [ ] Export the loader behind `load()` in `audio.ts`, which since PR 13 gives `{ buffer, frames, rate }`.
- [ ] `frontend/src/lib/waveform.ts`, plain functions with no Svelte in them:
  - `peaks(sample)`: one pass over the decoded buffer giving, for each block of 256 buffer frames, the lowest and highest value across all channels (decision 8).
  - `columns(sample, peaks, from, to, width)`: the lowest and highest value for each of `width` pixel columns covering file frames `from .. to`. It reads the blocks when a column spans 256 frames or more, and the buffer itself when it spans fewer.
  - File frames map to buffer positions by time: buffer frame = file frame x buffer rate / file rate. Keep that in one function.
- [ ] `frontend/src/lib/Waveform.svelte`: a `<canvas>` that draws the columns for a view (`from`, `to`), with the part outside `start .. end` dimmed. It redraws when its size, the view or the region changes, and draws at the screen's pixel ratio so it is sharp on high-density displays. Colours come from the CSS variables in `app.css`.
- [ ] `frontend/src/lib/TrimEditor.svelte`, on `Modal.svelte`: `role="dialog"`, Escape closes, clicking outside does nothing, focus opens on the Close button. Title: the pad's label and file name.
  - Zoom: **Zoom in**, **Zoom out** and **Whole sample** buttons, and the mouse wheel over the waveform, which zooms around the pointer. The closest zoom shows 64 file frames across the view.
  - Scroll: a range input under the waveform moves the view when zoomed in. It is disabled when the whole sample is in view.
  - **Play** plays the region with the pad's tuning, through `play()`.
  - A line of text with the sample's length in frames and seconds and its sample rate.
  - While the sample loads, the dialog says so. If it cannot be loaded, the dialog shows the error and offers only Close.
- [ ] `ui.trim` in `store.svelte.ts` holds the slot being edited, as `ui.picker` does. `App.svelte` shows the dialog. It closes when the project changes (New, Load).
- [ ] A **Start/End** button in the Pad settings row beside Play, Replace and Clear. It is disabled when the sample file is missing. Check the row at 960 px window width: if four buttons do not fit on one line, shorten labels or use the row beside Move and Copy; do not add a row.

**Done when:**

- The checks pass, including `npm run check` and `npm run format:check`.
- A scratch build measures the Pad settings panel as in "The pad settings panel has no room": no height added at 1280x720 or 1024x600.
- On the laptop: the dialog opens for Demo02's A1 and A5 and shows the same shape, with A5's region dimmed outside 328-3777. Zooming with buttons and wheel, scrolling, and Play all work. The dialog fits a window dragged to its smallest size. A long sample (several minutes) opens and zooms without the window freezing for more than a moment.
- With `MpckitW1.xpj`: the 22 kHz and 96 kHz pads show one second, and the region covers the whole waveform.

### PR W4: `trim-editor`

Makes the dialog edit.

- [ ] The dialog keeps its own `start` and `end`, copied from the pad when it opens (decision 4) and clamped to the file.
- [ ] Two markers on the waveform, **Start** and **End**, dragged with pointer events (`setPointerCapture`, so a drag keeps working outside the canvas). A marker cannot pass the other: start stays at least one frame before end. A marker outside the view is not drawn.
- [ ] Two number fields, Start and End, in frames, each with its time in milliseconds beside it (decision 7). The arrow keys step one frame. A typed value outside the allowed range is corrected when the field loses focus. These fields are the keyboard and screen-reader way to do everything the markers do; the canvas is hidden from the accessibility tree.
- [ ] **Go to start** and **Go to end** buttons: zoom to about 2,000 frames across, centred on that marker. This is the quick way to place a point exactly.
- [ ] Play uses the dialog's `start` and `end`, so a change can be heard before it is saved.
- [ ] **Reset** sets start to 0 and end to the last frame, in the dialog only.
- [ ] **Apply** calls `Service.SetTrim` through `run()`, then closes. It is disabled while start and end equal the pad's saved values. **Cancel** replaces Close. Escape cancels without asking.
- [ ] If `SetTrim` fails, the dialog stays open and the error shows in the usual place.

**Done when:**

- The checks pass.
- On the laptop: drag both markers, type both fields, step with the arrow keys, Go to start, Go to end, Play, Reset, Apply, Cancel and Escape all behave as described.
- Set Demo02's A1 to 328 and 3777: it sounds like A5 without the tuning.
- Apply marks the project as changed. Export it, load the export: start and end are kept.
- Copy a trimmed pad: the copy opens with the same start and end. Replace its sample: start and end are reset.
- Everything in the dialog can be done with the keyboard alone.

### PR W5: `release-0-6-0`

- [ ] **Device check** (bajdev, before the version changes). Build a kit with the W4 app holding: an untouched pad; a pad with only the start moved; one with only the end moved; one with both; and a trimmed pad that is tuned and in a mute group. Use one 44.1 kHz and one 48 kHz sample. Load it on the device and record, as `docs/checkpoint-b.md` does:
  1. Does the project load without a message?
  2. Does each pad play the same region as MPCkit's preview?
  3. Does the device's own sample edit screen show the same start and end numbers?
  4. Save the project on the device without changes and send the file back: which fields of the trimmed pads did the device change? In particular `LoopCrossfadeLength` (decision 6).
- [ ] Record the result in `docs/checkpoint-c.md`. If the device needs a field written differently, that is a new PR before the release, with the format doc's recipe updated from the device's file.
- [ ] `docs/xpj-format-and-scope.md`: move start and end trim and the waveform out of "2.0 or later", and update the "Start, end" row of the pad table. Add what the device check showed.
- [ ] `docs/build-plan.md`: the line that says trim is not editable.
- [ ] README: describe the editor in the feature list.
- [ ] Version 0.6.0 in every file the README's "Releasing" paragraph lists. After the merge, tag the merge commit `v0.6.0`.
- [ ] The MPCkit site (`mpckit-site`) gets its own PR for the new release.

**Done when** the device check is recorded with no open problem, the documents agree with the app, About shows 0.6.0, and the tag is on the merge commit.
