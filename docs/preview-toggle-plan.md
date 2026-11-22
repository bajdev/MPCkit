# MPCkit turn pad preview on click on or off: work plan

Written 2026-11-17. **Going ahead** (bajdev, 2026-11-18): decisions 1-3, 6 and 7 confirmed as proposed; 4 and 5 changed so the choice is not kept. The release version is bajdev's choice and is not set here.

The feature: a switch in the window that turns off playing a pad when it is clicked. It is on each time MPCkit starts, which is how MPCkit behaves today. The choice is not kept: a user who wants it off turns it off again after opening MPCkit.

One PR: `feat/preview-toggle`.

## What is already there

- `pickPad` in `App.svelte` selects the clicked pad and, if it has a sample, plays it with `play()` from `lib/audio.ts`.
- The Play button in Pad settings plays the selected pad. Changing the tuning plays the pad again, so the new tuning can be heard (`tune()` in `PadPanel.svelte`).
- The grid's hint says "Click a pad to select and hear it."
- MPCkit stores no preferences yet. Everything in the window starts fresh each time.
- The Pads panel has a header row ("Pad bank" on the left, "Bank A · 3 of 16 filled" on the right) with spare width. The window must fit without scrolling at 1024x600, and the Pad settings panel has no spare height (measured for the waveform plan), so the switch must not add a row.

## Decisions

Settled with bajdev on 2026-11-18. Agents follow them and do not reopen them:

| # | Topic | Proposal |
| --- | --- | --- |
| 1 | What the switch covers | Only clicking a pad on the grid. The Play button always plays: it is an explicit request. |
| 2 | Tuning preview | Follows the switch too: with it off, changing the tuning does not play the pad. Both are sounds the user did not ask for with a Play button. |
| 3 | Where it goes | In the Pads panel's header row, between "Pad bank" and the bank summary, as a small switch labelled "Play on click". Shown on the Pads tab only. |
| 4 | How it is kept | **Changed by bajdev:** it is not kept. The switch is on each time MPCkit starts and lives only in the window's state; no settings file, nothing in the project file. |
| 5 | A missing or broken settings file | **Dropped by bajdev** with decision 4: there is no settings file. |
| 6 | Turning it off while a pad sounds | The sound plays to its end. Turning the switch off only stops new previews. |
| 7 | Hint text | With the switch on: "Click a pad to select and hear it." Off: "Click a pad to select it." The rest of the hint is unchanged. |

Out of scope: keeping the choice between runs, other preferences, a settings dialog, a keyboard shortcut for the switch, and volume.

Not foreseen when the plan was written: the Pads header row also holds **Clear bank** (`docs/clear-bank-plan.md`). If the switch, the summary and Clear bank do not fit on one line at 960 px, the agent asks bajdev before shortening the summary.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and pull requests on GitHub, pull request descriptions, the checks before every push, the code rules and the release steps. Paths below are relative to `src/`. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes.
- The switch lives in the frontend store only. No Go change, no new bound method, and the bindings must not change.
- No new Go modules or npm packages.
- The waveform work (`feat/waveform`) also changes `App.svelte` and `PadPanel.svelte`. Start from the latest `origin/main`; if the waveform pull request is merged while this one is open, rebase.

## PR: `feat/preview-toggle`

Rewritten on 2026-11-18 for decisions 4 and 5: no settings file, so no Go work.

- [ ] Frontend:
  - The store holds the switch, on at start.
  - The switch (decision 3): a `<button role="switch" aria-checked>` labelled "Play on click", styled like the app's other small controls. It must not make the header row taller: check at 960, 1280 and 1440 px wide, and at 1024x600.
  - `pickPad` plays only when the switch is on. `tune()` in `PadPanel.svelte` likewise (decision 2). The Play button is unchanged (decision 1).
  - The hint text follows the switch (decision 7).
- [ ] README: a line about the switch, in the README pull request bajdev asked for after this one (with Clear bank), not in this PR.

**Done when:**

- The checks pass, including `npm run check` and `npm run format:check`.
- A scratch build, not committed, measures the Pads panel header and the Pad settings panel: no height added at 1280x720 or 1024x600.
- On the laptop:
  1. On start the switch is on, and clicking a pad plays it.
  2. Turn it off: clicking a pad selects it without sound, changing the tuning is silent, and the Play button still plays.
  3. Turn it on: clicks play again.
  4. Turn it off, close and reopen MPCkit: the switch is on again.
  5. The switch can be reached with Tab and changed with Space, and a screen reader reads it as "Play on click, switch, on".
