"use client";

import { latencyCeiling, linear } from "@/lib/chart";
import { formatMs, formatSeconds, outcomeLabel, ticks } from "@/lib/format";
import { useElementWidth } from "@/lib/hooks";
import type { RequestSample } from "@/lib/types";

interface Props {
  samples: RequestSample[];
  elapsedMs: number;
  estimatedTotalMs: number;
  p50: number | null;
  p95: number | null;
  finished: boolean;
  host: string;
}

const PAD = { top: 18, right: 64, bottom: 28, left: 48 };

/**
 * A chart-recorder strip: every response lands as a dot at the moment it
 * completed (x) and how long it took (y), while the pen sweeps right in real
 * time. P50/P95 guide lines move as the distribution changes.
 */
export function LiveStripChart({
  samples,
  elapsedMs,
  estimatedTotalMs,
  p50,
  p95,
  finished,
  host,
}: Props) {
  const [ref, width] = useElementWidth<HTMLDivElement>();
  const height = width > 0 && width < 520 ? 230 : 300;

  const landed = samples.map((s) => s.startOffsetMs + s.durationMs);
  const lastLanded = landed.length ? Math.max(...landed) : 0;
  const xMax = finished
    ? Math.max(lastLanded, elapsedMs, 1000)
    : Math.max(elapsedMs * 1.08, estimatedTotalMs, lastLanded, 2000);
  const { max: yMax, clipped } = latencyCeiling(
    samples.map((s) => s.durationMs),
    p95,
  );

  const innerW = Math.max(0, width - PAD.left - PAD.right);
  const innerH = height - PAD.top - PAD.bottom;
  const x = linear(0, xMax, PAD.left, PAD.left + innerW);
  const y = linear(0, yMax, PAD.top + innerH, PAD.top);
  const penX = x(Math.min(elapsedMs, xMax));
  const yTicks = ticks(yMax, 4);
  const xTicks = ticks(xMax, width < 520 ? 3 : 6);
  const waiting = samples.length === 0;

  return (
    <div
      ref={ref}
      className="chart-paper relative overflow-hidden rounded-lg border border-rule"
      style={{ height }}
    >
      {width > 0 ? (
        <svg
          width={width}
          height={height}
          role="img"
          aria-label={`Live latency strip chart: ${samples.length} responses plotted.`}
          className="absolute inset-0"
        >
          {/* y axis */}
          {yTicks.map((t) => (
            <g key={`y${t}`} className="transition-transform duration-500" style={{ transform: `translateY(${y(t)}px)` }}>
              <line x1={PAD.left} x2={PAD.left + innerW} stroke="var(--color-rule)" strokeWidth={1} />
              <text x={PAD.left - 8} dy="0.32em" textAnchor="end" className="fill-ink-3 text-[11px]">
                {formatMs(t)}
              </text>
            </g>
          ))}
          {/* x axis */}
          {xTicks.map((t) => (
            <g key={`x${t}`} className="transition-transform duration-500" style={{ transform: `translateX(${x(t)}px)` }}>
              <text y={height - 8} textAnchor="middle" className="fill-ink-3 text-[11px]">
                {formatSeconds(t)}
              </text>
            </g>
          ))}

          {/* recorded region */}
          <rect
            x={PAD.left}
            y={PAD.top}
            width={Math.max(0, penX - PAD.left)}
            height={innerH}
            fill="var(--color-pen-wash)"
            opacity={0.35}
          />

          {/* percentile guides */}
          {p50 != null ? <Guide y={y(Math.min(p50, yMax))} x1={PAD.left} x2={PAD.left + innerW} label={`P50 ${formatMs(p50)}`} /> : null}
          {p95 != null ? <Guide y={y(Math.min(p95, yMax))} x1={PAD.left} x2={PAD.left + innerW} label={`P95 ${formatMs(p95)}`} dashed /> : null}

          {/* dots */}
          {samples.map((s, i) => {
            const off = s.durationMs > yMax;
            const cy = off ? PAD.top + 4 : y(s.durationMs);
            return (
              <g
                key={s.n}
                className="transition-transform duration-500 ease-out"
                style={{ transform: `translate(${x(landed[i])}px, ${cy}px)` }}
              >
                <title>{`#${s.n}: ${formatMs(s.durationMs)}, ${outcomeLabel(s.status, s.error)}`}</title>
                <Mark ok={s.ok} offScale={off} />
              </g>
            );
          })}

          {/* pen head */}
          {!finished ? (
            <g className="transition-transform duration-150 ease-linear" style={{ transform: `translateX(${penX}px)` }}>
              <line y1={PAD.top} y2={PAD.top + innerH} stroke="var(--color-pen)" strokeWidth={1.5} />
              <circle cy={PAD.top + innerH} r={4} fill="var(--color-pen)" className="animate-pen" />
            </g>
          ) : null}
        </svg>
      ) : null}

      {waiting && !finished ? (
        <div className="pointer-events-none absolute inset-0 grid place-items-center px-6 text-center">
          <div className="rounded-md bg-panel/90 px-4 py-3">
            <p className="font-semibold text-ink">Waiting for the first responses</p>
            <p className="mt-1 text-sm text-ink-3">
              Requests are on their way to {host}. Each response lands here as a dot.
            </p>
          </div>
        </div>
      ) : null}

      {clipped ? (
        <p className="absolute top-1.5 left-12 text-[11px] text-ink-3">
          ▲ above the chart: slower than {formatMs(yMax)}
        </p>
      ) : null}
    </div>
  );
}

function Guide({ y, x1, x2, label, dashed }: { y: number; x1: number; x2: number; label: string; dashed?: boolean }) {
  return (
    <g className="transition-transform duration-500 ease-out" style={{ transform: `translateY(${y}px)` }}>
      <line x1={x1} x2={x2} stroke="var(--color-ink-2)" strokeWidth={1.25} strokeDasharray={dashed ? "5 4" : undefined} />
      <text x={x2 + 6} dy="0.32em" className="fill-ink-2 text-[11px] font-semibold">
        {label}
      </text>
    </g>
  );
}

function Mark({ ok, offScale }: { ok: boolean; offScale: boolean }) {
  const color = ok ? "var(--color-pen)" : "var(--color-fail)";
  if (offScale) {
    return <path d="M0 -5 L5 4 L-5 4 Z" fill={color} stroke="var(--color-panel)" strokeWidth={1.5} className="animate-dot-in [transform-box:fill-box] origin-center" />;
  }
  if (!ok) {
    return (
      <g className="animate-dot-in [transform-box:fill-box] origin-center">
        <circle r={5} fill="var(--color-panel)" />
        <path d="M-3.2 -3.2 L3.2 3.2 M3.2 -3.2 L-3.2 3.2" stroke={color} strokeWidth={2} strokeLinecap="round" />
      </g>
    );
  }
  return (
    <circle r={3.75} fill={color} stroke="var(--color-panel)" strokeWidth={1.5} className="animate-dot-in [transform-box:fill-box] origin-center" />
  );
}
