import { niceCeil } from "./format";

/** Linear scale from a domain [d0, d1] to a range [r0, r1]. */
export function linear(d0: number, d1: number, r0: number, r1: number) {
  const span = d1 - d0 || 1;
  return (v: number) => r0 + ((v - d0) / span) * (r1 - r0);
}

function quantile(sorted: number[], q: number): number {
  if (sorted.length === 0) return 0;
  const idx = Math.min(sorted.length - 1, Math.max(0, Math.ceil(q * sorted.length) - 1));
  return sorted[idx];
}

/**
 * Picks a y-axis maximum for latency dots. Extreme outliers (e.g. a 10s
 * timeout among 200ms responses) would flatten everything else, so when the
 * largest value is far above the bulk the axis is capped and the outliers are
 * drawn as off-scale markers at the top edge.
 */
export function latencyCeiling(durations: number[], p95Hint?: number | null) {
  if (durations.length === 0) return { max: 500, clipped: false };
  const sorted = [...durations].sort((a, b) => a - b);
  const largest = sorted[sorted.length - 1];
  const p95 = p95Hint ?? quantile(sorted, 0.95);
  const median = quantile(sorted, 0.5);
  const cap = niceCeil(Math.max(p95 * 1.6, median * 3, 50));
  if (largest <= cap) {
    return { max: niceCeil(Math.max(largest * 1.1, 50)), clipped: false };
  }
  return { max: cap, clipped: true };
}

/** Estimated total run time from progress so far, for the live x-axis. */
export function estimateTotalMs(elapsedMs: number, completed: number, total: number) {
  if (completed < 2 || elapsedMs <= 0) return Math.max(elapsedMs * 2, 3000);
  return (elapsedMs / completed) * total;
}
