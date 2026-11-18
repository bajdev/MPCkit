<script lang="ts">
  import { onMount } from "svelte";
  import AboutModal from "./lib/AboutModal.svelte";
  import { Service } from "./lib/api";
  import { play } from "./lib/audio";
  import BankBar from "./lib/BankBar.svelte";
  import ConfirmModal from "./lib/ConfirmModal.svelte";
  import PadPanel from "./lib/PadPanel.svelte";
  import PadPicker from "./lib/PadPicker.svelte";
  import { transfer, transferKind } from "./lib/pads";
  import SequencePanel from "./lib/SequencePanel.svelte";
  import SlotGrid, { type Cell } from "./lib/SlotGrid.svelte";
  import {
    app,
    confirm,
    exportFolder,
    exportZip,
    listen,
    loadProject,
    message,
    newProject,
    refresh,
    run,
    ui,
    type Tab,
  } from "./lib/store.svelte";
  import { BANKS, PER_BANK, bars, gridSlots, listText, tuneText } from "./lib/slots";

  onMount(() => {
    refresh();
    return listen();
  });

  const st = $derived(app.state);
  // The bindings type Go slices as `T[] | null`; Go always sends arrays.
  const pads = $derived(st?.pads ?? []);

  // Sequences by slot, for the grid and the panel.
  const seqBySlot = $derived(new Map((st?.sequences ?? []).map((q) => [q.slot, q])));

  function perBank(filled: (slot: number) => boolean): number[] {
    return Array.from({ length: BANKS.length }, (_, b) => {
      let n = 0;
      for (let i = 0; i < PER_BANK; i++) if (filled(b * PER_BANK + i)) n++;
      return n;
    });
  }

  // Export would fail on a missing sample file, so it is not offered at all.
  const exportBlocked = $derived((st?.missingCount ?? 0) > 0);
  const exportBlockedWhy = $derived(
    exportBlocked
      ? "Export is disabled while sample files are missing. Replace them, or put the files back and load the project again."
      : undefined,
  );

  const padCounts = $derived(perBank((s) => (pads[s]?.file ?? "") !== ""));
  const seqCounts = $derived(perBank((s) => seqBySlot.has(s)));

  const padCells: Cell[] = $derived(
    st
      ? gridSlots(ui.padBank).map((slot) => {
          const p = pads[slot];
          const badges = p.missing
            ? ["missing"]
            : [tuneText(p.coarse, p.fine), p.muteGroup ? "M" + p.muteGroup : "", p.hasNotes ? "notes" : ""].filter(
                Boolean,
              );
          return {
            slot,
            label: p.label,
            name: p.name,
            meta: p.file ? badges.join(" · ") : "",
            aria: `Pad ${p.label}, ${p.file || "empty"}${p.missing ? ", sample file missing" : ""}${p.hasNotes ? ", has recorded notes" : ""}`,
            warn: p.missing,
          };
        })
      : [],
  );

  const seqCells: Cell[] = $derived(
    st
      ? gridSlots(ui.seqBank).map((slot) => {
          const q = seqBySlot.get(slot);
          const label = pads[slot].label; // pad and sequence slots share labels
          return {
            slot,
            label,
            name: q?.name ?? "",
            meta: q ? `${q.bpm} · ${bars(q.bars)}${q.hasNotes ? " · notes" : ""}` : "",
            aria: `Sequence slot ${label}, ${q?.name ?? "empty"}`,
          };
        })
      : [],
  );

  function bankSlots(bank: number): number[] {
    return Array.from({ length: PER_BANK }, (_, i) => bank * PER_BANK + i);
  }

  // Clear bank on the Pads tab: asks first, listing the pads. The selected
  // pad stays selected, now empty.
  async function clearPadBank() {
    const bank = ui.padBank;
    const filled = bankSlots(bank)
      .map((s) => pads[s])
      .filter((p) => p.file !== "");
    const noted = filled.filter((p) => p.hasNotes).map((p) => p.label);
    const ok = await confirm({
      title: `Clear bank ${BANKS[bank]}?`,
      message:
        "These pads will be emptied. There is no undo." +
        (noted.length
          ? ` Sequences have recorded notes for ${listText(noted)}. The notes stay, and play nothing until ${noted.length === 1 ? "that pad is" : "those pads are"} filled again.`
          : ""),
      items: filled.map((p) => `${p.label} ${p.name}`),
      action: "Clear",
    });
    if (ok) await run(() => Service.ClearBank(bank));
  }

  // Clear bank on the Sequences tab: asks first, listing the sequences. A
  // project keeps one sequence, so Go adds a fresh one if the bank held them all.
  async function clearSeqBank() {
    const bank = ui.seqBank;
    const seqs = bankSlots(bank).flatMap((s) => seqBySlot.get(s) ?? []);
    const noted = seqs.filter((q) => q.hasNotes).map((q) => pads[q.slot].label);
    const first = bank * PER_BANK;
    const fresh = seqs.length === (st?.sequenceCount ?? 0);
    const ok = await confirm({
      title: `Clear bank ${BANKS[bank]}?`,
      message:
        "These sequences will be removed. There is no undo." +
        (noted.length
          ? ` ${listText(noted)} ${noted.length === 1 ? "has" : "have"} recorded notes, which will be deleted too.`
          : "") +
        (fresh
          ? ` A project needs one sequence, so an empty Sequence ${String(first + 1).padStart(2, "0")} is added in ${pads[first].label}.`
          : ""),
      items: seqs.map((q) => `${pads[q.slot].label} ${q.name}`),
      action: "Clear",
    });
    if (ok) await run(() => Service.RemoveSequenceBank(bank));
  }

  // Clicking a pad selects it and, if it has a sample, plays it.
  async function pickPad(slot: number) {
    ui.padSlot = slot;
    const p = pads[slot];
    if (ui.playOnClick && p?.audio) {
      try {
        await play(p);
      } catch (e) {
        app.error = message(e);
      }
    }
  }

  // A pad dropped on another: move, swap onto a filled pad, Alt to copy.
  function dropPad(from: number, to: number, copy: boolean) {
    transfer(transferKind(pads[from], pads[to], copy), from, to);
  }

  async function rename(e: Event & { currentTarget: HTMLInputElement }) {
    const input = e.currentTarget;
    const name = input.value;
    if (name === st?.name) return;
    await run(() => Service.SetName(name));
    input.value = app.state?.name ?? name; // shows the old name again if refused
  }

  function pickTab(t: Tab) {
    ui.tab = t;
  }

  // Arrow keys move between the two tabs, as in the WAI-ARIA tabs pattern.
  function tabKey(e: KeyboardEvent) {
    if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
      ui.tab = ui.tab === "pads" ? "sequences" : "pads";
      document.getElementById("tab-" + ui.tab)?.focus();
    }
  }
</script>

<div class="page">
  <header>
    <h1 class="visually-hidden">MPCkit</h1>
    <div class="name">
      <label class="eyebrow" for="project-name">Project name</label>
      <input
        id="project-name"
        type="text"
        value={st?.name ?? ""}
        spellcheck="false"
        autocomplete="off"
        onchange={rename}
      />
    </div>
    <div class="actions">
      <button type="button" class="btn" onclick={newProject}>New</button>
      <button type="button" class="btn" onclick={loadProject}>Load</button>
      <button type="button" class="btn" disabled={exportBlocked} title={exportBlockedWhy} onclick={exportZip}
        >Export zip</button
      >
      <button type="button" class="btn primary" disabled={exportBlocked} title={exportBlockedWhy} onclick={exportFolder}
        >Export to folder</button
      >
    </div>
    <button type="button" class="about-link" onclick={() => (ui.aboutOpen = true)}>About</button>
  </header>

  {#if app.error}
    <div class="error" role="alert">
      <span>{app.error}</span>
      <button type="button" class="dismiss" aria-label="Dismiss" onclick={() => (app.error = "")}>×</button>
    </div>
  {/if}

  <div class="tabs" role="tablist" aria-label="Views">
    <button
      id="tab-pads"
      type="button"
      role="tab"
      aria-selected={ui.tab === "pads"}
      aria-controls="view-pads"
      tabindex={ui.tab === "pads" ? 0 : -1}
      onclick={() => pickTab("pads")}
      onkeydown={tabKey}>Pads</button
    >
    <button
      id="tab-sequences"
      type="button"
      role="tab"
      aria-selected={ui.tab === "sequences"}
      aria-controls="view-sequences"
      tabindex={ui.tab === "sequences" ? 0 : -1}
      onclick={() => pickTab("sequences")}
      onkeydown={tabKey}>Sequences</button
    >
  </div>

  {#if st}
    {#if ui.tab === "pads"}
      <div id="view-pads" class="panels" role="tabpanel" aria-labelledby="tab-pads">
        <section class="panel grid-panel" aria-label="Pads">
          <div class="grid-head">
            <div class="head-start">
              <h2 class="eyebrow">Pad bank</h2>
              <button
                type="button"
                class="switch"
                role="switch"
                aria-checked={ui.playOnClick}
                onclick={() => (ui.playOnClick = !ui.playOnClick)}
              >
                <span class="track" aria-hidden="true"><span class="thumb"></span></span>
                Play on click
              </button>
            </div>
            <div class="head-end">
              <span class="mono summary">Bank {BANKS[ui.padBank]} · {padCounts[ui.padBank]} of 16 filled</span>
              <button type="button" class="head-btn" disabled={padCounts[ui.padBank] === 0} onclick={clearPadBank}
                >Clear bank</button
              >
            </div>
          </div>
          <BankBar
            bank={ui.padBank}
            counts={padCounts}
            what="pads filled"
            onpick={(b) => {
              ui.padBank = b;
              ui.padSlot = b * PER_BANK;
            }}
            ondragbank={(b) => (ui.padBank = b)}
            fileDrop
          />
          <SlotGrid cells={padCells} selected={ui.padSlot} onpick={pickPad} dropTarget ondroppad={dropPad} />
          <p class="hint grid-hint">
            {ui.playOnClick ? "Click a pad to select and hear it." : "Click a pad to select it."} Drag a pad to move it, drag
            onto a filled pad to swap, hold Alt while dragging to copy. Drag over a bank letter to reach another bank. Drop
            a WAV file on a pad to assign it, or several files or a folder on an empty bank to fill it.
          </p>
        </section>
        <PadPanel pad={pads[ui.padSlot]} limits={st.limits} />
      </div>
    {:else}
      <div id="view-sequences" class="panels" role="tabpanel" aria-labelledby="tab-sequences">
        <section class="panel grid-panel" aria-label="Sequence slots">
          <div class="grid-head">
            <h2 class="eyebrow">Sequence bank</h2>
            <div class="head-end">
              <span class="mono summary">Bank {BANKS[ui.seqBank]} · {seqCounts[ui.seqBank]} of 16 used</span>
              <button type="button" class="head-btn" disabled={seqCounts[ui.seqBank] === 0} onclick={clearSeqBank}
                >Clear bank</button
              >
            </div>
          </div>
          <BankBar
            bank={ui.seqBank}
            counts={seqCounts}
            what="slots used"
            onpick={(b) => {
              ui.seqBank = b;
              ui.seqSlot = b * PER_BANK;
            }}
          />
          <SlotGrid cells={seqCells} selected={ui.seqSlot} onpick={(s) => (ui.seqSlot = s)} />
          <p class="hint grid-hint">
            Each pad is one sequence slot, {st.limits.maxSequences} in total. Click an empty slot to add a sequence there.
          </p>
        </section>
        <SequencePanel slot={ui.seqSlot} seq={seqBySlot.get(ui.seqSlot)} limits={st.limits} />
      </div>
    {/if}

    <footer class="mono">
      <span>{st.sampleCount} of {st.limits.maxSamples} samples</span>
      <span>{st.sequenceCount} of {st.limits.maxSequences} sequences</span>
      <span>WAV files only</span>
      {#if st.missingCount}<span class="missing"
          >{st.missingCount} sample {st.missingCount === 1 ? "file" : "files"} missing: export is disabled</span
        >{/if}
      {#if st.dirty}<span class="dirty">Unsaved changes: export to keep them</span>{/if}
      {#if app.notice}<span class="notice" title={app.notice}>{app.notice}</span>{/if}
    </footer>
  {:else if !app.error}
    <p class="hint">Loading…</p>
  {/if}

  {#if ui.aboutOpen}
    <AboutModal onclose={() => (ui.aboutOpen = false)} />
  {/if}
  {#if ui.picker && st}
    <PadPicker picker={ui.picker} {pads} onclose={() => (ui.picker = null)} />
  {/if}
  {#if ui.confirm}
    <ConfirmModal c={ui.confirm} />
  {/if}
</div>

<style>
  /* Exactly the window's height: header, tabs, panels (which take the rest)
     and footer. Nothing scrolls at the page level. */
  .page {
    height: 100%;
    max-width: 1440px;
    margin: 0 auto;
    padding: 12px 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  /* The app's name is in the window title; the heading stays for screen readers. */
  .visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: 0;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .about-link {
    min-height: 40px;
    padding: 0 8px;
    background: transparent;
    color: var(--accent);
    border: 0;
    font: inherit;
    font-weight: 500;
    text-decoration: underline;
    text-underline-offset: 3px;
    cursor: pointer;
  }
  header {
    flex: none;
    display: flex;
    align-items: center;
    gap: 16px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--line);
  }
  .name {
    display: flex;
    align-items: center;
    gap: 10px;
    flex: 1 1 auto;
    min-width: 0;
    max-width: 380px;
  }
  .name label {
    white-space: nowrap;
  }
  .name input {
    height: 40px;
    min-width: 120px;
    padding: 0 12px;
    background: var(--surface);
    color: var(--text);
    border: 1px solid var(--edge);
    border-radius: 6px;
    font: inherit;
    font-weight: 600;
    width: 100%;
  }
  .actions {
    display: flex;
    gap: 8px;
    margin-left: auto;
  }
  .tabs {
    flex: none;
    display: flex;
    gap: 4px;
    border-bottom: 1px solid var(--line);
  }
  .tabs button {
    height: 40px;
    padding: 0 22px;
    background: transparent;
    color: var(--muted);
    border: 0;
    border-bottom: 3px solid transparent;
    font-family: var(--display);
    font-size: 16px;
    font-weight: 700;
    cursor: pointer;
  }
  .tabs button[aria-selected="true"] {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .grid-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
  }
  .summary {
    font-size: 12px;
    color: var(--muted);
  }
  .head-start {
    display: flex;
    align-items: center;
    gap: 16px;
  }
  /* "Play on click": as small as Clear bank, so the row is no taller. */
  .switch {
    margin: -4px 0;
    padding: 3px 0;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: none;
    border: 0;
    color: var(--soft);
    font: inherit;
    font-size: 12px;
    cursor: pointer;
  }
  .switch:hover {
    color: var(--text);
  }
  .track {
    position: relative;
    width: 26px;
    height: 14px;
    border-radius: 7px;
    background: var(--raised);
    border: 1px solid var(--edge);
  }
  .thumb {
    position: absolute;
    top: 1px;
    left: 1px;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--muted);
    transition: left 0.1s;
  }
  .switch[aria-checked="true"] .track {
    background: var(--accent);
    border-color: var(--accent);
  }
  .switch[aria-checked="true"] .thumb {
    left: 13px;
    background: var(--on-accent);
  }
  .head-end {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }
  /* Small, so the header row is no taller: the negative margin gives back
     what the padding and border add. */
  .head-btn {
    margin: -4px 0;
    padding: 3px 8px;
    background: var(--raised);
    color: var(--soft);
    border: 1px solid var(--edge);
    border-radius: 4px;
    font: inherit;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
  }
  .head-btn:hover:not(:disabled) {
    color: var(--text);
  }
  .head-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  footer {
    flex: none;
    display: flex;
    gap: 8px 24px;
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
    min-width: 0;
  }
  .dirty {
    color: var(--accent);
  }
  .missing {
    color: #ff8a8a;
  }
  .notice {
    margin-left: auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  /* Short window: give the grid the hint's lines. */
  @media (max-height: 600px) {
    .grid-hint {
      display: none;
    }
  }
  .error {
    flex: none;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 6px 6px 6px 12px;
    border: 1px solid #e5484d;
    border-radius: 6px;
    color: #ffb4b4;
    white-space: pre-line; /* missing-file lists come one per line */
  }
  .error span {
    flex: 1;
  }
  .dismiss {
    width: 32px;
    height: 32px;
    background: transparent;
    color: inherit;
    border: 0;
    font-size: 20px;
    cursor: pointer;
  }
</style>
