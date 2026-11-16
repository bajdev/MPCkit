<script lang="ts">
  import type { Pad } from "./api";
  import { loadSample, play, type Sample } from "./audio";
  import Modal from "./Modal.svelte";
  import { message } from "./store.svelte";
  import Waveform from "./Waveform.svelte";
  import { peaks, type Extremes } from "./waveform";

  // The start/end dialog for one pad (waveform plan, PR W3): shows the
  // sample with the pad's region, zooms, scrolls and plays. Read-only for
  // now; PR W4 adds the markers and fields.
  let { pad, onclose }: { pad: Pad; onclose: () => void } = $props();

  // The closest zoom: this many file frames across the view.
  const MIN_SPAN = 64;

  let sample = $state<Sample | null>(null);
  let blocks = $state<Extremes | null>(null);
  let loadError = $state("");
  let playError = $state("");
  let closeButton: HTMLButtonElement | undefined = $state();

  // The view: file frames `from` up to (not including) `to`.
  let from = $state(0);
  let to = $state(1);

  $effect(() => {
    const url = pad.audio;
    sample = blocks = null;
    loadError = "";
    if (!url) {
      loadError = "The sample file is missing.";
      return;
    }
    let current = true;
    loadSample(url).then(
      (s) => {
        if (!current) return;
        blocks = peaks(s);
        sample = s;
        from = 0;
        to = s.frames;
      },
      (e) => {
        if (current) loadError = message(e);
      },
    );
    return () => (current = false);
  });

  const frames = $derived(sample?.frames ?? 1);
  const span = $derived(to - from);
  const whole = $derived(span >= frames);

  // The pad's region, clamped to the file: a loaded project can hold a start
  // or end outside it. An end not after start means the last frame, as in
  // the preview.
  const regionStart = $derived(Math.min(Math.max(pad.start, 0), frames - 1));
  const regionEnd = $derived(pad.end > regionStart ? Math.min(pad.end, frames - 1) : frames - 1);

  /** Zooms by factor (below 1 zooms in), keeping file frame `at` where it is. */
  function zoom(factor: number, at = from + span / 2) {
    const next = Math.round(Math.min(frames, Math.max(MIN_SPAN, span * factor)));
    const ratio = span > 0 ? (at - from) / span : 0.5;
    const start = Math.round(Math.min(frames - next, Math.max(0, at - ratio * next)));
    from = start;
    to = start + next;
  }

  function scrollTo(start: number) {
    const width = span; // read first: span follows from and to
    from = start;
    to = start + width;
  }

  async function preview() {
    playError = "";
    try {
      await play(pad);
    } catch (e) {
      playError = message(e);
    }
  }

  function seconds(n: number, rate: number): string {
    return (n / rate).toFixed(3) + " s";
  }
</script>

<Modal titleId="trim-title" {onclose} focus={closeButton} width={880}>
  <h2 id="trim-title">{pad.label} · {pad.file}</h2>

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {:else if !sample || !blocks}
    <p class="hint" role="status">Loading the sample…</p>
  {:else}
    <p class="hint mono">
      {sample.frames.toLocaleString("en")} frames · {seconds(sample.frames, sample.rate)} · {sample.rate.toLocaleString(
        "en",
      )} Hz
    </p>
    <Waveform {sample} {blocks} {from} {to} start={regionStart} end={regionEnd} onzoom={zoom} />
    <input
      class="scroll"
      type="range"
      aria-label="Scroll the waveform"
      min="0"
      max={Math.max(0, frames - span)}
      step="1"
      value={from}
      disabled={whole}
      oninput={(e) => scrollTo(e.currentTarget.valueAsNumber)}
    />
    <div class="row">
      <button type="button" class="btn secondary" disabled={span <= MIN_SPAN} onclick={() => zoom(0.5)}>Zoom in</button>
      <button type="button" class="btn secondary" disabled={whole} onclick={() => zoom(2)}>Zoom out</button>
      <button type="button" class="btn secondary" disabled={whole} onclick={() => zoom(frames / span)}
        >Whole sample</button
      >
      <button type="button" class="btn primary" onclick={preview}>Play</button>
    </div>
    {#if playError}<p class="error" role="alert">{playError}</p>{/if}
  {/if}

  <div class="buttons">
    <button type="button" class="btn secondary" bind:this={closeButton} onclick={onclose}>Close</button>
  </div>
</Modal>

<style>
  h2 {
    margin: 0;
    font-family: var(--display);
    font-size: 20px;
    font-weight: 700;
    overflow-wrap: anywhere;
  }
  .scroll {
    width: 100%;
    margin: 0;
    accent-color: var(--accent);
  }
  .error {
    margin: 0;
    color: #ffb4b4;
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
  }
</style>
