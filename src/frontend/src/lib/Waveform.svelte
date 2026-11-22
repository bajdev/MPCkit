<script lang="ts">
  import type { Sample } from "./audio";
  import { columns, type Extremes } from "./waveform";

  // Draws file frames `from` up to `to` of a sample, with the part outside
  // the pad's region (start to end, both included) dimmed. Frames are the
  // file's own (see waveform.ts). With onmarker, the region's edges are
  // drawn as Start and End markers that can be dragged.
  let {
    sample,
    blocks,
    from,
    to,
    start,
    end,
    onzoom,
    onmarker,
  }: {
    sample: Sample;
    blocks: Extremes; // peaks(sample)
    from: number;
    to: number;
    start: number;
    end: number;
    onzoom: (factor: number, at: number) => void; // wheel: zoom by factor around file frame `at`
    onmarker?: (which: Marker, frame: number) => void; // a marker dragged to a frame; the caller keeps it in range
  } = $props();

  type Marker = "start" | "end";

  // How close to a marker, in CSS pixels, a press picks it up.
  const GRAB = 10;

  let dragging: Marker | null = null;

  // A marker's x in CSS pixels. Start sits at the left edge of its frame and
  // End at the right edge of its frame, where the dimming begins.
  function markerX(which: Marker, w: number): number {
    const frame = which === "start" ? start : end + 1;
    return ((frame - from) / (to - from)) * w;
  }

  // The marker within GRAB pixels of x, the nearer one if both are.
  function near(x: number, w: number): Marker | null {
    const ds = Math.abs(x - markerX("start", w));
    const de = Math.abs(x - markerX("end", w));
    if (Math.min(ds, de) > GRAB) return null;
    return ds <= de ? "start" : "end";
  }

  function frameAt(e: PointerEvent): number {
    const box = canvas!.getBoundingClientRect();
    return from + ((e.clientX - box.left) / box.width) * (to - from);
  }

  function pointerdown(e: PointerEvent) {
    if (!onmarker || !canvas || e.button !== 0) return;
    const box = canvas.getBoundingClientRect();
    dragging = near(e.clientX - box.left, box.width);
    if (!dragging) return;
    e.preventDefault();
    try {
      canvas.setPointerCapture(e.pointerId); // keeps the drag going outside the canvas
    } catch {
      // the pointer is already gone; the drag still works over the canvas
    }
  }

  function pointermove(e: PointerEvent) {
    if (!onmarker || !canvas) return;
    if (dragging) {
      const f = Math.round(frameAt(e));
      onmarker(dragging, dragging === "start" ? f : f - 1);
      return;
    }
    const box = canvas.getBoundingClientRect();
    canvas.style.cursor = near(e.clientX - box.left, box.width) ? "ew-resize" : "";
  }

  function pointerup(e: PointerEvent) {
    if (!dragging || !canvas) return;
    dragging = null;
    if (canvas.hasPointerCapture(e.pointerId)) canvas.releasePointerCapture(e.pointerId);
  }

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
    draw(canvas, width, height, sample, blocks, from, to, start, end, !!onmarker);
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
    marked: boolean,
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

    // The markers: a line with a tab at the top, not drawn outside the view.
    if (!marked) return;
    g.fillStyle = colour("--text");
    const line = Math.max(2, Math.round(2 * ratio));
    const tab = Math.round(10 * ratio);
    for (const [at, inward] of [
      [x(first), 1],
      [x(last + 1), -1],
    ]) {
      if (at < 0 || at > pw) continue;
      const left = Math.round(at - line / 2);
      g.fillRect(left, 0, line, ph);
      g.fillRect(inward > 0 ? left : left + line - tab, 0, tab, tab);
    }
  }
</script>

<!-- The waveform is a picture of what the text beside it says; screen readers skip it. -->
<div class="wave" bind:this={wrap}>
  <canvas
    bind:this={canvas}
    aria-hidden="true"
    onpointerdown={pointerdown}
    onpointermove={pointermove}
    onpointerup={pointerup}
    onpointercancel={pointerup}
  ></canvas>
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
    touch-action: none; /* a touch drag moves a marker, not the page */
  }
</style>
