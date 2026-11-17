<script lang="ts">
  import { Service, type Limits, type Pad } from "./api";
  import { play } from "./audio";
  import { app, message, run, ui } from "./store.svelte";
  import { signed } from "./slots";

  // Settings for the selected pad.
  let { pad, limits }: { pad: Pad; limits: Limits } = $props();

  // Slider values follow the pad, and track the thumb while it is dragged;
  // Go is told when it is let go.
  let coarse = $derived(pad.coarse);
  let fine = $derived(pad.fine);

  const groups = $derived(Array.from({ length: limits.maxMuteGroup }, (_, i) => i + 1));

  async function preview(p: Pad) {
    try {
      await play(p);
    } catch (e) {
      app.error = message(e);
    }
  }

  async function tune() {
    const slot = pad.slot;
    if (await run(() => Service.SetTune(slot, coarse, fine))) {
      const p = app.state?.pads?.[slot];
      if (p) preview(p); // hear the new tuning
    }
  }
</script>

<section class="panel settings" aria-label="Pad settings">
  <div class="panel-head">
    <div class="slot-id">{pad.label}</div>
    <div class="title">
      <h2 class="eyebrow">Pad settings</h2>
      <div>{pad.file || "Empty"}</div>
    </div>
  </div>

  {#if pad.file === ""}
    <div class="empty-box drop" data-file-drop-target data-slot={pad.slot}>
      <p>This pad is empty. Drop a WAV file on it, or choose one.</p>
      <button type="button" class="btn primary" onclick={() => run(() => Service.AssignSampleDialog(pad.slot))}
        >Choose WAV file</button
      >
    </div>
  {:else}
    <div class="stack">
      {#if pad.missing}
        <div class="missing" role="status">
          <strong>Sample file not found.</strong>
          <span class="mono">{pad.source}</span>
          <span>Replace it, or put the file back and load the project again. Export is disabled until then.</span>
        </div>
      {/if}
      <div class="row">
        <button type="button" class="btn primary" disabled={pad.missing} onclick={() => preview(pad)}>
          <svg
            aria-hidden="true"
            width="14"
            height="14"
            viewBox="0 0 14 14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linejoin="round"><path d="M3 2 L12 7 L3 12 Z" /></svg
          >
          Play
        </button>
        <button type="button" class="btn secondary" onclick={() => run(() => Service.AssignSampleDialog(pad.slot))}
          >Replace</button
        >
        <button type="button" class="btn secondary" onclick={() => run(() => Service.ClearPad(pad.slot))}>Clear</button>
      </div>

      <div class="control-group stack">
        <h3 class="eyebrow">Tuning</h3>
        <div class="pair">
          <div class="control">
            <div class="top">
              <label class="label" for="tune-coarse">Semitones</label>
              <span class="value">{signed(coarse)} st</span>
            </div>
            <input
              id="tune-coarse"
              type="range"
              min={limits.minCoarse}
              max={limits.maxCoarse}
              step="1"
              value={coarse}
              oninput={(e) => (coarse = e.currentTarget.valueAsNumber)}
              onchange={tune}
            />
            <div class="scale" aria-hidden="true">
              <span>{limits.minCoarse}</span><span>0</span><span>+{limits.maxCoarse}</span>
            </div>
          </div>
          <div class="control">
            <div class="top">
              <label class="label" for="tune-fine">Cents</label>
              <span class="value">{signed(fine)} ct</span>
            </div>
            <input
              id="tune-fine"
              type="range"
              min={limits.minFine}
              max={limits.maxFine}
              step="1"
              value={fine}
              oninput={(e) => (fine = e.currentTarget.valueAsNumber)}
              onchange={tune}
            />
            <div class="scale" aria-hidden="true">
              <span>{limits.minFine}</span><span>0</span><span>+{limits.maxFine}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="control">
        <label class="eyebrow" for="mute-group">Mute group</label>
        <select
          id="mute-group"
          class="field"
          value={pad.muteGroup}
          onchange={(e) => {
            const group = Number(e.currentTarget.value);
            run(() => Service.SetMuteGroup(pad.slot, group));
          }}
        >
          <option value={0}>Off</option>
          {#each groups as g (g)}
            <option value={g}>Group {g}</option>
          {/each}
        </select>
        <span class="hint">Pads in the same group cut each other off on the device.</span>
      </div>

      <div class="row divided">
        <button type="button" class="btn secondary" onclick={() => (ui.picker = { kind: "move", from: pad.slot })}
          >Move to pad</button
        >
        <button type="button" class="btn secondary" onclick={() => (ui.picker = { kind: "copy", from: pad.slot })}
          >Copy to pad</button
        >
        <button type="button" class="btn secondary" disabled={pad.missing} onclick={() => (ui.trim = pad.slot)}
          >Start/End</button
        >
      </div>
      {#if pad.hasNotes}
        <p class="hint notes">
          Recorded notes in a sequence play this pad. They stay on {pad.label} if the sample moves.
        </p>
      {/if}
    </div>
  {/if}
</section>

<style>
  .control-group {
    gap: 8px;
  }
  /* Semitones and cents side by side, to keep the panel short. */
  .pair {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
  }
  .notes {
    color: var(--accent);
  }
  .missing {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 12px;
    border: 1px solid #e5484d;
    border-radius: 6px;
    color: #ffb4b4;
    font-size: 12px;
  }
  .missing .mono {
    overflow-wrap: anywhere;
  }
  .drop:global(.file-drop-target-active) {
    border-color: var(--accent);
    background: rgba(245, 165, 36, 0.08);
  }
</style>
