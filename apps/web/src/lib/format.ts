/** "281ms" below one second, "1.42s" from one second up. */
export function formatMs(ms: number | null | undefined): string {
  if (ms == null || !Number.isFinite(ms)) return "–";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 10000) return `${(ms / 1000).toFixed(2)}s`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const m = Math.floor(ms / 60000);
  const s = Math.round((ms % 60000) / 1000);
  return `${m}m ${s}s`;
}

/** Running clock: "4.3s", "1m 02s". */
export function formatClock(ms: number): string {
  const safe = Math.max(0, ms);
  if (safe < 60000) return `${(safe / 1000).toFixed(1)}s`;
  const m = Math.floor(safe / 60000);
  const s = Math.floor((safe % 60000) / 1000);
  return `${m}m ${String(s).padStart(2, "0")}s`;
}

export function formatPercent(fraction: number, digits = 1): string {
  if (!Number.isFinite(fraction)) return "–";
  const pct = fraction * 100;
  if (pct === 0) return "0%";
  if (pct < 0.1) return "<0.1%";
  return `${pct.toFixed(pct >= 10 || Number.isInteger(pct) ? 0 : digits)}%`;
}

export function formatNumber(n: number): string {
  return new Intl.NumberFormat("en-US").format(n);
}

export function formatRate(perSecond: number): string {
  if (!Number.isFinite(perSecond)) return "–";
  return perSecond >= 100 ? perSecond.toFixed(0) : perSecond.toFixed(1);
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** Round up to a "nice" axis maximum: 1, 2, 2.5, 5 × 10^k. */
export function niceCeil(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 1;
  const exp = Math.floor(Math.log10(value));
  const base = 10 ** exp;
  for (const step of [1, 2, 2.5, 5, 10]) {
    if (value <= step * base) return step * base;
  }
  return 10 * base;
}

/** Evenly spaced ticks from 0 to max (inclusive), about `count` of them. */
export function ticks(max: number, count = 4): number[] {
  const step = niceCeil(max / count);
  const out: number[] = [];
  for (let v = 0; v <= max + 1e-9; v += step) out.push(v);
  return out;
}

const ERROR_LABELS: Record<string, string> = {
  timeout: "Timed out",
  connection_refused: "Connection refused",
  connection_reset: "Connection reset",
  dns: "DNS lookup failed",
  tls: "TLS handshake failed",
  blocked: "Blocked by profiler",
  too_many_redirects: "Too many redirects",
  other: "Other network error",
};

export function errorLabel(code: string | undefined): string {
  if (!code) return "No response";
  return ERROR_LABELS[code] ?? code.replace(/_/g, " ");
}

/** Short label for a request's outcome: "200", "timeout", ... */
export function outcomeLabel(status: number, error?: string): string {
  if (status > 0) return String(status);
  return error ? errorLabel(error).toLowerCase() : "no response";
}

export function hostAndPath(url: string): string {
  try {
    const u = new URL(url);
    return `${u.host}${u.pathname === "/" ? "" : u.pathname}${u.search}`;
  } catch {
    return url;
  }
}
