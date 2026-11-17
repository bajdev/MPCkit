# MPCkit rename a pad's sample file: work plan

Written 2026-11-17. **Not started.** The decisions marked "to confirm" need bajdev's answer before PR N1 starts. The release version is bajdev's choice and is not set here.

The feature: give a pad's sample a new file name inside the project. The exported copy in the `_[ProjectData]` folder gets the new name, in both folder and zip exports. The original file on the user's disk is never renamed, moved or written to.

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

Proposed by the plan, **to confirm with bajdev before PR N1 starts**:

| # | Topic | Proposal |
| --- | --- | --- |
| 1 | Shared files | Renaming renames the file for every pad that uses it. The dialog lists the other pads ("Also used by A3 and B7") so this is not a surprise. To give one pad its own name, copy it with a separate source: out of scope. |
| 2 | What the user types | The name without the extension. The extension stays as it is (`.wav`), and the dialog shows it after the field. |
| 3 | Sample name on the device | `name` and `layer.sampleName` follow the file name, as at assign: they are always the file name without its extension. There is no separate display name. |
| 4 | Not allowed | The same rules as a project name (`project.ValidName`): not empty, none of `< > : " / \ \| ? *` or control characters, not ending in a dot or a space, not a Windows device name (`CON`, `aux`, `com1`). The name is refused with that message; it is not changed silently. Leading and trailing spaces are trimmed first. |
| 5 | Name already taken | Refused, naming the pad that uses it ("kick.wav is already used by A2"). No automatic numbering: the user chose the name. Compared without case, as at assign, but a sample may change only the case of its own name (`Kick` to `kick`). |
| 6 | Missing source | Renaming is allowed: it does not read the source. Export stays blocked until the source is found, as now. |
| 7 | Length | At most 100 characters, extension included. The card's file system allows 255; 100 leaves room and is far beyond what the device's screen shows. Optional: bajdev loads a kit with a 100-character name and a name with accents (`Café kick`) on the device, and the plan is adjusted if either misbehaves. |
| 8 | Layers | Only the pad's first layer, the one Pad settings shows. Other layers in a project saved on the device are left alone, as the other pad edits do. |
| 9 | Command line | `-rename PAD=NAME` in `mpckit-cli`. |
| 10 | Where it goes in the window | A **Rename** button in the head of Pad settings, beside the file name, opening a small dialog on `Modal.svelte`. The head has spare width and the panel has no spare height (about 2 px at 1024x600, measured for the waveform plan). |

Out of scope: renaming files on the user's disk, separate names per pad for one shared file, renaming several samples at once, and renaming the project (already in the window).

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and pull requests on GitHub, pull request descriptions, the checks before every push, the code rules and the release steps. Paths below are relative to `src/`. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes.
- Never touch a source file. No call in this feature may open a source for writing, rename it or change its folder. The tests check the source's bytes and modification time after an export.
- The rename is all-or-nothing, like the other edits: work out every change first, then apply, so a refused name leaves the project as it was (`TestFailedEditsChangeNothing`).
- The waveform work (`feat/waveform`) changes the same files: `internal/project/pads.go`, `internal/app/app.go`, `TestBoundMethods`, the bindings and `PadPanel.svelte`. Start from the latest `origin/main`. If the waveform pull request is merged while this one is open, rebase, and regenerate the bindings rather than merging them by hand.
- Nothing may add height to the Pad settings panel. The window must still fit without scrolling at 1024x600, and the dialog must fit 960x540.

## PR order

```
decisions confirmed  →  PR N1 feat/rename-sample  →  PR N2 feat/rename-sample-window
```

| PR | Branch | What |
| --- | --- | --- |
| N1 | `feat/rename-sample` | `RenameSample` in the model, `Service.RenameSample`, `-rename` in the CLI, tests |
| N2 | `feat/rename-sample-window` | The Rename button and dialog |

No version change in either. bajdev picks the release.

## PRs and tasks

### PR N1: `feat/rename-sample`

- [ ] `func (p *Project) RenameSample(slot int, name string) error` in `internal/project/pads.go`:
  - The pad must hold a sample. The file to rename is the one in its first layer's `sampleFile` (decision 8).
  - Trim spaces, check the name with `ValidName` (decision 4) and the length (decision 7), then add the old file's extension (decision 2).
  - Refuse a name another pool entry has, in `path` or `name`, without case (decision 5). The entry's own name does not count. The message names a pad that uses the other file.
  - If the new name equals the old one exactly, do nothing and do not mark the project changed.
  - Otherwise set, with the `setter` so a failure changes nothing: `name` and `path` of the entry in both `samples[]` and `tracks[0].samples[]`; `layer.sampleName` and `layer.sampleFile` of every layer, on every pad, that points at the old file (decisions 1 and 3). Then move the `sources` entry to the new key and mark the project changed.
  - No other field changes. No file on disk is touched.
- [ ] `func (s *Service) RenameSample(slot int, name string) (State, error)` in `internal/app/app.go`, through `apply`. Add it to `TestBoundMethods`, regenerate the bindings and commit them.
- [ ] Each pad's state gets `sharedWith`: the labels of the other pads using the same file, for N2's dialog. Empty for an empty pad or an unshared file.
- [ ] `mpckit-cli`: `-rename PAD=NAME`, parsed by its own function like the other edit flags. `-h` gains exactly this entry.
- [ ] Tests in `internal/project`:
  - Rename a pad's sample: both pool lists, the layer, `Pad()` and `PoolFiles()` show the new name; the project is changed.
  - Two pads sharing a file: renaming from either renames both.
  - Each refused case of decisions 4, 5 and 7, and an empty pad, in a table. Each leaves the project as it was (add them to `TestFailedEditsChangeNothing`).
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

- [ ] A **Rename** button in the head of Pad settings, at the right of the file name (decision 10), shown only when the pad holds a sample. It must not make the head taller: check at 960, 1280 and 1440 px wide with a long file name, and let the file name wrap or shorten with an ellipsis rather than push the button onto its own line.
- [ ] `frontend/src/lib/RenameModal.svelte`, on `Modal.svelte`: `role="dialog"`, Escape closes, clicking outside does nothing, focus opens on the name field with its text selected.
  - Title: "Rename sample", with the pad's label.
  - One text field holding the current name without its extension, and the extension shown after it as plain text.
  - Under it: "Also used by A3 and B7. They get the new name too." when `sharedWith` is not empty.
  - One line saying the original file is not renamed: "Only the copy in exported projects gets this name."
  - **Rename** calls `Service.RenameSample` through `run()` and closes on success. Enter in the field does the same. If Go refuses the name, the dialog stays open and shows Go's message under the field, and focus returns to the field. **Cancel** closes.
  - **Rename** is disabled while the field, trimmed, is empty or equals the current name.
- [ ] `ui.rename` in `store.svelte.ts` holds the slot, as `ui.picker` does; `App.svelte` shows the dialog. It closes when the project changes (New, Load).
- [ ] The pad grid and Pad settings show the new name straight away, as after any edit.

**Done when:**

- The checks pass, including `npm run check` and `npm run format:check`.
- A scratch build, not committed, measures the Pad settings panel as the waveform plan describes: no height added at 1280x720 or 1024x600.
- On the laptop:
  1. Rename a pad's sample. The grid and Pad settings show the new name.
  2. Copy that pad to another and rename one of them: the dialog says the other pad shares the file, and both get the new name.
  3. Try a name with `?`, an empty name, and the name of another sample: each is refused with a clear message and the dialog stays open.
  4. Export to a folder and to a zip. The data folder and the zip hold the file under the new name, and the original file in its own folder still has its old name.
  5. Load the exported project: the pad shows the new name and plays.
  6. Everything in the dialog can be done with the keyboard alone.
- Optional device check (decision 7): the exported kit, with one long name and one accented name, loads on the device and each pad shows its name.
