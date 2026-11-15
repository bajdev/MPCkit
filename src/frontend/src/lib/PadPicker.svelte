<script lang="ts">
  import type { Pad } from "./api";
  import Modal from "./Modal.svelte";
  import { transfer, transferKind } from "./pads";
  import { BANKS, PER_BANK } from "./slots";
  import type { Picker } from "./store.svelte";

  // "Move to pad" / "Copy to pad": the keyboard-friendly route to what
  // dragging does.
  let { picker, pads, onclose }: { picker: Picker; pads: Pad[]; onclose: () => void } = $props();

  const src = $derived(pads[picker.from]);

  // Start on the first empty pad after the source, else the next pad.
  function firstTarget(): number {
    for (let i = 1; i < pads.length; i++) {
      const s = (picker.from + i) % pads.length;
      if (!pads[s].file) return s;
    }
    return (picker.from + 1) % pads.length;
  }
  const start = firstTarget();
  let bank = $state(Math.floor(start / PER_BANK));
  let pad = $state(start % PER_BANK);

  const target = $derived(bank * PER_BANK + pad);
  const kind = $derived(transferKind(src, pads[target], picker.kind === "copy"));
  const action = $derived(
    target === picker.from
      ? ""
      : kind === "move"
        ? "Move"
        : kind === "swap"
          ? "Swap"
          : pads[target].file
            ? "Copy and replace"
            : "Copy",
  );

  let firstField: HTMLSelectElement | undefined = $state();

  async function go() {
    if (!action) return;
    // Read everything before closing: closing clears ui.picker, and props
    // are live, so `picker` is null from then on.
    const what = kind;
    const from = picker.from;
    const to = target;
    onclose();
    await transfer(what, from, to);
  }
</script>

<!-- Escape closes; a click outside does nothing. -->
<Modal titleId="picker-title" {onclose} focus={firstField}>
  <h2 id="picker-title">{picker.kind === "move" ? "Move" : "Copy"} {src.label} to pad</h2>
  <p class="from">{src.name}</p>
  <div class="fields">
    <div class="control">
      <label class="label" for="picker-bank">Bank</label>
      <select id="picker-bank" class="field" bind:value={bank} bind:this={firstField}>
        {#each BANKS.split("") as letter, i (letter)}
          <option value={i}>{letter}</option>
        {/each}
      </select>
    </div>
    <div class="control">
      <label class="label" for="picker-pad">Pad</label>
      <select id="picker-pad" class="field" bind:value={pad}>
        {#each Array.from({ length: PER_BANK }, (_, i) => i) as i (i)}
          {@const p = pads[bank * PER_BANK + i]}
          <option value={i}>{p.label}{p.file ? " · " + p.name : " · empty"}</option>
        {/each}
      </select>
    </div>
  </div>
  <p class="hint">
    {#if !action}
      Choose a different pad.
    {:else if kind === "swap"}
      {pads[target].label} has a sample: the two pads will swap.
    {:else if kind === "copy" && pads[target].file}
      {pads[target].label} will be replaced.
    {:else}
      &nbsp;
    {/if}
  </p>
  <div class="buttons">
    <button type="button" class="btn secondary" onclick={onclose}>Cancel</button>
    <button type="button" class="btn primary" disabled={!action} onclick={go}
      >{action || (picker.kind === "move" ? "Move" : "Copy")}</button
    >
  </div>
</Modal>

<style>
  h2 {
    margin: 0;
    font-family: var(--display);
    font-size: 20px;
    font-weight: 700;
  }
  .from {
    margin: 0;
    color: var(--soft);
    overflow-wrap: anywhere;
  }
  .fields {
    display: grid;
    grid-template-columns: 1fr 2fr;
    gap: 10px;
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
