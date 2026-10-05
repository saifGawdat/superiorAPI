import { errorLabel, formatMs } from "@/lib/format";
import type { RequestSample } from "@/lib/types";

/** Streaming list of the latest completed requests, newest on top. */
export function LiveFeed({ feed, scaleMax }: { feed: RequestSample[]; scaleMax: number }) {
  return (
    <section aria-labelledby="feed-title" className="flex min-h-0 flex-col">
      <div className="mb-2 flex items-baseline justify-between">
        <h2 id="feed-title" className="text-sm font-semibold text-ink-2">
          Latest requests
        </h2>
        <span className="text-xs text-ink-3">newest first</span>
      </div>
      {feed.length === 0 ? (
        <div className="flex-1 rounded-lg border border-dashed border-rule px-4 py-6 text-sm text-ink-3">
          Requests appear here the moment they complete, with their time and status.
        </div>
      ) : (
        <ol className="overflow-hidden rounded-lg border border-rule bg-panel">
          {feed.map((r) => {
            const width = Math.max(3, Math.min(100, (r.durationMs / scaleMax) * 100));
            return (
              <li
                key={r.n}
                className="grid animate-row-in grid-cols-[3.25rem_1fr_4rem_5.5rem] items-center gap-2 border-b border-rule/70 px-3 py-1.5 text-sm last:border-b-0"
              >
                <span className="text-ink-3 tabular-nums">#{r.n}</span>
                <span aria-hidden className="h-1.5 overflow-hidden rounded-full bg-grid">
                  <span
                    className={`block h-full rounded-full ${r.ok ? "bg-pen" : "bg-fail"}`}
                    style={{ width: `${width}%` }}
                  />
                </span>
                <span className="text-right font-semibold tabular-nums text-ink">
                  {formatMs(r.durationMs)}
                </span>
                <StatusChip sample={r} />
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}

function StatusChip({ sample }: { sample: RequestSample }) {
  if (sample.status === 0) {
    return (
      <span
        className="truncate text-right text-xs font-semibold text-fail-ink"
        title={errorLabel(sample.error)}
      >
        {errorLabel(sample.error)}
      </span>
    );
  }
  const tone = sample.ok
    ? "bg-good-wash text-good-ink"
    : sample.status >= 500
      ? "bg-fail-wash text-fail-ink"
      : "bg-warn-wash text-warn-ink";
  return (
    <span className="justify-self-end">
      <span className={`rounded-sm px-1.5 py-0.5 text-xs font-bold tabular-nums ${tone}`}>
        {sample.status}
        <span className="sr-only">{sample.ok ? " ok" : " failed"}</span>
      </span>
    </span>
  );
}
