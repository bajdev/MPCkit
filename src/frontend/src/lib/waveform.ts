// Waveform data for the start/end editor (waveform plan, PR W3). Plain
// functions, no Svelte: the editor draws what these return.
//
// Frames here are always frames of the sample file, at its own rate, as a
// pad's start and end are. The decoded buffer usually runs at another rate
// (the sound output's), so file frames are turned into buffer positions in
// one place, bufferPos.

import type { Sample } from "./audio";

/** Buffer frames summed into one block by peaks. */
export const BLOCK = 256;

/** Lowest and highest value of each block or column, across all channels. */
export type Extremes = { min: Float32Array; max: Float32Array };

/** Where file frame `frame` falls in the decoded buffer, by time. */
export function bufferPos(sample: Sample, frame: number): number {
  return (frame * sample.buffer.sampleRate) / sample.rate;
}

/**
 * One pass over the decoded buffer: for each block of BLOCK buffer frames,
 * the lowest and highest value across all channels. Stereo is drawn as one
 * waveform, the loudest of the channels at each point (decision 8).
 */
export function peaks(sample: Sample): Extremes {
  const buf = sample.buffer;
  const blocks = Math.ceil(buf.length / BLOCK);
  const min = new Float32Array(blocks).fill(Infinity);
  const max = new Float32Array(blocks).fill(-Infinity);
  for (let c = 0; c < buf.numberOfChannels; c++) {
    const data = buf.getChannelData(c);
    for (let i = 0; i < data.length; i++) {
      const b = (i / BLOCK) | 0;
      const v = data[i];
      if (v < min[b]) min[b] = v;
      if (v > max[b]) max[b] = v;
    }
  }
  return { min, max };
}

/**
 * The lowest and highest value for each of `width` pixel columns covering
 * file frames `from` up to (not including) `to`. A column spanning BLOCK or
 * more buffer frames reads the blocks from peaks; a narrower one reads the
 * buffer itself, so the closest zoom still shows the real samples.
 */
export function columns(sample: Sample, blocks: Extremes, from: number, to: number, width: number): Extremes {
  const buf = sample.buffer;
  const channels = Array.from({ length: buf.numberOfChannels }, (_, c) => buf.getChannelData(c));
  const min = new Float32Array(width);
  const max = new Float32Array(width);
  const step = (to - from) / width; // file frames per column
  for (let x = 0; x < width; x++) {
    const a = bufferPos(sample, from + x * step);
    const b = bufferPos(sample, from + (x + 1) * step);
    let lo = Infinity;
    let hi = -Infinity;
    if (b - a >= BLOCK) {
      const last = Math.min(Math.ceil(b / BLOCK), blocks.min.length);
      for (let k = Math.floor(a / BLOCK); k < last; k++) {
        lo = Math.min(lo, blocks.min[k]);
        hi = Math.max(hi, blocks.max[k]);
      }
    } else {
      // At least one buffer frame each side, so the line between two
      // samples is drawn even when a column is narrower than a sample.
      const first = Math.max(0, Math.floor(a));
      const last = Math.min(buf.length - 1, Math.max(Math.ceil(b), first + 1));
      for (const data of channels) {
        for (let i = first; i <= last; i++) {
          if (data[i] < lo) lo = data[i];
          if (data[i] > hi) hi = data[i];
        }
      }
    }
    if (lo > hi) lo = hi = 0; // past the end of the buffer
    min[x] = lo;
    max[x] = hi;
  }
  return { min, max };
}

/**
 * The frames where the sample file crosses zero, in order (Go's
 * wav.ZeroCrossings: channels mixed to mono). Frames of the file, read from
 * the file itself, not the decoded buffer, which may run at another rate.
 */
export async function fetchZeros(url: string): Promise<Uint32Array> {
  const res = await fetch(url + (url.includes("?") ? "&" : "?") + "zeros=1");
  if (!res.ok) throw new Error((await res.text()) || `Could not read the sample (${res.status}).`);
  return new Uint32Array(await res.arrayBuffer());
}

// The index of the first crossing at or after frame f.
function firstAtOrAfter(zeros: Uint32Array, f: number): number {
  let lo = 0;
  let hi = zeros.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (zeros[mid] < f) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

/** The crossing nearest frame f within lo..hi (both included), or null if there is none. */
export function nearestZero(zeros: Uint32Array, f: number, lo: number, hi: number): number | null {
  const t = Math.min(Math.max(f, lo), hi);
  const i = firstAtOrAfter(zeros, t); // zeros[i] >= t >= lo, zeros[i - 1] < t <= hi
  const after = i < zeros.length && zeros[i] <= hi ? zeros[i] : null;
  const before = i > 0 && zeros[i - 1] >= lo ? zeros[i - 1] : null;
  if (after === null) return before;
  if (before === null) return after;
  return t - before <= after - t ? before : after;
}

/** The next crossing after frame f (dir 1) or before it (dir -1) within lo..hi, or null. */
export function stepZero(zeros: Uint32Array, f: number, dir: 1 | -1, lo: number, hi: number): number | null {
  if (dir > 0) {
    const i = firstAtOrAfter(zeros, f + 1);
    return i < zeros.length && zeros[i] <= hi && zeros[i] >= lo ? zeros[i] : null;
  }
  const j = firstAtOrAfter(zeros, f) - 1;
  return j >= 0 && zeros[j] >= lo && zeros[j] <= hi ? zeros[j] : null;
}
