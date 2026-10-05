"use client";

import { latencyCeiling, linear } from "@/lib/chart";
import { formatMs, formatSeconds, outcomeLabel, ticks } from "@/lib/format";
import { useElementWidth } from "@/lib/hooks";
import type { LatencyStats, RequestSample } from "@/lib/types";
import { GuideLine, LatencyMark } from "../charts/marks";

const PAD = { top: 22, right: 70, bottom: 28, left: 48 };
const HEIGHT = 280;

/** Every request: when it started (x) against how long it took (y). */
export function Timeline({
  requests,
  latency,
}: {
  requests: RequestSample[];
  latency: LatencyStats | null;
}) {
  const [ref, width] = useElementWidth<HTMLDivElement>();
  const xMax = Math.max(1000, ...requests.map((r) => r.startOffsetMs)) * 1.02;
  const { max: yMax, clipped } = latencyCeiling(
    requests.filter((r) => r.status !== 0).map((r) => r.durationMs),
    latency?.p95,
  );
  const innerW = Math.max(0, width - PAD.left - PAD.right);
  const innerH = HEIGHT - PAD.top - PAD.bottom;
  const x = linear(0, xMax, PAD.left, PAD.left + innerW);
  const y = linear(0, yMax, PAD.top + innerH, PAD.top);

  return (
    <>
      <div
        ref={ref}
        className="chart-paper relative overflow-hidden rounded-lg border border-rule"
        style={{ height: HEIGHT }}
      >
        {width > 0 ? (
          <svg
            width={width}
            height={HEIGHT}
            role="img"
            aria-label={`Timeline of ${requests.length} requests: start time against duration.`}
          >
            {ticks(yMax, 4).map((t) => (
              <g key={`y${t}`}>
                <line x1={PAD.left} x2={PAD.left + innerW} y1={y(t)} y2={y(t)} stroke="var(--color-rule)" />
                <text x={PAD.left - 8} y={y(t)} dy="0.32em" textAnchor="end" className="fill-ink-3 text-[11px]">
                  {formatMs(t)}
                </text>
              </g>
            ))}
            {ticks(xMax, width < 520 ? 3 : 6).map((t) => (
              <text key={`x${t}`} x={x(t)} y={HEIGHT - 8} textAnchor="middle" className="fill-ink-3 text-[11px]">
                {formatSeconds(t)}
              </text>
            ))}

            {latency ? (
              <>
                <GuideLine y={y(Math.min(latency.p50, yMax))} x1={PAD.left} x2={PAD.left + innerW} label={`P50 ${formatMs(latency.p50)}`} />
                <GuideLine y={y(Math.min(latency.p95, yMax))} x1={PAD.left} x2={PAD.left + innerW} label={`P95 ${formatMs(latency.p95)}`} dashed />
              </>
            ) : null}

            {requests.map((r) => {
              const off = r.durationMs > yMax;
              return (
                <g key={r.n} transform={`translate(${x(r.startOffsetMs)} ${off ? PAD.top - 9 : y(r.durationMs)})`}>
                  <title>{`#${r.n} started at ${formatSeconds(r.startOffsetMs)}: ${formatMs(r.durationMs)}, ${outcomeLabel(r.status, r.error)}`}</title>
                  <LatencyMark ok={r.ok} offScale={off} />
                </g>
              );
            })}
          </svg>
        ) : null}
      </div>
      {clipped ? (
        <p className="mt-1.5 text-xs text-ink-3">
          ▲ Markers along the top edge were slower than {formatMs(yMax)}.
        </p>
      ) : null}
    </>
  );
}
