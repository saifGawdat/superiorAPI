import {
  formatBytes,
  formatMs,
  formatNumber,
  formatPercent,
  formatRate,
} from "@/lib/format";
import type { Summary } from "@/lib/types";

interface Metric {
  label: string;
  value: string;
  tone?: "fail";
  hint?: string;
}

export function MetricsGrid({ summary }: { summary: Summary }) {
  const lat = summary.latency;
  const volume: Metric[] = [
    { label: "Total requests", value: formatNumber(summary.totalRequests) },
    {
      label: "Succeeded",
      value: formatNumber(summary.successfulRequests),
    },
    {
      label: "Failed",
      value: formatNumber(summary.failedRequests),
      tone: summary.failedRequests > 0 ? "fail" : undefined,
    },
    {
      label: "Error rate",
      value: formatPercent(summary.errorRate),
      tone: summary.errorRate > 0 ? "fail" : undefined,
    },
    { label: "Duration", value: formatMs(summary.durationMs) },
    {
      label: "Throughput",
      value: `${formatRate(summary.throughput)} req/s`,
      hint: `${formatBytes(summary.bytesReceived)} received`,
    },
  ];
  const latency: Metric[] = lat
    ? [
        { label: "Min", value: formatMs(lat.min) },
        { label: "Avg", value: formatMs(lat.avg) },
        { label: "P50", value: formatMs(lat.p50) },
        { label: "P90", value: formatMs(lat.p90) },
        { label: "P95", value: formatMs(lat.p95) },
        { label: "P99", value: formatMs(lat.p99) },
        { label: "Max", value: formatMs(lat.max) },
      ]
    : [];

  return (
    <div className="flex flex-col gap-6">
      <MetricRow title="Requests" metrics={volume} cols="grid-cols-2 sm:grid-cols-3 lg:grid-cols-6" />
      {lat ? (
        <MetricRow
          title="Latency"
          note={`From ${formatNumber(summary.latencySampleSize)} requests that got an HTTP response.`}
          metrics={latency}
          cols="grid-cols-3 sm:grid-cols-4 lg:grid-cols-7"
        />
      ) : (
        <p className="text-sm text-ink-2">
          No request got an HTTP response, so there are no latency figures.
        </p>
      )}
    </div>
  );
}

function MetricRow({
  title,
  note,
  metrics,
  cols,
}: {
  title: string;
  note?: string;
  metrics: Metric[];
  cols: string;
}) {
  return (
    <div>
      <div className="mb-2 flex flex-wrap items-baseline gap-x-3">
        <h3 className="text-sm font-semibold text-ink-2">{title}</h3>
        {note ? <p className="text-xs text-ink-3">{note}</p> : null}
      </div>
      <div className="overflow-hidden rounded-lg border border-rule bg-panel">
      <dl className={`-mr-px -mb-px grid ${cols}`}>
        {metrics.map((m) => (
          <div key={m.label} className="border-r border-b border-rule px-4 py-3">
            <dt className="text-sm text-ink-3">{m.label}</dt>
            <dd
              className={`mt-0.5 text-xl font-semibold font-stretch-semi-expanded tabular-nums sm:text-2xl ${
                m.tone === "fail" ? "text-fail-ink" : "text-ink"
              }`}
            >
              {m.value}
            </dd>
            {m.hint ? <dd className="text-xs text-ink-3">{m.hint}</dd> : null}
          </div>
        ))}
      </dl>
      </div>
    </div>
  );
}
