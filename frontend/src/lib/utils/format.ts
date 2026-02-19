const MINUTE = 60;
const HOUR = 3600;
const DAY = 86400;
const WEEK = 604800;

/** Formats an ISO timestamp as a human-friendly relative time */
export function formatRelativeTime(
  isoString: string | null | undefined,
): string {
  if (!isoString) return "—";

  const now = Date.now();
  const then = new Date(isoString).getTime();
  const diffSec = Math.floor((now - then) / 1000);

  if (diffSec < 0) return "just now";
  if (diffSec < MINUTE) return "just now";
  if (diffSec < HOUR) {
    const m = Math.floor(diffSec / MINUTE);
    return `${m}m ago`;
  }
  if (diffSec < DAY) {
    const h = Math.floor(diffSec / HOUR);
    return `${h}h ago`;
  }
  if (diffSec < WEEK) {
    const d = Math.floor(diffSec / DAY);
    return `${d}d ago`;
  }

  return new Date(isoString).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });
}

/** Formats an ISO timestamp as a readable date/time string */
export function formatTimestamp(
  isoString: string | null | undefined,
): string {
  if (!isoString) return "—";
  const d = new Date(isoString);
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** Truncates a string with ellipsis */
export function truncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s;
  return s.slice(0, maxLen - 1) + "\u2026";
}

/** Formats an agent name for display */
export function formatAgentName(agent: string): string {
  if (!agent) return "Unknown";
  // Capitalize first letter
  return agent.charAt(0).toUpperCase() + agent.slice(1);
}

/** Formats a number with commas */
export function formatNumber(n: number): string {
  return n.toLocaleString();
}

/**
 * Sanitize an HTML snippet from FTS search results.
 * Only allows <mark> tags for highlighting; strips everything else.
 */
export function sanitizeSnippet(html: string): string {
  return html
    .replace(/<mark>/gi, "\x00MARK_OPEN\x00")
    .replace(/<\/mark>/gi, "\x00MARK_CLOSE\x00")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\x00MARK_OPEN\x00/g, "<mark>")
    .replace(/\x00MARK_CLOSE\x00/g, "</mark>");
}
