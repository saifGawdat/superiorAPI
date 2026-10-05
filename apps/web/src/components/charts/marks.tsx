/** Shared SVG marks for the latency scatter charts. Drawn around (0, 0). */

export function LatencyMark({ ok, offScale }: { ok: boolean; offScale: boolean }) {
  const color = ok ? "var(--color-pen)" : "var(--color-fail)";
  const anim = "animate-dot-in [transform-box:fill-box] origin-center";
  if (offScale) {
    return (
      <path
        d="M0 -5 L5 4 L-5 4 Z"
        fill={color}
        stroke="var(--color-panel)"
        strokeWidth={1.5}
        className={anim}
      />
    );
  }
  if (!ok) {
    return (
      <g className={anim}>
        <circle r={5} fill="var(--color-panel)" />
        <path
          d="M-3.2 -3.2 L3.2 3.2 M3.2 -3.2 L-3.2 3.2"
          stroke={color}
          strokeWidth={2}
          strokeLinecap="round"
        />
      </g>
    );
  }
  return (
    <circle r={3.75} fill={color} stroke="var(--color-panel)" strokeWidth={1.5} className={anim} />
  );
}

/** Horizontal percentile guide with a right-hand label. */
export function GuideLine({
  y,
  x1,
  x2,
  label,
  dashed,
}: {
  y: number;
  x1: number;
  x2: number;
  label: string;
  dashed?: boolean;
}) {
  return (
    <g
      className="transition-transform duration-500 ease-out"
      style={{ transform: `translateY(${y}px)` }}
    >
      <line
        x1={x1}
        x2={x2}
        stroke="var(--color-ink-2)"
        strokeWidth={1.25}
        strokeDasharray={dashed ? "5 4" : undefined}
      />
      <text x={x2 + 6} dy="0.32em" className="fill-ink-2 text-[11px] font-semibold">
        {label}
      </text>
    </g>
  );
}

export function ScatterLegend({ guides = true }: { guides?: boolean }) {
  return (
    <p className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-xs text-ink-3">
      <span className="inline-flex items-center gap-1.5">
        <svg aria-hidden width="10" height="10">
          <circle cx="5" cy="5" r="4" fill="var(--color-pen)" />
        </svg>
        Succeeded
      </span>
      <span className="inline-flex items-center gap-1.5">
        <svg aria-hidden width="10" height="10">
          <path
            d="M1.5 1.5 L8.5 8.5 M8.5 1.5 L1.5 8.5"
            stroke="var(--color-fail)"
            strokeWidth="2"
            strokeLinecap="round"
          />
        </svg>
        Failed
      </span>
      {guides ? (
        <>
          <span className="inline-flex items-center gap-1.5">
            <svg aria-hidden width="18" height="10">
              <line x1="0" x2="18" y1="5" y2="5" stroke="var(--color-ink-2)" strokeWidth="1.25" />
            </svg>
            P50
          </span>
          <span className="inline-flex items-center gap-1.5">
            <svg aria-hidden width="18" height="10">
              <line
                x1="0"
                x2="18"
                y1="5"
                y2="5"
                stroke="var(--color-ink-2)"
                strokeWidth="1.25"
                strokeDasharray="5 4"
              />
            </svg>
            P95
          </span>
        </>
      ) : null}
    </p>
  );
}
