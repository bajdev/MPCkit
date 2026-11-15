<script lang="ts">
  import { Browser } from "@wailsio/runtime";
  import { Service, type About } from "./api";
  import Modal from "./Modal.svelte";

  let { onclose }: { onclose: () => void } = $props();

  let about = $state<About | null>(null);
  let closeButton: HTMLButtonElement | undefined = $state();

  $effect(() => {
    Service.About().then((a) => (about = a));
  });

  function open(e: MouseEvent) {
    // Open in the system browser, not inside the app window.
    e.preventDefault();
    if (about) Browser.OpenURL(about.repo);
  }
</script>

<Modal titleId="about-title" {onclose} closeOnBackdrop focus={closeButton} gap={14}>
  <h2 id="about-title">About MPCkit</h2>
  <p class="lead">
    A desktop project and kit editor for the MPC Sample. Build kits on your computer, then export them to the device.
  </p>
  <div class="facts">
    <span>Version {about?.version ?? ""}</span>
    {#if about?.commit}<span>Build {about.commit}</span>{/if}
    <span>{about?.license ?? ""}</span>
    {#if about}
      <a href={about.repo} onclick={open}>{about.repo.replace("https://", "")}</a>
    {/if}
  </div>
  <p class="hint">An independent project, not affiliated with Akai Professional.</p>
  <button type="button" class="btn primary close" bind:this={closeButton} onclick={onclose}>Close</button>
</Modal>

<style>
  h2 {
    margin: 0;
    font-family: var(--display);
    font-size: 22px;
    font-weight: 700;
  }
  .lead {
    margin: 0;
    color: var(--soft);
  }
  .facts {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-family: var(--mono);
    font-size: 12px;
    color: var(--soft);
  }
  .close {
    align-self: flex-end;
    padding: 0 20px;
  }
</style>
