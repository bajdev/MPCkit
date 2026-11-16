<script lang="ts">
  import { Service, type Limits, type Sequence } from "./api";
  import { app, confirm, run } from "./store.svelte";
  import { slotLabel } from "./slots";

  // Settings for the selected sequence slot: add, name, BPM, length, remove.
  let { slot, seq, limits }: { slot: number; seq: Sequence | undefined; limits: Limits } = $props();

  const seqNo = $derived(String(slot + 1).padStart(2, "0"));

  // Field values follow the sequence and are sent to Go when a field is
  // left or Enter is pressed. A refused value comes back from Go as the old
  // one, which resets the field.
  let name = $derived(seq?.name ?? "");
  let bpm = $derived(seq?.bpm ?? 120);
  let bars = $derived(seq?.bars ?? 2);

  async function save() {
    if (!seq) return;
    // Copied before any await: props are live and may change meanwhile.
    const slot = seq.slot;
    const newName = name;
    const newBpm = Number(bpm);
    const newBars = Number(bars);
    if (newName === seq.name && newBpm === seq.bpm && newBars === seq.bars) return;
    if (seq.hasNotes && newBars < seq.bars) {
      const ok = await confirm({
        title: "Shorten a sequence with notes?",
        message: `${seq.name} has recorded notes. Notes after bar ${newBars} will not play, but they are kept: lengthening the sequence again brings them back.`,
        items: [],
        action: "Shorten",
      });
      if (!ok) {
        bars = seq.bars;
        return;
      }
    }
    await run(() => Service.SetSequence(slot, newName, newBpm, newBars));
    // run() replaced the state; if Go refused, the fields still hold the
    // typed values, so put the stored ones back.
    const now = app.state?.sequences?.find((q) => q.slot === slot);
    if (now) {
      name = now.name;
      bpm = now.bpm;
      bars = now.bars;
    }
  }

  async function remove() {
    if (!seq) return;
    const slot = seq.slot; // copied before the await, as in save()
    const ok = await confirm({
      title: `Remove ${seq.name}?`,
      message: seq.hasNotes ? "Its recorded notes will be deleted too." : "The sequence has no recorded notes.",
      items: [],
      action: "Remove",
    });
    if (ok) await run(() => Service.RemoveSequence(slot));
  }

  function enter(e: KeyboardEvent) {
    if (e.key === "Enter") (e.currentTarget as HTMLInputElement).blur(); // blur fires change
  }

  function add() {
    return run(() => Service.AddSequence(slot));
  }
</script>

<section class="panel settings" aria-label="Sequence settings">
  <div class="panel-head">
    <div class="slot-id">{slotLabel(slot)}</div>
    <div class="title">
      <h2 class="eyebrow">Sequence settings</h2>
      <div>{seq ? "Sequence " + seqNo : "Empty slot"}</div>
    </div>
  </div>

  {#if !seq}
    <div class="empty-box">
      <p>No sequence in this slot.</p>
      <button type="button" class="btn primary" onclick={add}>Add sequence</button>
    </div>
  {:else}
    <div class="stack">
      <div class="control">
        <label class="label" for="seq-name">Name</label>
        <input
          id="seq-name"
          class="field"
          type="text"
          spellcheck="false"
          autocomplete="off"
          value={name}
          oninput={(e) => (name = e.currentTarget.value)}
          onchange={save}
          onkeydown={enter}
        />
      </div>
      <div class="pair">
        <div class="control">
          <label class="label" for="seq-bpm">BPM ({limits.minBPM}–{limits.maxBPM})</label>
          <input
            id="seq-bpm"
            class="field mono"
            type="number"
            min={limits.minBPM}
            max={limits.maxBPM}
            step="any"
            value={bpm}
            oninput={(e) => (bpm = e.currentTarget.valueAsNumber)}
            onchange={save}
            onkeydown={enter}
          />
        </div>
        <div class="control">
          <label class="label" for="seq-bars">Length (bars)</label>
          <input
            id="seq-bars"
            class="field mono"
            type="number"
            min="1"
            max={limits.maxBars}
            step="1"
            value={bars}
            oninput={(e) => (bars = e.currentTarget.valueAsNumber)}
            onchange={save}
            onkeydown={enter}
          />
        </div>
      </div>
      <span class="hint">Notes are recorded on the device. This screen sets name, tempo and length only.</span>
      {#if seq.hasNotes}
        <span class="hint notes">This sequence has recorded notes.</span>
      {/if}
      <div class="row divided">
        <button type="button" class="btn secondary" onclick={remove}>Remove sequence</button>
      </div>
    </div>
  {/if}
</section>

<style>
  .pair {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .notes {
    color: var(--accent);
  }
</style>
