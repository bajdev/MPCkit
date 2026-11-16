// Pad preview (build plan 4.5). Plays a pad's sample with its tuning and
// slice region, and cuts off other pads in the same mute group. A preview
// only: no effects.

export type Playable = {
  slot: number;
  audio: string; // URL from Go; empty for an empty pad
  coarse: number;
  fine: number;
  muteGroup: number;
  start: number; // first frame
  end: number; // last frame
};

type Voice = { slot: number; group: number; src: AudioBufferSourceNode; gain: GainNode };

// A loaded sample. The buffer is at whatever rate the webview decoded it to,
// usually the sound output's; frames and rate describe the file itself, as
// a pad's start and end do.
export type Sample = { buffer: AudioBuffer; frames: number; rate: number };

// Fade used when a pad is cut off, so the cut does not click. Our choice,
// not measured from the device.
const FADE = 0.008;

let ctx: AudioContext | null = null;
const samples = new Map<string, Promise<Sample>>();
const voices = new Map<number, Voice>(); // sounding pads, by slot

function context(): AudioContext {
  ctx ??= new AudioContext();
  if (ctx.state === "suspended") void ctx.resume();
  return ctx;
}

/**
 * Loads a pad's sample from Go, once: later calls for the same URL share the
 * first load. The start/end editor uses it too.
 */
export function loadSample(url: string): Promise<Sample> {
  let p = samples.get(url);
  if (!p) {
    p = fetchSample(url);
    samples.set(url, p);
    p.catch(() => samples.delete(url)); // try again next time
  }
  return p;
}

/**
 * Loads a sample from Go and decodes it with ac (the preview's context
 * unless given). Exported so a test page can decode at a chosen rate.
 */
export async function fetchSample(url: string, ac: BaseAudioContext = context()): Promise<Sample> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Could not load the sample (${res.status}).`);
  const frames = Number(res.headers.get("X-Mpckit-Frames"));
  const rate = Number(res.headers.get("X-Mpckit-Sample-Rate"));
  let buffer: AudioBuffer;
  try {
    buffer = await ac.decodeAudioData(await res.arrayBuffer());
  } catch {
    // The webview cannot decode this format: ask Go to decode it.
    return fetchDecoded(url, ac);
  }
  // Go always sends both headers; without them, fall back to the buffer.
  if (!(frames > 0 && rate > 0)) return { buffer, frames: buffer.length, rate: buffer.sampleRate };
  return { buffer, frames, rate };
}

// Go's decoded format: sample rate, channels, frames (uint32 LE), then each
// channel's frames as float32 LE. The buffer keeps the file's rate.
async function fetchDecoded(url: string, ac: BaseAudioContext): Promise<Sample> {
  const res = await fetch(url + (url.includes("?") ? "&" : "?") + "pcm=1");
  if (!res.ok) throw new Error((await res.text()) || `Could not decode the sample (${res.status}).`);
  const bytes = await res.arrayBuffer();
  const head = new DataView(bytes, 0, 12);
  const rate = head.getUint32(0, true);
  const channels = head.getUint32(4, true);
  const frames = head.getUint32(8, true);
  const buffer = ac.createBuffer(channels, frames, rate);
  for (let c = 0; c < channels; c++) {
    buffer.copyToChannel(new Float32Array(bytes, 12 + c * frames * 4, frames), c);
  }
  return { buffer, frames, rate };
}

/**
 * Where a pad's region starts in the sample, and how long it plays, in
 * seconds. start and end are frames of the file, so they are divided by the
 * file's rate, not the decoded buffer's. Both are clamped to the file; an
 * end that is not after start means the last frame.
 */
export function region(sample: Sample, start: number, end: number): { offset: number; duration: number } {
  const last = sample.frames - 1;
  const first = Math.min(Math.max(start, 0), last);
  const stop = end > first ? Math.min(end, last) : last;
  return { offset: first / sample.rate, duration: (stop - first + 1) / sample.rate };
}

function stop(slot: number) {
  const v = voices.get(slot);
  if (!v || !ctx) return;
  voices.delete(slot);
  const t = ctx.currentTime;
  v.gain.gain.setValueAtTime(v.gain.gain.value, t);
  v.gain.gain.linearRampToValueAtTime(0, t + FADE);
  v.src.stop(t + FADE);
}

/** Plays a pad. Throws if the sample cannot be loaded or decoded. */
export async function play(p: Playable): Promise<void> {
  if (!p.audio) return;
  const sample = await loadSample(p.audio);
  const c = context();

  stop(p.slot); // pads are monophonic: a new hit restarts the pad
  if (p.muteGroup) {
    for (const v of [...voices.values()]) {
      if (v.group === p.muteGroup) stop(v.slot);
    }
  }

  const gain = c.createGain();
  gain.connect(c.destination);
  const src = c.createBufferSource();
  src.buffer = sample.buffer;
  src.detune.value = p.coarse * 100 + p.fine; // pitch and speed together, as on the device
  src.connect(gain);

  const { offset, duration } = region(sample, p.start, p.end);
  src.start(0, offset, duration);

  const voice: Voice = { slot: p.slot, group: p.muteGroup, src, gain };
  voices.set(p.slot, voice);
  src.onended = () => {
    if (voices.get(p.slot) === voice) voices.delete(p.slot);
    gain.disconnect();
  };
}

/** Stops everything, e.g. when the project changes. */
export function stopAll() {
  for (const slot of [...voices.keys()]) stop(slot);
}
