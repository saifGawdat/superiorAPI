import { errorLabel, formatNumber, formatPercent } from "@/lib/format";
import type { Summary } from "@/lib/types";

const REASONS: Record<string, string> = {
  "200": "OK",
  "201": "Created",
  "202": "Accepted",
  "204": "No Content",
  "301": "Moved Permanently",
  "302": "Found",
  "304": "Not Modified",
  "307": "Temporary Redirect",
  "308": "Permanent Redirect",
  "400": "Bad Request",
  "401": "Unauthorized",
  "403": "Forbidden",
  "404": "Not Found",
  "405": "Method Not Allowed",
  "408": "Request Timeout",
  "409": "Conflict",
  "413": "Payload Too Large",
  "422": "Unprocessable Content",
  "429": "Too Many Requests",
  "500": "Internal Server Error",
  "502": "Bad Gateway",
  "503": "Service Unavailable",
  "504": "Gateway Timeout",
};

function codeColor(code: number): string {
  if (code >= 500) return "bg-fail";
  if (code >= 400) return "bg-warn";
  if (code >= 300) return "bg-neutral";
  return "bg-good";
}

interface Row {
  key: string;
  label: string;
  detail: string;
  count: number;
  bar: string;
  striped?: boolean;
}

export function StatusCodes({ summary }: { summary: Summary }) {
  const denominator = Math.max(1, summary.completedRequests);
  const http: Row[] = Object.entries(summary.statusCodes)
    .sort(([a], [b]) => Number(a) - Number(b))
    .map(([code, count]) => ({
      key: code,
      label: code,
      detail: REASONS[code] ?? "",
      count,
      bar: codeColor(Number(code)),
    }));
  const transport: Row[] = Object.entries(summary.errorTypes)
    .sort(([, a], [, b]) => b - a)
    .map(([type, count]) => ({
      key: type,
      label: errorLabel(type),
      detail: "",
      count,
      bar: "bg-fail",
      striped: true,
    }));
  const rateLimited = summary.statusCodes["429"] ?? 0;

  return (
    <div className="flex flex-col gap-5 rounded-lg border border-rule bg-panel px-5 py-5">
      {http.length > 0 ? (
        <RowGroup title="HTTP responses" rows={http} denominator={denominator} />
      ) : null}
      {transport.length > 0 ? (
        <RowGroup title="No HTTP response" rows={transport} denominator={denominator} />
      ) : null}
      {http.length === 0 && transport.length === 0 ? (
        <p className="text-sm text-ink-2">No requests completed.</p>
      ) : null}
      {rateLimited > 0 ? (
        <p className="rounded-md bg-warn-wash px-3 py-2.5 text-sm leading-relaxed text-warn-ink">
          <span className="font-semibold">
            {formatNumber(rateLimited)} request{rateLimited === 1 ? " was" : "s were"} rate
            limited (429).
          </span>{" "}
          The API asked us to slow down, so those timings measure its rate limiter rather than
          the endpoint itself. Try fewer requests at a time.
        </p>
      ) : null}
    </div>
  );
}

function RowGroup({ title, rows, denominator }: { title: string; rows: Row[]; denominator: number }) {
  return (
    <div>
      <h3 className="mb-2 text-sm font-semibold text-ink-2">{title}</h3>
      <ul className="flex flex-col gap-2.5">
        {rows.map((r) => {
          const fraction = r.count / denominator;
          return (
            <li key={r.key} className="grid grid-cols-[minmax(7rem,11rem)_1fr_auto] items-center gap-3 text-sm">
              <span className="min-w-0 truncate">
                <span className="font-bold text-ink tabular-nums">{r.label}</span>
                {r.detail ? <span className="ml-1.5 text-ink-3">{r.detail}</span> : null}
              </span>
              <span className="h-3 overflow-hidden rounded-sm bg-grid">
                <span
                  className={`block h-full rounded-r-[4px] ${r.bar}`}
                  style={{
                    width: `${Math.max(1, fraction * 100)}%`,
                    backgroundImage: r.striped
                      ? "repeating-linear-gradient(135deg, transparent 0 3px, rgba(255,255,255,0.55) 3px 6px)"
                      : undefined,
                  }}
                />
              </span>
              <span className="w-24 text-right tabular-nums text-ink-2">
                <span className="font-semibold text-ink">{formatNumber(r.count)}</span>{" "}
                <span className="text-ink-3">({formatPercent(fraction)})</span>
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
