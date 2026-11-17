<script lang="ts">
  import type { Sample } from "./audio";
  import { columns, type Extremes } from "./waveform";

  // Draws file frames `from` up to `to` of a sample, with the part outside
  // the pad's region (start to end, both included) dimmed. Frames are the
  // file's own (see waveform.ts).
  let {
    sample,
    blocks,
    from,
    to,
    start,
    end,
    onzoom,
  }: {
    sample: Sample;
    blocks: Extremes; // peaks(sample)
    from: number;
    to: number;
    start: number;
    end: number;
    onzoom: (factor: number, at: number) => void; // wheel: zoom by factor around file frame `at`
  } = $props();

  let wrap: HTMLDivElement | undefined = $state();
  let canvas: HTMLCanvasElement | undefined = $state();
  let width = $state(0);
  let height = $state(0);

  // Follow the element's size.
  $effect(() => {
    if (!wrap) return;
    const observer = new ResizeObserver(([entry]) => {
      width = entry.contentRect.width;
      height = entry.contentRect.height;
    });
    observer.observe(wrap);
    return () => observer.disconnect();
  });

  // The wheel zooms around the pointer. Listened to directly, not passively,
  // so the dialog does not scroll as well.
  $effect(() => {
    if (!canvas) return;
    const el = canvas;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      const box = el.getBoundingClientRect();
      const at = from + ((e.clientX - box.left) / box.width) * (to - from);
      onzoom(e.deltaY < 0 ? 0.8 : 1.25, at);
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  });

  $effect(() => {
    draw(canvas, width, height, sample, blocks, from, to, start, end);
  });

  function draw(
    el: HTMLCanvasElement | undefined,
    w: number,
    h: number,
    s: Sample,
    b: Extremes,
    viewFrom: number,
    viewTo: number,
    first: number,
    last: number,
  ) {
    if (!el || w <= 0 || h <= 0) return;
    // Draw at the screen's pixel ratio, so the line is sharp on
    // high-density displays.
    const ratio = window.devicePixelRatio || 1;
    const pw = Math.round(w * ratio);
    const ph = Math.round(h * ratio);
    if (el.width !== pw) el.width = pw;
    if (el.height !== ph) el.height = ph;
    const g = el.getContext("2d");
    if (!g) return;
    const css = getComputedStyle(el);
    const colour = (name: string) => css.getPropertyValue(name).trim();

    g.fillStyle = colour("--bg");
    g.fillRect(0, 0, pw, ph);
    const mid = ph / 2;
    g.fillStyle = colour("--line");
    g.fillRect(0, Math.floor(mid), pw, Math.max(1, Math.round(ratio)));

    const cols = columns(s, b, viewFrom, viewTo, pw);
    g.fillStyle = colour("--accent");
    for (let x = 0; x < pw; x++) {
      const top = mid - cols.max[x] * mid;
      const bottom = mid - cols.min[x] * mid;
      g.fillRect(x, top, 1, Math.max(1, bottom - top));
    }

    // Dim what is outside the region. Frame f covers x from f to f + 1.
    const x = (frame: number) => ((frame - viewFrom) / (viewTo - viewFrom)) * pw;
    g.globalAlpha = 0.7;
    g.fillStyle = colour("--bg");
    const left = Math.min(pw, Math.max(0, x(first)));
    const right = Math.min(pw, Math.max(0, x(last + 1)));
    if (left > 0) g.fillRect(0, 0, left, ph);
    if (right < pw) g.fillRect(right, 0, pw - right, ph);
    g.globalAlpha = 1;
  }
</script>

<!-- The waveform is a picture of what the text beside it says; screen readers skip it. -->
<div class="wave" bind:this={wrap}>
  <canvas bind:this={canvas} aria-hidden="true"></canvas>
</div>

<style>
  .wave {
    position: relative;
    height: 180px;
    border: 1px solid var(--line);
    border-radius: 6px;
    overflow: hidden;
  }
  canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    display: block;
  }
</style>
