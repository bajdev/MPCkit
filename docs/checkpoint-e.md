# Device check E: moving, swapping and copying sequences

Device check D1 of `docs/sequence-drag-plan.md`. It answers whether a sequence keeps working when MPCkit changes its slot, and what else in the file follows a sequence's slot.

## Part 1: the device's own project

`testdata/device/DemoSeqMove.xpj` (with its two WAVs), saved by bajdev on the MPC Sample on 2026-11-18:

- Sequences A1 "One" (2 bars), A2 "Two" (4 bars, with notes) and A3 "Three" (8 bars), all at 110 BPM.
- Song 1 plays One, Two, Three, Two.
- **The device cannot move a sequence; it can copy one.** A2 was copied to B5.
- "Three" was selected, so it is the current sequence.

What the file shows:

- **The device's copy** (B5, `key` 20) is identical to A2 apart from `key`, name and notes included. It is first in `sequences[]`, as the device puts the newest sequence first. So `sequences[]` is now `[B5, A3, A2, A1]`.
- **Songs** name sequences as `{"item.sequenceIndex": N, "item.repeats": 1}`. Song 1's items are 0, 1, 2, 1. That fits both "slot" and "position among the sequences in slot order", so part 2 tells them apart.
- **`clipPlayerData.trackClipTransportMap`** still has one entry per track, `[{"key": 0, "value": 0}]`, with four sequences in the project. It does not follow sequences.
- `currentSequence` is 2: A3 "Three".

## Part 2: MPCkit's changes

`go run ./tools/checkpoint-e -out <folder>` (from `src/`) writes these projects from DemoSeqMove. Each changes only the fields listed. In all four, the B5 copy is renamed "Two B5" so it can be told apart from A2's "Two" on the song screen.

| Project | Change |
| --- | --- |
| MpckitQ1 | "Two" moved A2 → C9 (`key` 1 → 40). The song is left as it is. |
| MpckitQ2 | The same move, and the song's items 1 → 40. |
| MpckitQ3 | "One" and "Three" swapped (`key` 0 ↔ 2), the song's items 0 ↔ 2, and `currentSequence` 2 → 0. |
| MpckitQ4 | "Two" copied to H16: a clone of the entry with `key` 127, first in `sequences[]`. |

## Results

Run on the MPC Sample on 2026-11-18 by bajdev. All four loaded.

| Project | Result |
|---|---|
| MpckitQ1 | "Two" shows in C9 at 110 BPM, 4 bars. The song plays A1, then an **unused A2** (blank, 2 bars at the default 128 BPM), A3, then the unused A2 again. |
| MpckitQ2 | The song plays One, Two, Three, Two correctly. |
| MpckitQ3 | The song plays One, Two, Three, Two correctly. On open, A1 was the active sequence, and it is "Three". |
| MpckitQ4 | The copy is in H16 and the original in A2. "Three" is still current. |

## What this settles

- **`item.sequenceIndex` is a sequence slot.** In Q1, the song still pointed at slot 1 (A2), which no longer held a sequence. So moving or swapping a sequence must update every song item that names its slot. Q2 and Q3 show that doing so is enough.
- **`currentSequence` is a slot** (format doc, open question 7). In Q3 it is 0, and `sequences[]` is `[B5, Three, Two, One]`. Read as a position in the list, 0 would be "Two B5"; the device opened "Three" in A1.
- **The recipe:**
  - A move changes the sequence's `key`.
  - A swap exchanges the two `key`s.
  - A copy clones the entry with the new `key` and puts it first, as the device does.
  - Song items that name a moved or swapped slot follow it.
  - `currentSequence` follows a moved or swapped sequence.
  - A copy changes neither the songs nor the current sequence.
  - Nothing else changes: `trackClipTransportMap` stays as it is.
