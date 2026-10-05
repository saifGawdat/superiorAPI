"use client";

import { linear } from "@/lib/chart";
import { formatMs, formatNumber, niceCeil, ticks } from "@/lib/format";
import { useElementWidth } from "@/lib/hooks";
import type { HistogramBucket, LatencyStats } from "@/lib/types";

const PAD = { top: 44, right: 12, bottom: 30, left: 36 };
const HEIGHT = 250;

function isOverflow(buckets: HistogramBucket[]): boolean {
  if (buckets.length < 3) return false;
  const widths = buckets.slice(0, -1).map((b) => b.toMs - b.fromMs).sort((a, b) => a - b);
  const median = widths[Math.floor(widths.length / 2)];
  const last = buckets[buckets.length - 1];
  return last.toMs - last.fromMs > median * 1.5;
}

/** Position of a latency value in bar units (bucket index + fraction). */
function bucketPosition(buckets: HistogramBucket[], v: number): number {
  for (let i = 0; i < buckets.length; i++) {
    const b = buckets[i];
    if (v < b.toMs || i === buckets.length - 1) {
      const frac = (v - b.fromMs) / (b.toMs - b.fromMs || 1);
      return i + Math.min(1, Math.max(0, frac));
    }
  }
  return buckets.length;
}

/**
 * Latency distribution. Buckets are drawn at equal width so a wide overflow
 * bucket doesn't squash the rest; percentile markers interpolate within
 * their bucket.
 */
export function Histogram({
  buckets,
  latency,
}: {
  buckets: HistogramBucket[];
  latency: LatencyStats | null;
}) {
  const [ref, width] = useElementWidth<HTMLDivElement>();
  const overflow = isOverflow(buckets);
  const maxCount = niceCeil(Math.max(1, ...buckets.map((b) => b.count)));
  const innerW = Math.max(0, width - PAD.left - PAD.right);
  const innerH = HEIGHT - PAD.top - PAD.bottom;
  const band = innerW / Math.max(1, buckets.length);
  const y = linear(0, maxCount, PAD.top + innerH, PAD.top);
  const xAt = (pos: number) => PAD.left + pos * band;
  const gap = band > 14 ? 2 : 1;
  const labelEvery = Math.max(1, Math.ceil(46 / Math.max(band, 1)));

  const markers = latency
    ? [
        { key: "P50", value: latency.p50 },
        { key: "P95", value: latency.p95 },
        { key: "P99", value: latency.p99 },
      ]
    : [];
  let prevX = -Infinity;
  let prevRow = -1;
  const placed = markers.map((m) => {
    const mx = xAt(bucketPosition(buckets, m.value));
    const row = mx - prevX < 64 ? prevRow + 1 : 0;
    prevX = mx;
    prevRow = row;
    return { ...m, x: mx, row };
  });

  return (
    <div ref={ref} className="chart-paper overflow-hidden rounded-lg border border-rule" style={{ height: HEIGHT }}>
      {width > 0 ? (
        <svg width={width} height={HEIGHT} role="img" aria-label="Latency histogram">
          {ticks(maxCount, 3).map((t) => (
            <g key={t}>
              <line x1={PAD.left} x2={PAD.left + innerW} y1={y(t)} y2={y(t)} stroke="var(--color-rule)" />
              <text x={PAD.left - 6} y={y(t)} dy="0.32em" textAnchor="end" className="fill-ink-3 text-[11px]">
                {formatNumber(t)}
              </text>
            </g>
          ))}

          {buckets.map((b, i) => {
            const last = i === buckets.length - 1;
            const h = y(0) - y(b.count);
            const range = last && overflow ? `${formatMs(b.fromMs)} and slower` : `${formatMs(b.fromMs)}–${formatMs(b.toMs)}`;
            return (
              <g key={`${b.fromMs}-${b.toMs}`} className="group">
                <title>{`${range}: ${formatNumber(b.count)} request${b.count === 1 ? "" : "s"}`}</title>
                <rect x={xAt(i)} y={PAD.top} width={band} height={innerH} fill="transparent" />
                {b.count > 0 ? (
                  <path
                    d={roundedTop(xAt(i) + gap / 2, y(b.count), Math.max(1, band - gap), h, Math.min(4, (band - gap) / 2, h))}
                    fill={last && overflow ? "url(#overflow-hatch)" : "var(--color-pen)"}
                    className="transition-opacity group-hover:opacity-75"
                  />
                ) : null}
                {last && overflow ? (
                  <text x={xAt(i + 1)} y={HEIGHT - 10} textAnchor="end" className="fill-ink-3 text-[11px]">
                    {formatMs(b.fromMs)}+
                  </text>
                ) : i % labelEvery === 0 && !(overflow && i === buckets.length - 2) ? (
                  <text x={xAt(i)} y={HEIGHT - 10} textAnchor={i === 0 ? "start" : "middle"} className="fill-ink-3 text-[11px]">
                    {formatMs(b.fromMs)}
                  </text>
                ) : null}
              </g>
            );
          })}
          {!overflow && buckets.length > 0 ? (
            <text x={xAt(buckets.length)} y={HEIGHT - 10} textAnchor="end" className="fill-ink-3 text-[11px]">
              {formatMs(buckets[buckets.length - 1].toMs)}
            </text>
          ) : null}

          {placed.map((m) => (
            <g key={m.key}>
              <line x1={m.x} x2={m.x} y1={PAD.top - 6 - m.row * 13} y2={PAD.top + innerH} stroke="var(--color-ink)" strokeWidth={1.25} strokeDasharray={m.key === "P50" ? undefined : "4 3"} />
              <text
                x={m.x}
                y={PAD.top - 10 - m.row * 13}
                textAnchor={m.x > width - 70 ? "end" : m.x < 70 ? "start" : "middle"}
                className="fill-ink text-[11px] font-semibold"
              >
                {m.key} {formatMs(m.value)}
              </text>
            </g>
          ))}

          <defs>
            <pattern id="overflow-hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
              <rect width="6" height="6" fill="var(--color-pen-wash)" />
              <line x1="0" y1="0" x2="0" y2="6" stroke="var(--color-pen)" strokeWidth="3" />
            </pattern>
          </defs>
        </svg>
      ) : null}
    </div>
  );
}

/** A bar path with rounded top corners, anchored flat on the baseline. */
function roundedTop(x: number, y: number, w: number, h: number, r: number): string {
  const rr = Math.max(0, r);
  return `M${x},${y + h} V${y + rr} Q${x},${y} ${x + rr},${y} H${x + w - rr} Q${x + w},${y} ${x + w},${y + rr} V${y + h} Z`;
}
