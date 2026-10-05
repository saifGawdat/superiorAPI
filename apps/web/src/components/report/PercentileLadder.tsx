import { formatMs } from "@/lib/format";
import type { LatencyStats } from "@/lib/types";

const RUNGS: { key: keyof LatencyStats; label: string; meaning: string }[] = [
  { key: "p50", label: "P50", meaning: "Half of requests were faster" },
  { key: "p90", label: "P90", meaning: "9 in 10 were faster" },
  { key: "p95", label: "P95", meaning: "19 in 20 were faster" },
  { key: "p99", label: "P99", meaning: "99 in 100 were faster" },
  { key: "max", label: "Max", meaning: "The slowest request" },
];

/** Horizontal bars for each percentile, scaled to the slowest request. */
export function PercentileLadder({ latency }: { latency: LatencyStats }) {
  const max = Math.max(latency.max, 1);
  return (
    <ol className="flex flex-col gap-4 rounded-lg border border-rule bg-panel px-5 py-5">
      {RUNGS.map((r) => {
        const value = latency[r.key];
        const pct = Math.max(0.6, (value / max) * 100);
        return (
          <li
            key={r.key}
            className="grid grid-cols-[2.75rem_minmax(0,1fr)_4.25rem] items-center gap-x-3"
          >
            <span className="text-sm font-bold text-ink font-stretch-semi-expanded">
              {r.label}
            </span>
            <span className="h-3 rounded-sm bg-grid">
              <span
                className={`block h-full rounded-r-[4px] ${r.key === "max" ? "bg-ink-3" : "bg-pen"}`}
                style={{ width: `${pct}%` }}
              />
            </span>
            <span className="text-right text-sm font-semibold tabular-nums text-ink">
              {formatMs(value)}
            </span>
            <span className="col-start-2 col-end-4 mt-0.5 text-xs text-ink-3">{r.meaning}</span>
          </li>
        );
      })}
    </ol>
  );
}
