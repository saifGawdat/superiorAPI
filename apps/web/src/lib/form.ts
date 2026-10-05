import {
  methodAllowsBody,
  type ErrorField,
  type HttpMethod,
  type Limits,
  type StartTestRequest,
} from "./types";

/** Raw form state. Numbers stay strings so inputs can be edited freely. */
export interface FormValues {
  url: string;
  method: HttpMethod;
  requests: string;
  concurrency: string;
  timeoutMs: string;
  headersText: string;
  body: string;
}

export type FieldErrors = Partial<Record<ErrorField, string>>;

export const INITIAL_VALUES: FormValues = {
  url: "",
  method: "GET",
  requests: "100",
  concurrency: "10",
  timeoutMs: "",
  headersText: "",
  body: "",
};

export const EXAMPLE_VALUES: Partial<FormValues> = {
  url: "https://jsonplaceholder.typicode.com/posts/1",
  method: "GET",
  requests: "50",
  concurrency: "5",
};

const HEADER_NAME = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/;

/** Parses `Key: Value` lines. Blank lines are ignored. */
export function parseHeaders(text: string): {
  headers: Record<string, string>;
  error?: string;
} {
  const headers: Record<string, string> = {};
  const lines = text.split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line) continue;
    const colon = line.indexOf(":");
    if (colon === -1) {
      return { headers, error: `Line ${i + 1} needs a colon, like "Accept: application/json".` };
    }
    const name = line.slice(0, colon).trim();
    const value = line.slice(colon + 1).trim();
    if (!name) {
      return { headers, error: `Line ${i + 1} is missing a header name before the colon.` };
    }
    if (!HEADER_NAME.test(name)) {
      return { headers, error: `Line ${i + 1}: "${name}" isn't a valid header name.` };
    }
    headers[name] = value;
  }
  return { headers };
}

function parseInteger(raw: string): number | null {
  const trimmed = raw.trim();
  if (!/^\d+$/.test(trimmed)) return null;
  return Number(trimmed);
}

function byteLength(text: string): number {
  return new TextEncoder().encode(text).length;
}

/** Client-side validation mirroring the backend rules. */
export function buildRequest(
  values: FormValues,
  limits: Limits,
): { request: StartTestRequest | null; errors: FieldErrors } {
  const errors: FieldErrors = {};

  const url = values.url.trim();
  if (!url) {
    errors.url = "Enter the endpoint URL to test.";
  } else {
    try {
      const parsed = new URL(url);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
        errors.url = "Use an http:// or https:// URL.";
      }
    } catch {
      errors.url = "That doesn't look like a full URL. Include https://.";
    }
  }

  const requests = parseInteger(values.requests);
  if (requests == null || requests < 1 || requests > limits.maxRequests) {
    errors.requests = `Choose between 1 and ${limits.maxRequests} requests.`;
  }

  const concurrency = parseInteger(values.concurrency);
  if (concurrency == null || concurrency < 1 || concurrency > limits.maxConcurrency) {
    errors.concurrency = `Choose between 1 and ${limits.maxConcurrency} at a time.`;
  }

  let timeoutMs: number | undefined;
  if (values.timeoutMs.trim()) {
    const t = parseInteger(values.timeoutMs);
    if (t == null || t < 1 || t > limits.maxTimeoutMs) {
      errors.timeoutMs = `Choose a timeout between 1 and ${limits.maxTimeoutMs} ms.`;
    } else {
      timeoutMs = t;
    }
  }

  const { headers, error: headerError } = parseHeaders(values.headersText);
  if (headerError) errors.headers = headerError;

  let body: string | null = null;
  if (methodAllowsBody(values.method) && values.body !== "") {
    if (byteLength(values.body) > limits.maxBodyBytes) {
      errors.body = `The body is larger than the ${Math.round(limits.maxBodyBytes / 1024)} KB limit.`;
    } else {
      body = values.body;
    }
  }

  if (Object.keys(errors).length > 0 || requests == null || concurrency == null) {
    return { request: null, errors };
  }

  return {
    request: {
      url,
      method: values.method,
      headers,
      body,
      requests,
      concurrency,
      ...(timeoutMs != null ? { timeoutMs } : {}),
    },
    errors,
  };
}
