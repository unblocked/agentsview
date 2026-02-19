<script lang="ts">
  import { untrack } from "svelte";
  import { messages } from "../../stores/messages.svelte.js";
  import { ui } from "../../stores/ui.svelte.js";

  interface Props {
    scrollOffset: number;
    scrollHeight: number;
    clientHeight: number;
    onClickIndex: (index: number) => void;
  }

  let {
    scrollOffset,
    scrollHeight,
    clientHeight,
    onClickIndex,
  }: Props = $props();

  let canvasEl: HTMLCanvasElement | undefined = $state(undefined);
  let containerEl: HTMLDivElement | undefined = $state(undefined);
  let canvasHeight: number = $state(0);
  const canvasWidth = 60;
  let staticCanvas: HTMLCanvasElement | null = null;

  // Colors derived from theme
  let colors = $derived({
    user: ui.theme === "dark" ? "#60a5fa" : "#2563eb",
    assistant: ui.theme === "dark" ? "#a78bfa" : "#7c3aed",
    thinking: "#c026d3",
    tool: ui.theme === "dark" ? "#fbbf24" : "#d97706",
    viewport:
      ui.theme === "dark"
        ? "rgba(255,255,255,0.08)"
        : "rgba(0,0,0,0.08)",
    viewportBorder:
      ui.theme === "dark"
        ? "rgba(255,255,255,0.15)"
        : "rgba(0,0,0,0.12)",
    bg: ui.theme === "dark" ? "#0e0e0e" : "#f0ede8",
  });

  // Observe container resize
  $effect(() => {
    if (!containerEl) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        canvasHeight = entry.contentRect.height;
      }
    });
    ro.observe(containerEl);
    return () => ro.disconnect();
  });

  function ensureCanvasSize(
    canvas: HTMLCanvasElement,
    width: number,
    height: number,
    dpr: number,
  ) {
    const targetWidth = Math.max(1, Math.round(width * dpr));
    const targetHeight = Math.max(1, Math.round(height * dpr));
    if (
      canvas.width !== targetWidth ||
      canvas.height !== targetHeight
    ) {
      canvas.width = targetWidth;
      canvas.height = targetHeight;
    }

    const widthCss = `${width}px`;
    const heightCss = `${height}px`;
    if (canvas.style.width !== widthCss) {
      canvas.style.width = widthCss;
    }
    if (canvas.style.height !== heightCss) {
      canvas.style.height = heightCss;
    }
  }

  function drawOverlay() {
    if (!canvasEl || canvasHeight === 0 || !staticCanvas) return;
    const _ = colors; // Subscribe to theme changes
    const dpr = window.devicePixelRatio || 1;

    ensureCanvasSize(
      canvasEl,
      canvasWidth,
      canvasHeight,
      dpr,
    );

    const ctx = canvasEl.getContext("2d");
    if (!ctx) return;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    ctx.drawImage(staticCanvas, 0, 0, canvasWidth, canvasHeight);

    if (scrollHeight <= 0 || clientHeight <= 0) return;
    const vpHeight = Math.max(
      8,
      (clientHeight / scrollHeight) * canvasHeight,
    );
    const maxScroll = Math.max(1, scrollHeight - clientHeight);
    const vpTopRange = Math.max(0, canvasHeight - vpHeight);
    const vpTop = Math.min(
      vpTopRange,
      Math.max(0, (scrollOffset / maxScroll) * vpTopRange),
    );

    ctx.fillStyle = colors.viewport;
    ctx.fillRect(0, vpTop, canvasWidth, vpHeight);

    ctx.strokeStyle = colors.viewportBorder;
    ctx.lineWidth = 1;
    ctx.strokeRect(0.5, vpTop + 0.5, canvasWidth - 1, vpHeight - 1);
  }

  // Render static minimap snapshot.
  $effect(() => {
    if (canvasHeight === 0) return;
    const entries = messages.minimap;
    const newestFirst = ui.sortNewestFirst;
    const _ = colors; // Subscribe to theme changes
    if (typeof document === "undefined") return;
    if (!staticCanvas) {
      staticCanvas = document.createElement("canvas");
    }
    if (!staticCanvas) return;
    const dpr = window.devicePixelRatio || 1;

    ensureCanvasSize(
      staticCanvas,
      canvasWidth,
      canvasHeight,
      dpr,
    );
    const ctx = staticCanvas.getContext("2d");
    if (!ctx) return;

    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);

    // Background
    ctx.fillStyle = colors.bg;
    ctx.fillRect(0, 0, canvasWidth, canvasHeight);

    if (entries.length === 0) {
      untrack(() => drawOverlay());
      return;
    }

    const barX = 4;
    const barWidth = 44;
    const dotSize = 6;
    const dotHeight = 3;

    // Draw at most a small multiple of screen-space bars.
    const drawBars = Math.min(
      entries.length,
      Math.max(1, Math.floor(canvasHeight * 4)),
    );

    for (let i = 0; i < drawBars; i++) {
      const visualSampleIdx = Math.min(
        entries.length - 1,
        Math.floor(((i + 0.5) / drawBars) * entries.length),
      );
      const sampleIdx = newestFirst
        ? entries.length - 1 - visualSampleIdx
        : visualSampleIdx;
      const entry = entries[sampleIdx]!;
      const y = (i / drawBars) * canvasHeight;
      const nextY = ((i + 1) / drawBars) * canvasHeight;
      const barHeight = nextY - y;

      ctx.fillStyle =
        entry.role === "user" ? colors.user : colors.assistant;
      ctx.fillRect(barX, y, barWidth, barHeight);

      // Thinking dot
      if (entry.has_thinking) {
        ctx.fillStyle = colors.thinking;
        ctx.fillRect(
          barX + barWidth + 2,
          y,
          dotSize,
          dotHeight,
        );
      }

      // Tool use dot
      if (entry.has_tool_use) {
        ctx.fillStyle = colors.tool;
        ctx.fillRect(
          barX + barWidth + 2,
          y + (entry.has_thinking ? dotHeight + 1 : 0),
          dotSize,
          dotHeight,
        );
      }

    }
    untrack(() => drawOverlay());
  });

  // Update viewport overlay on scroll metric changes.
  $effect(() => {
    drawOverlay();
  });

  function handleClick(e: MouseEvent) {
    if (!containerEl || messages.minimap.length === 0) return;
    const rect = containerEl.getBoundingClientRect();
    const y = e.clientY - rect.top;
    const clampedY = Math.max(0, Math.min(y, canvasHeight));
    const ratio = canvasHeight > 0 ? clampedY / canvasHeight : 0;
    const newestFirst = ui.sortNewestFirst;
    const visualIdx = Math.max(
      0,
      Math.min(
        Math.floor(ratio * messages.minimap.length),
        messages.minimap.length - 1,
      ),
    );
    const idx = newestFirst
      ? messages.minimap.length - 1 - visualIdx
      : visualIdx;
    const entry = messages.minimap[idx];
    if (entry) {
      onClickIndex(entry.ordinal);
    }
  }
</script>

<div
  class="minimap-container"
  bind:this={containerEl}
  onclick={handleClick}
  role="presentation"
>
  <canvas bind:this={canvasEl}></canvas>
</div>

<style>
  .minimap-container {
    width: 100%;
    height: 100%;
    cursor: pointer;
    overflow: hidden;
  }

  canvas {
    display: block;
  }
</style>
