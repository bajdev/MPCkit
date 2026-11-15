// Dragging pads inside the page. Files dragged in from the system are
// handled by Wails (data-file-drop-target), which ignores drags without
// files, so the two do not meet.

export const PAD_TYPE = "application/x-mpckit-pad";

// The slot being dragged. Kept here because dataTransfer's data cannot be
// read during dragover, only its types.
let dragging: number | null = null;

export function startDrag(e: DragEvent, slot: number) {
  dragging = slot;
  e.dataTransfer?.setData(PAD_TYPE, String(slot));
  if (e.dataTransfer) e.dataTransfer.effectAllowed = "copyMove";
}

export function endDrag() {
  dragging = null;
}

/** True while a pad (not a file) is being dragged over the page. */
export function isPadDrag(e: DragEvent): boolean {
  return dragging !== null && !!e.dataTransfer?.types.includes(PAD_TYPE);
}

export function draggedSlot(): number | null {
  return dragging;
}
