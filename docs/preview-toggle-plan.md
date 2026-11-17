# MPCkit turn pad preview on click on or off: work plan

Written 2026-11-17. **Not started.** The decisions marked "to confirm" need bajdev's answer before the PR starts. The release version is bajdev's choice and is not set here.

The feature: a switch in the window that turns off playing a pad when it is clicked. It is on by default, which is how MPCkit behaves today. The choice is kept when MPCkit is closed and opened again.

One PR: `feat/preview-toggle`.

## What is already there

- `pickPad` in `App.svelte` selects the clicked pad and, if it has a sample, plays it with `play()` from `lib/audio.ts`.
- The Play button in Pad settings plays the selected pad. Changing the tuning plays the pad again, so the new tuning can be heard (`tune()` in `PadPanel.svelte`).
- The grid's hint says "Click a pad to select and hear it."
- MPCkit stores no preferences yet. Everything in the window starts fresh each time.
- The Pads panel has a header row ("Pad bank" on the left, "Bank A · 3 of 16 filled" on the right) with spare width. The window must fit without scrolling at 1024x600, and the Pad settings panel has no spare height (measured for the waveform plan), so the switch must not add a row.

## Decisions

Proposed by the plan, **to confirm with bajdev before the PR starts**:

| # | Topic | Proposal |
| --- | --- | --- |
| 1 | What the switch covers | Only clicking a pad on the grid. The Play button always plays: it is an explicit request. |
| 2 | Tuning preview | Follows the switch too: with it off, changing the tuning does not play the pad. Both are sounds the user did not ask for with a Play button. |
| 3 | Where it goes | In the Pads panel's header row, between "Pad bank" and the bank summary, as a small switch labelled "Play on click". Shown on the Pads tab only. |
| 4 | How it is kept | In a small settings file that Go reads and writes: `MPCkit/settings.json` in the system's user config folder (`os.UserConfigDir()`: `%AppData%` on Windows, `~/.config` on Linux, `~/Library/Application Support` on macOS). Not in the project file: it is the user's preference, not part of a kit. Not in the webview's own storage, whose lifetime differs between the three webviews and cannot be tested on the server. |
| 5 | A missing or broken settings file | Use the defaults, without a message. A broken file is replaced the next time a setting changes. A file that cannot be written gives a one-line error, and the switch still works until MPCkit closes. |
| 6 | Turning it off while a pad sounds | The sound plays to its end. Turning the switch off only stops new previews. |
| 7 | Hint text | With the switch on: "Click a pad to select and hear it." Off: "Click a pad to select it." The rest of the hint is unchanged. |

Out of scope: other preferences, a settings dialog, a keyboard shortcut for the switch, and volume.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and pull requests on GitHub, pull request descriptions, the checks before every push, the code rules and the release steps. Paths below are relative to `src/`. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes.
- The settings file holds only preferences, never paths or project data. The window reads and changes preferences only through the two methods below; no bound method takes a file path.
- No new Go modules or npm packages.
- The waveform work (`feat/waveform`) also changes `App.svelte` and `PadPanel.svelte`. Start from the latest `origin/main`; if the waveform pull request is merged while this one is open, rebase.

## PR: `feat/preview-toggle`

- [ ] New package `internal/settings` (no Wails):
  - `type Settings struct { PreviewOnClick bool }`, with JSON name `previewOnClick`.
  - `Defaults()`: `PreviewOnClick` true.
  - `Load(path) Settings`: the file's values over the defaults. A missing, unreadable or broken file, or a missing field, gives the defaults (decision 5). Unknown fields are ignored, so a later version's file still loads.
  - `Save(path, Settings) error`: writes to a temporary file in the same folder and renames it into place, creating the folder if needed.
  - `Path() (string, error)`: `os.UserConfigDir()` joined with `MPCkit/settings.json`.
- [ ] `internal/app`: the service loads the settings once at start. Two bound methods, added to `TestBoundMethods`, with the bindings regenerated and committed:
  - `Settings() Settings`
  - `SetPreviewOnClick(on bool) (Settings, error)`: changes the value, saves it, and returns the settings. If saving fails, the value still changes for this session and the error says the setting will not be kept (decision 5).
  - The service takes the settings path from `main.go`, so tests use `t.TempDir()`.
- [ ] Frontend:
  - The store holds the settings, read once at start.
  - The switch (decision 3): a `<button role="switch" aria-checked>` labelled "Play on click", styled like the app's other small controls. It must not make the header row taller: check at 960, 1280 and 1440 px wide, and at 1024x600.
  - `pickPad` plays only when the setting is on. `tune()` in `PadPanel.svelte` likewise (decision 2). The Play button is unchanged (decision 1).
  - The hint text follows the switch (decision 7).
- [ ] Tests in `internal/settings`: defaults with no file; a saved file loads back; a broken file and a file with an unknown field; `Save` into a folder that does not exist yet; a failed save leaves any old file whole.
- [ ] Tests in `internal/app`: `Settings` and `SetPreviewOnClick` through the service, and a new service started on the same path reads the saved value.
- [ ] README: one line in the usage section about the switch.

**Done when:**

- The checks pass, including `npm run check` and `npm run format:check`.
- A scratch build, not committed, measures the Pads panel header and the Pad settings panel: no height added at 1280x720 or 1024x600.
- On the laptop:
  1. First start, with no settings file: the switch is on, and clicking a pad plays it.
  2. Turn it off: clicking a pad selects it without sound, changing the tuning is silent, and the Play button still plays.
  3. Close and reopen MPCkit: the switch is still off. `%AppData%\MPCkit\settings.json` holds `"previewOnClick": false`.
  4. Turn it on: clicks play again.
  5. Replace the file's contents with `nonsense` and reopen: the switch is on, with no message.
  6. The switch can be reached with Tab and changed with Space, and a screen reader reads it as "Play on click, switch, on".
