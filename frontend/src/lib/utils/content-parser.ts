import type { Message } from "../api/types.js";

export type SegmentType = "text" | "thinking" | "tool" | "code";

export interface ContentSegment {
  type: SegmentType;
  content: string;
  /** Tool name or code language, when applicable */
  label?: string;
}

/**
 * Regex patterns matching Go backend at
 * internal/server/export.go:403-412
 */
const THINKING_RE =
  /\[Thinking\]\n?([\s\S]*?)(?=\n\[|\n\n\[|$)/g;

const TOOL_NAMES =
  "Tool|Read|Write|Edit|Bash|Glob|Grep|Task|" +
  "Question|Todo List|Entering Plan Mode|" +
  "Exiting Plan Mode";

const TOOL_RE = new RegExp(
  `\\[(${TOOL_NAMES})([^\\]]*)\\]([\\s\\S]*?)(?=\\n\\[|\\n\\n|$)`,
  "g",
);

const CODE_BLOCK_RE = /```(\w*)\n([\s\S]*?)```/g;

interface Match {
  start: number;
  end: number;
  segment: ContentSegment;
}

const MAX_TOOL_ONLY_CACHE = 12000;
const MAX_SEGMENT_CACHE = 8000;
const toolOnlyCache = new Map<string, boolean>();
const segmentCache = new Map<string, ContentSegment[]>();

function cachePut<K, V>(cache: Map<K, V>, key: K, value: V, max: number) {
  if (cache.has(key)) return;
  if (cache.size >= max) {
    const first = cache.keys().next();
    if (!first.done) {
      cache.delete(first.value);
    }
  }
  cache.set(key, value);
}

/** Returns true if the message contains only tool calls (no text) */
export function isToolOnly(msg: Message): boolean {
  const key = `${msg.role}|${msg.has_tool_use ? 1 : 0}|${msg.content}`;
  const cached = toolOnlyCache.get(key);
  if (cached !== undefined) {
    return cached;
  }

  if (msg.role !== "assistant") return false;
  if (!msg.has_tool_use) {
    cachePut(toolOnlyCache, key, false, MAX_TOOL_ONLY_CACHE);
    return false;
  }
  const stripped = msg.content
    .replace(THINKING_RE, "")
    .replace(TOOL_RE, "")
    .trim();
  // Reset lastIndex since regexes have the global flag
  THINKING_RE.lastIndex = 0;
  TOOL_RE.lastIndex = 0;
  const result = stripped.length === 0;
  cachePut(toolOnlyCache, key, result, MAX_TOOL_ONLY_CACHE);
  return result;
}

/** Parse message content into typed segments */
export function parseContent(text: string): ContentSegment[] {
  if (!text) return [];
  const cached = segmentCache.get(text);
  if (cached) return cached;

  const matches: Match[] = [];

  // Collect thinking blocks
  for (const m of text.matchAll(THINKING_RE)) {
    matches.push({
      start: m.index!,
      end: m.index! + m[0].length,
      segment: {
        type: "thinking",
        content: (m[1] ?? "").trim(),
      },
    });
  }

  // Collect tool blocks
  for (const m of text.matchAll(TOOL_RE)) {
    const toolName = m[1] ?? "";
    const toolArgs = (m[2] ?? "").trim();
    const label = toolArgs
      ? `${toolName} ${toolArgs}`
      : toolName;
    matches.push({
      start: m.index!,
      end: m.index! + m[0].length,
      segment: {
        type: "tool",
        content: (m[3] ?? "").trim(),
        label,
      },
    });
  }

  // Collect code blocks
  for (const m of text.matchAll(CODE_BLOCK_RE)) {
    // Skip code blocks already inside tool/thinking blocks
    const idx = m.index!;
    const insideOther = matches.some(
      (o) => idx >= o.start && idx < o.end,
    );
    if (insideOther) continue;

    matches.push({
      start: idx,
      end: idx + m[0].length,
      segment: {
        type: "code",
        content: m[2] ?? "",
        label: m[1] || undefined,
      },
    });
  }

  if (matches.length === 0) {
    const onlyText: ContentSegment[] = [
      { type: "text", content: text.trimEnd() },
    ];
    cachePut(segmentCache, text, onlyText, MAX_SEGMENT_CACHE);
    return onlyText;
  }

  // Sort by position, remove overlaps
  matches.sort((a, b) => a.start - b.start);
  const deduped: Match[] = [];
  let lastEnd = 0;
  for (const m of matches) {
    if (m.start < lastEnd) continue;
    deduped.push(m);
    lastEnd = m.end;
  }

  // Build final segments with text gaps
  const segments: ContentSegment[] = [];
  let pos = 0;

  for (const m of deduped) {
    if (m.start > pos) {
      const gap = text.slice(pos, m.start).trimEnd();
      if (gap) {
        segments.push({ type: "text", content: gap });
      }
    }
    segments.push(m.segment);
    pos = m.end;
  }

  if (pos < text.length) {
    const tail = text.slice(pos).trimEnd();
    if (tail) {
      segments.push({ type: "text", content: tail });
    }
  }

  cachePut(segmentCache, text, segments, MAX_SEGMENT_CACHE);
  return segments;
}
