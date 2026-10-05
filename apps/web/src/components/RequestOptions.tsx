"use client";

import type { FieldErrors, FormValues } from "@/lib/form";
import { formatNumber } from "@/lib/format";
import { methodAllowsBody, type Limits } from "@/lib/types";
import { FieldError } from "./ui";

interface Props {
  values: FormValues;
  limits: Limits;
  errors: FieldErrors;
  onChange: (patch: Partial<FormValues>) => void;
}

const textarea =
  "block w-full rounded-md border border-rule bg-white px-3 py-2.5 text-[0.95rem] leading-relaxed text-ink outline-none transition-colors placeholder:text-ink-3/70 focus:border-pen focus-visible:outline-none aria-[invalid=true]:border-fail";

export function RequestOptions({ values, limits, errors, onChange }: Props) {
  const bodyAllowed = methodAllowsBody(values.method);

  return (
    <div className="grid gap-8 md:grid-cols-2">
      <div className={bodyAllowed ? "" : "md:col-span-2"}>
        <label htmlFor="headers" className="mb-2 block text-sm font-semibold text-ink-2">
          Headers <span className="font-normal text-ink-3">(optional)</span>
        </label>
        <textarea
          id="headers"
          rows={4}
          spellCheck={false}
          autoComplete="off"
          placeholder={"Authorization: Bearer <token>\nAccept: application/json"}
          value={values.headersText}
          onChange={(e) => onChange({ headersText: e.target.value })}
          aria-invalid={Boolean(errors.headers)}
          aria-describedby={`headers-help${errors.headers ? " headers-error" : ""}`}
          className={textarea}
        />
        <FieldError id="headers-error" message={errors.headers} />
        <p id="headers-help" className="mt-2 max-w-prose text-sm leading-relaxed text-ink-3">
          One per line as <span className="text-ink-2">Key: Value</span>. Headers and tokens go
          to our server only to make these requests. They are never stored, and the report never
          shows their values.
        </p>
      </div>

      {bodyAllowed ? (
        <div>
          <label htmlFor="body" className="mb-2 block text-sm font-semibold text-ink-2">
            Request body <span className="font-normal text-ink-3">(optional)</span>
          </label>
          <textarea
            id="body"
            rows={4}
            spellCheck={false}
            placeholder={'{"name": "Ada"}'}
            value={values.body}
            onChange={(e) => onChange({ body: e.target.value })}
            aria-invalid={Boolean(errors.body)}
            aria-describedby={`body-help${errors.body ? " body-error" : ""}`}
            className={textarea}
          />
          <FieldError id="body-error" message={errors.body} />
          <p id="body-help" className="mt-2 text-sm text-ink-3">
            Sent as-is with every request. Remember to add a Content-Type header. Up to{" "}
            {formatNumber(Math.round(limits.maxBodyBytes / 1024))} KB.
          </p>
        </div>
      ) : null}

      <details
        className="group md:col-span-2"
        open={Boolean(errors.timeoutMs) || undefined}
      >
        <summary className="inline-flex cursor-pointer list-none items-center gap-2 rounded-sm text-sm font-semibold text-ink-2 hover:text-ink [&::-webkit-details-marker]:hidden">
          <svg
            aria-hidden
            viewBox="0 0 12 12"
            className="size-3 transition-transform group-open:rotate-90"
          >
            <path d="M4 2l4 4-4 4" fill="none" stroke="currentColor" strokeWidth="1.8" />
          </svg>
          Advanced
        </summary>
        <div className="mt-4 flex flex-wrap items-end gap-3 pl-5">
          <div>
            <label htmlFor="timeoutMs" className="mb-1.5 block text-sm text-ink-2">
              Timeout per request
            </label>
            <div className="flex items-center gap-2">
              <input
                id="timeoutMs"
                type="text"
                inputMode="numeric"
                placeholder={String(limits.defaultTimeoutMs)}
                value={values.timeoutMs}
                onChange={(e) => onChange({ timeoutMs: e.target.value })}
                aria-invalid={Boolean(errors.timeoutMs)}
                aria-describedby={`timeout-help${errors.timeoutMs ? " timeout-error" : ""}`}
                className="w-28 rounded-md border border-rule bg-white px-3 py-2 tabular-nums text-ink outline-none focus:border-pen focus-visible:outline-none aria-[invalid=true]:border-fail"
              />
              <span className="text-sm text-ink-3">ms</span>
            </div>
          </div>
          <p id="timeout-help" className="pb-2 text-sm text-ink-3">
            Default {formatNumber(limits.defaultTimeoutMs)} ms, up to{" "}
            {formatNumber(limits.maxTimeoutMs)} ms. Slower requests count as failed.
          </p>
          <div className="basis-full">
            <FieldError id="timeout-error" message={errors.timeoutMs} />
          </div>
        </div>
      </details>
    </div>
  );
}
