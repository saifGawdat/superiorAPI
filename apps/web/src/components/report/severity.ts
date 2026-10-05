import type { Severity, Verdict } from "@/lib/types";

export interface Tone {
  label: string;
  glyph: string;
  /** Solid mark (glyph disc, rule). */
  solid: string;
  /** Text colour that passes contrast on paper. */
  text: string;
  /** Soft background wash. */
  wash: string;
  border: string;
}

const GOOD: Tone = {
  label: "Good",
  glyph: "✓",
  solid: "bg-good",
  text: "text-good-ink",
  wash: "bg-good-wash",
  border: "border-good",
};
const INFO: Tone = {
  label: "Note",
  glyph: "i",
  solid: "bg-pen",
  text: "text-pen-deep",
  wash: "bg-pen-wash",
  border: "border-pen",
};
const WARNING: Tone = {
  label: "Warning",
  glyph: "!",
  solid: "bg-warn",
  text: "text-warn-ink",
  wash: "bg-warn-wash",
  border: "border-warn",
};
const CRITICAL: Tone = {
  label: "Critical",
  glyph: "✕",
  solid: "bg-fail",
  text: "text-fail-ink",
  wash: "bg-fail-wash",
  border: "border-fail",
};

export function severityTone(severity: Severity): Tone {
  switch (severity) {
    case "good":
      return GOOD;
    case "warning":
      return WARNING;
    case "critical":
      return CRITICAL;
    default:
      return INFO;
  }
}

export function verdictTone(verdict: Verdict): Tone {
  switch (verdict) {
    case "healthy":
      return { ...GOOD, label: "Healthy" };
    case "warning":
      return { ...WARNING, label: "Needs attention" };
    case "critical":
      return { ...CRITICAL, label: "Critical" };
    default:
      return { ...INFO, label: "Worth a look" };
  }
}
