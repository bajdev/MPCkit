# MPCkit rename a pad's sample file: work plan

Written 2026-11-17. **Going ahead** (bajdev, 2026-11-17): every decision below is settled, three of them changed from the first proposal (5, 7 and 10). The release version is bajdev's choice and is not set here.

The feature: give a pad's sample a new file name inside the project, from a name field in Pad settings. The exported copy in the `_[ProjectData]` folder gets the new name, in both folder and zip exports. The original file on the user's disk is never renamed, moved or written to.

Two PRs: N1 adds the rename to the model, the app service and `mpckit-cli`; N2 adds it to the window.

## How it fits what is there

MPCkit already works this way for every sample: it keeps the source file where it is, records a name for the copy, and writes the copy under that name at export. Renaming changes only the recorded name.

- **The pool.** A project holds one entry per sample file, in `samples[]` and again in `tracks[0].samples[]`. Each entry has `name` (the file name without its extension) and `path` (the file name). Each pad's layer points at its file with `layer.sampleFile` and `layer.sampleName`. Assigning a sample sets all of these from the source's file name (format doc, "Assign a sample to a pad").
- **Sources.** `Project.sources` maps each pool file name to the file it is copied from. Export copies `sources[path]` to `<data folder>/<path>` (`internal/export`). The source is only ever read.
- **Shared files.** Two pads that use the same source share one pool entry and one file (`fileForSource`, device-tested). Copy to pad also shares it. So a file name belongs to the file, not to one pad.
- **Name rules at assign.** `poolFileName` replaces characters the device's card or Windows cannot store, and `uniqueFile` adds `001`, `002` and so on when a name is taken, comparing without case.

### Why this needs no device check

A renamed sample gives exactly the project that assigning a source file with that name gives today, and that recipe is device-tested. The only new thing is that users can now type a name, rather than having one from a file, so names may get longer or use characters that no test file has used so far. Decision 7 deals with that.

## Decisions

Settled with bajdev on 2026-11-17. Decisions 5, 7 and 10 were changed from the first proposal:

| # | Topic | Decision |
| --- | --- | --- |
| 1 | Shared files | Renaming renames the file for every pad that uses it. Pad settings names the other pads ("Also used by A3 and B7") so this is not a surprise. To give one pad its own name, copy it with a separate source: out of scope. |
| 2 | What the user types | The name without the extension. The extension stays as it is (`.wav`), and Pad settings shows it after the field. |
| 3 | Sample name on the device | `name` and `layer.sampleName` follow the file name, as at assign: they are always the file name without its extension. There is no separate display name. |
| 4 | Not allowed | The same rules as a project name (`project.ValidName`): not empty, none of `< > : " / \ \| ? *` or control characters, not ending in a dot or a space, not a Windows device name (`CON`, `aux`, `com1`). The name is refused with that message; it is not changed silently. Leading and trailing spaces are trimmed first. |
| 5 | Name already taken | Numbered automatically when renaming, as at assign (`uniqueFile`): renaming to `kick` while another sample is `kick` gives `kick001`. Not many names should clash, and this is the simplest for the user. Compared without case, as at assign, but a sample may change only the case of its own name (`Kick` to `kick`) without a number. The field then shows the name the sample got, which is exactly what export writes. |
| 6 | Missing source | Renaming is allowed: it does not read the source. Export stays blocked until the source is found, as now. |
| 7 | Length | At most 50 characters, extension included: up to 46 typed, plus `.wav`. The card's file system allows 255; 50 is plenty for a sample name. Optional: bajdev loads a kit with a 50-character name and a name with accents (`Café kick`) on the device, and the plan is adjusted if either misbehaves. |
| 8 | Layers | Only the pad's first layer, the one Pad settings shows. Other layers in a project saved on the device are left alone, as the other pad edits do. |
| 9 | Command line | `-rename PAD=NAME` in `mpckit-cli`. |
| 10 | Where it goes in the window | In Pad settings, not in a dialog: the sample's current name is always shown in a text field, which can be edited, with a **Rename** button to save it. The panel has no spare height (about 1 px at 1024x600, measured for the waveform plan), so the field must take the place of what is there, not add a row. |

Out of scope: renaming files on the user's disk, separate names per pad for one shared file, renaming several samples at once, and renaming the project (already in the window).

Still to settle before PR N2 (not needed for N1): with no dialog and no spare height, where Pad settings shows "Also used by A3 and B7" (decision 1) and a refused name's message. The error banner, where other refused edits show, is one option for the message.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and pull requests on GitHub, pull request descriptions, the checks before every push, the code rules and the release steps. Paths below are relative to `src/`. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes.
- Never touch a source file. No call in this feature may open a source for writing, rename it or change its folder. The tests check the source's bytes and modification time after an export.
- The rename is all-or-nothing, like the other edits: work out every change first, then apply, so a refused name leaves the project as it was (`TestFailedEditsChangeNothing`).
- The waveform work (`feat/waveform`) changes the same files: `internal/project/pads.go`, `internal/app/app.go`, `TestBoundMethods`, the bindings and `PadPanel.svelte`. Start from the latest `origin/main`. If the waveform pull request is merged while this one is open, rebase, and regenerate the bindings rather than merging them by hand.
- Nothing may add height to the Pad settings panel. The window must still fit without scrolling at 1024x600 and 960x540.

## PR order

```
decisions confirmed  →  PR N1 feat/rename-sample  →  PR N2 feat/rename-sample-window
```

| PR | Branch | What |
| --- | --- | --- |
| N1 | `feat/rename-sample` | `RenameSample` in the model, `Service.RenameSample`, `-rename` in the CLI, tests |
| N2 | `feat/rename-sample-window` | The name field and Rename button in Pad settings |

No version change in either. bajdev picks the release.

## PRs and tasks

### PR N1: `feat/rename-sample`

- [ ] `func (p *Project) RenameSample(slot int, name string) error` in `internal/project/pads.go`:
  - The pad must hold a sample. The file to rename is the one in its first layer's `sampleFile` (decision 8).
  - Trim spaces, check the name with `ValidName` (decision 4) and the length (decision 7), then add the old file's extension (decision 2).
  - If the new name equals the old one exactly, do nothing and do not mark the project changed.
  - If another pool entry has the name, in `path` or `name`, without case, number it with `uniqueFile` as at assign (decision 5). The entry's own name does not count, so a case-only change keeps its number-free name.
  - Otherwise set, with the `setter` so a failure changes nothing: `name` and `path` of the entry in both `samples[]` and `tracks[0].samples[]`; `layer.sampleName` and `layer.sampleFile` of every layer, on every pad, that points at the old file (decisions 1 and 3). Then move the `sources` entry to the new key and mark the project changed.
  - No other field changes. No file on disk is touched.
- [ ] `func (s *Service) RenameSample(slot int, name string) (State, error)` in `internal/app/app.go`, through `apply`. Add it to `TestBoundMethods`, regenerate the bindings and commit them.
- [ ] Each pad's state gets `sharedWith`: the labels of the other pads using the same file, for N2. Empty for an empty pad or an unshared file.
- [ ] `mpckit-cli`: `-rename PAD=NAME`, parsed by its own function like the other edit flags. `-h` gains exactly this entry.
- [ ] Tests in `internal/project`:
  - Rename a pad's sample: both pool lists, the layer, `Pad()` and `PoolFiles()` show the new name; the project is changed.
  - Two pads sharing a file: renaming from either renames both.
  - Each refused case of decisions 4 and 7, and an empty pad, in a table. Each leaves the project as it was (add them to `TestFailedEditsChangeNothing`).
  - A name another sample has, in any case, is numbered (`kick001`), and a second clash gives `kick002`.
  - A case-only rename of the sample's own name works.
  - A pad loaded from `testdata/device/Demo02.xpj`, renamed, then exported and loaded again, keeps the new name, and the data folder holds the file under the new name and not the old one.
  - After that export, the source file has the same bytes, name and modification time as before.
  - Rename, then assign the same source to another pad: the new pad reuses the renamed entry (`fileForSource`) and does not add a second copy.
  - A WAV export of a project with no rename stays byte-identical: `TestGoldenMpckitT1` and `TestExportMatchesCLIRecipe` pass unchanged.
- [ ] Tests in `internal/app`: `RenameSample` through the service, and `sharedWith` in the state.

**Done when** the tests pass, the bindings hold `RenameSample`, and this works on the server:

```
mpckit-cli -name RenameTest -out /tmp/rename -pad A1=<a WAV> -copy A1:A2 -rename A2="Big kick"
mpckit-cli -load /tmp/rename/RenameTest.xpj -name RenameTest2 -out /tmp/rename2
```

In both exports, pads A1 and A2 point at `Big kick.wav`, the data folder holds only that file, and the source WAV is unchanged.

### PR N2: `feat/rename-sample-window`

Rewritten on 2026-11-17 for decision 10: a field in Pad settings, no dialog. The two points under "Still to settle" are settled before this PR starts.

- [ ] In Pad settings, when the pad holds a sample, the file name shown today becomes a text field holding the name without its extension, with the extension shown after it as plain text and a **Rename** button beside it (decision 10). It must not make the panel taller: check at 960, 1280 and 1440 px wide with a long file name.
  - **Rename** calls `Service.RenameSample` through `run()`. Enter in the field does the same. Escape puts the current name back.
  - **Rename** is disabled while the field, trimmed, is empty or equals the current name.
  - The field follows the pad: selecting another pad, or a rename that numbered the name (decision 5), shows the name the sample has.
  - The note that the original file is not renamed ("Only the copy in exported projects gets this name.") goes where it adds no height, for example the field's description for screen readers and its tooltip.
- [ ] The pad grid shows the new name straight away, as after any edit.

**Done when:**

- The checks pass, including `npm run check` and `npm run format:check`.
- A scratch build, not committed, measures the Pad settings panel as the waveform plan describes: no height added at 1280x720, 1024x600 or 960x540.
- On the laptop:
  1. Rename a pad's sample. The grid and Pad settings show the new name.
  2. Copy that pad to another and rename one of them: Pad settings says the other pad shares the file, and both get the new name.
  3. Try a name with `?` and an empty name: each is refused with a clear message, and the field keeps what was typed. Try the name of another sample: it is numbered (`kick001`), and the field shows that.
  4. Export to a folder and to a zip. The data folder and the zip hold the file under the new name, and the original file in its own folder still has its old name.
  5. Load the exported project: the pad shows the new name and plays.
  6. Renaming can be done with the keyboard alone.
- Optional device check (decision 7): the exported kit, with one long name and one accented name, loads on the device and each pad shows its name.
