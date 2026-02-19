<script lang="ts">
  import { untrack } from "svelte";
  import { sessions } from "../../stores/sessions.svelte.js";
  import SessionItem from "./SessionItem.svelte";
  import { formatNumber } from "../../utils/format.js";

  const ITEM_HEIGHT = 40;
  const OVERSCAN = 10;
  const LOAD_AHEAD = 50;

  let containerRef: HTMLDivElement | undefined = $state(undefined);
  let scrollTop = $state(0);
  let viewportHeight = $state(0);

  let totalCount = $derived(
    Math.max(sessions.total, sessions.sessions.length),
  );

  let startIndex = $derived(
    Math.max(
      0,
      Math.floor(scrollTop / ITEM_HEIGHT) - OVERSCAN,
    ),
  );

  let endIndex = $derived.by(() => {
    if (totalCount === 0) return -1;
    const visibleCount = Math.ceil(
      viewportHeight / ITEM_HEIGHT,
    );
    const last = startIndex + visibleCount + OVERSCAN * 2;
    return Math.max(
      startIndex,
      Math.min(totalCount - 1, last),
    );
  });

  let virtualRows = $derived.by(() => {
    if (totalCount === 0 || endIndex < startIndex) return [];
    const rows = [];
    for (let i = startIndex; i <= endIndex; i++) {
      rows.push({
        index: i,
        key: i,
        size: ITEM_HEIGHT,
        start: i * ITEM_HEIGHT,
      });
    }
    return rows;
  });

  let totalSize = $derived(totalCount * ITEM_HEIGHT);

  $effect(() => {
    if (!containerRef) return;
    viewportHeight = containerRef.clientHeight;
    const ro = new ResizeObserver(() => {
      if (!containerRef) return;
      viewportHeight = containerRef.clientHeight;
    });
    ro.observe(containerRef);
    return () => ro.disconnect();
  });

  // Clamp stale scrollTop when count shrinks (e.g. project filter).
  $effect(() => {
    if (!containerRef) return;
    const maxTop = Math.max(
      0,
      totalSize - containerRef.clientHeight,
    );
    if (scrollTop > maxTop) {
      scrollTop = maxTop;
      containerRef.scrollTop = maxTop;
    }
  });

  // Keep fetching pages until the visible window is backed by
  // loaded sessions. This allows large scrollbar jumps to recover
  // without requiring repeated manual scroll events.
  $effect(() => {
    const loaded = sessions.sessions.length;
    if (loaded === 0) return;
    if (sessions.loading) return;
    if (!sessions.nextCursor) return;
    if (endIndex < loaded - LOAD_AHEAD) return;

    untrack(() => {
      void sessions.loadMore();
    });
  });

  // Load more when visible items approach loaded boundary.
  function handleScroll() {
    if (!containerRef) return;
    scrollTop = containerRef.scrollTop;

    const loaded = sessions.sessions.length;
    if (loaded === 0) return;
    if (sessions.loading) return;
    if (!sessions.nextCursor) return;
    if (endIndex < loaded - LOAD_AHEAD) return;

    // Keep this untracked; loading state changes should not
    // retrigger scroll-side effects.
    untrack(() => {
      void sessions.loadMore();
    });
  }
</script>

<div class="session-list-header">
  <span class="session-count">
    {formatNumber(sessions.total)} sessions
  </span>
  {#if sessions.loading}
    <span class="loading-indicator">loading</span>
  {/if}
</div>

<div
  class="session-list-scroll"
  bind:this={containerRef}
  onscroll={handleScroll}
>
  <div
    style="height: {totalSize}px; width: 100%; position: relative;"
  >
    {#each virtualRows as row (row.key)}
      {@const session = sessions.sessions[row.index]}
      <div
        style="position: absolute; top: 0; left: 0; width: 100%; height: {row.size}px; transform: translateY({row.start}px);"
      >
        {#if session}
          <SessionItem {session} />
        {:else}
          <div class="session-placeholder"></div>
        {/if}
      </div>
    {/each}
  </div>
</div>

<style>
  .session-list-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    font-size: 11px;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border-muted);
    flex-shrink: 0;
  }

  .session-count {
    font-weight: 500;
  }

  .loading-indicator {
    color: var(--accent-green);
  }

  .session-placeholder {
    height: 100%;
    background: var(--bg-secondary, #1e1e1e);
    opacity: 0.4;
    border-radius: 4px;
    margin: 2px 12px;
  }

  .session-list-scroll {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
  }
</style>
