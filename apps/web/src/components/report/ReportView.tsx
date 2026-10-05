"use client";

import type { ReactNode } from "react";
import { formatMs, formatNumber } from "@/lib/format";
import type { TestResult } from "@/lib/types";
import { Banner } from "../Banner";
import { ScatterLegend } from "../charts/marks";
import { Button } from "../ui";
import { Findings } from "./Findings";
import { Histogram } from "./Histogram";
import { MetricsGrid } from "./MetricsGrid";
import { PercentileLadder } from "./PercentileLadder";
import { StatusCodes } from "./StatusCodes";
import { Timeline } from "./Timeline";
import { VerdictCard } from "./VerdictCard";

interface Props {
  result: TestResult;
  rerunning: boolean;
  onRunAgain: () => void;
  onNewTest: () => void;
}

export function ReportView({ result, rerunning, onRunAgain, onNewTest }: Props) {
  const { summary, analysis, histogram, requests } = result;
  const latency = summary?.latency ?? null;

  const actions = (
    <div className="flex flex-wrap gap-3">
      <Button onClick={onRunAgain} disabled={rerunning}>
        {rerunning ? "Starting…" : "Run again"}
      </Button>
      <Button variant="secondary" onClick={onNewTest} className="!py-3 !text-base">
        New test
      </Button>
    </div>
  );

  return (
    <article aria-labelledby="report-title" className="flex flex-col gap-12">
      <div className="flex flex-col gap-5">
        <VerdictCard result={result} />
        {actions}
      </div>

      {!summary ? (
        <Banner tone="warning" title="No results were recorded">
          The test ended before any request finished, so there is nothing to report.
        </Banner>
      ) : null}

      {analysis ? (
        <Section title="What we found" note="Each finding separates what we measured from what might explain it.">
          <Findings findings={analysis.findings} />
        </Section>
      ) : null}

      {summary ? (
        <Section title="The numbers">
          <MetricsGrid summary={summary} />
          {summary.slowestRequest ? (
            <p className="mt-3 text-sm text-ink-3">
              Slowest request: #{summary.slowestRequest.n} took{" "}
              {formatMs(summary.slowestRequest.durationMs)}
              {summary.slowestRequest.status > 0 ? ` (status ${summary.slowestRequest.status})` : " (no response)"}.
            </p>
          ) : null}
        </Section>
      ) : null}

      {latency && histogram && histogram.length > 0 ? (
        <div className="grid gap-12 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] lg:gap-8">
          <Section title="Latency distribution" note="How many requests fell into each time range.">
            <Histogram buckets={histogram} latency={latency} />
          </Section>
          <Section title="Percentiles" note="Scaled to the slowest request.">
            <PercentileLadder latency={latency} />
          </Section>
        </div>
      ) : null}

      {requests && requests.length > 0 ? (
        <Section
          title="Every request over time"
          note={`${formatNumber(requests.length)} requests by start time. Look for drift, bursts or a slow warm-up.`}
        >
          <Timeline requests={requests} latency={latency} />
          <ScatterLegend guides={latency != null} />
        </Section>
      ) : null}

      {summary ? (
        <Section title="Status codes and errors">
          <StatusCodes summary={summary} />
        </Section>
      ) : null}

      {analysis && analysis.notes.length > 0 ? (
        <footer className="border-t border-rule pt-5">
          <ul className="flex max-w-prose flex-col gap-1.5 text-xs leading-relaxed text-ink-3">
            {analysis.notes.map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
        </footer>
      ) : null}

      <div className="flex flex-wrap items-center gap-4 border-t border-rule pt-6">
        {actions}
      </div>
    </article>
  );
}

function Section({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section className="min-w-0">
      <div className="mb-4">
        <h2 className="text-xl font-bold tracking-tight font-stretch-semi-expanded">{title}</h2>
        {note ? <p className="mt-1 text-sm text-ink-3">{note}</p> : null}
      </div>
      {children}
    </section>
  );
}
