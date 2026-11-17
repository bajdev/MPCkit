# MPCkit clear a whole bank: work plan

Written 2026-11-17. **Not started.** The decisions marked "to confirm" need bajdev's answer before the PR starts. The release version is bajdev's choice and is not set here.

The feature: one button that clears every pad in the bank shown on the Pads tab, and one that removes every sequence in the bank shown on the Sequences tab. Both ask first.

One PR: `feat/clear-bank`.

## What is already there

- **Pads.** `ClearPad` resets one pad to the blank pad and drops its pool entry when no other pad uses the file (`clear` in `internal/project/pads.go`). The Clear button in Pad settings calls it without asking.
- **Sequences.** `RemoveSequence` deletes one sequence. It refuses to remove the project's only sequence, and moves "current sequence" to the lowest remaining slot when the current one goes. Remove sequence in Sequence settings asks first, and says whether recorded notes will be deleted.
- **Recorded notes** belong to sequences and point at pad slots, not samples. Clearing a pad leaves notes that play it in place; they play nothing until the pad is filled again.
- **Confirm dialog.** `confirm()` in `store.svelte.ts` shows `ConfirmModal` with a title, a message, an optional list of items and the confirming button's label.
- **Room.** Each tab's grid panel has a header row ("Pad bank" or "Sequence bank" on the left, "Bank A · 3 of 16 filled" on the right) with spare width. The window must fit without scrolling at 1024x600, so nothing may add a row. The preview switch plan (`docs/preview-toggle-plan.md`) also puts a control in the Pads header.

## Decisions

Proposed by the plan, **to confirm with bajdev before the PR starts**:

| # | Topic | Proposal |
| --- | --- | --- |
| 1 | Where the button goes | A small **Clear bank** button at the right end of each tab's header row, after the bank summary. Disabled when the bank shown is empty. |
| 2 | Asking first | Always, with the confirm dialog: "Clear bank A?", the pads or sequences listed by label and name, and a **Clear** button. There is no undo. |
| 3 | Pads with recorded notes | Cleared like any pad. The dialog adds: "Sequences have recorded notes for A3 and A7. The notes stay, and play nothing until those pads are filled again." Same rule as the single Clear button. |
| 4 | Sequences with recorded notes | Removed with their notes. The dialog says which ones have notes. |
| 5 | The project's last sequence | A project must keep one sequence. If clearing the bank would remove every sequence, the bank's first slot gets a fresh empty sequence, made exactly as **Add sequence** makes one there (named for its slot, such as "Sequence 01" for A1 or "Sequence 17" for B1, 2 bars), and the dialog says so. |
| 6 | All or nothing | The whole bank is cleared in one step in Go. If any part fails, nothing changes. The project is marked changed once. |
| 7 | Selection | The selected pad or slot stays where it is, now empty. |
| 8 | Command line | `-clearbank BANK` and `-rmseqbank BANK` in `mpckit-cli` (BANK is A-H). |

Out of scope: clearing several banks at once, clearing the whole project (New does that), undo, and clearing only some pads of a bank.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and pull requests on GitHub, pull request descriptions, the checks before every push, the code rules and the release steps. Paths below are relative to `src/`. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes.
- Reuse the single-pad and single-sequence code. The bank functions must leave the project exactly as clearing each pad, or removing each sequence, one at a time would (apart from decision 5).
- No new Go modules or npm packages.
- The waveform work (`feat/waveform`) and the other planned features also change `App.svelte`, `internal/app/app.go` and the bindings. Start from the latest `origin/main`; rebase if one of them is merged while this pull request is open.

## PR: `feat/clear-bank`

- [ ] `internal/project`:
  - `func (p *Project) ClearBank(bank int) error`: clears pads `bank*16 .. bank*16+15` as `clear` does, dropping pool entries no pad uses any more. Works on a copy of what it changes and applies only if everything succeeded (decision 6). An empty bank is not an error and does not mark the project changed.
  - `func (p *Project) RemoveSequenceBank(bank int) error`: removes every sequence in slots `bank*16 .. bank*16+15`. If none would be left, adds a fresh sequence in the bank's first slot as `AddSequence` does (decision 5). "Current sequence" ends on the lowest remaining slot if it was in the bank. Same all-or-nothing rule.
  - Both check the bank number (0-7).
- [ ] `internal/app`: `ClearBank(bank int) (State, error)` and `RemoveSequenceBank(bank int) (State, error)` through `apply`. Add both to `TestBoundMethods`, regenerate the bindings and commit them.
- [ ] `mpckit-cli`: `-clearbank BANK` and `-rmseqbank BANK` (decision 8), each with its own parse function. `-h` gains exactly these two entries.
- [ ] Frontend:
  - The **Clear bank** button in both header rows (decision 1). Check it fits on one line at 960, 1280 and 1440 px wide, and at 1024x600, together with the preview switch if that has been merged. If it does not fit at 960, shorten the summary ("A · 3/16") before anything else.
  - The confirm dialog of decisions 2-5. The pads list shows "A1 Kick", "A2 Snare" and so on; the sequences list shows "A1 Sequence 01".
  - The calls go through `run()`, so an error shows in the usual place.
- [ ] Tests in `internal/project`:
  - A bank with 5 pads, two sharing one file and one sharing a file with a pad in another bank: after `ClearBank`, the bank is blank, the shared file of the other bank is still in the pool, and the other files are gone from both pool lists.
  - The result equals clearing the same pads one by one with `ClearPad`.
  - An empty bank leaves the project unchanged and not marked changed.
  - Pads with recorded notes are cleared; the notes are still in the sequence.
  - `RemoveSequenceBank` with sequences in two banks removes only the bank's; current sequence moves when needed.
  - `RemoveSequenceBank` on the bank holding every sequence leaves one fresh sequence in the bank's first slot (decision 5).
  - A bad bank number is refused and changes nothing.
  - A project cleared, exported and loaded again shows the same state.
- [ ] Tests in `internal/app` for both methods.

**Done when** the tests pass, the bindings hold both methods, and on the server:

```
mpckit-cli -name BankTest -out /tmp/bank -pad A1=<a WAV> -pad A2=<a WAV> -pad B1=<a WAV> -clearbank A
```

exports a project with only B1 filled and one sample in the pool.

On the laptop, with `testdata/device/DemoFile.xpj` loaded (it has recorded notes):

1. Pads tab, bank A: **Clear bank** asks first, lists the pads, and mentions the notes. Cancel changes nothing; Clear empties the bank, and the sample count in the footer drops.
2. Bank with no samples: **Clear bank** is disabled.
3. Sequences tab: **Clear bank** on a bank with sequences asks first, says which have notes, and removes them.
4. A project whose only sequences are in bank A: clearing bank A leaves one empty "Sequence 01", as the dialog said.
5. Export, load the export: everything is as it was before the export.
6. The window still fits at its smallest size, with both header rows on one line.
