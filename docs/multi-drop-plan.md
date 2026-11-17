# MPCkit drop several WAVs or a folder: work plan

Written 2026-11-17. **Not started.** The decisions marked "to confirm" need bajdev's answer before PR M1 starts. The release version is bajdev's choice and is not set here.

The feature: drag several WAV files, or a folder, from the file manager onto MPCkit, and they are assigned to pads in one go.

- Drop on a **pad**: the files go to that pad and the pads after it in the same bank.
- Drop on a **bank letter** (A-H): the files fill that bank from pad 1, up to 16.
- A dropped **folder** gives its first 16 WAV files.

Two PRs: M1 does the work in Go and the command line, and handles drops on a pad. M2 makes the bank letters drop targets and updates the texts.

## What is already there

- `handleFileDrops` in `main.go` listens for Wails' `WindowFilesDropped`. It reads `data-slot` from the element dropped on, takes the **first** `.wav` among the dropped paths, assigns it with `Host.AssignSamplePath` and ignores the rest.
- Pad buttons in `SlotGrid.svelte` and the empty-pad box in `PadPanel.svelte` carry `data-file-drop-target` and `data-slot`. The bank buttons in `BankBar.svelte` are not drop targets; they only react to pads dragged inside the window.
- Dropped paths stay in Go. `AssignSamplePath` is on `Host`, which the window cannot call (review plan, S4 and PR 8b). This feature keeps it that way: no bound method takes a path.
- `AssignSample` is all-or-nothing per pad, reuses the pool entry when the same source is assigned again, and refuses a new file once the project has 128 samples (`MaxSamples`).
- The window shows `app.error` for failures and `app.notice` for short success messages. Go can send an error to the window (`eventError`) but not a notice yet.
- Wails reports dropped folders as paths on Linux (GTK file list) and macOS (file names pasteboard). Windows was not checked in Wails' source; M1's laptop test settles it.

## Decisions

Proposed by the plan, **to confirm with bajdev before PR M1 starts**:

| # | Topic | Proposal |
| --- | --- | --- |
| 1 | Filled pads | A drop never replaces a sample except on the pad dropped on, which is replaced as today. Every other file goes to the next **empty** pad. A bank drop fills only empty pads. No confirmation dialog is needed, because nothing is replaced that the user did not drop onto. |
| 2 | Where the files stop | At pad 16 of the bank dropped on. They never spill into the next bank. Files that do not fit are listed in the message. |
| 3 | Order | By file name, ignoring case, with numbers in names compared as numbers (`kick 2` before `kick 10`). The order the file manager sends is not reliable, so it is not used. |
| 4 | Pad order | A1, A2 ... A16: the pads' numbers, which run from the bottom-left of the grid. |
| 5 | Folders | Only the WAV files directly in the folder; subfolders are not entered. Hidden files (names starting with a dot) and links are skipped: the user picked the folder, not each file in it. A link dropped by itself is followed, as today. |
| 6 | Files and folders together | Everything is gathered into one list (dropped WAVs plus each folder's WAVs), sorted as in decision 3, then placed. |
| 7 | "First 16" | The first files that can be used. A file that is not a usable WAV (damaged, compressed, more than two channels, too long) is skipped and named in the message, and the next file takes its pad. |
| 8 | Partial success | Each file is assigned on its own. Files that worked stay assigned when a later one fails. The message lists what was skipped and why. |
| 9 | Sample limit | Assigning stops when the project reaches 128 samples. The message says how many files were not used and why. |
| 10 | Message | One line after a drop with more than one file, as a notice: "Assigned 12 files to A1-A12." Skipped files follow on their own lines, at most 10 named and then "and 5 more". Failures that left nothing assigned show as an error instead. |
| 11 | Non-WAV files | Ignored without being listed, except when nothing usable was dropped: then the current message, "Only WAV files can be assigned to a pad." (When `docs/audio-formats-plan.md` is built, "WAV" there means every supported format.) |
| 12 | Command line | `-fill PAD=PATH` in `mpckit-cli`, where PATH is a WAV file or a folder, using the same code as a drop on that pad. It can repeat. |

Out of scope: dropping onto the Sequences tab, filling several banks from one drop, choosing which files of a folder to use, entering subfolders, and replacing filled pads other than the one dropped on.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and pull requests on GitHub, pull request descriptions, the checks before every push, the code rules and the release steps. Paths below are relative to `src/`. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes.
- No bound method may take a path. `TestBoundMethods` and `frontend/bindings` must not change in either PR.
- Read only what is needed: list a folder's names with `os.ReadDir`, and read WAV headers only for the files about to be assigned, not the whole folder. A folder of thousands of files must not freeze the window.
- No new Go modules or npm packages. The name sort is a small function with its own tests.
- The waveform work (`feat/waveform`) also changes `main.go` and the frontend store. Start from the latest `origin/main`; if the waveform pull request is merged while this one is open, rebase.

## PR order

```
decisions confirmed  →  PR M1 feat/multi-drop  →  PR M2 feat/drop-on-bank
```

| PR | Branch | What |
| --- | --- | --- |
| M1 | `feat/multi-drop` | Gathering, sorting and placing files in `internal/project`; `-fill` in the CLI; several files and folders dropped on a pad; the notice event |
| M2 | `feat/drop-on-bank` | Bank letters as drop targets; texts in the window |

No version change in either. bajdev picks the release.

## PRs and tasks

### PR M1: `feat/multi-drop`

- [ ] In `internal/project` (no Wails), new file `fill.go`:
  - `func GatherWAVs(paths []string) (files []string, ignored int, err error)`: dropped WAV files as they are; for each folder, the WAV files directly in it, skipping hidden files and links (decision 5); other files counted in `ignored` (decision 11). Then sort (decision 3). A path that cannot be read is an error naming it only if nothing else was dropped; otherwise it is skipped.
  - `func (p *Project) Fill(start int, mode FillMode, files []string) FillResult`: places files from `start` to the end of its bank (decision 2). With `FillFromPad`, the first file goes on `start` whatever it holds, and the rest on empty pads (decision 1). With `FillBank`, only empty pads are used, from pad 1 of the bank. Each file goes through `AssignSample`; a refused file is recorded and the next file tries the same pad (decisions 7 and 8). Stops at the sample limit (decision 9).
  - `FillResult` lists the slots assigned, and the files skipped with their reasons, the files that did not fit, and those left over at the sample limit, each with its own reason.
  - The name sort as its own function.
- [ ] `Host.DropFiles(slot int, mode project.FillMode, paths []string) (State, string, error)` in `internal/app`: gathers, fills, and returns the state and the message text of decision 10. One file on a pad keeps today's behaviour and messages exactly.
- [ ] `main.go`: `handleFileDrops` passes every dropped path to `Host.DropFiles` with the slot from `data-slot` and `FillFromPad`. A new `notice` event, registered like the others, carries the success message; the window shows it as `app.notice`.
- [ ] `mpckit-cli`: `-fill PAD=PATH` (decision 12), with its own parse function. `-h` gains exactly this entry. It prints the same message as the window.
- [ ] Tests in `internal/project`, with WAVs written in the test (`writeWAV`) into `t.TempDir()`:
  - The name sort: case, numbers (`2` before `10`, `01` and `1`), names without numbers, accents.
  - A folder of 20 WAVs dropped on A1 of an empty bank fills A1-A16; the other 4 are reported as not fitting.
  - Dropped on A5 with A7 filled: A5, A6, A8, A9 ... are used; A7 keeps its sample.
  - Dropped on a filled A5: A5's sample is replaced.
  - A damaged WAV in the middle is skipped and the next file takes its pad.
  - A folder with a subfolder, a hidden file, a link, a text file and an MP3: only the plain WAVs are used.
  - Files and a folder dropped together are sorted into one list.
  - A project with 126 samples takes 2 more and reports the rest.
  - The same WAV dropped twice shares one pool entry.
  - Nothing usable dropped: no change to the project.
- [ ] Tests in `internal/app` for the message text in each case of decision 10, and that one file on a pad gives the same result as before.

**Done when** the tests pass, the bindings are unchanged, and on the server:

```
mpckit-cli -name FillTest -out /tmp/fill -fill A1=<folder with 20 WAVs> -fill B3=<one WAV>
```

exports a project with A1-A16 and B3 assigned, and prints the names of the 4 files that did not fit.

On the laptop (Windows), and on Linux if possible:

1. Drop 5 WAVs from Explorer onto an empty A1: A1-A5 are filled in name order, and the notice says so.
2. Drop a folder of 20 WAVs onto B1: B1-B16 are filled, and the notice names the 4 left out.
3. Drop 3 WAVs onto a filled pad followed by a filled pad: the dropped-on pad is replaced, the filled pad after it is skipped.
4. Drop one WAV onto a pad: exactly as before.
5. Drop only a text file: "Only WAV files can be assigned to a pad."

### PR M2: `feat/drop-on-bank`

- [ ] `BankBar.svelte`: each bank button gets `data-file-drop-target` and `data-bank`, only on the Pads tab. The highlight while a file is dragged over it matches the pads' (`file-drop-target-active`).
- [ ] `main.go`: a drop on an element with `data-bank` calls `Host.DropFiles` with pad 1 of that bank and `FillBank`. The window switches to that bank, so the user sees where the files went.
- [ ] Dragging pads inside the window over a bank letter still switches bank, as now.
- [ ] Texts: the grid hint in `App.svelte` becomes "... Drop WAV files or a folder on a pad or a bank letter to assign them." The "WAV files only" note stays. The README's usage section says the same.

**Done when:**

- The checks pass, including `npm run check` and `npm run format:check`.
- On the laptop:
  1. Drop a folder of 20 WAVs onto the C bank letter while bank A is shown: the window shows bank C with C1-C16 filled.
  2. Drop 3 WAVs onto a bank letter whose bank has 14 pads filled: the 2 empty pads get the first 2 files, and the notice names the third.
  3. Drag a pad over a bank letter: the bank switches, as before.
  4. On the Sequences tab, the bank letters do not react to dropped files.
