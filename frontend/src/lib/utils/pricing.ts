import type { ModelTokenUsage } from "../api/types.js";

/** Per-million-token pricing for a model (USD). */
interface ModelPricing {
  input: number;
  output: number;
  cacheWrite: number; // 5-minute ephemeral cache creation
  cacheRead: number;
}

/** Known Claude model pricing (per million tokens, USD). */
const PRICING: Record<string, ModelPricing> = {
  // Opus 4.6
  "claude-opus-4-6": { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5 },
  // Opus 4.5
  "claude-opus-4-5": { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5 },
  "claude-opus-4-5-20251101": { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5 },
  // Opus 4.1
  "claude-opus-4-1": { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.5 },
  // Opus 4.0
  "claude-opus-4-0-20250514": { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.5 },
  // Sonnet 4.6
  "claude-sonnet-4-6": { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3 },
  // Sonnet 4.5
  "claude-sonnet-4-5-20250514": { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3 },
  "claude-sonnet-4-5-20250929": { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3 },
  // Sonnet 4.0
  "claude-sonnet-4-0-20250514": { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3 },
  // Sonnet 3.7
  "claude-sonnet-3-7-20250219": { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3 },
  // Haiku 4.5
  "claude-haiku-4-5-20251001": { input: 1, output: 5, cacheWrite: 1.25, cacheRead: 0.1 },
  // Haiku 3.5
  "claude-3-5-haiku-20241022": { input: 0.8, output: 4, cacheWrite: 1, cacheRead: 0.08 },
  // Opus 3
  "claude-3-opus-20240229": { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.5 },
  // Haiku 3
  "claude-3-haiku-20240307": { input: 0.25, output: 1.25, cacheWrite: 0.3, cacheRead: 0.03 },
};

/** Attempt to match a model ID to known pricing using prefix matching. */
function findPricing(modelId: string): ModelPricing | null {
  if (PRICING[modelId]) return PRICING[modelId];
  // Try prefix match for versioned model IDs
  for (const [key, pricing] of Object.entries(PRICING)) {
    if (modelId.startsWith(key)) return pricing;
  }
  // Try matching by family name
  if (modelId.includes("opus-4-6") || modelId.includes("opus-4.6")) return PRICING["claude-opus-4-6"];
  if (modelId.includes("opus-4-5") || modelId.includes("opus-4.5")) return PRICING["claude-opus-4-5"];
  if (modelId.includes("opus-4-1") || modelId.includes("opus-4.1")) return PRICING["claude-opus-4-1"];
  if (modelId.includes("opus-4-0") || modelId.includes("opus-4")) return PRICING["claude-opus-4-0-20250514"];
  if (modelId.includes("sonnet-4-6") || modelId.includes("sonnet-4.6")) return PRICING["claude-sonnet-4-6"];
  if (modelId.includes("sonnet-4-5") || modelId.includes("sonnet-4.5")) return PRICING["claude-sonnet-4-5-20250514"];
  if (modelId.includes("sonnet-4-0") || modelId.includes("sonnet-4")) return PRICING["claude-sonnet-4-0-20250514"];
  if (modelId.includes("sonnet-3-7") || modelId.includes("sonnet-3.7")) return PRICING["claude-sonnet-3-7-20250219"];
  if (modelId.includes("haiku-4-5") || modelId.includes("haiku-4.5")) return PRICING["claude-haiku-4-5-20251001"];
  if (modelId.includes("haiku-3-5") || modelId.includes("3-5-haiku")) return PRICING["claude-3-5-haiku-20241022"];
  if (modelId.includes("haiku-3") || modelId.includes("3-haiku")) return PRICING["claude-3-haiku-20240307"];
  if (modelId.includes("opus-3")) return PRICING["claude-3-opus-20240229"];
  return null;
}

/** Calculate the cost in USD for a given model's token usage. */
export function calculateModelCost(
  modelId: string,
  usage: ModelTokenUsage,
): number | null {
  const pricing = findPricing(modelId);
  if (!pricing) return null;
  const mtok = 1_000_000;
  return (
    (usage.input_tokens / mtok) * pricing.input +
    (usage.output_tokens / mtok) * pricing.output +
    (usage.cache_creation_input_tokens / mtok) * pricing.cacheWrite +
    (usage.cache_read_input_tokens / mtok) * pricing.cacheRead
  );
}

/** Format a USD cost as a compact string. */
export function formatCost(usd: number): string {
  if (usd < 0.01) return "<$0.01";
  if (usd < 1) return "$" + usd.toFixed(2);
  if (usd < 10) return "$" + usd.toFixed(2);
  return "$" + usd.toFixed(1);
}

/** Get a short display name for a model ID. */
export function shortModelName(modelId: string): string {
  // Strip "claude-" prefix and date suffixes for display
  let name = modelId.replace(/^claude-/, "");
  // Remove date suffix like -20250514
  name = name.replace(/-\d{8}$/, "");
  return name;
}
