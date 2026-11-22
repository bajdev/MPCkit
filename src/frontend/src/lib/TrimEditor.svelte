<script lang="ts">
  import { Service, type Pad } from "./api";
  import { loadSample, play, type Sample } from "./audio";
  import Modal from "./Modal.svelte";
  import { app, message, run } from "./store.svelte";
  import Waveform from "./Waveform.svelte";
  import { fetchZeros, nearestZero, peaks, stepZero, type Extremes } from "./waveform";

  // The start/end dialog for one pad (waveform plan, PRs W3 and W4): shows
  // the sample, zooms, scrolls and plays, and edits where the pad starts and
  // ends. The dialog works on its own copy; Apply saves both in one step,
  // and Cancel or Escape drops the changes (decision 4).
  let { pad, onclose }: { pad: Pad; onclose: () => void } = $props();

  // The closest zoom: this many file frames across the view.
  const MIN_SPAN = 64;
  // Go to start and Go to end show about this many frames across.
  const CLOSE_SPAN = 2000;

  let sample = $state<Sample | null>(null);
  let blocks = $state<Extremes | null>(null);
  let loadError = $state("");
  let playError = $state("");
  let applyError = $state("");

  // Snap to zero (PR W4b): markers and typed values go to the nearest frame
  // where the sample crosses zero. On when the dialog opens.
  let snap = $state(true);
  let zeros = $state<Uint32Array | null>(null); // null until read, or if it cannot be
  let cancelButton: HTMLButtonElement | undefined = $state();

  // The view: file frames `from` up to (not including) `to`.
  let from = $state(0);
  let to = $state(1);

  // The dialog's own region, in file frames, both included. Start is at
  // least one frame before end: at least 2 frames play (decision 5).
  let start = $state(0);
  let end = $state(0);

  $effect(() => {
    const url = pad.audio;
    sample = blocks = null;
    zeros = null;
    loadError = "";
    if (!url) {
      loadError = "The sample file is missing.";
      return;
    }
    let current = true;
    fetchZeros(url).then(
      (z) => {
        if (current) zeros = z;
      },
      () => {}, // snapping is then unavailable; the dialog works without it
    );
    loadSample(url).then(
      (s) => {
        if (!current) return;
        blocks = peaks(s);
        sample = s;
        from = 0;
        to = s.frames;
        // The pad's region, clamped to the file: a loaded project can hold
        // a start or end outside it. An end not after start means the last
        // frame, as in the preview.
        const last = s.frames - 1;
        start = Math.max(0, Math.min(pad.start, last - 1));
        end = pad.end > start ? Math.min(pad.end, last) : last;
      },
      (e) => {
        if (current) loadError = message(e);
      },
    );
    return () => (current = false);
  });

  const frames = $derived(sample?.frames ?? 2);
  const last = $derived(frames - 1);
  const span = $derived(to - from);
  const whole = $derived(span >= frames);
  const changed = $derived(start !== pad.start || end !== pad.end);

  const snapping = $derived(snap && zeros !== null);

  // Start is 0 to end - 1, End is start + 1 to the last frame. While
  // snapping, each goes to the nearest crossing in its range, or stays where
  // it was put if the range has none.
  function setStart(f: number) {
    const v = Math.min(Math.max(Math.round(f), 0), end - 1);
    start = snapping ? (nearestZero(zeros!, v, 0, end - 1) ?? v) : v;
  }

  function setEnd(f: number) {
    const v = Math.max(Math.min(Math.round(f), last), start + 1);
    end = snapping ? (nearestZero(zeros!, v, start + 1, last) ?? v) : v;
  }

  // While snapping, Up and Down in a field jump to the next or previous
  // crossing instead of the next frame.
  function step(e: KeyboardEvent, which: "start" | "end") {
    if (!snapping || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) return;
    e.preventDefault();
    const dir = e.key === "ArrowUp" ? 1 : -1;
    if (which === "start") start = stepZero(zeros!, start, dir, 0, end - 1) ?? start;
    else end = stepZero(zeros!, end, dir, start + 1, last) ?? end;
  }

  /** Zooms by factor (below 1 zooms in), keeping file frame `at` where it is. */
  function zoom(factor: number, at = from + span / 2) {
    const next = Math.round(Math.min(frames, Math.max(MIN_SPAN, span * factor)));
    const ratio = span > 0 ? (at - from) / span : 0.5;
    const first = Math.round(Math.min(frames - next, Math.max(0, at - ratio * next)));
    from = first;
    to = first + next;
  }

  function scrollTo(first: number) {
    const width = span; // read first: span follows from and to
    from = first;
    to = first + width;
  }

  /** Shows about CLOSE_SPAN frames centred on frame f. */
  function goTo(f: number) {
    const next = Math.min(frames, CLOSE_SPAN);
    const first = Math.round(Math.min(frames - next, Math.max(0, f - next / 2)));
    from = first;
    to = first + next;
  }

  function reset() {
    start = 0;
    end = last;
  }

  async function preview() {
    playError = "";
    try {
      await play({ ...pad, start, end }); // the dialog's region, before it is saved
    } catch (e) {
      playError = message(e);
    }
  }

  async function apply() {
    applyError = "";
    const slot = pad.slot;
    if (await run(() => Service.SetTrim(slot, start, end))) {
      onclose();
    } else {
      applyError = app.error; // also in the window's banner, behind the dialog
    }
  }

  // A typed value is used while it is in range, and corrected when the
  // field loses focus (or Enter is pressed).
  function typed(e: Event & { currentTarget: HTMLInputElement }, which: "start" | "end") {
    const v = e.currentTarget.valueAsNumber;
    if (!Number.isInteger(v)) return;
    if (which === "start" && v >= 0 && v < end) start = v;
    if (which === "end" && v > start && v <= last) end = v;
  }

  function corrected(e: Event & { currentTarget: HTMLInputElement }, which: "start" | "end") {
    const v = e.currentTarget.valueAsNumber;
    if (which === "start") setStart(Number.isFinite(v) ? v : start);
    else setEnd(Number.isFinite(v) ? v : end);
    e.currentTarget.value = String(which === "start" ? start : end);
  }

  function seconds(n: number, rate: number): string {
    return (n / rate).toFixed(3) + " s";
  }

  function ms(f: number, rate: number): string {
    return ((f / rate) * 1000).toFixed(1) + " ms";
  }
</script>

<Modal titleId="trim-title" {onclose} focus={cancelButton} width={880}>
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
    <Waveform
      {sample}
      {blocks}
      {from}
      {to}
      {start}
      {end}
      onzoom={zoom}
      onmarker={(which, f) => (which === "start" ? setStart(f) : setEnd(f))}
    />
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
      <label class="snap" title={zeros ? undefined : "Finding the zero crossings…"}>
        <input type="checkbox" bind:checked={snap} disabled={!zeros} />
        Snap to zero
      </label>
    </div>
    <div class="points">
      <div class="point">
        <label class="label" for="trim-start">Start</label>
        <input
          id="trim-start"
          class="field mono"
          type="number"
          min="0"
          max={end - 1}
          step="1"
          value={start}
          aria-describedby="trim-start-ms"
          oninput={(e) => typed(e, "start")}
          onkeydown={(e) => step(e, "start")}
          onchange={(e) => corrected(e, "start")}
        />
        <span id="trim-start-ms" class="ms mono">{ms(start, sample.rate)}</span>
        <button type="button" class="btn secondary" onclick={() => goTo(start)}>Go to start</button>
      </div>
      <div class="point">
        <label class="label" for="trim-end">End</label>
        <input
          id="trim-end"
          class="field mono"
          type="number"
          min={start + 1}
          max={last}
          step="1"
          value={end}
          aria-describedby="trim-end-ms"
          oninput={(e) => typed(e, "end")}
          onkeydown={(e) => step(e, "end")}
          onchange={(e) => corrected(e, "end")}
        />
        <span id="trim-end-ms" class="ms mono">{ms(end, sample.rate)}</span>
        <button type="button" class="btn secondary" onclick={() => goTo(end)}>Go to end</button>
      </div>
    </div>
    {#if playError}<p class="error" role="alert">{playError}</p>{/if}
    {#if applyError}<p class="error" role="alert">{applyError}</p>{/if}
  {/if}

  <div class="buttons">
    {#if sample}
      <button type="button" class="btn secondary reset" onclick={reset} disabled={start === 0 && end === last}
        >Reset</button
      >
    {/if}
    <button type="button" class="btn secondary" bind:this={cancelButton} onclick={onclose}>Cancel</button>
    {#if sample}
      <button type="button" class="btn primary" disabled={!changed} onclick={apply}>Apply</button>
    {/if}
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
  .snap {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-left: auto;
    color: var(--soft);
    font-size: 13px;
    cursor: pointer;
  }
  .snap input {
    accent-color: var(--accent);
  }
  /* Start and End side by side, each: label, field, time, Go to. */
  .points {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px 24px;
  }
  .point {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .point .field {
    width: 9em;
    flex: none;
  }
  .ms {
    flex: 1;
    min-width: 0;
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
  .buttons .reset {
    margin-right: auto;
  }
</style>
