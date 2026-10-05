import { formatNumber, hostAndPath } from "@/lib/format";
import type { TestResult } from "@/lib/types";
import { MethodTag } from "../ui";
import { verdictTone } from "./severity";

function formatTime(iso?: string): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function VerdictCard({ result }: { result: TestResult }) {
  const analysis = result.analysis;
  const tone = analysis ? verdictTone(analysis.verdict) : null;
  const cancelled = result.status === "cancelled";
  const done = result.summary?.completedRequests ?? result.progress?.completed ?? 0;
  const total = result.summary?.totalRequests ?? result.config.requests;
  const finished = formatTime(result.finishedAt);

  return (
    <div
      className={`rounded-lg border-l-[6px] bg-panel px-5 py-6 sm:px-8 sm:py-8 ${
        tone ? tone.border : "border-rule"
      }`}
    >
      {tone ? (
        <p className={`flex items-center gap-2 text-sm font-semibold ${tone.text}`}>
          <span
            aria-hidden
            className={`grid size-5 place-items-center rounded-full text-[11px] font-bold text-white ${tone.solid}`}
          >
            {tone.glyph}
          </span>
          {tone.label}
        </p>
      ) : null}
      <h1
        id="report-title"
        className="mt-3 max-w-[22ch] text-3xl leading-[1.1] font-bold tracking-tight font-stretch-expanded text-balance sm:text-5xl"
      >
        {analysis?.headline ?? (cancelled ? "Test cancelled" : "Test finished")}
      </h1>
      <p className="mt-5 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-sm text-ink-2">
        <MethodTag method={result.config.method} />
        <span className="truncate font-semibold text-ink">{hostAndPath(result.config.url)}</span>
        <span>
          {formatNumber(total)} requests, {result.config.concurrency} at a time
        </span>
        {finished ? <span>finished {finished}</span> : null}
      </p>
      {cancelled ? (
        <p className="mt-4 max-w-prose rounded-md bg-warn-wash px-3 py-2 text-sm text-warn-ink">
          Cancelled after {formatNumber(done)} of {formatNumber(total)} requests. Everything below
          covers only the requests that finished.
        </p>
      ) : null}
    </div>
  );
}
