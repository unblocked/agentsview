<script lang="ts">
  import { fade } from "svelte/transition";
  import { analytics } from "../../stores/analytics.svelte.js";

  function formatNum(n: number): string {
    if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
    if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
    return n.toLocaleString();
  }

  function pct(n: number): string {
    return `${(n * 100).toFixed(1)}%`;
  }

  interface Card {
    label: string;
    value: () => string;
    sub?: () => string;
  }

  const cards: Card[] = [
    {
      label: "Sessions",
      value: () =>
        formatNum(analytics.summary?.total_sessions ?? 0),
    },
    {
      label: "Messages",
      value: () =>
        formatNum(analytics.summary?.total_messages ?? 0),
    },
    {
      label: "Projects",
      value: () =>
        String(analytics.summary?.active_projects ?? 0),
    },
    {
      label: "Active Days",
      value: () =>
        String(analytics.summary?.active_days ?? 0),
    },
    {
      label: "Messages/Session",
      value: () => {
        const s = analytics.summary;
        if (!s) return "-";
        return `${s.avg_messages}`;
      },
      sub: () => {
        const s = analytics.summary;
        if (!s) return "";
        return `med ${s.median_messages} / p90 ${s.p90_messages}`;
      },
    },
    {
      label: "Concentration",
      value: () => pct(analytics.summary?.concentration ?? 0),
      sub: () => analytics.summary?.most_active_project ?? "",
    },
  ];
</script>

<div class="summary-cards">
  {#each cards as card}
    <div class="card" class:loading={analytics.loading.summary}>
      {#if analytics.loading.summary}
        <div class="skeleton-value"></div>
        <div class="skeleton-label"></div>
      {:else if analytics.errors.summary}
        <span class="card-value error">--</span>
        <span class="card-label">{card.label}</span>
      {:else}
        <span class="card-value" in:fade={{ duration: 150 }}>
          {card.value()}
        </span>
        <span class="card-label">{card.label}</span>
        {#if card.sub}
          {@const subtext = card.sub()}
          {#if subtext}
            <span class="card-sub">{subtext}</span>
          {/if}
        {/if}
      {/if}
    </div>
  {/each}
</div>

{#if analytics.errors.summary}
  <div class="error-bar">
    <span>{analytics.errors.summary}</span>
    <button
      class="retry-btn"
      onclick={() => analytics.fetchSummary()}
    >
      Retry
    </button>
  </div>
{/if}

<style>
  .summary-cards {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .card {
    flex: 1;
    min-width: 120px;
    padding: 12px;
    background: var(--bg-surface);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .card-value {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
    line-height: 1.2;
  }

  .card-value.error {
    color: var(--text-muted);
  }

  .card-label {
    font-size: 11px;
    color: var(--text-muted);
    font-weight: 500;
  }

  .card-sub {
    font-size: 10px;
    color: var(--text-muted);
    margin-top: 2px;
  }

  .skeleton-value {
    width: 60px;
    height: 20px;
    background: var(--bg-inset);
    border-radius: var(--radius-sm);
    animation: pulse 1.5s ease-in-out infinite;
  }

  .skeleton-label {
    width: 80px;
    height: 12px;
    background: var(--bg-inset);
    border-radius: var(--radius-sm);
    animation: pulse 1.5s ease-in-out infinite;
    animation-delay: 0.1s;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }

  .error-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-surface);
    border: 1px solid var(--accent-red);
    border-radius: var(--radius-sm);
    font-size: 11px;
    color: var(--accent-red);
  }

  .retry-btn {
    padding: 2px 8px;
    border: 1px solid var(--accent-red);
    border-radius: var(--radius-sm);
    font-size: 11px;
    color: var(--accent-red);
    cursor: pointer;
  }

  .retry-btn:hover {
    background: var(--accent-red);
    color: #fff;
  }
</style>
