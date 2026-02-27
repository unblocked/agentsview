<script lang="ts">
  import { onMount } from "svelte";

  interface MenuItem {
    label: string;
    onclick: () => void;
  }

  interface Props {
    x: number;
    y: number;
    items: MenuItem[];
    onclose: () => void;
  }

  let { x, y, items, onclose }: Props = $props();

  let menuRef: HTMLDivElement | undefined = $state(undefined);

  // Adjust position to keep menu in viewport.
  let adjustedX = $state(0);
  let adjustedY = $state(0);

  // Initialize from props.
  $effect(() => {
    adjustedX = x;
    adjustedY = y;
  });

  onMount(() => {
    if (!menuRef) return;
    const rect = menuRef.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    if (x + rect.width > vw) adjustedX = vw - rect.width - 4;
    if (y + rect.height > vh) adjustedY = vh - rect.height - 4;
  });

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      onclose();
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      onclose();
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="context-backdrop"
  onclick={handleBackdropClick}
  oncontextmenu={(e) => { e.preventDefault(); handleBackdropClick(e); }}
  onkeydown={handleKeydown}
>
  <div
    class="context-menu"
    bind:this={menuRef}
    style="left: {adjustedX}px; top: {adjustedY}px;"
  >
    {#each items as item}
      <button
        class="context-item"
        onclick={() => {
          item.onclick();
        }}
      >
        {item.label}
      </button>
    {/each}
  </div>
</div>

<style>
  .context-backdrop {
    position: fixed;
    inset: 0;
    z-index: 200;
  }

  .context-menu {
    position: fixed;
    min-width: 180px;
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-lg);
    padding: 4px;
    z-index: 201;
    animation: ctx-in 0.1s ease-out;
  }

  @keyframes ctx-in {
    from {
      opacity: 0;
      transform: scale(0.95);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  .context-item {
    display: block;
    width: 100%;
    padding: 6px 12px;
    font-size: 12px;
    color: var(--text-secondary);
    text-align: left;
    border-radius: 4px;
    transition: background 0.08s, color 0.08s;
  }

  .context-item:hover {
    background: var(--bg-surface-hover);
    color: var(--text-primary);
  }
</style>
