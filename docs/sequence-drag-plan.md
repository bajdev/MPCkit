# MPCkit rearrange sequences by dragging: work plan

Written 2026-11-17. **Going ahead** (bajdev, 2026-11-18): device check D1 is done (`docs/checkpoint-e.md`), and all eight decisions are confirmed with what it found. The release version is bajdev's choice and is not set here.

The feature: on the Sequences tab, drag a sequence onto another slot to move it there, drag it onto a filled slot to swap the two, and hold Alt while dragging to copy it. This is how pads already work on the Pads tab.

Two PRs after the device check: Q1 adds move, swap and copy to the model, the app service and `mpckit-cli`; Q2 adds dragging in the window.

## What is already there

- **File format.** `sequences[]` holds only the sequences that exist. Each entry is `{key, value}`, where `key` is the slot (0-127) and `value` the sequence itself. The list is not in slot order: the device puts the newest first, and MPCkit's `AddSequence` does the same. `currentSequence` holds the slot of the sequence selected on the device (format doc, open question 7: treated as a slot, which checkpoint B4 supports).
- **Model.** `Sequences()` sorts by `key`. `AddSequence`, `RemoveSequence` and `SetSequence` all find a sequence by its `key`.
- **Recorded notes** live inside each sequence and point at pad slots. Moving a sequence does not change what its notes play.
- **Songs.** Every project has `songs[0-31]` with `name`, `ignoreTempo` and `items`. In all device files so far, `items` is empty, so how a song refers to a sequence (by slot or by position in `sequences[]`) is not known. MPCkit does not edit songs.
- **One field not understood.** `clipPlayerData.trackClipTransportMap` has, for each track, `[{"key": 0, "value": 0}]`. The `key` 0 could be a sequence slot. Both device files with two sequences have only key 0, so it may be something else.
- **Window.** The pad grid and the sequence grid are both `SlotGrid.svelte`. Dragging is already built for pads (`lib/drag.ts`, the `ondroppad` prop), including switching bank by dragging over a bank letter (`ondragbank` on `BankBar`). The sequence grid passes neither yet.

## Device check D1 (before any code)

The question: does a sequence keep working when MPCkit changes its `key`, and what else in the file follows a sequence's slot?

**Part 1: let the device do it.** On the device, in a new project:

1. Make three sequences: A1 "One" (2 bars), A2 "Two" (4 bars), A3 "Three" (8 bars). Record a few notes in "Two".
2. Make a song that plays One, Two, Three, Two.
3. If the device can move or copy a sequence to another slot, move "Two" to B5. Note how it is done, or that the device cannot.
4. Select "Three" so it is the current sequence.
5. Save as `DemoSeqMove` and copy the project file into `testdata/device/`.

That file answers: how `songs[].items` names a sequence; whether `trackClipTransportMap` gets an entry per sequence; and, if step 3 was possible, every field the device changes when a sequence moves.

**Part 2: MPCkit's way, before the code exists.** From `DemoSeqMove`, a short script (kept under `tools/`, like `tools/checkpoint-a`) writes three projects by changing `key` and `currentSequence` only:

| Project | Change |
| --- | --- |
| `MpckitQ1` | "Two" moved from its slot to C9 |
| `MpckitQ2` | "One" and "Three" swapped |
| `MpckitQ3` | "Two" copied to H16, keeping the original |

For each, on the device: does it load without a message; does each sequence show in its new slot with its name, length and notes; does the current sequence open; does the song still play the right sequences in the right order?

Record the answers in `docs/checkpoint-e.md`, in the layout of `docs/checkpoint-b.md`.

## Decisions

Confirmed by bajdev on 2026-11-18, after D1. Agents follow them and do not reopen them:

| # | Topic | Proposal |
| --- | --- | --- |
| 1 | What dragging does | As for pads: drop on an empty slot moves; drop on a filled slot swaps; Alt while dropping copies, onto an empty slot only. The cursor shows which. |
| 2 | Recipe | Move and swap change only `key` (and `currentSequence`, decision 4). Copy clones the whole entry, notes included, with the new `key`, and puts it first in `sequences[]`, as `AddSequence` does. Anything else D1 shows the device changing is added to the recipe before Q1 starts. |
| 3 | Names | Unchanged. A sequence called "Sequence 01" keeps that name in slot B5. The user renames it if they want. |
| 4 | Current sequence | Follows the sequence: if the moved or swapped sequence was current, `currentSequence` becomes its new slot. A copy never becomes current. |
| 5 | Songs | D1 showed that song items name sequences by slot (`item.sequenceIndex`). Moving and swapping update every item naming those slots, so songs keep playing the same sequences. |
| 6 | Sequence limit | Copy is refused when the project has 128 sequences (it cannot, with 128 slots, but the check stays). |
| 7 | Bank switching | Dragging a sequence over a bank letter shows that bank, as for pads. |
| 8 | Command line | `-moveseq FROM:TO`, `-swapseq A:B` and `-copyseq FROM:TO` in `mpckit-cli`, with slots written as on the grid (`A1`, `C9`). |

Out of scope: reordering songs, editing song items, moving several sequences at once, renumbering default names.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies: branches and pull requests on GitHub, pull request descriptions, the checks before every push, the code rules and the release steps. Paths below are relative to `src/`. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the pull request's notes.
- The recipe comes from D1's files, not from guesses. Q1's tests compare MPCkit's output with `DemoSeqMove` the way `TestRebuildDemo02` compares with `Demo02`.
- `testdata/device/DemoSeqMove.xpj` is a device fixture: never change it once committed.
- No new Go modules or npm packages.
- Other planned features change `App.svelte`, `SlotGrid.svelte`, `internal/app/app.go` and the bindings. Start from the latest `origin/main`; rebase if one of them is merged while this pull request is open.

## PR order

```
D1 device check  →  decisions confirmed  →  PR Q1 feat/sequence-move  →  PR Q2 feat/sequence-drag
```

| PR | Branch | What |
| --- | --- | --- |
| Q1 | `feat/sequence-move` | `MoveSequence`, `SwapSequences`, `CopySequence` in the model and the service; CLI flags; the D1 fixture and script |
| Q2 | `feat/sequence-drag` | Dragging on the Sequences tab |

No version change in either. bajdev picks the release.

## PRs and tasks

### PR Q1: `feat/sequence-move`

- [ ] Commit `testdata/device/DemoSeqMove.xpj` and the D1 script under `tools/`, and add D1's results to the format doc: a "Move, swap or copy a sequence" recipe next to "Add a sequence", what songs and `trackClipTransportMap` turned out to be, and the answer to open question 7 if D1 settled it.
- [ ] `internal/project/sequences.go`:
  - `func (p *Project) MoveSequence(from, to int) error`: `from` must hold a sequence and `to` must be empty.
  - `func (p *Project) SwapSequences(a, b int) error`: both must hold a sequence.
  - `func (p *Project) CopySequence(from, to int) error`: `from` must hold a sequence and `to` must be empty (decision 1).
  - Each applies the recipe of decision 2, the current-sequence rule of decision 4 and the song rule of decision 5, all or nothing, and marks the project changed. Moving or swapping a slot onto itself does nothing.
- [ ] `internal/app`: `MoveSequence`, `SwapSequences` and `CopySequence` through `apply`. Add them to `TestBoundMethods`, regenerate the bindings and commit them.
- [ ] `mpckit-cli`: `-moveseq`, `-swapseq` and `-copyseq` (decision 8), each with its own parse function. `-h` gains exactly these three entries.
- [ ] Tests:
  - Each operation on `DemoSeqMove`: `Sequences()` shows the result; names, lengths, BPM and notes travel with the sequence; nothing else in the file changes apart from the fields the recipe names (compare the trees, as `TestRebuildDemo02` does).
  - If the device could move a sequence in D1: MPCkit's move on the "before" state gives the device's file, apart from known noise.
  - Current sequence follows a move and a swap, and stays on a copy.
  - Songs per decision 5.
  - Refused cases change nothing: empty source, filled target for move and copy, empty slot for swap, out-of-range slots.
  - A project with moved, swapped and copied sequences, exported and loaded again, shows the same `Sequences()`.

**Done when** the tests pass, the bindings hold the three methods, and on the server

```
mpckit-cli -load testdata/device/DemoSeqMove.xpj -name SeqTest -out /tmp/seq -moveseq A2:C9 -swapseq A1:A3 -copyseq C9:H16
```

exports a project whose sequences are in the slots asked for. Then bajdev loads that export on the device: every sequence plays from its new slot, and the song plays the same sequences as before.

### PR Q2: `feat/sequence-drag`

- [ ] In `App.svelte`, the sequence grid gets `ondroppad` and its `BankBar` gets `ondragbank`, as the pad grid has. Drop on an empty slot calls `MoveSequence`, on a filled slot `SwapSequences`, with Alt on an empty slot `CopySequence` (decision 1). Alt onto a filled slot does nothing, and the cursor shows that.
- [ ] Only filled slots can be dragged. Empty slots stay as they are: clicking one still selects it to add a sequence.
- [ ] After a move, the moved sequence is selected in its new slot.
- [ ] The Sequences tab's hint adds: "Drag a sequence to move it, onto another to swap them, with Alt to copy."
- [ ] README: the same in the usage section.

**Done when:**

- The checks pass, including `npm run check` and `npm run format:check`.
- On the laptop, with `testdata/device/DemoFile.xpj` loaded:
  1. Drag "Sequence 02" to an empty slot: it moves, and is selected there.
  2. Drag it onto "Intro": the two swap.
  3. Alt-drag one to an empty slot in bank C, reaching bank C by dragging over its letter: a copy appears, and the original stays.
  4. Alt-drag onto a filled slot: nothing happens.
  5. Click an empty slot: it is selected, ready to add a sequence, as before.
  6. Export and load the export: every sequence is where it was put.
  7. Dragging pads on the Pads tab works as before.
