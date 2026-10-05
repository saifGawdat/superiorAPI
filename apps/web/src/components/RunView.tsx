"use client";

import { useEffect, useMemo, useState } from "react";
import { cancelTest, isApiError } from "@/lib/api";
import { estimateTotalMs } from "@/lib/chart";
import { hostAndPath } from "@/lib/format";
import { useTicker } from "@/lib/hooks";
import type { StartTestRequest, TestResult } from "@/lib/types";
import { useTestRun } from "@/lib/useTestRun";
import { Banner } from "./Banner";
import { LiveFeed } from "./live/LiveFeed";
import { LiveProgress } from "./live/LiveProgress";
import { LiveStripChart } from "./live/LiveStripChart";
import { Button, MethodTag } from "./ui";

const HANDOFF_DELAY_MS = 900;
/** Don't extrapolate the clock further than this past the last update. */
const MAX_CLOCK_DRIFT_MS = 1500;

interface Props {
  testId: string;
  request: StartTestRequest;
  onDone: (result: TestResult) => void;
  onBack: () => void;
}

export function RunView({ testId, request, onDone, onBack }: Props) {
  const run = useTestRun(testId);
  const [cancel, setCancel] = useState<{ state: "idle" | "pending" } | { state: "failed"; message: string }>({ state: "idle" });
  const finished = run.result != null;
  const now = useTicker(!finished && !run.fatal);
  const [startedAt] = useState(() => (typeof performance === "undefined" ? 0 : performance.now()));

  // Hold the finished state on screen briefly so the run visibly completes.
  useEffect(() => {
    if (!run.result) return;
    const result = run.result;
    const id = setTimeout(() => onDone(result), HANDOFF_DELAY_MS);
    return () => clearTimeout(id);
  }, [run.result, onDone]);

  const p = run.progress;
  const total = p?.total ?? request.requests;
  const completed = p?.completed ?? 0;

  let elapsedMs: number;
  let drift = 0;
  if (run.result) {
    elapsedMs = run.result.summary?.durationMs ?? p?.elapsedMs ?? 0;
  } else if (p && run.receivedAt != null) {
    drift = Math.min(Math.max(0, now - run.receivedAt), MAX_CLOCK_DRIFT_MS);
    elapsedMs = p.elapsedMs + drift;
  } else {
    elapsedMs = Math.max(0, now - startedAt);
  }

  const throughput = p && p.elapsedMs > 300 && completed > 0 ? completed / (p.elapsedMs / 1000) : null;
  const etaMs =
    !finished && throughput && completed >= 2
      ? Math.max(0, ((total - completed) / throughput) * 1000 - drift)
      : null;

  const samples = useMemo(() => Array.from(run.samples.values()), [run.samples]);
  const feedScale = Math.max(
    (p?.p95 ?? 0) * 1.5,
    ...run.feed.filter((r) => r.status !== 0).map((r) => r.durationMs),
    100,
  );

  const announce = finished
    ? `Test ${run.result?.status === "cancelled" ? "cancelled" : "finished"}. Preparing the report.`
    : `${Math.floor((completed / Math.max(total, 1)) * 10) * 10} percent done, ${p?.failed ?? 0} failed so far.`;

  const requestCancel = async () => {
    setCancel({ state: "pending" });
    try {
      await cancelTest(testId);
    } catch (err) {
      setCancel({
        state: "failed",
        message: isApiError(err) ? err.message : "The cancel request failed.",
      });
    }
  };

  const host = hostAndPath(request.url);

  return (
    <section aria-labelledby="run-title" className="flex flex-col gap-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 id="run-title" className="flex min-w-0 items-center gap-2 text-lg font-semibold">
            <MethodTag method={request.method} />
            <span className="truncate">{host}</span>
          </h1>
          <p className="mt-1 text-sm text-ink-3">
            <StatusLine transport={run.transport} finished={finished} status={run.result?.status} concurrency={request.concurrency} />
          </p>
        </div>
        {!finished && !run.fatal ? (
          <div className="flex flex-col items-end gap-1">
            <Button variant="secondary" onClick={requestCancel} disabled={cancel.state === "pending"}>
              {cancel.state === "pending" ? "Cancelling…" : "Cancel test"}
            </Button>
            {cancel.state === "failed" ? <p className="text-xs text-fail-ink">{cancel.message}</p> : null}
          </div>
        ) : null}
      </div>

      <p className="sr-only" role="status" aria-live="polite">
        {announce}
      </p>

      {run.fatal ? (
        <Banner tone="error" title="This test can't be followed" action={<Button variant="secondary" onClick={onBack}>Back to setup</Button>}>
          {run.fatal}
        </Banner>
      ) : run.connectionIssue ? (
        <Banner tone="warning" title="Connection trouble">
          {run.connectionIssue}
        </Banner>
      ) : null}

      <LiveProgress
        completed={completed}
        succeeded={p?.succeeded ?? 0}
        failed={p?.failed ?? 0}
        total={total}
        elapsedMs={elapsedMs}
        etaMs={etaMs}
        throughput={throughput}
        p50={p?.p50 ?? null}
        p95={p?.p95 ?? null}
        finished={finished}
      />

      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div>
          <h2 className="mb-2 text-sm font-semibold text-ink-2">
            Response times as they land
          </h2>
          <LiveStripChart
            samples={samples}
            elapsedMs={elapsedMs}
            estimatedTotalMs={estimateTotalMs(p?.elapsedMs ?? elapsedMs, completed, total)}
            p50={p?.p50 ?? null}
            p95={p?.p95 ?? null}
            finished={finished}
            host={host}
          />
          <Legend />
        </div>
        <LiveFeed feed={run.feed} scaleMax={feedScale} />
      </div>
    </section>
  );
}

function StatusLine({
  transport,
  finished,
  status,
  concurrency,
}: {
  transport: string;
  finished: boolean;
  status?: string;
  concurrency: number;
}) {
  if (finished) {
    return status === "cancelled" ? "Cancelled. Building a report from the requests that finished…" : "Finished. Building your report…";
  }
  if (transport === "connecting") return "Connecting to the live results…";
  const mode = transport === "polling" ? "Checking for results every 0.7s (live stream unavailable)" : "Streaming live results";
  return `${mode}, ${concurrency} request${concurrency === 1 ? "" : "s"} in flight at a time.`;
}

function Legend() {
  return (
    <p className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-xs text-ink-3">
      <span className="inline-flex items-center gap-1.5">
        <svg aria-hidden width="10" height="10"><circle cx="5" cy="5" r="4" fill="var(--color-pen)" /></svg>
        Succeeded
      </span>
      <span className="inline-flex items-center gap-1.5">
        <svg aria-hidden width="10" height="10"><path d="M1.5 1.5 L8.5 8.5 M8.5 1.5 L1.5 8.5" stroke="var(--color-fail)" strokeWidth="2" strokeLinecap="round" /></svg>
        Failed
      </span>
      <span className="inline-flex items-center gap-1.5">
        <svg aria-hidden width="18" height="10"><line x1="0" x2="18" y1="5" y2="5" stroke="var(--color-ink-2)" strokeWidth="1.25" /></svg>
        P50
      </span>
      <span className="inline-flex items-center gap-1.5">
        <svg aria-hidden width="18" height="10"><line x1="0" x2="18" y1="5" y2="5" stroke="var(--color-ink-2)" strokeWidth="1.25" strokeDasharray="5 4" /></svg>
        P95
      </span>
    </p>
  );
}
