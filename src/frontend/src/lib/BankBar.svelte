<script lang="ts">
  import { isPadDrag } from "./drag";
  import { BANKS, PER_BANK } from "./slots";

  let {
    bank,
    counts,
    what,
    onpick,
    ondragbank,
  }: {
    bank: number;
    counts: number[]; // filled slots per bank
    what: string; // "pads filled", "slots used"
    onpick: (bank: number) => void;
    ondragbank?: (bank: number) => void; // a pad dragged over a bank button
  } = $props();
</script>

<div class="banks" role="group" aria-label="Banks">
  {#each BANKS.split("") as letter, i (letter)}
    <button
      type="button"
      class:on={i === bank}
      aria-pressed={i === bank}
      aria-label="Bank {letter}, {counts[i]} of {PER_BANK} {what}"
      onclick={() => onpick(i)}
      ondragover={(e) => {
        // Dragging a pad over a bank shows that bank, to drop on its pads.
        if (!ondragbank || !isPadDrag(e)) return;
        e.preventDefault();
        if (e.dataTransfer) e.dataTransfer.dropEffect = "none";
        if (i !== bank) ondragbank(i);
      }}>{letter}</button
    >
  {/each}
</div>

<style>
  .banks {
    display: grid;
    grid-template-columns: repeat(8, minmax(0, 1fr));
    gap: 6px;
  }
  button {
    height: 36px;
    padding: 0;
    background: var(--raised);
    color: var(--text);
    border: 1px solid var(--edge);
    border-radius: 6px;
    font-family: var(--display);
    font-size: 15px;
    font-weight: 700;
    cursor: pointer;
  }
  button.on {
    background: var(--accent);
    color: var(--on-accent);
    border-color: var(--accent);
  }
</style>
