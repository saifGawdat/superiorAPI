"use client";

import { useEffect, useReducer } from "react";
import { eventsUrl, getTest, isApiError } from "./api";
import type { Progress, RequestSample, TestResult } from "./types";

const POLL_INTERVAL_MS = 700;
const POLL_MAX_BACKOFF_MS = 5000;
const FEED_SIZE = 12;

export type Transport = "connecting" | "stream" | "polling";

export interface RunState {
  transport: Transport;
  progress: Progress | null;
  /** performance.now() when the latest progress arrived (for clock smoothing). */
  receivedAt: number | null;
  /** Every request seen so far via `recent`, keyed by `n`. */
  samples: ReadonlyMap<number, RequestSample>;
  /** Latest requests, newest first, in the order they arrived. */
  feed: RequestSample[];
  result: TestResult | null;
  /** Transient connection trouble; the hook keeps retrying. */
  connectionIssue: string | null;
  /** Unrecoverable, e.g. the test no longer exists. */
  fatal: string | null;
}

type Action =
  | { type: "transport"; transport: Transport }
  | { type: "progress"; progress: Progress; at: number }
  | { type: "done"; result: TestResult; at: number }
  | { type: "issue"; message: string | null }
  | { type: "fatal"; message: string };

const initialState: RunState = {
  transport: "connecting",
  progress: null,
  receivedAt: null,
  samples: new Map(),
  feed: [],
  result: null,
  connectionIssue: null,
  fatal: null,
};

function mergeRecent(state: RunState, progress: Progress) {
  const fresh = (progress.recent ?? []).filter((r) => !state.samples.has(r.n));
  if (fresh.length === 0) return { samples: state.samples, feed: state.feed };
  const samples = new Map(state.samples);
  for (const r of fresh) samples.set(r.n, r);
  // `recent` is newest first, so fresh items keep their order on top.
  const feed = [...fresh, ...state.feed].slice(0, FEED_SIZE);
  return { samples, feed };
}

function reducer(state: RunState, action: Action): RunState {
  switch (action.type) {
    case "transport":
      return { ...state, transport: action.transport };
    case "progress": {
      if (state.result) return state;
      return {
        ...state,
        ...mergeRecent(state, action.progress),
        progress: action.progress,
        receivedAt: action.at,
        connectionIssue: null,
      };
    }
    case "done": {
      const progress = action.result.progress ?? state.progress;
      const merged = progress ? mergeRecent(state, progress) : state;
      // `recent` only holds the latest 12, so fast runs skip some requests
      // live. The final result has all of them: fill in the gaps.
      let samples = merged.samples;
      const all = action.result.requests ?? [];
      if (all.some((r) => !samples.has(r.n))) {
        const filled = new Map(samples);
        for (const r of all) if (!filled.has(r.n)) filled.set(r.n, r);
        samples = filled;
      }
      return {
        ...state,
        samples,
        feed: merged.feed,
        progress,
        receivedAt: action.at,
        result: action.result,
        connectionIssue: null,
      };
    }
    case "issue":
      return { ...state, connectionIssue: action.message };
    case "fatal":
      return { ...state, fatal: action.message };
  }
}

function parse<T>(raw: unknown): T | null {
  if (typeof raw !== "string") return null;
  try {
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

/**
 * Follows a running test: Server-Sent Events first, falling back to polling
 * GET /api/tests/{id} if the stream errors before `done`.
 * Mount with `key={testId}` so a new test starts from a clean state.
 */
export function useTestRun(testId: string): RunState {
  const [state, dispatch] = useReducer(reducer, initialState);

  useEffect(() => {
    let closed = false;
    let finished = false;
    let source: EventSource | null = null;
    let pollTimer: ReturnType<typeof setTimeout> | undefined;
    let failures = 0;
    const abort = new AbortController();

    const finish = (result: TestResult) => {
      finished = true;
      source?.close();
      dispatch({ type: "done", result, at: performance.now() });
    };

    const poll = async () => {
      if (closed || finished) return;
      try {
        const result = await getTest(testId, abort.signal);
        if (closed) return;
        failures = 0;
        if (result.status === "running") {
          if (result.progress) {
            dispatch({ type: "progress", progress: result.progress, at: performance.now() });
          }
          dispatch({ type: "issue", message: null });
          pollTimer = setTimeout(poll, POLL_INTERVAL_MS);
        } else {
          finish(result);
        }
      } catch (err) {
        if (closed) return;
        if (isApiError(err) && err.status === 404) {
          dispatch({
            type: "fatal",
            message: "This test is no longer available on the server. It may have expired.",
          });
          return;
        }
        failures += 1;
        dispatch({
          type: "issue",
          message: isApiError(err)
            ? `${err.message} Retrying…`
            : "Lost contact with the profiler. Retrying…",
        });
        const delay = Math.min(POLL_INTERVAL_MS * 2 ** failures, POLL_MAX_BACKOFF_MS);
        pollTimer = setTimeout(poll, delay);
      }
    };

    const startPolling = () => {
      if (closed || finished) return;
      dispatch({ type: "transport", transport: "polling" });
      void poll();
    };

    if (typeof EventSource === "undefined") {
      startPolling();
    } else {
      source = new EventSource(eventsUrl(testId));
      source.addEventListener("open", () => {
        dispatch({ type: "transport", transport: "stream" });
      });
      source.addEventListener("progress", (e) => {
        const progress = parse<Progress>((e as MessageEvent).data);
        if (progress) dispatch({ type: "progress", progress, at: performance.now() });
      });
      source.addEventListener("done", (e) => {
        const result = parse<TestResult>((e as MessageEvent).data);
        source?.close();
        if (result) finish(result);
        else startPolling();
      });
      source.addEventListener("error", () => {
        if (finished || closed) return;
        source?.close();
        startPolling();
      });
    }

    return () => {
      closed = true;
      source?.close();
      clearTimeout(pollTimer);
      abort.abort();
    };
  }, [testId]);

  return state;
}
