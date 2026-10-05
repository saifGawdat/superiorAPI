"use client";

import { methodBg } from "@/lib/methods";
import { HTTP_METHODS, type HttpMethod } from "@/lib/types";

interface Props {
  value: HttpMethod;
  onChange: (method: HttpMethod) => void;
  invalid?: boolean;
  describedBy?: string;
}

/**
 * Segmented control for the HTTP method. Native radio buttons give arrow-key
 * navigation and screen reader support; a colored indicator glides behind the
 * selected segment.
 */
export function MethodPicker({ value, onChange, invalid, describedBy }: Props) {
  const index = Math.max(HTTP_METHODS.indexOf(value), 0);

  return (
    <div
      role="radiogroup"
      aria-label="HTTP method"
      aria-invalid={invalid || undefined}
      aria-describedby={describedBy}
      className="relative grid w-full grid-cols-5 rounded-full border border-rule bg-white p-1 shadow-sm sm:w-[31rem]"
    >
      <span
        aria-hidden
        className={`pointer-events-none absolute inset-y-1 left-1 w-[calc((100%-0.5rem)/5)] rounded-full shadow-md transition-[translate,background-color] duration-300 ease-out motion-reduce:transition-none ${methodBg(value)}`}
        style={{ translate: `${index * 100}% 0` }}
      />
      {HTTP_METHODS.map((m) => {
        const selected = m === value;
        return (
          <label
            key={m}
            className={`relative flex cursor-pointer items-center justify-center gap-1.5 rounded-full px-1 py-2 text-xs font-bold tracking-normal sm:tracking-wide sm:font-stretch-semi-expanded transition-colors duration-300 select-none has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-pen has-[:focus-visible]:ring-offset-2 motion-reduce:transition-none sm:text-sm ${
              selected ? "text-white" : "text-ink-2 hover:text-ink"
            }`}
          >
            <input
              type="radio"
              name="method"
              value={m}
              checked={selected}
              onChange={() => onChange(m)}
              className="sr-only"
            />
            <span
              aria-hidden
              className={`hidden size-1.5 shrink-0 rounded-full transition-colors duration-300 sm:block ${
                selected ? "bg-white/80" : methodBg(m)
              }`}
            />
            {m}
          </label>
        );
      })}
    </div>
  );
}
