// Helpers shared by the pad and sequence grids.

export const BANKS = "ABCDEFGH";
export const PER_BANK = 16;

/** "A1" for slot 0, "B2" for slot 17. Matches Go's project.SlotName. */
export function slotLabel(slot: number): string {
  return BANKS[Math.floor(slot / PER_BANK)] + ((slot % PER_BANK) + 1);
}

/**
 * The 16 slots of a bank in grid order, top-left first. Pad 1 sits
 * bottom-left, as on the device: the top row is pads 13-16.
 */
export function gridSlots(bank: number): number[] {
  const out: number[] = [];
  for (let row = 3; row >= 0; row--) {
    for (let col = 0; col < 4; col++) {
      out.push(bank * PER_BANK + row * 4 + col);
    }
  }
  return out;
}

/** "+12.40" style tuning text; empty when untuned. */
export function tuneText(coarse: number, fine: number): string {
  if (!coarse && !fine) return "";
  const v = coarse + fine / 100;
  return (v > 0 ? "+" : "") + v.toFixed(2);
}

/** "+5" / "-3" / "0". */
export function signed(n: number): string {
  return (n > 0 ? "+" : "") + n;
}

export function bars(n: number): string {
  return n + (n === 1 ? " bar" : " bars");
}
