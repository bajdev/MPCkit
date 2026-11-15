# Device checkpoint B: round trips through the app

Build plan section 7, after M8. Each project below starts from `testdata\device\DemoFile.xpj`, a project the device saved, and is changed and exported **in the app**. Then it is loaded on the MPC Sample.

DemoFile, for reference:
- Pads: A01 kick, A02 snare, A03 hat 1, A04 hat 2 (A03 and A04 in mute group 1), B01 pad loop, B02 bass.
- Sequence 01 "Intro" (110 BPM, 2 bars, notes on A01–A03) and Sequence 02 (110 BPM, 4 bars, notes on A01–A04).

For each project: **Load** DemoFile, make the changes, set the project name, then **Export to folder** (straight onto the card) or **Export zip**. Load DemoFile fresh for each one; don't build them on top of each other.

## Projects

### MpckitB1: no changes (B1)

Only rename to `MpckitB1`.

On the device: both sequences play exactly as DemoFile does.

### MpckitB2: move a pad that has notes (B2)

Drag **A01 → A05**. The app warns that A01's notes stay on A01; confirm.

On the device: A05 plays the kick. In the sequences the kick is gone, because the notes still point at A01, which is now empty.

### MpckitB3: copy across banks, swap pads with notes (B3)

1. **Alt**-drag **B01 → C01** (copy into another bank: drag over the **C** letter first).
2. Drag **A02 onto A03** (swap; the app warns about both pads' notes; confirm).

On the device:
- C01 plays the same pad loop as B01, and B01 still plays.
- A02 plays hat 1 and A03 plays the snare. In the sequences, the snare and hat 1 parts have changed places.

### MpckitB4: remove the current sequence, add one in another bank (B4)

1. Sequences tab: select **A01 "Intro"** → **Remove sequence** → confirm.
2. Select slot **B01** (sequence 17) → **Add sequence** → name `Bridge`, BPM `100`, length `4`.

On the device:
- The list shows Sequence 02 and Bridge (slot 17), and no Intro.
- **Which sequence is selected when the project opens?** Intro was the current sequence; the app made the lowest remaining slot (Sequence 02) current. This answers open question 7 in the format doc (is `currentSequence` a slot or a list position?).
- Bridge is empty, 4 bars, 100 BPM.

### MpckitB5: shorten a sequence that has notes (B5)

Sequence 02: length **4 → 2** bars (the app asks first; confirm).

On the device:
- Sequence 02 is 2 bars and plays the notes in bars 1–2.
- **What happened to the notes from bars 3–4?** Lengthen it back to 4 bars on the device: do they come back, or are they gone?

## Results

Run on the MPC Sample on 2026-11-11 by bajdev. All five loaded and passed.

| Project | Loads | Result |
|---|---|---|
| MpckitB1 | yes | Worked: plays as DemoFile. |
| MpckitB2 | yes | Worked: A05 plays the kick; the notes stayed on A01. |
| MpckitB3 | yes | Worked: the copy in bank C plays; the swapped pads changed places in the sequences. |
| MpckitB4 | yes | Worked. Sequence 02 (A02) was selected on open. |
| MpckitB5 | yes | Sequence 02 plays its 2 bars correctly. Lengthened back to 4 bars on the device, the notes in bars 3 and 4 are still there and play. |

B4 does not settle open question 7. In MpckitB4 the stored list is [Bridge (slot 17), Sequence 02 (slot 2)] and `currentSequence` is 1, so "slot 1" and "position 1 in the list" both mean Sequence 02.
