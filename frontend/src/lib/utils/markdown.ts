import { Marked } from "marked";
import DOMPurify from "dompurify";

const parser = new Marked({
  gfm: true,
  breaks: true,
});

const MAX_MARKDOWN_CACHE = 6000;
const markdownCache = new Map<string, string>();

function cacheGet(key: string): string | undefined {
  const value = markdownCache.get(key);
  if (value === undefined) return undefined;
  // Move to end of insertion order for LRU eviction
  markdownCache.delete(key);
  markdownCache.set(key, value);
  return value;
}

function cachePut(key: string, value: string) {
  if (markdownCache.has(key)) return;
  if (markdownCache.size >= MAX_MARKDOWN_CACHE) {
    const first = markdownCache.keys().next();
    if (!first.done) {
      markdownCache.delete(first.value);
    }
  }
  markdownCache.set(key, value);
}

export function renderMarkdown(text: string): string {
  if (!text) return "";
  const cached = cacheGet(text);
  if (cached !== undefined) {
    return cached;
  }
  // Trim trailing whitespace — with breaks:true, trailing
  // newlines become <br> tags that add invisible height.
  const html = parser.parse(text.trimEnd()) as string;
  const safe = DOMPurify.sanitize(html);
  cachePut(text, safe);
  return safe;
}
