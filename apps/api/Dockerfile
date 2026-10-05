# Build context: apps/api

FROM golang:1.25 AS build
WORKDIR /src

COPY . .
# Static binary so it runs on a distroless base. GOWORK=off because go.work
# lives at the repo root, outside this build context.
RUN CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# distroless/static ships CA certificates (needed for HTTPS targets) and runs
# as an unprivileged user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server

ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/server"]
