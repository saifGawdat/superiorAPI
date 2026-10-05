// Contract types for the superiorAPI backend. Field names match the JSON
// exactly; all durations are milliseconds (float, 0.1 precision).

export const HTTP_METHODS = [
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "HEAD",
  "OPTIONS",
] as const;

export type HttpMethod = (typeof HTTP_METHODS)[number];

/** Methods for which the backend accepts a request body. */
export const BODY_METHODS: readonly HttpMethod[] = ["POST", "PUT", "PATCH", "DELETE"];

export function methodAllowsBody(method: HttpMethod): boolean {
  return BODY_METHODS.includes(method);
}

export interface StartTestRequest {
  url: string;
  method: HttpMethod;
  headers: Record<string, string>;
  body: string | null;
  requests: number;
  concurrency: number;
  timeoutMs?: number;
}

export interface StartTestResponse {
  testId: string;
}

export type ErrorField =
  | "url"
  | "method"
  | "headers"
  | "body"
  | "requests"
  | "concurrency"
  | "timeoutMs";

export type ApiErrorCode =
  | "invalid_request"
  | "target_not_allowed"
  | "dns_failed"
  | "rate_limited"
  | "not_found"
  | (string & {});

export interface ApiErrorBody {
  error: {
    code: ApiErrorCode;
    message: string;
    field?: ErrorField;
  };
}

export interface Limits {
  maxRequests: number;
  maxConcurrency: number;
  maxTimeoutMs: number;
  defaultTimeoutMs: number;
  maxBodyBytes: number;
  probeRegion: string;
}

export const DEFAULT_LIMITS: Limits = {
  maxRequests: 500,
  maxConcurrency: 50,
  maxTimeoutMs: 30000,
  defaultTimeoutMs: 10000,
  maxBodyBytes: 65536,
  probeRegion: "local",
};

export type TransportError =
  | "timeout"
  | "connection_refused"
  | "connection_reset"
  | "dns"
  | "tls"
  | "blocked"
  | "too_many_redirects"
  | "other"
  | (string & {});

export interface RequestSample {
  n: number;
  startOffsetMs: number;
  durationMs: number;
  /** 0 means no HTTP response was received; see `error`. */
  status: number;
  ok: boolean;
  bytes: number;
  error?: TransportError;
}

export interface Progress {
  completed: number;
  succeeded: number;
  failed: number;
  total: number;
  elapsedMs: number;
  p50: number | null;
  p95: number | null;
  /** Up to 12 most recently completed requests, newest first. */
  recent: RequestSample[];
}

export type TestStatus = "running" | "completed" | "cancelled";

export interface TestConfig {
  url: string;
  method: HttpMethod;
  requests: number;
  concurrency: number;
  timeoutMs: number;
  headerNames: string[];
  hasBody: boolean;
}

export interface LatencyStats {
  min: number;
  max: number;
  avg: number;
  p50: number;
  p90: number;
  p95: number;
  p99: number;
}

export interface Summary {
  totalRequests: number;
  completedRequests: number;
  successfulRequests: number;
  failedRequests: number;
  /** Fraction 0..1. */
  errorRate: number;
  durationMs: number;
  throughput: number;
  latency: LatencyStats | null;
  latencySampleSize: number;
  statusCodes: Record<string, number>;
  errorTypes: Record<string, number>;
  bytesReceived: number;
  slowestRequest: { n: number; durationMs: number; status: number } | null;
}

export interface HistogramBucket {
  fromMs: number;
  toMs: number;
  count: number;
}

export type Verdict = "healthy" | "info" | "warning" | "critical";
export type Severity = "good" | "info" | "warning" | "critical";

export interface Finding {
  id: string;
  severity: Severity;
  title: string;
  observation: string;
  possibleCauses: string[];
  suggestion?: string;
}

export interface Analysis {
  verdict: Verdict;
  headline: string;
  findings: Finding[];
  notes: string[];
}

export interface TestResult {
  id: string;
  status: TestStatus;
  config: TestConfig;
  probeRegion: string;
  startedAt: string;
  finishedAt?: string;
  progress: Progress;
  summary?: Summary;
  histogram?: HistogramBucket[];
  analysis?: Analysis;
  requests?: RequestSample[];
}
