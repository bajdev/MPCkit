// Go is the single source of truth: every service call returns a fresh State
// and the store replaces its copy. UI-only state (tab, bank, selection,
// dialogs) lives here too and never goes to Go (build plan 4.6).
import { Events } from "@wailsio/runtime";
import { EventNames, Service, type ExportResult, type State } from "./api";
import { stopAll } from "./audio";

export type Tab = "pads" | "sequences";

export const app = $state({
  state: null as State | null,
  error: "", // shown until dismissed or the next successful call
  notice: "", // short success message, e.g. where an export went
});

export type Confirm = {
  title: string;
  message: string;
  items: string[];
  action: string; // label of the confirming button
  resolve: (ok: boolean) => void;
};

/** Open "Move to pad" / "Copy to pad" picker. */
export type Picker = { kind: "move" | "copy"; from: number };

export const ui = $state({
  tab: "pads" as Tab,
  padBank: 0,
  padSlot: 0,
  seqBank: 0,
  seqSlot: 0,
  aboutOpen: false,
  confirm: null as Confirm | null,
  picker: null as Picker | null,
  trim: null as number | null, // slot whose start/end dialog is open
});

export function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

/**
 * Runs a service call that returns State. Go returns the state even when
 * the edit fails, but a rejected call loses it, so fetch it again then.
 */
export async function run(call: () => Promise<State>): Promise<boolean> {
  try {
    app.state = await call();
    app.error = "";
    return true;
  } catch (e) {
    app.error = message(e);
    try {
      app.state = await Service.State();
    } catch {
      // keep the old state; the error is already shown
    }
    return false;
  }
}

/** Fetches the current project from Go. */
export function refresh(): Promise<boolean> {
  return run(() => Service.State());
}

/** Asks before throwing away unsaved changes. True means go ahead. */
async function discardOK(): Promise<boolean> {
  if (!app.state?.dirty) return true;
  return confirm({
    title: "Discard unsaved changes?",
    message: "This project has changes that have not been exported. They will be lost.",
    items: [],
    action: "Discard",
  });
}

function resetSelection() {
  ui.padBank = ui.padSlot = ui.seqBank = ui.seqSlot = 0;
  ui.trim = null; // its pad belongs to the old project
}

/** Starts a new, empty project. */
export async function newProject(): Promise<void> {
  if (!(await discardOK())) return;
  stopAll();
  app.notice = "";
  if (await run(() => Service.New())) resetSelection();
}

/** Asks for a project file and opens it. */
export async function loadProject(): Promise<void> {
  if (!(await discardOK())) return;
  try {
    const res = await Service.Load();
    if (!res.path) return; // dialog cancelled
    stopAll();
    app.state = res.state;
    resetSelection();
    app.error = res.warnings?.length ? "Loaded with warnings:\n" + res.warnings.join("\n") : "";
    app.notice = "Loaded " + res.path;
  } catch (e) {
    app.error = message(e);
  }
}

/** Listens for state and errors Go sends on its own (e.g. after a file drop). */
export function listen(): () => void {
  const offState = Events.On(EventNames.State, (e) => {
    app.state = e.data;
    app.error = "";
  });
  const offError = Events.On(EventNames.Error, (e) => {
    app.error = e.data;
  });
  // The window's close button, with unsaved changes: Go waits for us.
  const offClose = Events.On(EventNames.Close, async () => {
    if (ui.confirm) return; // already asking something
    const ok = await confirm({
      title: "Close without exporting?",
      message: "This project has changes that have not been exported. They will be lost.",
      items: [],
      action: "Close",
    });
    if (ok) await Service.CloseWindow();
  });
  return () => {
    offState();
    offError();
    offClose();
  };
}

/** Shows the confirm dialog and resolves with the user's answer. */
export function confirm(c: Omit<Confirm, "resolve">): Promise<boolean> {
  return new Promise((resolve) => {
    ui.confirm = {
      ...c,
      resolve: (ok) => {
        ui.confirm = null;
        resolve(ok);
      },
    };
  });
}

// exportWith asks where to export, then writes there. Go remembers the
// chosen target; the frontend never passes a path back.
async function exportWith(
  choose: () => Promise<string>,
  write: (overwrite: boolean) => Promise<ExportResult>,
): Promise<void> {
  if (app.state?.missingCount) return; // the buttons are disabled too
  try {
    const target = await choose();
    if (!target) return; // dialog cancelled
    let res = await write(false);
    if (res.exists?.length) {
      const ok = await confirm({
        title: "Replace existing files?",
        message: "These already exist and will be replaced:",
        items: res.exists,
        action: "Replace",
      });
      if (!ok) {
        // Go forgets the target, so nothing can overwrite it without asking.
        await Service.CancelExport();
        return;
      }
      res = await write(true);
    }
    app.error = "";
    app.notice = "Exported to " + res.path;
    await refresh();
  } catch (e) {
    app.error = message(e);
  }
}

export function exportFolder(): Promise<void> {
  return exportWith(
    () => Service.ChooseExportFolder(),
    (overwrite) => Service.ExportFolder(overwrite),
  );
}

export function exportZip(): Promise<void> {
  return exportWith(
    () => Service.ChooseZipPath(),
    (overwrite) => Service.ExportZip(overwrite),
  );
}
