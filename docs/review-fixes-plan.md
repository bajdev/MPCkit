# MPCkit review fixes: work plan

Written 2026-11-12; PR 8b added 2026-11-13; PRs 10-12 (readability) added 2026-11-13; PR 13 (a preview bug found later) added 2026-11-14. Ten PRs (1-9 and 8b), worked one at a time, fix the findings of the MPCkit source code review of `e32802a` (v0.5.1, 2026-11-12). PRs 1-4 ship the security fixes as **0.5.2**; PRs 5-9, including 8b, ship the hardening work as **0.5.3**. Finding IDs (S1, M3, R1 and so on) refer to that review. Each PR below repeats what an agent needs from it, so the plan works without access to the review.

All of it is done: PRs 1-4 shipped as 0.5.2, PRs 5-9 and 8b as 0.5.3, and PRs 10-13 as 0.5.5. PRs 10-12 tidy the code for human review (R2-R7) and change no behaviour. PR 13 fixes a bug in the pad preview that was found on 2026-11-14, after the review.

**Since 2026-11-16 the project is on GitHub** (https://github.com/bajdev/MPCkit), with a fresh history and the app under `src/`. The PRs below were worked in the earlier private repository. Their commit IDs and branch names refer to that history, and paths in this plan are relative to `src/`, as in the other docs. The Decisions and the Guidelines for agents below are updated for GitHub; where they disagree with the repository's `CLAUDE.md` or `CONTRIBUTING.md`, those win.

## Decisions

These were settled with the user on 2026-11-12. Agents follow them and do not reopen them.

| Topic | Decision |
| --- | --- |
| Decompression limit (S2) | 256 MB of decompressed project data (lowered from 512 MB on 2026-11-12, during PR 2: a blank project is 3 MB and 256 MB holds about half a million recorded events). |
| Bound paths (S4) | Approved: `LoadPath` and `AssignSamplePath` stop being bound; export methods stop taking a path. |
| Pad labels (M6) | Always the "A1" format (bank letter, pad number without a leading zero). |
| Releases | 0.5.2 after PRs 1-3, 0.5.3 after PRs 5-8 and 8b, 0.5.5 after PRs 10-13 (0.5.4 was skipped, by choice). PRs in between do not change the version. |
| CI (S6) | Out of scope for the review fixes. Since the move to GitHub (2026-11-16), CI builds and tests every pull request on Linux, Windows and macOS (`.github/workflows/build.yml`). |
| PR records (R8) | R8 was dropped and the record files kept, while the project was private. Since the move to GitHub (2026-11-16), the pull request description replaces them. |
| Formatter (R2) | Approved: add `prettier` and `prettier-plugin-svelte` as dev dependencies and reformat the whole frontend in one PR (PR 11). Decided 2026-11-13. |
| Readability PRs (10-12) | No version bump and no release of their own; they ship in whichever release comes next. Decided 2026-11-13. |
| S5, M9, M10, M11 | Skipped for now (2026-11-13). They stay in the Backlog and need decisions before they can be planned. |
| Linked WAVs (PR 3) | A WAV the user picks that is a link is followed at assign: the pool file is named after the link, and its source is the file the link points to. Export refuses any source that is a link or sits in a linked folder. Decided 2026-11-12. |
| New sample names (PR 3) | Besides `/`, `\` and control characters, the characters the device's FAT card forbids (`: * ? " < > \|`) also become `_`, and a name Windows reserves for a device (`CON.wav`, `aux.wav`, `com1.wav`) gets `_` added (`CON_.wav`). Decided 2026-11-12 and 2026-11-13. |

Open: none. An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes for the reviewer, instead of guessing.

## Guidelines for agents

These apply to every PR. Where they conflict with a PR's tasks, ask before going on.

### Branches and PRs

1. Work on one PR at a time. Start each branch from the latest `origin/main`.
2. Name branches as `CLAUDE.md` asks: short and descriptive, such as `fix/export-windows-paths` or `feat/appimage`. Never push to `main`: it is protected.
3. Push with `git push -u origin <branch>`, then open a pull request with `gh pr create`. bajdev reviews and merges every pull request by hand. **Agents never approve or merge a pull request, their own included,** even when their GitHub access would allow it.
4. Before adding commits to a pushed branch, check that its pull request is still open (`gh pr view <branch>`).
5. After bajdev merges, `git fetch --prune` and delete the local branch. Delete the branch on GitHub only if GitHub has not already done so.
6. End every commit message with the attribution line the session gives. Apart from that line, never put a model name in commits, code or pull requests.

### Pull request descriptions

Every pull request fills in `.github/pull_request_template.md`. Its description holds what the record files used to:

- **What this changes**, in plain words, naming the finding IDs fixed.
- **How it was tested**: the template's checklist, then what ran on the server and numbered steps for bajdev's Windows laptop (cmd or PowerShell commands, not `sh`).
- **Notes for the reviewer**: decisions made along the way, anything left for later, questions for bajdev.

### Checks before every push

CI runs `go vet`, `go test` and a build on Linux, Windows and macOS for every pull request, and its three `Build (...)` checks must pass. Run these on the server too, from `src/`, before pushing; the VPS has Node 24, the GTK 4 and WebKitGTK 6 build files and Xvfb:

```
cd src
export PATH=$PATH:~/go/bin
export GOTOOLCHAIN=go1.27.4   # openSUSE's Go defaults to local and ignores go.mod's toolchain line
go test ./...
go vet ./...
staticcheck ./...
govulncheck ./...
(cd frontend && npm ci && npm run check && npm run build)
(cd frontend && npm run format:check)
wails3 build
```

To check that the app starts, run `timeout 15 xvfb-run -a bin/mpckit` and read its log. Windows-only failures (file locking, paths) do not show on the server; CI's Windows build and the laptop catch them.

### Code

- Match the code around the change: short functions, comments that say why, plain error messages. New identifiers follow the existing naming.
- Every bug fix comes with a test that fails before the fix. Build crafted project files inside the test (decode a file from `testdata/device`, change the tree, write it to `t.TempDir()`), rather than committing new binary fixtures.
- Do not change: `testdata/device/*` and `testdata/accepted/*`; the byte-identical round trip of `.xpj` files (the `xpj` tests prove it); the edit recipes in `docs/xpj-format-and-scope.md`; device limits in `project`.
- No new Go modules or npm packages without asking bajdev.
- After changing a bound method in `internal/app`, regenerate the bindings with `wails3 task common:generate:bindings` and commit `frontend/bindings`.
- Svelte 5: props are live, so a handler that clears the state behind a prop reads the prop first. Wails types Go slices as `T[] | null`; normalise with `?? []`.
- The app is "MPCkit" wherever people see it; file names such as `bin/mpckit` and `mpckit-cli` stay lowercase. The Go module path is `github.com/bajdev/MPCkit`. Keep the "not affiliated with Akai Professional" notes.

### Releases

Releases follow `CLAUDE.md`:

1. When bajdev asks an agent for a release, the agent pushes a `vX.Y.Z` tag (0.x while in beta) on an up-to-date `main`. Agents do not tag on their own initiative.
2. CI builds all three systems, stamps the version from the tag into the About window, and makes a draft release. An agent may create or edit the draft, for example to write its notes.
3. bajdev reviews the draft and publishes it. **Agents never publish a release.**

Some files still carry a fixed version that the tag does not change. Keep them in step in a pull request before the tag:
- `APP_VERSION` in `Taskfile.yml`, the fallback for local builds;
- `version` in `build/config.yml`;
- Windows: `build/windows/info.json` (four places), `wails.exe.manifest` and `nsis/wails_tools.nsh`;
- macOS: `build/darwin/Info.plist` and `Info.dev.plist` (two places each);
- Linux: `build/linux/nfpm/nfpm.yaml`.

## PR order

```
0.5.2: security fixes
  PR 1 harden-load  →  PR 2 limit-decompression  →  PR 3 safe-export  →  PR 4 release-0-5-2 (tag v0.5.2)
0.5.3: hardening
  PR 5 unbind-paths  →  PR 6 atomic-edits  →  PR 7 a1-labels  →  PR 8 build-cleanup  →  PR 8b bound-surface  →  PR 9 release-0-5-3 (tag v0.5.3)
Readability (no release)
  PR 10 go-readability  →  PR 11 frontend-format  →  PR 12 shared-modal
Preview fix (no release of its own)
  PR 13 preview-timing
```

Each PR starts from `main` after the previous one is merged. The order matters in four places: PR 3 reuses PR 1's file name check, PR 6 builds on the nodes PR 1 stores on `Project`, PRs 5 and 7 both change `internal/app` and the frontend, and PR 8b finishes what PR 5 started and must be merged before PR 9. PR 12 starts after PR 11, so the new modal is written in the Prettier style.

## PRs and tasks

| # | Branch | Fixes | Release |
| --- | --- | --- | --- |
| 1 | `harden-load` | S1 (load side), S3, M2 | 0.5.2 |
| 2 | `limit-decompression` | S2 | 0.5.2 |
| 3 | `safe-export` | S1 (export side and assign), M1, M5, R1 (part) | 0.5.2 |
| 4 | `release-0-5-2` | Version 0.5.2 | 0.5.2 |
| 5 | `unbind-paths` | S4 | 0.5.3 |
| 6 | `atomic-edits` | M3, M4 | 0.5.3 |
| 7 | `a1-labels` | M6 | 0.5.3 |
| 8 | `build-cleanup` | S6 (without CI), M7, M8, R1 (rest) | 0.5.3 |
| 8b | `bound-surface` | S4 (rest) | 0.5.3 |
| 9 | `release-0-5-3` | Version 0.5.3 | 0.5.3 |
| 10 | `go-readability` | R3 (Go), R4, R5, R6 | none |
| 11 | `frontend-format` | R2, R3 (Svelte) | none |
| 12 | `shared-modal` | R7 | none |
| 13 | `preview-timing` | preview bug (not in the review) | none; ships in the next release |

### PR 1. Reject malformed projects at load (`harden-load`)

**Why.** Pool file names come from the project file and are joined to folders as they are, so `../../../secret.txt` reads and writes outside the project (S1). Missing fields cause nil-pointer panics: a pool entry without `path` in `project.Load`, a sequence without `bpm` in `Sequences()`. In the app, a panic in the file-drop handler exits MPCkit (S3). The project's main nodes are looked up again on every call with the error dropped (M2).

**Tasks**

- [ ] In `project.wrap`, resolve `samples`, `tracks[0].samples`, `tracks[0].program.drum.instruments` and `sequences` once and keep them on `Project`; `pool()`, `trackPool()`, `pads()` and `sequences()` return the stored nodes.
- [ ] In `wrap` or `Load`, check every pool entry has a string `path`, and every pad layer's `sampleFile` is a string or missing. Each name must be a plain file name: not empty, not `.` or `..`, no `/`, `\` or control characters, and `filepath.Base(name) == name`. Refuse the file otherwise, naming the bad entry.
- [ ] Check each sequence has a numeric `key`, an object `value`, a number `bpm` and `lengthBars`, and a string `name`. Refuse the file with a clear error otherwise.
- [ ] Make `xpj.Node.Int`, `Float` and `Lookup` safe on a nil node (return an error or nil), and replace remaining direct `.Text`, `.Int()` and `.Float()` uses on lookups in `project` with `text()`, `num()` or a checked equivalent. This covers `data.version` in `Load`.
- [ ] In `main.go`, set `PanicHandler` in `application.Options`: log the panic and emit the `error` event with a short message, so a missed case cannot close the app.
- [ ] Add `FuzzLoad` in `internal/project`: seed it with the files in `testdata/device`, write each fuzzed input to `t.TempDir()`, load it and call `Pad`, `Sequences` and `PoolFiles`; none may panic.

**Done when**

- Tests show that loading a project with a pool `path` of `../x.wav`, `a/b.wav` or `..` fails, and so does a project missing `path` or `bpm`, each with an error naming the problem.
- `go test -fuzz=FuzzLoad -fuzztime=2m ./internal/project` finds nothing.
- All existing tests pass unchanged, including the round-trip tests.

### PR 2. Cap decompressed size (`limit-decompression`)

**Why.** `xpj.Read` reads the whole gzip stream into memory. A 1 MB file padded with spaces used 3.1 GB and 10 seconds (S2).

**Tasks**

- [ ] Add `const MaxDecompressed = 256 << 20` in `internal/xpj` with a comment giving the reason.
- [ ] In `Read`, read through `io.LimitReader(zr, MaxDecompressed+1)` and return `xpj: project is larger than 256 MB when decompressed` past the limit.

**Done when**

- A test writes a gzip stream that decompresses to just over the limit and gets the error. Build the stream in the test with `gzip.NewWriter`; do not commit it.
- A file at the limit still decodes.

### PR 3. Safe export (`safe-export`)

**Why.** Export copies each pool file from its source and writes it into the data folder by name (S1, export side). It follows symlinks, so a link in a shared kit's data folder exports whatever it points to. With overwrite set, `Folder` deletes the old export before moving the new one in; if a rename fails, both are lost, although the comment promises otherwise (M1, R1). The `export` package has the lowest coverage, 74.2%, mostly on these paths (M5). Found after PR 1: `AssignSample` names a new pool file after the WAV's own file name, so on Linux or macOS a file called `kick\one.wav` is accepted and exported, and loading that export then fails PR 1's check. On Windows and on the device's card the backslash would also read as a folder separator.

**Tasks**

- [ ] In `collect`, use `os.Lstat` and refuse a source that is a symlink or not a regular file; report it like a missing file.
- [ ] In `Folder` and `writeZip`, refuse a pool name that is not a plain file name (same rule as PR 1), as a second line of defence.
- [ ] In `AssignSample`, make the new pool file name pass `PlainFileName` before `uniqueFile`: replace `/`, `\` and control characters with `_`; if the result is empty, `.` or `..`, use `sample.wav`. The sample name (`stem`) follows from the cleaned file name. Do this in one helper in `project` with a comment giving the reason.
- [ ] Rewrite the overwrite step in `Folder`: rename the old `.xpj` and data folder to temporary names inside the temp folder, rename the new pair into place, then delete the old pair. If the second rename fails, put the old pair back.
- [ ] If `dest` does not exist, return `export folder does not exist: <dest>` instead of the bare `stat` error.
- [ ] Correct the `Folder` comment to say what the code now guarantees.
- [ ] Add tests for: a symlinked source, a bad pool name, overwrite when the new rename fails (make the destination name unusable, for example a directory where the `.xpj` should go), and a missing destination.

**Done when**

- Assigning a WAV named `kick\one.wav` (create it in the test; Linux allows the backslash) gives the pool name `kick_one.wav`, and the exported project loads again without error.
- Export of a project whose data folder holds a symlink fails and writes nothing.
- An overwrite that fails leaves the previous export complete.
- `go test -cover ./internal/export` shows at least 85%.

### PR 4. Release 0.5.2 (`release-0-5-2`)

**Tasks**

- [ ] Change the version to 0.5.2 in every file the README's "Releasing" paragraph lists, and update the stale example in the `Version` comment in `internal/app/app.go`.
- [ ] Add `toolchain go1.27.4` to `go.mod` so builds pick up the `net/http` fix (GO-2026-6859). If Go 1.27.4 is not yet released, stop and ask bajdev.
- [ ] The PR record lists what 0.5.2 fixes, for the beta testers.

**Done when** the laptop build shows "Version 0.5.2" in About and `govulncheck ./...` reports no vulnerabilities. After the merge, tag `v0.5.2` ("MPCkit 0.5.2 (beta)").

### PR 5. Stop exposing raw paths to the webview (`unbind-paths`)

**Why.** Any script in the webview can call `LoadPath`, `AssignSamplePath`, `ExportFolder` and `ExportZip` with any path; `ExportFolder` with overwrite set deletes a folder (S4). The frontend never calls the first two.

**Tasks**

- [ ] Mark `LoadPath` and `AssignSamplePath` with `//wails:ignore`. `main.go`'s file-drop handler still calls `AssignSamplePath` from Go.
- [ ] Have `ChooseExportFolder` and `ChooseZipPath` store the chosen path on the `Service`; `ExportFolder(overwrite)` and `ExportZip(overwrite)` use it and fail if none was chosen. Clear it after a successful export and on `New` and `Load`.
- [ ] Update `frontend/src/lib/store.svelte.ts` (`exportWith`) for the new signatures; regenerate the bindings.
- [ ] Update `internal/app` tests.

**Done when** the generated `service.ts` has no method taking a path, export with and without overwrite still works on the laptop, and `npm run check` is clean.

### PR 6. All-or-nothing edits (`atomic-edits`)

**Why.** `AssignSample` clears the pad and may add a pool entry before steps that can fail, so a failure leaves the pad empty. `SetTune` and `SetMuteGroup` set `Dirty` even when the write failed (M3). `app` writes `p.Dirty` and `export` reads `p.Sources` directly (M4).

**Tasks**

- [ ] In each edit method, do the lookups and checks that can fail first, then change the tree; set `Dirty` only after success.
- [ ] Add `Project.MarkExported()` and `Project.Source(file) (string, bool)`; make `Dirty` and `Sources` unexported and update `app`, `export`, the CLI and the tools.
- [ ] Tests: a failing `AssignSample` (for example a pad whose `layersv` is empty) leaves the pad and the pool as they were, and `Dirty` unchanged.

**Done when** those tests pass and no package outside `project` writes project fields.

### PR 7. One pad label format: "A1" (`a1-labels`)

**Why.** `app.Label` and `slots.ts` give "A01"; `project.SlotName` and the CLI use "A1". Decision: "A1" everywhere (M6).

**Tasks**

- [ ] Keep `project.SlotName` as the one Go implementation; remove `app.Label` and use `SlotName`.
- [ ] Change `slotLabel` in `frontend/src/lib/slots.ts` to "A1" and fix the comments that say "A01" (`app.go`, `SlotGrid.svelte`, `slots.ts`).
- [ ] Do not change sequence names such as "Sequence 01": that is the device's naming, not a slot label.
- [ ] Update tests that expect "A01".

**Done when** the pad grid, pad panel, sequence grid, confirm dialogs and the CLI summary all show "A1" style labels on the laptop, and the layout still fits the window with no scrolling.

### PR 8. Build and repository clean-up (`build-cleanup`)

**Why.** No lockfile, so builds can differ (S6). Server-mode and Docker tasks from the Wails template would expose the bound methods over HTTP (S6). The version lives in several files and `Service.Version()` is unused (M7). Template leftovers hide the real build steps (M8). The `wav` package comment says it does not decode audio, but it does (R1).

**Tasks**

- [ ] Commit `frontend/package-lock.json` from `npm install`. Remove `minimum-release-age` from `.npmrc` (npm warns on it) or ask bajdev whether to keep it for pnpm.
- [ ] Remove the `build:server`, `run:server`, `build:docker`, `run:docker` and `setup:docker` tasks and their entries in the root `Taskfile.yml`.
- [ ] Remove the unused blocks in `build/config.yml`: the `ios` comment block, the `fileAssociations` sample and `other: My Other Data`.
- [ ] Remove the bound `Service.Version()` (the frontend uses `About()`); regenerate the bindings.
- [ ] Fix the `wav` package comment.

**Done when** `npm ci` works from the lockfile, `wails3 build` still builds on the server and the laptop, and `wails3 task --list` shows no server or Docker tasks.

### PR 8b. Bind only what the window needs (`bound-surface`)

**Why.** PR 5 hid `LoadPath` and `AssignSamplePath` with `//wails:ignore`, but that directive only keeps them out of the generated `frontend/bindings`. At runtime Wails 3.1.0 registers every exported method of a bound type by reflection (`getMethods` in `pkg/application/bindings.go`), and reflection cannot see comments. So a script in the window can still call `Call.ByName("<module>/internal/app.Service.LoadPath", "<any path>")`; on 2026-11-13 a scratch build did exactly that and Go loaded the file. The same holds for `AssignSamplePath` and the other Go-only methods (`Dirty`, `SetCloser`, `AudioMiddleware`). Also, when the user declines "Replace existing files?", Go keeps the chosen export target, so a later `ExportFolder(true)` would overwrite it without asking.

**Tasks**

- [ ] Keep `Service` as the type `main.go` binds, with only the methods the frontend calls. Move the Go-only methods (`LoadPath`, `AssignSamplePath`, `Dirty`, `SetCloser`, `AudioMiddleware`) to a second, unbound type in `internal/app` that shares the same state, for example `type Host struct{ s *Service }`, with `NewService` returning both. `Service.Load` and `AssignSampleDialog` call unexported helpers that `Host` also uses. Remove the `//wails:ignore` directives that are no longer needed.
- [ ] Update `main.go` to use `Host` for the close hook, the file-drop handler and the audio middleware.
- [ ] Add `TestBoundMethods` in `internal/app`: list the exported methods of `*Service` by reflection and compare them with an explicit list in the test. Its failure message says that a new bound method must not take a file path and must be added to the list on purpose.
- [ ] Add a bound `CancelExport()` that forgets the chosen targets, and call it from `exportWith` in `store.svelte.ts` when the user declines the replace prompt.
- [ ] Regenerate the bindings; update the `internal/app` tests.

**Done when**

- `TestBoundMethods` passes, and fails if a method such as `LoadPath(string)` is added back to `Service`.
- In a scratch build (temporary lines in `frontend/src/main.ts`, not committed), `Call.ByName(".../app.Service.LoadPath", path)` is rejected with "unknown bound method name". Say in the PR record how this was checked.
- On the laptop: Load, file drop onto a pad, the window-close prompt, export to folder and zip (with and without replace, and declining replace), and sample preview all work as before.

### PR 9. Release 0.5.3 (`release-0-5-3`)

Same steps as PR 4, with 0.5.3. The PR record lists what changed for testers, including the new "A1" labels. After the merge, tag `v0.5.3` ("MPCkit 0.5.3 (beta)").

### Readability PRs: what "no behaviour change" means

PRs 10-12 only rename, move, reformat and comment code. Each one proves that nothing changed:

- `go test ./...` passes with only renamed identifiers changed in the tests; no test is removed or loosened.
- **Exports are byte-identical.** Before the first commit, on `main`, run `go run ./cmd/mpckit-cli -load testdata/device/DemoFile.xpj -out <before>`; after the last, run it again into `<after>`, then `cmp <before>/DemoFile.xpj <after>/DemoFile.xpj` and `diff -r` the two data folders. Note the result in Testing.
- No bound method changes: `TestBoundMethods` passes untouched and `frontend/bindings` does not change.
- The PR record has a **Testing** list for the laptop that walks both tabs and every dialog.

### PR 10. Go readability (`go-readability`)

**Why.** Several names and functions make review harder than it needs to be. `state()` in `internal/app/app.go` uses `p`, `v`, `q`, `f`, `m` and `st` across one long function (R3). `wrap` builds a `Project` in `project` but prefixes an error in `xpj`; `edit` is an error-collecting type in `project` and a lock-and-return method on `Service` (R4). `main()` in `main.go` is 95 lines and mixes window sizing, the close guard, the file-drop handler and the panic handler; the CLI's `init()` in `cmd/mpckit-cli/main.go` registers nine flags as closures in 103 lines (R5). `uniqueFile` in `internal/project/pads.go` keeps file names and sample names in one map, told apart by a `"\x00"` suffix, and `!(bpm >= MinBPM && bpm <= MaxBPM)` in `sequences.go` is written that way to reject NaN with no comment saying so (R6).

**Tasks**

- [ ] Split `state()` into short helpers (for example `limits()`, `missingSources()`, `padState(slot)`, `sequenceStates()`) with descriptive local names.
- [ ] Rename `xpj.wrap` to `wrapErr`, `project.wrap` to `fromFile`, `Service.edit` to `apply`, and the `project.edit` type to `setter` (or names of the same clarity; say which in the record). Update the comments that mention them.
- [ ] In `main.go`, move the window sizing, the close guard and the file-drop handler into named functions, each with the comment that is there now. `main()` should read as a list of steps.
- [ ] In the CLI, give each flag its own named parse function (`parsePadOp`, `parseTuneOp`, ...) and register them from a short table in `init()`. Flag names, usage text and output stay exactly the same.
- [ ] Replace the `"\x00"` keys in `uniqueFile` with two maps, `takenFiles` and `takenNames`.
- [ ] Add a comment to the BPM check saying that it is written this way so NaN fails it.

**Done when** the "no behaviour change" checks pass, `go run ./cmd/mpckit-cli -h` prints the same text as before, and no function in `main.go`, `cmd/mpckit-cli/main.go` or `internal/app/app.go` is longer than 60 lines.

### PR 11. Format the frontend with Prettier (`frontend-format`)

**Why.** Eight markup lines are over 160 characters, the longest 250 (`App.svelte`, `SequencePanel.svelte`, `PadPanel.svelte`); a change to one attribute shows as the whole line changed (R2). There is no formatter, so style drifts. `save()` in `SequencePanel.svelte` uses `s`, `n`, `b` and `l` for slot, name, BPM and bars (R3).

**Tasks**

- [ ] Add `prettier` and `prettier-plugin-svelte` as dev dependencies (approved; take the current releases that `min-release-age` allows) and commit the updated lockfile.
- [ ] Add `frontend/.prettierrc` with `"printWidth": 120`, `"plugins": ["prettier-plugin-svelte"]`, and options that match the current style (double quotes, semicolons, trailing commas); add a `.prettierignore` for `dist`, `bindings` and `node_modules`.
- [ ] Add `"format": "prettier --write ."` and `"format:check": "prettier --check ."` to `frontend/package.json`.
- [ ] Commit the reformat on its own, with nothing else in that commit, so the PR has one large mechanical commit and small reviewable ones.
- [ ] In a separate commit, rename the one-letter variables in `save()` and `remove()` in `SequencePanel.svelte` (`slot`, `newName`, `newBpm`, `newBars`).
- [ ] Watch for whitespace changes in markup: Prettier can move text next to inline elements. Check each tab and dialog on the laptop for changed spacing and fix any with Prettier's own ignore comments only where needed.

**Done when** `npm run format:check` passes, no line under `frontend/src` is longer than 120 characters except where Prettier cannot break it, `npm run check` is clean, and the laptop walk-through shows no visual change.

### PR 12. One modal component (`shared-modal`)

**Why.** `AboutModal`, `ConfirmModal` and `PadPicker` each repeat the same backdrop, dialog box, Escape handling, focus-on-open and styles (R7). A fix to one is easily missed in the others.

**Tasks**

- [ ] Add `frontend/src/lib/Modal.svelte`: backdrop, dialog box and shared styles; props for the title id, an optional description id, `role` (`dialog` or `alertdialog`), `onclose` (called on Escape) and `closeOnBackdrop` (default false); the content as a snippet. The opener passes the element to focus, or `Modal` focuses the first focusable element.
- [ ] Rebuild the three dialogs on it, keeping each one's current behaviour exactly:

| Dialog | role | Escape | Click outside | Focus on open |
| --- | --- | --- | --- | --- |
| About | `dialog` | closes | closes | Close button |
| Confirm | `alertdialog`, with `aria-describedby` | Cancel | nothing | Cancel button |
| Move/Copy to pad | `dialog` | closes | nothing | Bank select |

- [ ] Remove the styles the three no longer need.

**Done when** `npm run check` and `npm run format:check` pass, and on the laptop each dialog matches the table: it opens with focus where the table says, Escape and clicking outside do what it says, Tab stays usable, and screen-reader names are unchanged (check with Narrator or the browser's accessibility tree in `wails3 dev`).

### PR 13. Play the right part of each sample (`preview-timing`)

Not a review finding: found on 2026-11-14 while planning the waveform editor (`docs/waveform-plan.md`, where it was PR W1).

**The bug.** `play()` in `frontend/src/lib/audio.ts` treats a pad's `start` and `end` as positions in the decoded buffer. They are frames of the file, at the file's own sample rate. The webview decodes every file to the sample rate of the sound output, so the two only match when file and output run at the same rate.

Measured on the VPS (WebKitGTK) with the one-second files in `testdata/wav-formats`:

| File | `end` in the project | Decoded at 44.1 kHz | Decoded at 48 kHz |
| --- | --- | --- | --- |
| 22 kHz | 22049 | 44099 frames | 47998 frames |
| 44.1 kHz | 44099 | 44100 | 47999 |
| 48 kHz | 47999 | 44100 | 48000 |
| 96 kHz | 95999 | 44100 | 48000 |

So on a 44.1 kHz output a 22 kHz sample plays its first half only, and on a 48 kHz output every 44.1 kHz sample loses its last 8%. A file with a higher rate than the output plays in full, but a trimmed region of it (such as Demo02's pad A5) starts and ends in the wrong place. Only the preview is wrong; exports are not affected.

**Branch.** `preview-timing`, from `main` after PR 12 is merged. bajdev may instead have it added to the `shared-modal` branch before that is merged. In that case: keep it in its own commits, describe both changes in the PR record, and PR 12's "no behaviour change" checks apply to everything except the preview.

**Tasks**

- [ ] In `serveAudio` (`internal/app/audio.go`), read the source's header with `wav.ReadFile` and add two response headers to the plain WAV response: `X-Mpckit-Frames` and `X-Mpckit-Sample-Rate`. If the header cannot be read, answer as `?pcm=1` does on a decode error.
- [ ] In `audio.ts`, make the cache hold `{ buffer, frames, rate }` in place of the bare buffer. `frames` and `rate` describe the file: from the two headers, or from the 12-byte header of the `?pcm=1` answer.
- [ ] In `play()`, turn frames into seconds with the file's rate: offset `start / rate`, duration `(end - start + 1) / rate`. Clamp `start` and `end` to `0 .. frames - 1`, and treat an `end` that is not after `start` as the last frame, as now.
- [ ] Extend `TestAudio` for the two headers.

**Done when:**

- `TestAudio` checks both headers against a file of known length and rate.
- On the laptop, with `testdata/wav-formats/MpckitW1.xpj` loaded: every pad plays for a full second, and Demo02's pad A5 still plays shorter and higher than A1.
- A scratch build, not committed, logs offset and duration for each `MpckitW1` pad under both a 44.1 kHz and a 48 kHz `OfflineAudioContext`: every pad gives offset 0 and duration 1 s. The PR record's Testing says how it was run.
- `TestBoundMethods` and `frontend/bindings` are unchanged: no bound method changes.

**No version change.** The fix ships in whichever release comes next.

## Backlog

Not scheduled; skipped for now (2026-11-13). Each needs decisions before it can be planned; none blocks a release.

| Finding | What | Where |
| --- | --- | --- |
| S5 | Preview decode holds a long sample in memory three times; stream frames to the response instead | `internal/app/audio.go` |
| M9 | Dialog cancel is detected by comparing error text; pin it with a test | `dialogs.go` |
| M10 | Internal error text (`xpj: path ...`) reaches the user; show one plain sentence and log the detail | `internal/app`, `main.go` |
| M11 | `state()` stats every pool file and scans all events on each call, under the lock | `internal/app/app.go` |

R2-R7 were done as PRs 10-12. R8 (PR record layout) and CI (part of S6) were dropped by decision; both came back in another form with the move to GitHub (see Decisions).

A possible later feature, the waveform and start/end editor, has its own plan in `docs/waveform-plan.md`. The preview bug found while planning it is PR 13 here. Support for the other audio formats the device imports is planned in `docs/audio-formats-plan.md`.
