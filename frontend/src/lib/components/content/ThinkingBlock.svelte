<script lang="ts">
  interface Props {
    content: string;
  }

  let { content }: Props = $props();
  let collapsed: boolean = $state(true);
</script>

<div class="thinking-block">
  <button
    class="thinking-header"
    onclick={() => (collapsed = !collapsed)}
  >
    <span class="thinking-chevron" class:open={!collapsed}>
      &#9656;
    </span>
    <span class="thinking-label">THINKING</span>
  </button>
  {#if !collapsed}
    <div class="thinking-content">{content}</div>
  {/if}
</div>

<style>
  .thinking-block {
    border-left: 3px solid var(--accent-purple);
    background: var(--thinking-bg);
    border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
    margin: 0;
  }

  .thinking-header {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    width: 100%;
    text-align: left;
    font-size: 10px;
    font-weight: 600;
    color: var(--text-muted);
    letter-spacing: 0.05em;
  }

  .thinking-header:hover {
    color: var(--text-secondary);
  }

  .thinking-chevron {
    display: inline-block;
    font-size: 10px;
    transition: transform 0.15s;
  }

  .thinking-chevron.open {
    transform: rotate(90deg);
  }

  .thinking-content {
    padding: 4px 12px 8px;
    font-size: 12px;
    font-style: italic;
    color: var(--text-secondary);
    white-space: pre-wrap;
    word-wrap: break-word;
    line-height: 1.6;
  }
</style>
