import type { Finding } from "@/lib/types";
import { severityTone } from "./severity";

const ORDER = { critical: 0, warning: 1, info: 2, good: 3 } as const;

/**
 * Each finding separates what was measured (fact) from what might explain it
 * (possibilities) and what to try next.
 */
export function Findings({ findings }: { findings: Finding[] }) {
  if (findings.length === 0) {
    return <p className="text-ink-2">No findings. Nothing in this run stood out.</p>;
  }
  const sorted = [...findings].sort(
    (a, b) => (ORDER[a.severity] ?? 2) - (ORDER[b.severity] ?? 2),
  );
  return (
    <ul className="flex flex-col gap-4">
      {sorted.map((f) => (
        <FindingItem key={f.id} finding={f} />
      ))}
    </ul>
  );
}

function FindingItem({ finding }: { finding: Finding }) {
  const tone = severityTone(finding.severity);
  const hasCauses = finding.possibleCauses.length > 0;
  const hasMore = hasCauses || Boolean(finding.suggestion);

  return (
    <li className="rounded-lg border border-rule bg-panel">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 px-5 pt-4">
        <span
          className={`inline-flex items-center gap-1.5 rounded-sm px-1.5 py-0.5 text-xs font-bold ${tone.wash} ${tone.text}`}
        >
          <span aria-hidden>{tone.glyph}</span>
          {tone.label}
        </span>
        <h3 className="text-lg font-semibold text-ink">{finding.title}</h3>
      </div>

      <div
        className={`grid gap-x-8 gap-y-4 px-5 pt-3 pb-5 ${
          hasMore ? "md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]" : ""
        }`}
      >
        <div>
          <p className="text-sm font-semibold text-ink-2">Observed</p>
          <p className="mt-1 text-[1.05rem] leading-relaxed text-ink">{finding.observation}</p>
        </div>

        {hasMore ? (
          <div className="flex flex-col gap-4 md:border-l md:border-dashed md:border-rule md:pl-8">
            {hasCauses ? (
              <div>
                <p className="text-sm font-semibold text-ink-2">Possible causes</p>
                <p className="text-xs text-ink-3">Possibilities to check, not a diagnosis.</p>
                <ul className="mt-2 flex flex-col gap-1.5 text-ink-2">
                  {finding.possibleCauses.map((cause) => (
                    <li key={cause} className="flex gap-2 leading-snug">
                      <span aria-hidden className="mt-1.5 size-2 shrink-0 rounded-full border border-ink-3" />
                      <span>{cause}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {finding.suggestion ? (
              <div className="rounded-md bg-pen-wash/70 px-3 py-2.5">
                <p className="text-sm font-semibold text-pen-deep">Try this</p>
                <p className="mt-0.5 leading-snug text-ink">{finding.suggestion}</p>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </li>
  );
}
