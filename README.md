# superiorAPI

A black-box API performance profiler. Point it at an HTTP endpoint, choose how many requests to send and how many to run at once, and watch the results arrive live. When the run ends you get a report with latency percentiles, a histogram, status codes and a plain-language analysis.

The analysis keeps two things apart: what was **observed** (for example "P99 is 4.2× the median") and what **might** explain it (for example "cold starts" or "connection pool exhaustion"). It never claims to know what happens inside your server.

## Features

- **Real load:** a Go worker pool sends real HTTP requests with the method, headers and body you choose. You set the request count (up to 500) and concurrency (up to 50).
- **Live progress:** the server streams updates over Server-Sent Events, and the page falls back to polling if SSE is unavailable. While the test runs you see a live strip chart, running P50/P95 and a feed of recent requests. You can cancel at any point and still get a report on what finished.
- **Report:**
  - min, avg, P50, P90, P95, P99 and max latency, plus throughput and error rate;
  - a latency histogram;
  - a breakdown of status codes and transport errors (timeout, refused, reset, DNS, TLS, and so on);
  - rule-based findings for rate limiting, server and client errors, tail latency, cold starts, and latency that rises or falls over the run.
- **SSRF protection:** see [Security](#security).
- **No database and no accounts:** results live in memory and expire after 30 minutes.

## Repository layout

```
apps/
  api/   Go 1.25 backend: SSRF policy, benchmark runner, metrics, analyzer, HTTP API
  web/   Next.js 16 frontend (App Router, Tailwind CSS v4, hand-built SVG charts)
```

This is a monorepo: `apps/web` is an npm workspace and `apps/api` belongs to a Go workspace (`go.work`).

## Getting started

Requirements: Go 1.25+ and Node.js 20+.

```bash
npm install

# terminal 1: API on http://localhost:8080
npm run dev:api

# terminal 2: web app on http://localhost:3000
npm run dev:web
```

The web app calls `http://localhost:8080` by default. To use a different API address, set `NEXT_PUBLIC_API_URL`.

Private and loopback addresses are blocked even in development. If you want to profile a service running on your own machine, start the API with `ALLOW_PRIVATE_TARGETS=true`. Never set this on a public deployment.

### Scripts

| Command           | What it does                         |
| ----------------- | ------------------------------------ |
| `npm run dev:api` | Run the Go API                       |
| `npm run dev:web` | Run the Next.js dev server           |
| `npm run build`   | Build the API and the web app        |
| `npm run lint`    | `go vet` the API and ESLint the web app |
| `npm test`        | Run the Go test suite                |

## Configuration

The API reads its settings from environment variables. Every variable is optional.

| Variable                      | Default                 | Purpose |
| ----------------------------- | ----------------------- | ------- |
| `PORT`                        | `8080`                  | Listen port |
| `ALLOWED_ORIGINS`             | `http://localhost:3000` | CORS origins, comma-separated; `*` allows any |
| `PROBE_REGION`                | `local`                 | Shown to users as the place requests come from |
| `TRUST_PROXY_HEADERS`         | `false`                 | Identify clients by the rightmost `X-Forwarded-For` entry. Enable only behind a reverse proxy you control. |
| `ALLOW_PRIVATE_TARGETS`       | `false`                 | Turn off SSRF IP blocking (local development only) |
| `ALLOWED_PORTS`               | `80,443,8080,8443`      | Destination ports that may be targeted; `*` allows any |
| `MAX_REQUESTS`                | `500`                   | Most requests in one test |
| `MAX_CONCURRENCY`             | `50`                    | Most requests in flight at once |
| `DEFAULT_TIMEOUT_MS`          | `10000`                 | Per-request timeout when the user doesn't set one |
| `MAX_TIMEOUT_MS`              | `30000`                 | Longest per-request timeout a user may set |
| `MAX_BODY_BYTES`              | `65536`                 | Largest request body |
| `MAX_RESPONSE_BYTES`          | `5242880`               | Response bytes read per request before the rest is discarded |
| `MAX_REDIRECTS`               | `5`                     | Redirects followed per request |
| `MAX_HEADERS`                 | `50`                    | Most custom headers |
| `MAX_ACTIVE_TESTS`            | `20`                    | Tests running at once across all users |
| `MAX_ACTIVE_TESTS_PER_CLIENT` | `1`                     | Tests one client may run at once |
| `MAX_ACTIVE_TESTS_PER_TARGET` | `2`                     | Tests that may hit the same host at once |
| `MAX_TESTS_PER_HOUR`          | `30`                    | Tests one client may start per hour |
| `MAX_TEST_DURATION_MS`        | `120000`                | Hard limit on how long a test may run |
| `RESULT_TTL_MS`               | `1800000`               | How long finished results stay available |

## API

| Method & path                  | Description |
| ------------------------------ | ----------- |
| `GET /healthz`                 | Liveness check |
| `GET /api/limits`              | Limits the server currently enforces |
| `POST /api/tests`              | Start a test and return `{ "testId": "..." }` |
| `GET /api/tests/{id}`          | Current snapshot, or the full report once finished |
| `GET /api/tests/{id}/events`   | SSE stream: `progress` events, then a single `done` event |
| `POST /api/tests/{id}/cancel`  | Stop a running test |

Example: start a test.

```bash
curl -X POST http://localhost:8080/api/tests \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://dummyjson.com/products/1",
    "method": "GET",
    "headers": {},
    "body": null,
    "requests": 100,
    "concurrency": 10
  }'
```

Errors come back as `{ "error": { "code", "message", "field" } }`, where `field` names the input that caused the problem. Results report header names only; header values such as API keys are never echoed back.

## Security

The server sends requests to URLs that users supply, so its main threat is SSRF: being used to reach networks it shouldn't.

- **Blocked addresses:** loopback, private, link-local (including the cloud metadata address `169.254.169.254`), carrier-grade NAT, multicast, reserved and documentation ranges for IPv4 and IPv6. This also covers IPv6 ranges that can embed IPv4 addresses (NAT64, 6to4, Teredo).
- **Blocked hostnames:** `localhost`, `*.local`, `*.internal`, `metadata.google.internal` and similar.
- **Checks at connect time:** every connection is checked in the dialer's `Control` hook, after DNS resolution and right before the socket connects. That stops DNS rebinding and redirects to internal addresses as well. The HTTP client ignores proxy environment variables, so a proxy can't bypass the check.
- **Port allowlist:** only the ports in `ALLOWED_PORTS` may be targeted.
- **Abuse limits:** per-client, per-target and global caps on running tests, an hourly quota, a hard limit on test duration, and bounded request and response sizes.

## Development

```bash
npm test                          # Go unit tests (SSRF policy, runner, metrics, analyzer, test manager, HTTP API)
npm run lint                      # go vet + ESLint
npx tsc --noEmit -p apps/web      # type-check the frontend
```
