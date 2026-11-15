<script lang="ts">
  import Modal from "./Modal.svelte";
  import type { Confirm } from "./store.svelte";

  let { c }: { c: Confirm } = $props();

  let cancelButton: HTMLButtonElement | undefined = $state();
</script>

<!-- Escape cancels; a click outside does nothing. -->
<Modal
  titleId="confirm-title"
  descriptionId="confirm-message"
  role="alertdialog"
  onclose={() => c.resolve(false)}
  focus={cancelButton}
  width={520}
>
  <h2 id="confirm-title">{c.title}</h2>
  <p id="confirm-message">{c.message}</p>
  {#if c.items.length}
    <ul class="mono">
      {#each c.items as item (item)}
        <li>{item}</li>
      {/each}
    </ul>
  {/if}
  <div class="buttons">
    <button type="button" class="btn secondary" bind:this={cancelButton} onclick={() => c.resolve(false)}>Cancel</button
    >
    <button type="button" class="btn primary" onclick={() => c.resolve(true)}>{c.action}</button>
  </div>
</Modal>

<style>
  h2 {
    margin: 0;
    font-family: var(--display);
    font-size: 20px;
    font-weight: 700;
  }
  p {
    margin: 0;
    color: var(--soft);
  }
  ul {
    margin: 0;
    padding-left: 18px;
    font-size: 12px;
    color: var(--soft);
    overflow-wrap: anywhere;
    max-height: 160px;
    overflow-y: auto;
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
