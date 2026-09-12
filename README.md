# QRIS Gate

[![CI](https://github.com/qrisgate/qrisgate/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/qrisgate/qrisgate/actions/workflows/ci-cd.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qrisgate/qrisgate)](https://goreportcard.com/report/github.com/qrisgate/qrisgate)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Open-source API that turns a merchant **static QRIS** payload into **dynamic** QR codes per order, with PostgreSQL persistence, Prometheus metrics, and optional OpenTelemetry tracing.

Built with Go 1.26, [Echo v4](https://echo.labstack.com/), [pgx v5](https://github.com/jackc/pgx), and EMVCo TLV parsing ported from [qris-dinamis-ts](https://github.com/qrisgate/qris-dinamis-ts).

## Features

- Parse, validate, and convert QRIS (static → dynamic) with CRC16-CCITT
- REST API for app provisioning (admin) and payment creation (API key)
- Idempotent `POST /v1/payments` on `(app_id, order_id)`
- Optional fee, expiry, and per-payment callback URL
- Webhook endpoint registration
- Admin claim by amount (`POST /v1/payments/claim`) for paywatch
- Background sweep marks past-due pending payments `expired`
- Webhook delivery with `Idempotency-Key`, sync retries, and async retry loop
- Returns `qris_string` and `qr_image_base64` (PNG)
- Readiness probe (`/readyz`) for Postgres
- Rate limiting and request body limits on payment routes
- Goose SQL migrations, Docker Compose for local Postgres/API
- Prometheus metrics on a separate port (`METRICS_ADDR`)
- Optional OTLP tracing via `OTEL_EXPORTER_OTLP_ENDPOINT`

## Install

### Docker Compose — build from source (dev)

```bash
git clone https://github.com/qrisgate/qrisgate.git
cd qrisgate
cp .env.example .env
docker compose up --build -d
```

API: http://localhost:8080 · Metrics: http://localhost:9090/metrics

### Docker Hub (production)

Image: [`qrisgate/qrisgate`](https://hub.docker.com/r/qrisgate/qrisgate) — pick a tag from [Releases](https://github.com/qrisgate/qrisgate/releases) (`v0.1.0` → `0.1.0`).

Pull the image:

```bash
docker pull qrisgate/qrisgate:0.1.0
# or latest: docker pull qrisgate/qrisgate:latest
```

Run the full stack (Postgres + API):

```bash
git clone https://github.com/qrisgate/qrisgate.git
cd qrisgate
cp .env.example .env
# set ADMIN_TOKEN

export QRISGATE_TAG=0.1.0
export ADMIN_TOKEN=your-secret
make docker-hub-up
curl -s http://127.0.0.1:8080/healthz
```

`docker compose -f docker-compose.hub.yml pull` also works if you skip `docker pull` above.

Pin a release tag in production; avoid `latest`.

## Quick start

```bash
cp .env.example .env
# start Postgres (or use docker compose)
export DATABASE_URL=postgres://qrisgate:qrisgate@localhost:5432/qrisgate?sslmode=disable
export ADMIN_TOKEN=dev-admin-token

make run-api
```

With Docker:

```bash
docker compose up --build
```

### Observability stack (Jaeger + Prometheus)

```bash
make docker-obs
```

| Service | URL |
|---------|-----|
| API | http://localhost:8080 |
| Metrics | http://localhost:9090/metrics |
| Prometheus | http://localhost:9091 |
| Jaeger UI | http://localhost:16686 |

Query metrics in Prometheus UI or `curl localhost:9090/metrics`. Add Grafana later if you want dashboards.

To trace locally without Docker, run [Jaeger all-in-one](https://www.jaegertracing.io/docs/getting-started/) and set `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317`.

## API overview

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | — | Liveness |
| GET | `/readyz` | — | Readiness (Postgres) |
| POST | `/v1/apps` | `Authorization: Bearer $ADMIN_TOKEN` | Register app + static QRIS |
| POST | `/v1/apps/:id/webhooks` | Bearer admin | Register HTTPS webhook |
| POST | `/v1/payments` | `X-API-Key` | Create dynamic QR (idempotent) |
| GET | `/v1/payments/:id` | `X-API-Key` | Fetch payment + QR |
| POST | `/v1/payments/claim` | Bearer admin | Claim pending by amount for one app (paywatch) |
| POST | `/v1/payments/:id/paid` | Bearer admin | Mark payment paid |

See [internal/openapi/openapi.yaml](./internal/openapi/openapi.yaml) for the OpenAPI 3 spec. With `SWAGGER_ACTIVE=true` and `ENV=development`, browse the API at http://localhost:8080/swagger/

### Create app

```bash
curl -s -X POST http://localhost:8080/v1/apps \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Demo","merchant_qris":"<static-qris>"}'
```

Response includes a one-time `api_key` (`qg_…` prefix). Store it securely; only the hash is kept server-side.

### Register webhook

```bash
curl -s -X POST http://localhost:8080/v1/apps/$APP_ID/webhooks \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/webhooks/qris"}'
```

### Claim payment (paywatch / admin)

Matches the oldest pending payment **for that app** with the same amount (not expired, created within 24h of `paid_at`), marks it paid, and dispatches webhooks. Idempotent on `(provider, external_id)`.

```bash
curl -s -X POST http://localhost:8080/v1/payments/claim \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"app_id":"'$APP_ID'","amount":15000,"provider":"gobiz","external_id":"tx-123","paid_at":"2026-09-11T10:00:00Z"}'
```

### Create payment

```bash
curl -s -X POST http://localhost:8080/v1/payments \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "ORD-1",
    "amount": 15000,
    "fee": {"type": "fixed", "value": 1000},
    "expires_in": 900,
    "callback_url": "https://example.com/orders/ORD-1/callback"
  }'
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ADDR` | `:8080` | HTTP listen address |
| `DATABASE_URL` | local Postgres DSN | Required for API |
| `ADMIN_TOKEN` | — | Bearer token for admin routes |
| `RATE_LIMIT_PER_MIN` | `100` | Per-IP limit on `/v1/payments` |
| `DEFAULT_EXPIRES_IN_SEC` | `900` | Default payment TTL (15 min) |
| `HTTP_READ_TIMEOUT_SEC` | `15` | Server read timeout |
| `HTTP_WRITE_TIMEOUT_SEC` | `30` | Server write timeout |
| `HTTP_IDLE_TIMEOUT_SEC` | `60` | Server idle timeout |
| `HTTP_BODY_LIMIT` | `64K` | Max request body (Echo middleware) |
| `METRICS_ENABLED` | `true` | Expose Prometheus |
| `METRICS_ADDR` | `:9090` | Metrics listen address |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | Enable OTLP gRPC export when set |
| `OTEL_SERVICE_NAME` | `qrisgate-api` | Service name for traces |
| `SWAGGER_ACTIVE` | `false` | Enable Swagger UI at `/swagger` (disabled when `ENV=production`) |

## Project layout

```
cmd/api/          HTTP server and routes
internal/qris/    TLV parser, validator, converter, QR PNG
internal/payment/ Payment use case
internal/admin/   App provisioning + webhooks
internal/health/  Readiness handlers
db/migrations/    Goose SQL
```

## Development

```bash
make test
make lint
make tidy
make run-api
make docker-up       # Postgres + API (build from source)
make docker-hub-up   # Postgres + API (pull from Docker Hub)
```

Migrations run automatically on API startup (`goose`).

## Contributing

Issues and pull requests are welcome. For code changes:

1. Fork, branch from `main`, run `make test` (and `make lint` for Go changes).
2. Keep PRs small and focused.
3. Add tests for behavior changes in `internal/`.
4. New DB changes → new file under `db/migrations/`.
5. API changes → update `internal/openapi/openapi.yaml` and this README.

You need Go **1.26+** and PostgreSQL. CI runs tests with a ~55% coverage floor.

## Request flow (handlers)

```
HTTP (Echo)
  └─ cmd/api/route/route.go       registers routes + middleware
       ├─ middleware.go           Recover, RequestID, CORS, logger, Prometheus
       ├─ otelecho                distributed traces (when OTLP configured)
       │
       ├─ GET /readyz             health.ReadyHandler (Postgres)
       │
       ├─ POST /v1/apps           admin.Handler.CreateApp
       │     └─ admin.Service      validate merchant_qris → generate qg_ API key → store app
       │
       ├─ POST /v1/apps/:id/webhooks  admin.Handler.CreateWebhook
       │
       ├─ POST /v1/payments       payment.Handler.Create (rate limited)
       │     └─ payment.Service    auth API key → idempotent lookup → qris.Convert → DB → QR PNG
       │
       └─ GET /v1/payments/:id     payment.Handler.Get
             └─ payment.Service    auth → load payment (scoped to app) → encode QR
```

## License

MIT — see [LICENSE](./LICENSE).
