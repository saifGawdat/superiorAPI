"use client";

import { useCallback, useEffect, useState } from "react";
import { Banner } from "@/components/Banner";
import { ConfigForm } from "@/components/ConfigForm";
import { RunView } from "@/components/RunView";
import { Button } from "@/components/ui";
import { API_URL, fetchLimits, isApiError, startTest } from "@/lib/api";
import {
  buildRequest,
  EXAMPLE_VALUES,
  INITIAL_VALUES,
  type FieldErrors,
  type FormValues,
} from "@/lib/form";
import {
  DEFAULT_LIMITS,
  type Limits,
  type StartTestRequest,
  type TestResult,
} from "@/lib/types";

type Phase =
  | { name: "config" }
  | { name: "running"; testId: string; request: StartTestRequest }
  | { name: "report"; result: TestResult; request: StartTestRequest };

interface Notice {
  title: string;
  message?: string;
}

export default function Home() {
  const [phase, setPhase] = useState<Phase>({ name: "config" });
  const [values, setValues] = useState<FormValues>(INITIAL_VALUES);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [notice, setNotice] = useState<Notice | null>(null);
  const [limits, setLimits] = useState<Limits>(DEFAULT_LIMITS);
  const [backendDown, setBackendDown] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const loadLimits = useCallback((signal?: AbortSignal) => {
    fetchLimits(signal)
      .then((l) => {
        setLimits(l);
        setBackendDown(false);
      })
      .catch((err) => {
        if (signal?.aborted) return;
        if (isApiError(err) && err.kind === "network") setBackendDown(true);
      });
  }, []);

  useEffect(() => {
    const ctrl = new AbortController();
    loadLimits(ctrl.signal);
    return () => ctrl.abort();
  }, [loadLimits]);

  const change = (patch: Partial<FormValues>) => {
    setValues((v) => ({ ...v, ...patch }));
    setErrors((prev) => {
      const next = { ...prev };
      for (const key of Object.keys(patch)) {
        if (key === "headersText") delete next.headers;
        else delete next[key as keyof FieldErrors];
      }
      return next;
    });
  };

  const start = async (request: StartTestRequest) => {
    setSubmitting(true);
    setNotice(null);
    try {
      const { testId } = await startTest(request);
      setBackendDown(false);
      setPhase({ name: "running", testId, request });
      window.scrollTo({ top: 0 });
    } catch (err) {
      setPhase({ name: "config" });
      if (!isApiError(err)) {
        setNotice({ title: "Something went wrong starting the test." });
      } else if (err.kind === "network") {
        setBackendDown(true);
      } else if (err.field) {
        setErrors({ [err.field]: err.message });
      } else if (err.status === 429) {
        setNotice({ title: "The profiler is busy right now", message: err.message });
      } else {
        setNotice({ title: "The test didn't start", message: err.message });
      }
    } finally {
      setSubmitting(false);
    }
  };

  const finishRun = useCallback((result: TestResult) => {
    setPhase((current) =>
      current.name === "running" ? { name: "report", result, request: current.request } : current,
    );
    window.scrollTo({ top: 0 });
  }, []);

  const backToConfig = useCallback(() => setPhase({ name: "config" }), []);

  const submit = () => {
    const { request, errors: found } = buildRequest(values, limits);
    setErrors(found);
    if (request) void start(request);
  };

  return (
    <div className="flex min-h-full flex-1 flex-col">
      <header className="border-b border-rule">
        <div className="mx-auto flex w-full max-w-6xl items-baseline justify-between gap-4 px-5 py-4 sm:px-8">
          <p className="text-lg font-bold tracking-tight font-stretch-expanded">superiorAPI</p>
          <p className="text-sm text-ink-3">API performance profiler</p>
        </div>
      </header>

      <main className="mx-auto w-full max-w-6xl flex-1 px-5 pt-10 pb-20 sm:px-8 sm:pt-14">
        {backendDown ? (
          <div className="mb-8">
            <Banner
              tone="error"
              title={`Can't reach the profiler backend at ${API_URL}`}
              action={
                <Button variant="secondary" onClick={() => loadLimits()}>
                  Check again
                </Button>
              }
            >
              Start the API server, or set <code className="font-semibold">NEXT_PUBLIC_API_URL</code>{" "}
              to its address and restart the web app.
            </Banner>
          </div>
        ) : null}

        {phase.name === "config" ? (
          <section aria-labelledby="config-title" className="max-w-3xl">
            <h1
              id="config-title"
              className="text-4xl leading-[1.05] font-bold tracking-tight font-stretch-expanded sm:text-5xl"
            >
              Profile an API endpoint
            </h1>
            <p className="mt-4 mb-10 max-w-prose text-lg leading-relaxed text-ink-2">
              We send real requests to your endpoint, plot every response as it lands, and tell
              you what the latency and errors say about it.
            </p>
            {notice ? (
              <div className="mb-8">
                <Banner tone="warning" title={notice.title}>
                  {notice.message}
                </Banner>
              </div>
            ) : null}
            <ConfigForm
              values={values}
              limits={limits}
              errors={errors}
              submitting={submitting}
              onChange={change}
              onSubmit={submit}
              onExample={() => change(EXAMPLE_VALUES)}
            />
          </section>
        ) : phase.name === "running" ? (
          <RunView
            key={phase.testId}
            testId={phase.testId}
            request={phase.request}
            onDone={finishRun}
            onBack={backToConfig}
          />
        ) : (
          <p>Report for {phase.result.id}</p>
        )}
      </main>
    </div>
  );
}
