# MPCkit drop several WAVs or a folder on an empty bank: work plan

Written 2026-11-17, simplified the same day at bajdev's request. **Not started.** The decisions marked "to confirm" need bajdev's answer before the PR starts. The release version is bajdev's choice and is not set here.

The feature: drag several WAV files, or a folder, from the file manager onto an **empty bank**, and the first 16 fill its pads (C1-C16 for bank C, and so on).

The purpose is to keep it simple: drop a batch of samples into a spare bank, click through them to hear them, move or copy the ones worth keeping to the pads you want, and clear the rest. So it only ever fills an empty bank. It never replaces a sample and never has to work around filled pads.

One PR: `feat/bank-drop`.

## What is already there

- `handleFileDrops` in `main.go` listens for Wails' `WindowFilesDropped`. It reads `data-slot` from the element dropped on, takes the **first** `.wav` among the dropped paths, assigns it with `Host.AssignSamplePath` and ignores the rest.
- Pad buttons in `SlotGrid.svelte` and the empty-pad box in `PadPanel.svelte` carry `data-file-drop-target` and `data-slot`. The bank buttons in `BankBar.svelte` are not drop targets; they only react to pads dragged inside the window.
- Dropped paths stay in Go. `AssignSamplePath` is on `Host`, which the window cannot call (review plan, S4 and PR 8b). This feature keeps it that way: no bound method takes a path.
- `AssignSample` is all-or-nothing per pad, reuses the pool entry when the same source is assigned again, and refuses a new file once the project has 128 samples (`MaxSamples`).
- The window shows `app.error` for failures and `app.notice` for short success messages. Go can send an error to the window (`eventError`) but not a notice yet.
- Moving, copying and clearing pads already exist, which is what the user does after the drop. The planned **Clear bank** button (`docs/clear-bank-plan.md`) suits this use, but this plan does not depend on it.
- Wails reports dropped folders as paths on Linux (GTK file list) and macOS (file names pasteboard). Windows was not checked in Wails' source; the laptop test settles it.

## Decisions

Proposed by the plan, **to confirm with bajdev before the PR starts**:

| # | Topic | Proposal |
| --- | --- | --- |
| 1 | Where to drop | On a bank letter (A-H) whose bank is empty, or on any pad of the bank shown when that bank is empty. Either way the files fill the bank from pad 1, and the window shows that bank. |
| 2 | Bank not empty | Several files or a folder dropped on a bank that holds any sample are refused, and nothing changes: "Bank C is not empty. Drop several files or a folder on an empty bank." |
| 3 | One file on a pad | Unchanged: it is assigned to that pad, replacing what is there, in any bank. |
| 4 | How many | The first 16 usable files. The rest are left out, and the message says how many. |
| 5 | Order | By file name, ignoring case, with numbers in names compared as numbers (`kick 2` before `kick 10`). The order the file manager sends is not reliable, so it is not used. Pads fill in number order, 1 to 16. |
| 6 | Folders | Only the WAV files directly in the folder; subfolders are not entered. Hidden files (names starting with a dot) and links are skipped: the user picked the folder, not each file in it. |
| 7 | Files and folders together | Everything is gathered into one list (dropped WAVs plus each folder's WAVs), sorted as in decision 5, then placed. |
| 8 | Files that cannot be used | A damaged or unsupported WAV is skipped, and the next file takes its pad. Files already assigned stay assigned. |
| 9 | Sample limit | Assigning stops when the project reaches 128 samples, and the message says so. |
| 10 | Message | One line, as a notice: "Assigned 16 files to C1-C16. 7 more were left out." Files skipped as unusable follow on their own lines, at most 5 named, then "and 3 more". If nothing could be assigned, the message is an error instead. Non-WAV files are ignored without a mention, unless nothing usable was dropped: then "Only WAV files can be assigned to a pad." (When `docs/audio-formats-plan.md` is built, "WAV" means every supported format.) |
| 11 | Command line | `-fill BANK=PATH` in `mpckit-cli`, where PATH is a WAV file or a folder, using the same code. The bank must be empty. |

Out of scope: dropping on a bank that holds samples, filling from a chosen pad, spilling into the next bank, entering subfolders, choosing which files of a folder to use, and the Sequences tab.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and pull requests on GitHub, pull request descriptions, the checks before every push, the code rules and the release steps. Paths below are relative to `src/`. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes.
- No bound method may take a path. `TestBoundMethods` and `frontend/bindings` must not change.
- Read only what is needed: list a folder's names with `os.ReadDir`, and read WAV headers only for the files about to be assigned, not the whole folder. A folder of thousands of files must not freeze the window.
- No new Go modules or npm packages. The name sort is a small function with its own tests.
- Other planned features and `feat/waveform` also change `main.go`, `App.svelte` and the frontend store. Start from the latest `origin/main`; rebase if one of them is merged while this pull request is open.

## PR: `feat/bank-drop`

- [ ] In `internal/project` (no Wails), new file `fill.go`:
  - `func GatherWAVs(paths []string) (files []string, ignored int)`: dropped WAV files as they are; for each folder, the WAV files directly in it, skipping hidden files and links (decision 6); other files and unreadable paths counted in `ignored`. Then sort (decision 5).
  - `func (p *Project) FillBank(bank int, files []string) (FillResult, error)`: refuses a bank that holds any sample (decision 2) or a bad bank number, changing nothing. Otherwise assigns files to pads 1-16 of the bank in order, each through `AssignSample`. A refused file is recorded and the next file tries the same pad (decision 8). Stops at 16 pads or the sample limit (decisions 4 and 9).
  - `FillResult`: the slots filled, the files skipped with their reasons, and how many were left out.
  - The name sort as its own function.
- [ ] `Host.DropOnBank(bank int, paths []string) (State, string, error)` in `internal/app`: gathers, fills, and returns the state and the message text of decision 10.
- [ ] `main.go`, `handleFileDrops`:
  - A drop on an element with `data-bank`, or more than one path or any folder dropped on an element with `data-slot`, goes to `Host.DropOnBank` with that bank (decision 1). The window then shows that bank.
  - One WAV file dropped on a pad goes to `Host.AssignSamplePath` as now (decision 3).
  - A new `notice` event, registered like the others, carries the success message; the window shows it as `app.notice`.
- [ ] `BankBar.svelte`: each bank button gets `data-file-drop-target` and `data-bank`, on the Pads tab only, with the same highlight as the pads while a file is dragged over it (`file-drop-target-active`). Dragging pads inside the window over a bank letter still switches bank, as now.
- [ ] `mpckit-cli`: `-fill BANK=PATH` (decision 11), with its own parse function. `-h` gains exactly this entry. It prints the same message as the window.
- [ ] Texts: the grid hint in `App.svelte` becomes "... Drop a WAV file on a pad to assign it, or several files or a folder on an empty bank to fill it." The README's usage section says the same.
- [ ] Tests in `internal/project`, with WAVs written in the test (`writeWAV`) into `t.TempDir()`:
  - The name sort: case, numbers (`2` before `10`, `01` and `1`), names without numbers, accents.
  - A folder of 20 WAVs on an empty bank C fills C1-C16, and 4 are left out.
  - A bank with one sample is refused and nothing changes.
  - A damaged WAV in the middle is skipped and the next file takes its pad.
  - A folder with a subfolder, a hidden file, a link, a text file and an MP3: only the plain WAVs are used.
  - Files and a folder dropped together are sorted into one list.
  - A project with 126 samples takes 2 more and reports the rest.
  - The same WAV dropped twice fills two pads sharing one pool entry.
  - Nothing usable dropped: no change to the project.
- [ ] Tests in `internal/app` for the message text in each case of decision 10.

**Done when** the checks pass, the bindings are unchanged, and on the server

```
mpckit-cli -name FillTest -out /tmp/fill -fill C=<folder with 20 WAVs>
```

exports a project with C1-C16 assigned, and prints that 4 were left out.

On the laptop (Windows), and on Linux if possible:

1. Drop a folder of 20 WAVs from Explorer onto the C bank letter while bank A is shown: the window shows bank C with C1-C16 filled in name order, and the notice says 4 were left out.
2. Drop 5 WAVs onto any pad of an empty bank D: D1-D5 are filled.
3. Drop 5 WAVs onto bank A, which has samples: refused with the message, nothing changes.
4. Drop one WAV onto a filled pad in bank A: it is replaced, exactly as before.
5. Drop only a text file: "Only WAV files can be assigned to a pad."
6. Click through the filled pads to hear them, move two to bank A, and clear the rest. The project is as expected.
7. On the Sequences tab, the bank letters do not react to dropped files.
