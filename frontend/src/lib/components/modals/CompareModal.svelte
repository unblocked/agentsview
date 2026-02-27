<script lang="ts">
  import { ui } from "../../stores/ui.svelte.js";
  import type { SessionGroup } from "../../stores/sessions.svelte.js";
  import type { ModelTokenUsage } from "../../api/types.js";
  import {
    formatTokenCount,
    formatDuration,
    truncate,
  } from "../../utils/format.js";
  import {
    calculateModelCost,
    formatCost,
    shortModelName,
  } from "../../utils/pricing.js";
  import { generateComparison } from "../../api/client.js";
  import { renderMarkdown } from "../../utils/markdown.js";

  let groupA = $derived(ui.compareGroupA);
  let groupB = $derived(ui.compareGroupB);

  interface GroupMetrics {
    sessions: number;
    totalMessages: number;
    userMessages: number;
    inputTokens: number;
    outputTokens: number;
    cacheWrite: number;
    cacheRead: number;
    cost: number;
    durationMs: number | null;
    autonomy: number;
    models: string[];
  }

  function computeMetrics(
    group: SessionGroup | null,
  ): GroupMetrics | null {
    if (!group) return null;

    let inputTokens = 0;
    let outputTokens = 0;
    let cacheWrite = 0;
    let cacheRead = 0;
    let totalMessages = 0;
    let userMessages = 0;
    let cost = 0;
    const modelSet = new Set<string>();
    let earliestStart: number | null = null;
    let latestEnd: number | null = null;

    for (const s of group.sessions) {
      inputTokens += s.input_tokens;
      outputTokens += s.output_tokens;
      cacheWrite += s.cache_creation_input_tokens;
      cacheRead += s.cache_read_input_tokens;
      totalMessages += s.message_count;
      userMessages += s.user_message_count;

      if (s.started_at) {
        const ts = new Date(s.started_at).getTime();
        if (earliestStart === null || ts < earliestStart) {
          earliestStart = ts;
        }
      }
      if (s.ended_at) {
        const ts = new Date(s.ended_at).getTime();
        if (latestEnd === null || ts > latestEnd) {
          latestEnd = ts;
        }
      }

      if (s.token_usage_by_model) {
        for (const [modelId, usage] of Object.entries(
          s.token_usage_by_model,
        )) {
          if (modelId.startsWith("<")) continue;
          modelSet.add(shortModelName(modelId));
          const c = calculateModelCost(modelId, usage);
          if (c !== null) cost += c;
        }
      }
    }

    const durationMs =
      earliestStart !== null && latestEnd !== null
        ? latestEnd - earliestStart
        : null;

    const autonomy =
      totalMessages > 0
        ? 1 - userMessages / totalMessages
        : 0;

    return {
      sessions: group.sessions.length,
      totalMessages,
      userMessages,
      inputTokens,
      outputTokens,
      cacheWrite,
      cacheRead,
      cost,
      durationMs,
      autonomy,
      models: [...modelSet],
    };
  }

  let metricsA = $derived(computeMetrics(groupA));
  let metricsB = $derived(computeMetrics(groupB));

  let aiContent: string = $state("");
  let aiLoading: boolean = $state(false);
  let aiPhase: string = $state("");
  let aiError: string = $state("");

  let abortFn: (() => void) | null = null;

  function groupLabel(group: SessionGroup | null): string {
    if (!group) return "";
    return truncate(
      group.firstMessage ?? group.project,
      40,
    );
  }

  function formatPct(a: number, b: number): string {
    if (a === 0 && b === 0) return "0%";
    if (a === 0) return "+100%";
    const pct = ((b - a) / a) * 100;
    const sign = pct > 0 ? "+" : "";
    return `${sign}${pct.toFixed(0)}%`;
  }

  function formatPp(a: number, b: number): string {
    const diff = (b - a) * 100;
    const sign = diff > 0 ? "+" : "";
    return `${sign}${diff.toFixed(0)}pp`;
  }

  function deltaClass(
    a: number,
    b: number,
    lowerBetter: boolean,
  ): string {
    if (a === b) return "";
    if (lowerBetter) return b < a ? "better" : "worse";
    return b > a ? "better" : "worse";
  }

  async function generateAI() {
    if (!groupA || !groupB) return;
    aiLoading = true;
    aiError = "";
    aiContent = "";
    aiPhase = "starting";

    const handle = generateComparison(
      groupA.sessions.map((s) => s.id),
      groupB.sessions.map((s) => s.id),
      (phase) => {
        aiPhase = phase;
      },
    );
    abortFn = handle.abort;

    try {
      aiContent = await handle.done;
    } catch (e: any) {
      if (e.name !== "AbortError") {
        aiError = e.message || "Generation failed";
      }
    } finally {
      aiLoading = false;
      abortFn = null;
    }
  }

  function close() {
    if (abortFn) abortFn();
    ui.activeModal = null;
    ui.clearCompare();
  }

  function handleOverlayClick(e: MouseEvent) {
    if (
      (e.target as HTMLElement).classList.contains(
        "compare-overlay",
      )
    ) {
      close();
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="compare-overlay"
  onclick={handleOverlayClick}
  onkeydown={(e) => {
    if (e.key === "Escape") close();
  }}
>
  <div class="compare-modal">
    <div class="compare-header">
      <h3 class="compare-title">Compare Sessions</h3>
      <button class="close-btn" onclick={close}>&times;</button>
    </div>

    <div class="compare-body">
      {#if metricsA && metricsB && groupA && groupB}
        <div class="group-labels">
          <div class="group-label label-a">
            <span class="group-tag">A</span>
            {groupLabel(groupA)}
          </div>
          <div class="group-label label-b">
            <span class="group-tag">B</span>
            {groupLabel(groupB)}
          </div>
        </div>

        <table class="metrics-table">
          <thead>
            <tr>
              <th class="col-metric">Metric</th>
              <th class="col-val">Group A</th>
              <th class="col-val">Group B</th>
              <th class="col-delta">Delta</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td class="col-metric">Sessions</td>
              <td class="col-val">{metricsA.sessions}</td>
              <td class="col-val">{metricsB.sessions}</td>
              <td class="col-delta">{metricsB.sessions - metricsA.sessions > 0 ? "+" : ""}{metricsB.sessions - metricsA.sessions}</td>
            </tr>
            <tr>
              <td class="col-metric">Turns (user msgs)</td>
              <td class="col-val">{metricsA.userMessages}</td>
              <td class="col-val">{metricsB.userMessages}</td>
              <td class="col-delta {deltaClass(metricsA.userMessages, metricsB.userMessages, true)}">
                {formatPct(metricsA.userMessages, metricsB.userMessages)}
              </td>
            </tr>
            <tr>
              <td class="col-metric">Total messages</td>
              <td class="col-val">{metricsA.totalMessages}</td>
              <td class="col-val">{metricsB.totalMessages}</td>
              <td class="col-delta">{formatPct(metricsA.totalMessages, metricsB.totalMessages)}</td>
            </tr>
            <tr>
              <td class="col-metric">Input tokens</td>
              <td class="col-val">{formatTokenCount(metricsA.inputTokens)}</td>
              <td class="col-val">{formatTokenCount(metricsB.inputTokens)}</td>
              <td class="col-delta {deltaClass(metricsA.inputTokens, metricsB.inputTokens, true)}">
                {formatPct(metricsA.inputTokens, metricsB.inputTokens)}
              </td>
            </tr>
            <tr>
              <td class="col-metric">Output tokens</td>
              <td class="col-val">{formatTokenCount(metricsA.outputTokens)}</td>
              <td class="col-val">{formatTokenCount(metricsB.outputTokens)}</td>
              <td class="col-delta {deltaClass(metricsA.outputTokens, metricsB.outputTokens, true)}">
                {formatPct(metricsA.outputTokens, metricsB.outputTokens)}
              </td>
            </tr>
            <tr>
              <td class="col-metric">Cost</td>
              <td class="col-val cost">{formatCost(metricsA.cost)}</td>
              <td class="col-val cost">{formatCost(metricsB.cost)}</td>
              <td class="col-delta {deltaClass(metricsA.cost, metricsB.cost, true)}">
                {formatPct(metricsA.cost, metricsB.cost)}
              </td>
            </tr>
            <tr>
              <td class="col-metric">Duration</td>
              <td class="col-val">{metricsA.durationMs ? formatDuration(metricsA.durationMs) : "--"}</td>
              <td class="col-val">{metricsB.durationMs ? formatDuration(metricsB.durationMs) : "--"}</td>
              <td class="col-delta {metricsA.durationMs && metricsB.durationMs ? deltaClass(metricsA.durationMs, metricsB.durationMs, true) : ''}">
                {metricsA.durationMs && metricsB.durationMs ? formatPct(metricsA.durationMs, metricsB.durationMs) : "--"}
              </td>
            </tr>
            <tr>
              <td class="col-metric">Autonomy</td>
              <td class="col-val">{(metricsA.autonomy * 100).toFixed(0)}%</td>
              <td class="col-val">{(metricsB.autonomy * 100).toFixed(0)}%</td>
              <td class="col-delta {deltaClass(metricsA.autonomy, metricsB.autonomy, false)}">
                {formatPp(metricsA.autonomy, metricsB.autonomy)}
              </td>
            </tr>
            <tr>
              <td class="col-metric">Models</td>
              <td class="col-val models">{metricsA.models.join(", ") || "--"}</td>
              <td class="col-val models">{metricsB.models.join(", ") || "--"}</td>
              <td class="col-delta"></td>
            </tr>
          </tbody>
        </table>

        <div class="ai-section">
          <div class="ai-header">
            <h4 class="ai-title">AI Comparison</h4>
            {#if !aiLoading && !aiContent}
              <button class="generate-btn" onclick={generateAI}>
                Generate AI Comparison
              </button>
            {/if}
            {#if aiLoading}
              <span class="ai-phase">{aiPhase}...</span>
            {/if}
          </div>

          {#if aiError}
            <div class="ai-error">{aiError}</div>
          {/if}

          {#if aiContent}
            <div class="ai-content markdown-body">
              {@html renderMarkdown(aiContent)}
            </div>
          {/if}
        </div>
      {:else}
        <p class="empty-state">No groups selected for comparison.</p>
      {/if}
    </div>
  </div>
</div>

<style>
  .compare-overlay {
    position: fixed;
    inset: 0;
    background: var(--overlay-bg);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .compare-modal {
    width: 720px;
    max-width: 90vw;
    max-height: 85vh;
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .compare-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-default);
    flex-shrink: 0;
  }

  .compare-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .close-btn {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 16px;
    color: var(--text-muted);
    border-radius: var(--radius-sm);
  }

  .close-btn:hover {
    background: var(--bg-surface-hover);
    color: var(--text-primary);
  }

  .compare-body {
    overflow-y: auto;
    padding: 16px;
  }

  .group-labels {
    display: flex;
    gap: 12px;
    margin-bottom: 12px;
  }

  .group-label {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text-secondary);
    padding: 6px 10px;
    border-radius: var(--radius-sm);
    background: var(--bg-inset);
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .group-tag {
    font-size: 10px;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 4px;
    color: white;
    flex-shrink: 0;
  }

  .label-a .group-tag {
    background: var(--accent-blue);
  }

  .label-b .group-tag {
    background: var(--accent-green);
  }

  .metrics-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 11px;
    font-variant-numeric: tabular-nums;
    margin-bottom: 16px;
  }

  .metrics-table th {
    color: var(--text-muted);
    font-weight: 500;
    text-align: right;
    padding: 4px 10px;
    border-bottom: 1px solid var(--border-default);
    letter-spacing: 0.02em;
    font-size: 10px;
    text-transform: uppercase;
  }

  .metrics-table td {
    padding: 5px 10px;
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border-muted);
  }

  .col-metric {
    text-align: left !important;
    font-weight: 500;
    color: var(--text-primary) !important;
    white-space: nowrap;
  }

  .col-val {
    text-align: right;
  }

  .col-val.cost {
    font-weight: 550;
  }

  .col-val.models {
    font-size: 10px;
    font-family: "SF Mono", "Menlo", "Consolas", monospace;
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .col-delta {
    text-align: right;
    font-size: 10px;
    color: var(--text-muted);
  }

  .col-delta.better {
    color: var(--accent-green);
    font-weight: 550;
  }

  .col-delta.worse {
    color: var(--accent-rose);
    font-weight: 550;
  }

  .ai-section {
    border-top: 1px solid var(--border-default);
    padding-top: 12px;
  }

  .ai-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;
  }

  .ai-title {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .generate-btn {
    padding: 5px 12px;
    font-size: 11px;
    font-weight: 500;
    color: white;
    background: var(--accent-blue);
    border-radius: var(--radius-sm);
    transition: opacity 0.1s;
  }

  .generate-btn:hover {
    opacity: 0.9;
  }

  .ai-phase {
    font-size: 11px;
    color: var(--accent-green);
    animation: pulse 1.5s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }

  .ai-error {
    font-size: 11px;
    color: var(--accent-rose);
    padding: 8px;
    background: color-mix(in srgb, var(--accent-rose) 8%, transparent);
    border-radius: var(--radius-sm);
    margin-bottom: 8px;
  }

  .ai-content {
    font-size: 12px;
    line-height: 1.6;
    color: var(--text-secondary);
  }

  .ai-content :global(h1),
  .ai-content :global(h2),
  .ai-content :global(h3) {
    color: var(--text-primary);
    margin: 12px 0 6px;
  }

  .ai-content :global(h1) { font-size: 16px; }
  .ai-content :global(h2) { font-size: 14px; }
  .ai-content :global(h3) { font-size: 13px; }

  .ai-content :global(p) {
    margin: 6px 0;
  }

  .ai-content :global(ul),
  .ai-content :global(ol) {
    padding-left: 20px;
    margin: 6px 0;
  }

  .ai-content :global(li) {
    margin: 3px 0;
  }

  .ai-content :global(strong) {
    color: var(--text-primary);
    font-weight: 600;
  }

  .ai-content :global(code) {
    font-family: "SF Mono", "Menlo", "Consolas", monospace;
    font-size: 11px;
    padding: 1px 4px;
    background: var(--bg-inset);
    border-radius: 3px;
  }

  .empty-state {
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
    padding: 24px;
  }
</style>
