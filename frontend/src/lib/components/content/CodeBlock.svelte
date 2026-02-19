<script lang="ts">
  interface Props {
    content: string;
    language?: string;
  }

  let { content, language }: Props = $props();
</script>

<div class="code-block">
  {#if language}
    <div class="code-lang">{language}</div>
  {/if}
  <pre class="code-content"><code>{content}</code></pre>
</div>

<style>
  .code-block {
    background: var(--bg-inset);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-sm);
    margin: 4px 0;
    overflow: hidden;
  }

  .code-lang {
    padding: 2px 8px;
    font-size: 10px;
    font-weight: 500;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border-muted);
  }

  .code-content {
    padding: 8px;
    font-family: var(--font-mono);
    font-size: 11px;
    line-height: 1.5;
    color: var(--text-primary);
    overflow-x: auto;
  }

  .code-content code {
    font-family: inherit;
  }
</style>
