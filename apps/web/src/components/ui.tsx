import type { ButtonHTMLAttributes } from "react";
import { methodBg } from "@/lib/methods";

export function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null;
  return (
    <p id={id} className="mt-1.5 text-sm font-medium text-fail-ink">
      {message}
    </p>
  );
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "quiet";
};

const VARIANTS = {
  primary:
    "bg-pen text-white hover:bg-pen-deep disabled:bg-ink-3 px-5 py-3 text-base font-semibold",
  secondary:
    "border border-rule bg-panel text-ink hover:border-ink-3 hover:bg-white px-4 py-2.5 text-sm font-semibold",
  quiet: "text-pen-deep underline-offset-4 hover:underline px-1 py-1 text-sm font-semibold",
} as const;

export function Button({ variant = "primary", className = "", ...props }: ButtonProps) {
  return (
    <button
      type="button"
      {...props}
      className={`inline-flex items-center justify-center gap-2 rounded-md transition-colors disabled:cursor-not-allowed ${VARIANTS[variant]} ${className}`}
    />
  );
}

export function MethodTag({ method }: { method: string }) {
  return (
    <span
      className={`rounded-sm px-1.5 py-0.5 text-xs font-bold tracking-wide text-white font-stretch-semi-expanded ${methodBg(method)}`}
    >
      {method}
    </span>
  );
}
