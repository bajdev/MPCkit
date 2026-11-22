<script lang="ts" module>
  export type Cell = {
    slot: number;
    label: string; // "A1"
    name: string; // sample or sequence name; empty for an empty slot
    meta: string; // small line under the name
    aria: string;
    warn?: boolean; // something is wrong with this slot, e.g. its sample is missing
  };
</script>

<script lang="ts">
  import { draggedSlot, endDrag, isPadDrag, startDrag } from "./drag";

  // A 4x4 grid of pads or sequence slots, used by both tabs.
  let {
    cells,
    selected,
    onpick,
    dropTarget = false,
    ondroppad,
    candrop,
  }: {
    cells: Cell[]; // 16 cells in grid order, top-left first
    selected: number;
    onpick: (slot: number) => void;
    dropTarget?: boolean; // accept WAV files dropped from the system
    ondroppad?: (from: number, to: number, copy: boolean) => void; // filled cells can be dragged onto others
    candrop?: (from: number, to: number, copy: boolean) => boolean; // false: the cursor shows no drop here
  } = $props();

  function allowed(e: DragEvent, slot: number): boolean {
    const from = draggedSlot();
    return from === null || !candrop || candrop(from, slot, e.altKey);
  }

  let over: number | null = $state(null); // cell a pad is dragged over
  let copying = $state(false); // Alt held during the drag

  function dragover(e: DragEvent, slot: number) {
    if (!ondroppad || !isPadDrag(e)) return;
    if (!allowed(e, slot)) {
      over = null; // no preventDefault: the drop is refused
      if (e.dataTransfer) e.dataTransfer.dropEffect = "none";
      return;
    }
    e.preventDefault(); // allow the drop
    copying = e.altKey;
    if (e.dataTransfer) e.dataTransfer.dropEffect = e.altKey ? "copy" : "move";
    over = slot === draggedSlot() ? null : slot;
  }

  function drop(e: DragEvent, slot: number) {
    if (!ondroppad || !isPadDrag(e) || !allowed(e, slot)) return;
    e.preventDefault();
    const from = draggedSlot();
    over = null;
    endDrag();
    if (from !== null && from !== slot) ondroppad(from, slot, e.altKey);
  }
</script>

<div class="grid">
  {#each cells as c (c.slot)}
    <button
      type="button"
      class:filled={c.name !== ""}
      class:warn={c.warn}
      class:on={c.slot === selected}
      class:over={c.slot === over}
      class:copying={c.slot === over && copying}
      aria-pressed={c.slot === selected}
      aria-label={c.aria}
      data-file-drop-target={dropTarget ? "" : undefined}
      data-slot={c.slot}
      onclick={() => onpick(c.slot)}
      draggable={!!ondroppad && c.name !== ""}
      ondragstart={(e) => startDrag(e, c.slot)}
      ondragend={() => {
        over = null;
        endDrag();
      }}
      ondragover={(e) => dragover(e, c.slot)}
      ondragleave={() => {
        if (over === c.slot) over = null;
      }}
      ondrop={(e) => drop(e, c.slot)}
    >
      <span class="num">{c.label}</span>
      <span class="name">{c.name}</span>
      <span class="meta">{c.meta}</span>
    </button>
  {/each}
</div>

<style>
  /* Fills the height its panel gives it: four equal rows. */
  .grid {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    grid-template-rows: repeat(4, minmax(0, 1fr));
    gap: 8px;
  }
  button {
    min-height: 0;
    padding: 6px 8px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    justify-content: space-between;
    gap: 4px;
    text-align: left;
    background: var(--cell-empty);
    color: var(--text);
    border: 2px solid var(--line);
    border-radius: 8px;
    font: inherit;
    cursor: pointer;
    overflow: hidden;
  }
  button.filled {
    background: var(--cell-filled);
    border-color: var(--cell-filled-edge);
  }
  button.on {
    border-color: var(--accent);
  }
  button.warn .meta {
    color: #ff8a8a;
  }
  /* A pad dragged over this cell: dashed for move or swap, solid for copy. */
  button.over {
    border-style: dashed;
    border-color: var(--accent);
  }
  button.over.copying {
    border-style: solid;
  }
  button[draggable="true"] {
    cursor: grab;
  }
  /* Wails adds this class while a file is dragged over a drop target. */
  button:global(.file-drop-target-active) {
    border-color: var(--accent);
    background: rgba(245, 165, 36, 0.12);
  }
  .num {
    font-family: var(--mono);
    font-size: 11px;
    font-weight: 500;
    color: var(--muted);
  }
  button.on .num {
    color: var(--accent);
  }
  .name {
    font-size: 12px;
    font-weight: 600;
    line-height: 1.25;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--soft);
    min-height: 15px;
    white-space: nowrap;
  }
</style>
