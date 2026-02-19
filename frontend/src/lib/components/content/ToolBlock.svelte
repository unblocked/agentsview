<script lang="ts">
  interface Props {
    content: string;
    label?: string;
  }

  let { content, label }: Props = $props();
  let collapsed: boolean = $state(true);

  let previewLine = $derived(
    content.split("\n")[0]?.slice(0, 80) ?? "",
  );
</script>

<div class="tool-block">
  <button
    class="tool-header"
    onclick={() => (collapsed = !collapsed)}
  >
    <span class="tool-chevron" class:open={!collapsed}>
      &#9656;
    </span>
    {#if label}
      <span class="tool-label">{label}</span>
    {/if}
    {#if collapsed && previewLine}
      <span class="tool-preview">{previewLine}</span>
    {/if}
  </button>
  {#if !collapsed}
    <pre class="tool-content">{content}</pre>
  {/if}
</div>

<style>
  .tool-block {
    border-left: 3px solid var(--accent-amber);
    background: var(--tool-bg);
    border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
    margin: 0;
  }

  .tool-header {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    width: 100%;
    text-align: left;
    font-size: 11px;
    color: var(--text-secondary);
    min-width: 0;
  }

  .tool-header:hover {
    color: var(--text-primary);
  }

  .tool-chevron {
    display: inline-block;
    font-size: 10px;
    transition: transform 0.15s;
    flex-shrink: 0;
  }

  .tool-chevron.open {
    transform: rotate(90deg);
  }

  .tool-label {
    font-weight: 600;
    font-size: 10px;
    color: var(--accent-amber);
    text-transform: uppercase;
    letter-spacing: 0.03em;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .tool-preview {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  .tool-content {
    padding: 4px 12px 8px;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-secondary);
    line-height: 1.5;
    overflow-x: auto;
  }
</style>
