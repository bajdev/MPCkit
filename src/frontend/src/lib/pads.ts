// Moving, copying and swapping pads (build plan M6), shared by drag and drop
// and the "Move to pad" / "Copy to pad" buttons.
import { Service, type Pad } from "./api";
import { app, confirm, message, run, ui } from "./store.svelte";
import { PER_BANK } from "./slots";

export type Transfer = "move" | "swap" | "copy";

/** What dropping or sending pad `from` onto `to` does. */
export function transferKind(from: Pad, to: Pad, copy: boolean): Transfer {
  if (copy) return "copy";
  return to.file ? "swap" : "move";
}

/**
 * Moves, swaps or copies a pad, asking first when that would replace a
 * sample or change what recorded notes play. Recorded notes stay on their
 * slots (format doc section 3): a note keeps pointing at the slot, not the
 * sample.
 */
export async function transfer(kind: Transfer, from: number, to: number): Promise<void> {
  try {
    await doTransfer(kind, from, to);
  } catch (e) {
    app.error = message(e);
  }
}

async function doTransfer(kind: Transfer, from: number, to: number): Promise<void> {
  const pads = app.state?.pads;
  if (!pads || from === to) return;
  const src = pads[from];
  const dst = pads[to];

  const items: string[] = [];
  if (kind === "copy" && dst.file) {
    items.push(`${dst.label} (${dst.name}) will be replaced.`);
  }
  // Pads whose sound changes: both for move and swap, the target for copy.
  const changed = kind === "copy" ? [dst] : [src, dst];
  for (const p of changed) {
    if (p.hasNotes) {
      items.push(
        `${p.label} has recorded notes. They stay on ${p.label} and will play ${soundAfter(kind, p, src, dst)}.`,
      );
    }
  }
  if (items.length) {
    const ok = await confirm({
      title: `${verb(kind)} ${src.label} ${kind === "swap" ? "and" : "to"} ${dst.label}?`,
      message: "Please check:",
      items,
      action: verb(kind),
    });
    if (!ok) return;
  }

  const call =
    kind === "move"
      ? () => Service.MovePad(from, to)
      : kind === "swap"
        ? () => Service.SwapPads(from, to)
        : () => Service.CopyPad(from, to);
  if (await run(call)) {
    ui.padBank = Math.floor(to / PER_BANK);
    ui.padSlot = to;
  }
}

function verb(kind: Transfer): string {
  return kind === "move" ? "Move" : kind === "swap" ? "Swap" : "Copy";
}

// What a pad's notes will play once the transfer is done.
function soundAfter(kind: Transfer, p: Pad, src: Pad, dst: Pad): string {
  if (p.slot === src.slot) {
    return kind === "swap" ? describe(dst) : "nothing (the pad will be empty)";
  }
  return describe(src);
}

function describe(p: Pad): string {
  return p.file ? `"${p.name}"` : "nothing (the pad will be empty)";
}
