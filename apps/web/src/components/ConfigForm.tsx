"use client";

import type { FormEvent } from "react";
import type { FieldErrors, FormValues } from "@/lib/form";
import { formatNumber } from "@/lib/format";
import type { Limits } from "@/lib/types";
import { MethodPicker } from "./MethodPicker";
import { RequestOptions } from "./RequestOptions";
import { Button, FieldError } from "./ui";

interface Props {
  values: FormValues;
  limits: Limits;
  errors: FieldErrors;
  submitting: boolean;
  onChange: (patch: Partial<FormValues>) => void;
  onSubmit: () => void;
  onExample: () => void;
}

function describedBy(...ids: (string | false | undefined)[]) {
  const list = ids.filter(Boolean).join(" ");
  return list || undefined;
}

export function ConfigForm({
  values,
  limits,
  errors,
  submitting,
  onChange,
  onSubmit,
  onExample,
}: Props) {
  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit();
  };

  const countInput =
    "w-[5.5ch] rounded-none border-0 border-b-2 border-ink/70 bg-transparent px-0.5 pb-0.5 text-center text-2xl font-semibold text-pen-deep font-stretch-expanded tabular-nums outline-none transition-colors focus:border-pen focus-visible:outline-none aria-[invalid=true]:border-fail sm:text-3xl";

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-10">
      <div>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <label htmlFor="url" className="text-sm font-semibold text-ink-2">
            Endpoint
          </label>
          <MethodPicker
            value={values.method}
            onChange={(method) => onChange({ method })}
            invalid={Boolean(errors.method)}
            describedBy={describedBy(errors.method && "method-error")}
          />
        </div>
        <div
          className={`overflow-hidden rounded-xl border-2 bg-white shadow-sm transition-colors focus-within:border-pen ${
            errors.url ? "border-fail" : "border-ink"
          }`}
        >
          <input
            id="url"
            type="url"
            inputMode="url"
            autoComplete="url"
            spellCheck={false}
            placeholder="https://api.example.com/users"
            value={values.url}
            onChange={(e) => onChange({ url: e.target.value })}
            aria-invalid={Boolean(errors.url)}
            aria-describedby={describedBy(errors.url && "url-error")}
            className="w-full bg-transparent px-4 py-4 text-lg text-ink outline-none placeholder:text-ink-3/70 focus-visible:outline-none sm:text-xl"
          />
        </div>
        <FieldError id="url-error" message={errors.url} />
        <FieldError id="method-error" message={errors.method} />
        <p className="mt-2 text-sm text-ink-3">
          No endpoint handy?{" "}
          <Button variant="quiet" onClick={onExample} className="!px-0">
            Try an example
          </Button>
        </p>
      </div>

      <fieldset>
        <legend className="mb-3 text-sm font-semibold text-ink-2">Load</legend>
        <p className="flex flex-wrap items-baseline gap-x-3 gap-y-4 text-2xl text-ink sm:text-3xl">
          <span>Send</span>
          <span className="inline-flex flex-col items-center">
            <input
              id="requests"
              type="text"
              inputMode="numeric"
              aria-label="Number of requests"
              value={values.requests}
              onChange={(e) => onChange({ requests: e.target.value })}
              aria-invalid={Boolean(errors.requests)}
              aria-describedby={describedBy("requests-hint", errors.requests && "requests-error")}
              className={countInput}
            />
            <span id="requests-hint" className="mt-1 text-xs text-ink-3">
              up to {formatNumber(limits.maxRequests)}
            </span>
          </span>
          <span>requests,</span>
          <span className="inline-flex flex-col items-center">
            <input
              id="concurrency"
              type="text"
              inputMode="numeric"
              aria-label="Concurrent requests"
              value={values.concurrency}
              onChange={(e) => onChange({ concurrency: e.target.value })}
              aria-invalid={Boolean(errors.concurrency)}
              aria-describedby={describedBy(
                "concurrency-hint",
                errors.concurrency && "concurrency-error",
              )}
              className={countInput}
            />
            <span id="concurrency-hint" className="mt-1 text-xs text-ink-3">
              up to {limits.maxConcurrency}
            </span>
          </span>
          <span>at a time.</span>
        </p>
        <FieldError id="requests-error" message={errors.requests} />
        <FieldError id="concurrency-error" message={errors.concurrency} />
      </fieldset>

      <RequestOptions values={values} limits={limits} errors={errors} onChange={onChange} />

      <div className="flex flex-wrap items-center gap-4 border-t border-rule pt-6">
        <Button type="submit" disabled={submitting} className="min-w-44">
          {submitting ? "Starting…" : "Start test"}
        </Button>
        <p className="text-sm text-ink-3">
          Requests are sent from our profiler server (region: {limits.probeRegion}).
        </p>
      </div>
    </form>
  );
}
