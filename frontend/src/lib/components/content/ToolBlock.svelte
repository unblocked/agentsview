<!-- ABOUTME: Renders a collapsible tool call block with metadata tags and content. -->
<!-- ABOUTME: Supports Task tool calls with inline subagent conversation expansion. -->
<script lang="ts">
  import type { ToolCall } from "../../api/types.js";
  import SubagentInline from "./SubagentInline.svelte";

  interface Props {
    content: string;
    label?: string;
    toolCall?: ToolCall;
  }

  let { content, label, toolCall }: Props = $props();
  let collapsed: boolean = $state(true);

  let previewLine = $derived(
    content.split("\n")[0]?.slice(0, 100) ?? "",
  );

  let isUnblocked = $derived(
    toolCall?.tool_name?.startsWith("mcp__unblocked__") ?? false,
  );

  /** Clean display name for the tool */
  let displayLabel = $derived.by(() => {
    if (!toolCall?.tool_name) return label;
    if (isUnblocked) {
      // "mcp__unblocked__context_engine" -> "context_engine"
      const parts = toolCall.tool_name.split("__");
      return parts.length >= 3 ? parts.slice(2).join("__") : toolCall.tool_name;
    }
    return label;
  });

  /** Parsed input parameters from structured tool call data */
  let inputParams = $derived.by(() => {
    if (!toolCall?.input_json) return null;
    try {
      return JSON.parse(toolCall.input_json);
    } catch {
      return null;
    }
  });

  /** For Task tool calls, extract key metadata fields */
  let taskMeta = $derived.by(() => {
    if (toolCall?.tool_name !== "Task" || !inputParams)
      return null;
    const meta: { label: string; value: string }[] = [];
    if (inputParams.subagent_type) {
      meta.push({
        label: "type",
        value: inputParams.subagent_type,
      });
    }
    if (inputParams.description) {
      meta.push({
        label: "description",
        value: inputParams.description,
      });
    }
    return meta.length ? meta : null;
  });

  /** For TaskCreate, show subject and description */
  let taskCreateMeta = $derived.by(() => {
    if (toolCall?.tool_name !== "TaskCreate" || !inputParams)
      return null;
    const meta: { label: string; value: string }[] = [];
    if (inputParams.subject) {
      meta.push({ label: "subject", value: inputParams.subject });
    }
    if (inputParams.description) {
      meta.push({ label: "description", value: inputParams.description });
    }
    return meta.length ? meta : null;
  });

  /** For TaskUpdate, show taskId and status */
  let taskUpdateMeta = $derived.by(() => {
    if (toolCall?.tool_name !== "TaskUpdate" || !inputParams)
      return null;
    const meta: { label: string; value: string }[] = [];
    if (inputParams.taskId) {
      meta.push({ label: "task", value: `#${inputParams.taskId}` });
    }
    if (inputParams.status) {
      meta.push({ label: "status", value: inputParams.status });
    }
    if (inputParams.subject) {
      meta.push({ label: "subject", value: inputParams.subject });
    }
    return meta.length ? meta : null;
  });

  /** Combined metadata for any tool type */
  let metaTags = $derived(
    taskMeta ?? taskCreateMeta ?? taskUpdateMeta ?? null,
  );

  let taskPrompt = $derived(
    toolCall?.tool_name === "Task"
      ? inputParams?.prompt ?? null
      : null,
  );

  let subagentSessionId = $derived(
    toolCall?.tool_name === "Task"
      ? toolCall?.subagent_session_id ?? null
      : null,
  );

  /** Formatted input_json for generic tools (non-Task-family) */
  let genericInputDisplay = $derived.by(() => {
    if (!inputParams) return null;
    if (metaTags || taskPrompt) return null; // handled by special renderers
    return JSON.stringify(inputParams, null, 2);
  });

  /** Preview text: use query param for unblocked, else first line of content */
  let preview = $derived.by(() => {
    if (isUnblocked && inputParams?.query) {
      return inputParams.query;
    }
    return previewLine;
  });
</script>

<div class="tool-block" class:unblocked={isUnblocked}>
  <button
    class="tool-header"
    onclick={() => {
      const sel = window.getSelection();
      if (sel && sel.toString().length > 0) return;
      collapsed = !collapsed;
    }}
  >
    <span class="tool-chevron" class:open={!collapsed}>
      &#9656;
    </span>
    {#if isUnblocked}
      <span class="unblocked-tag">UNBLOCKED</span>
    {/if}
    {#if displayLabel}
      <span class="tool-label">{displayLabel}</span>
    {/if}
    {#if collapsed && preview}
      <span class="tool-preview">{preview}</span>
    {/if}
    {#if toolCall?.result_content_length}
      <span class="tool-result-size">
        {toolCall.result_content_length > 1000
          ? `${(toolCall.result_content_length / 1000).toFixed(1)}k`
          : toolCall.result_content_length} chars
      </span>
    {/if}
  </button>
  {#if !collapsed}
    {#if metaTags}
      <div class="tool-meta">
        {#each metaTags as { label: metaLabel, value }}
          <span class="meta-tag">
            <span class="meta-label">{metaLabel}:</span>
            {value}
          </span>
        {/each}
      </div>
    {/if}
    {#if taskPrompt}
      <pre class="tool-content">{taskPrompt}</pre>
    {:else if content}
      <pre class="tool-content">{content}</pre>
    {:else if genericInputDisplay}
      <pre class="tool-content">{genericInputDisplay}</pre>
    {/if}
    {#if toolCall?.result_content}
      <div class="tool-result-section">
        <span class="result-label">Result</span>
      </div>
      <pre class="tool-content tool-result">{toolCall.result_content}</pre>
    {/if}
  {/if}
  {#if subagentSessionId}
    <SubagentInline sessionId={subagentSessionId} />
  {/if}
</div>

<style>
  .tool-block {
    border-left: 2px solid var(--accent-amber);
    background: var(--tool-bg);
    border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
    margin: 0;
  }

  .tool-block.unblocked {
    border-left: 3px solid var(--accent-purple);
    background: color-mix(in srgb, var(--accent-purple) 6%, var(--tool-bg, transparent));
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent-purple) 12%, transparent);
  }

  .tool-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 10px;
    width: 100%;
    text-align: left;
    font-size: 12px;
    color: var(--text-secondary);
    min-width: 0;
    border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
    transition: background 0.1s;
    user-select: text;
  }

  .tool-header:hover {
    background: var(--bg-surface-hover);
    color: var(--text-primary);
  }

  .unblocked .tool-header:hover {
    background: color-mix(in srgb, var(--accent-purple) 10%, transparent);
  }

  .tool-chevron {
    display: inline-block;
    font-size: 10px;
    transition: transform 0.15s;
    flex-shrink: 0;
    color: var(--text-muted);
  }

  .tool-chevron.open {
    transform: rotate(90deg);
  }

  .unblocked-tag {
    font-size: 8px;
    font-weight: 750;
    letter-spacing: 0.06em;
    color: white;
    background: var(--accent-purple);
    padding: 1px 5px;
    border-radius: 3px;
    flex-shrink: 0;
    line-height: 1.5;
  }

  .tool-label {
    font-family: var(--font-mono);
    font-weight: 500;
    font-size: 11px;
    color: var(--accent-amber);
    white-space: nowrap;
    flex-shrink: 0;
  }

  .unblocked .tool-label {
    color: var(--accent-purple);
    font-weight: 600;
  }

  .tool-preview {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  .tool-result-size {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-muted);
    white-space: nowrap;
    flex-shrink: 0;
    margin-left: auto;
    opacity: 0.6;
  }

  .tool-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    padding: 6px 14px;
    border-top: 1px solid var(--border-muted);
  }

  .meta-tag {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
    background: var(--bg-inset);
    padding: 2px 6px;
    border-radius: var(--radius-sm);
  }

  .meta-label {
    color: var(--text-secondary);
    font-weight: 500;
  }

  .tool-content {
    padding: 8px 14px 10px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.5;
    overflow-x: auto;
    border-top: 1px solid var(--border-muted);
  }

  .unblocked .tool-content {
    border-top-color: color-mix(in srgb, var(--accent-purple) 20%, transparent);
  }

  .tool-result-section {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 14px;
    border-top: 1px solid var(--border-muted);
  }

  .result-label {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--accent-green, #4ade80);
  }

  .tool-result {
    border-top: none;
    color: var(--text-muted);
    max-height: 400px;
    overflow-y: auto;
  }
</style>
