<script lang="ts">
  import type { Snippet } from "svelte";

  // The backdrop, box, Escape handling and focus-on-open shared by every
  // dialog. The content and its own styles stay with each dialog.
  let {
    titleId,
    descriptionId,
    role = "dialog",
    onclose,
    closeOnBackdrop = false,
    focus,
    width = 440,
    gap = 12,
    children,
  }: {
    titleId: string; // id of the heading that names the dialog
    descriptionId?: string; // id of the text that describes it, if any
    role?: "dialog" | "alertdialog";
    onclose: () => void; // Escape, and a click outside if closeOnBackdrop
    closeOnBackdrop?: boolean;
    focus?: HTMLElement; // focused on open; else the first focusable element
    width?: number; // the box's largest width, in pixels
    gap?: number; // space between the box's children, in pixels
    children: Snippet;
  } = $props();

  let box: HTMLDivElement | undefined = $state();

  $effect(() => {
    const target = focus ?? box?.querySelector<HTMLElement>("button, [href], input, select, textarea");
    target?.focus();
  });

  function keydown(e: KeyboardEvent) {
    if (e.key === "Escape") onclose();
  }
</script>

<svelte:window onkeydown={keydown} />

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions (a click outside closes only dialogs that allow it; Escape does the same from the keyboard) -->
<div class="backdrop" onclick={(e) => closeOnBackdrop && e.target === e.currentTarget && onclose()}>
  <div
    class="dialog"
    bind:this={box}
    {role}
    aria-modal="true"
    aria-labelledby={titleId}
    aria-describedby={descriptionId}
    style:max-width="{width}px"
    style:gap="{gap}px"
  >
    {@render children()}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    padding: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(10, 11, 13, 0.78);
  }
  .dialog {
    width: 100%;
    padding: 24px;
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border: 1px solid var(--edge);
    border-radius: 12px;
  }
</style>
