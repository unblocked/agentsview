<script lang="ts">
  import type { Session } from "../../api/types.js";
  import { formatTokenCount, formatDuration } from "../../utils/format.js";
  import {
    calculateModelCost,
    formatCost,
    shortModelName,
  } from "../../utils/pricing.js";
  import { messages as messagesStore } from "../../stores/messages.svelte.js";

  interface Props {
    session: Session;
  }

  let { session }: Props = $props();

  let models = $derived.by(() => {
    const byModel = session.token_usage_by_model;
    if (!byModel) return [];
    return Object.entries(byModel)
      .filter(([modelId]) => !modelId.startsWith("<"))
      .sort(([, a], [, b]) => {
        const totalA =
          a.input_tokens +
          a.output_tokens +
          a.cache_creation_input_tokens +
          a.cache_read_input_tokens;
        const totalB =
          b.input_tokens +
          b.output_tokens +
          b.cache_creation_input_tokens +
          b.cache_read_input_tokens;
        return totalB - totalA;
      })
      .map(([modelId, usage]) => ({
        modelId,
        name: shortModelName(modelId),
        usage,
        cost: calculateModelCost(modelId, usage),
      }));
  });

  let totalCost = $derived(
    models.reduce((sum, m) => sum + (m.cost ?? 0), 0),
  );

  let hasTokens = $derived(
    session.input_tokens > 0 ||
      session.output_tokens > 0 ||
      session.cache_creation_input_tokens > 0 ||
      session.cache_read_input_tokens > 0,
  );

  let assistantDurationMs = $derived.by(() => {
    const msgs = messagesStore.messages;
    if (msgs.length < 2) return null;

    let totalMs = 0;
    let turnStartTs: number | null = null;
    let lastNonUserTs: number | null = null;

    for (const msg of msgs) {
      const ts = new Date(msg.timestamp).getTime();
      if (isNaN(ts)) continue;

      if (msg.role === "user") {
        if (turnStartTs !== null && lastNonUserTs !== null) {
          totalMs += lastNonUserTs - turnStartTs;
        }
        turnStartTs = ts;
        lastNonUserTs = null;
      } else {
        if (turnStartTs !== null) {
          lastNonUserTs = ts;
        }
      }
    }

    if (turnStartTs !== null && lastNonUserTs !== null) {
      totalMs += lastNonUserTs - turnStartTs;
    }

    return totalMs > 0 ? totalMs : null;
  });
</script>

{#if hasTokens}
  <div class="token-panel">
    {#if models.length > 0}
      <table class="token-table">
        <thead>
          <tr>
            <th class="col-model">Model</th>
            <th class="col-num">Input</th>
            <th class="col-num">Output</th>
            <th class="col-num">Cache Read</th>
            <th class="col-num">Cache Write</th>
            <th class="col-cost">Cost</th>
            <th class="col-num">Duration</th>
          </tr>
        </thead>
        <tbody>
          {#each models as m, i}
            <tr>
              <td class="col-model model-name">{m.name}</td>
              <td class="col-num"
                >{formatTokenCount(m.usage.input_tokens)}</td
              >
              <td class="col-num"
                >{formatTokenCount(m.usage.output_tokens)}</td
              >
              <td class="col-num"
                >{formatTokenCount(
                  m.usage.cache_read_input_tokens,
                )}</td
              >
              <td class="col-num"
                >{formatTokenCount(
                  m.usage.cache_creation_input_tokens,
                )}</td
              >
              <td class="col-cost"
                >{m.cost !== null ? formatCost(m.cost) : "—"}</td
              >
              <td class="col-num"
                >{#if i === 0 && models.length === 1 && assistantDurationMs}{formatDuration(assistantDurationMs)}{/if}</td
              >
            </tr>
          {/each}
        </tbody>
        {#if models.length > 1}
          <tfoot>
            <tr class="total-row">
              <td class="col-model">Total</td>
              <td class="col-num"
                >{formatTokenCount(session.input_tokens)}</td
              >
              <td class="col-num"
                >{formatTokenCount(session.output_tokens)}</td
              >
              <td class="col-num"
                >{formatTokenCount(
                  session.cache_read_input_tokens,
                )}</td
              >
              <td class="col-num"
                >{formatTokenCount(
                  session.cache_creation_input_tokens,
                )}</td
              >
              <td class="col-cost">{formatCost(totalCost)}</td>
              <td class="col-num">{assistantDurationMs ? formatDuration(assistantDurationMs) : ""}</td>
            </tr>
          </tfoot>
        {/if}
      </table>
    {:else}
      <div class="token-summary">
        <span>{formatTokenCount(session.input_tokens)} in</span>
        <span>{formatTokenCount(session.output_tokens)} out</span>
        <span
          >{formatTokenCount(session.cache_read_input_tokens)}
          cache read</span
        >
        <span
          >{formatTokenCount(
            session.cache_creation_input_tokens,
          )} cache write</span
        >
        {#if assistantDurationMs}
          <span>{formatDuration(assistantDurationMs)}</span>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .token-panel {
    padding: 6px 14px;
    border-bottom: 1px solid var(--border-muted);
    flex-shrink: 0;
  }

  .token-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 10px;
    font-variant-numeric: tabular-nums;
  }

  .token-table th {
    color: var(--text-muted);
    font-weight: 500;
    text-align: right;
    padding: 1px 8px 2px 0;
    letter-spacing: 0.02em;
  }

  .token-table td {
    padding: 1px 8px 1px 0;
    color: var(--text-secondary);
  }

  .col-model {
    text-align: left !important;
    width: 1%;
    white-space: nowrap;
  }

  .col-num {
    text-align: right;
  }

  .col-cost {
    text-align: right;
    font-weight: 550;
    color: var(--text-primary);
  }

  .model-name {
    font-family: "SF Mono", "Menlo", "Consolas", monospace;
    font-size: 9.5px;
    color: var(--text-muted);
  }

  .total-row td {
    border-top: 1px solid var(--border-muted);
    font-weight: 550;
    padding-top: 3px;
    color: var(--text-primary);
  }

  .total-row .col-cost {
    color: var(--text-primary);
  }

  .token-summary {
    display: flex;
    gap: 12px;
    font-size: 10px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
</style>
