import type { ReactNode } from "react";

type Tone = "error" | "warning" | "info";

const TONES: Record<Tone, { box: string; mark: string; glyph: string }> = {
  error: { box: "border-fail/40 bg-fail-wash", mark: "bg-fail text-white", glyph: "!" },
  warning: { box: "border-warn/40 bg-warn-wash", mark: "bg-warn text-white", glyph: "!" },
  info: { box: "border-pen/30 bg-pen-wash", mark: "bg-pen text-white", glyph: "i" },
};

export function Banner({
  tone,
  title,
  children,
  action,
}: {
  tone: Tone;
  title: string;
  children?: ReactNode;
  action?: ReactNode;
}) {
  const t = TONES[tone];
  return (
    <div
      role={tone === "error" ? "alert" : "status"}
      className={`flex flex-wrap items-start gap-3 rounded-md border px-4 py-3 ${t.box}`}
    >
      <span
        aria-hidden
        className={`mt-0.5 grid size-5 shrink-0 place-items-center rounded-full text-xs font-bold ${t.mark}`}
      >
        {t.glyph}
      </span>
      <div className="min-w-0 flex-1 text-sm leading-relaxed">
        <p className="font-semibold text-ink">{title}</p>
        {children ? <div className="text-ink-2">{children}</div> : null}
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}
