"use client";

import { formatClock, formatMs, formatNumber, formatRate } from "@/lib/format";
import { useAnimatedNumber } from "@/lib/hooks";

interface Props {
  completed: number;
  succeeded: number;
  failed: number;
  total: number;
  elapsedMs: number;
  etaMs: number | null;
  throughput: number | null;
  p50: number | null;
  p95: number | null;
  finished: boolean;
}

/** Big progress readout plus the live counters row. */
export function LiveProgress(props: Props) {
  const { completed, succeeded, failed, total, elapsedMs, etaMs, finished } = props;
  const okPct = total > 0 ? (succeeded / total) * 100 : 0;
  const failPct = total > 0 ? (failed / total) * 100 : 0;
  const shownCompleted = useAnimatedNumber(completed, 180) ?? completed;

  return (
    <div>
      <div className="flex flex-wrap items-end justify-between gap-x-8 gap-y-2">
        <p className="leading-none">
          <span className="text-6xl font-bold tracking-tight font-stretch-expanded tabular-nums sm:text-7xl">
            {formatNumber(Math.round(shownCompleted))}
          </span>
          <span className="ml-3 text-xl text-ink-3 sm:text-2xl">
            of {formatNumber(total)} requests
          </span>
        </p>
        <p className="flex gap-6 pb-1 text-sm text-ink-2 tabular-nums">
          <span>
            <span className="font-semibold text-ink">{formatClock(elapsedMs)}</span> elapsed
          </span>
          <span>
            {finished ? (
              "done"
            ) : etaMs == null ? (
              "estimating time left…"
            ) : etaMs < 1000 ? (
              "almost done"
            ) : (
              <>
                about <span className="font-semibold text-ink">{formatClock(etaMs)}</span> left
              </>
            )}
          </span>
        </p>
      </div>

      <div
        role="progressbar"
        aria-label="Requests completed"
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={completed}
        className="mt-5 flex h-3 overflow-hidden rounded-full bg-grid"
      >
        <span className="h-full bg-pen transition-[width] duration-300 ease-out" style={{ width: `${okPct}%` }} />
        <span className="h-full bg-fail transition-[width] duration-300 ease-out" style={{ width: `${failPct}%` }} />
      </div>

      <dl className="mt-6 grid grid-cols-2 gap-x-6 gap-y-5 sm:grid-cols-5">
        <Readout label="Succeeded" value={formatNumber(succeeded)} />
        <Readout label="Failed" value={formatNumber(failed)} tone={failed > 0 ? "fail" : undefined} />
        <LatencyReadout label="P50 latency" value={props.p50} />
        <LatencyReadout label="P95 latency" value={props.p95} />
        <Readout
          label="Throughput"
          value={props.throughput == null ? "–" : `${formatRate(props.throughput)}/s`}
        />
      </dl>
    </div>
  );
}

function Readout({ label, value, tone }: { label: string; value: string; tone?: "fail" }) {
  return (
    <div className="border-l-2 border-rule pl-3">
      <dt className="text-sm text-ink-3">{label}</dt>
      <dd
        className={`mt-1 text-2xl font-semibold font-stretch-expanded tabular-nums ${
          tone === "fail" ? "text-fail-ink" : "text-ink"
        }`}
      >
        {value}
      </dd>
    </div>
  );
}

function LatencyReadout({ label, value }: { label: string; value: number | null }) {
  const animated = useAnimatedNumber(value);
  return (
    <div className="border-l-2 border-pen/60 pl-3">
      <dt className="text-sm text-ink-3">{label}</dt>
      <dd className="mt-1 text-2xl font-semibold text-ink font-stretch-expanded tabular-nums">
        {animated == null ? (
          <span className="text-ink-3">waiting</span>
        ) : (
          <span key={Math.round(value ?? 0)} className="animate-tick">
            {formatMs(animated)}
          </span>
        )}
      </dd>
    </div>
  );
}
