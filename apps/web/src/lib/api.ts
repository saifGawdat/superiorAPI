import {
  DEFAULT_LIMITS,
  type ApiErrorBody,
  type ErrorField,
  type Limits,
  type StartTestRequest,
  type StartTestResponse,
  type TestResult,
} from "./types";

export const API_URL = (
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"
).replace(/\/+$/, "");

/**
 * A failed API call. `kind: "network"` means the backend could not be reached
 * at all (server down, wrong NEXT_PUBLIC_API_URL, CORS failure).
 */
export class ApiError extends Error {
  readonly kind: "network" | "http";
  readonly status: number;
  readonly code: string;
  readonly field?: ErrorField;

  constructor(opts: {
    kind: "network" | "http";
    status: number;
    code: string;
    message: string;
    field?: ErrorField;
  }) {
    super(opts.message);
    this.name = "ApiError";
    this.kind = opts.kind;
    this.status = opts.status;
    this.code = opts.code;
    this.field = opts.field;
  }
}

export function isApiError(err: unknown): err is ApiError {
  return err instanceof ApiError;
}

function isErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== "object" || value === null) return false;
  const inner = (value as { error?: unknown }).error;
  return (
    typeof inner === "object" &&
    inner !== null &&
    typeof (inner as { message?: unknown }).message === "string"
  );
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, {
      ...init,
      headers: {
        Accept: "application/json",
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
      cache: "no-store",
    });
  } catch {
    throw new ApiError({
      kind: "network",
      status: 0,
      code: "network_error",
      message: `Can't reach the profiler backend at ${API_URL}.`,
    });
  }

  let data: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }

  if (!res.ok) {
    if (isErrorBody(data)) {
      throw new ApiError({
        kind: "http",
        status: res.status,
        code: data.error.code,
        message: data.error.message,
        field: data.error.field,
      });
    }
    throw new ApiError({
      kind: "http",
      status: res.status,
      code: "http_error",
      message: `The backend answered with HTTP ${res.status}.`,
    });
  }

  return data as T;
}

export async function fetchLimits(signal?: AbortSignal): Promise<Limits> {
  const data = await request<Partial<Limits>>("/api/limits", { signal });
  return { ...DEFAULT_LIMITS, ...data };
}

export function startTest(body: StartTestRequest): Promise<StartTestResponse> {
  return request<StartTestResponse>("/api/tests", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function getTest(id: string, signal?: AbortSignal): Promise<TestResult> {
  return request<TestResult>(`/api/tests/${encodeURIComponent(id)}`, { signal });
}

export async function cancelTest(id: string): Promise<void> {
  await request<unknown>(`/api/tests/${encodeURIComponent(id)}/cancel`, {
    method: "POST",
  });
}

export function eventsUrl(id: string): string {
  return `${API_URL}/api/tests/${encodeURIComponent(id)}/events`;
}
