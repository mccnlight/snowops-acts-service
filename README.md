# Snowops Acts Service

The Acts service implements EPIC 9 – automated creation of contractor acceptance certificates (форма Р-1) without manual data entry on the frontend. The endpoint validates user roles, aggregates trips for the requested period, persists audit data (act + act_trip) and returns a ready-to-print PDF.

## Features

- Role-aware generation (`AKIMAT_ADMIN`, `KGU_ZKH_ADMIN`, `CONTRACTOR_ADMIN` only).
- Data source: `contracts`, `tickets`, `trips`, `organizations`; persists into `act` and `act_trip`.
- Filters trips by contract, date range, status whitelist and removes trips already included in other acts.
- Calculates totals, VAT, budget overrun notice and writes audit logs in a single transaction.
- Generates PDF form R‑1 (UTF-8, Noto Sans font) with requisites, totals, warning banner and signature lines.

## Configuration

| Variable | Description | Default |
| --- | --- | --- |
| `APP_ENV` | `development` / `production` | `development` |
| `HTTP_HOST`, `HTTP_PORT` | HTTP bind | `0.0.0.0`, `7093` |
| `DB_DSN` | Postgres DSN | `postgres://postgres:postgres@localhost:5450/snowops_acts?sslmode=disable` |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME` | Pool tuning | `20`, `10`, `1h` |
| `JWT_ACCESS_SECRET` | JWT secret for role extraction | – |
| `ACTS_VAT_RATE` | VAT rate (%) | `12` |
| `ACTS_VALID_STATUSES` | Comma-separated trip statuses included into acts | `OK` |
| `ACTS_NUMBER_PREFIX` | Prefix for act_number field | `AKT` |
| `ACTS_WORK_DESCRIPTION` | Text in the PDF row (set to Russian template if needed) | English placeholder |

> The PDF uses the open-source Noto Sans font (SIL Open Font License). See `internal/pdf/fonts/OFL.txt`.

## Database

`internal/db/migrations.go` creates:

- `act` – log of generated acts with totals, VAT, creator metadata.
- `act_trip` – mapping between acts and trips (`trip_id` unique to avoid duplicates).

Existing tables `contracts`, `tickets`, `trips`, `organizations`, `users` must already exist in the shared SnowOps cluster.

## Run locally

```bash
cd deploy
docker compose up -d

cd ..
APP_ENV=development \
DB_DSN="postgres://postgres:postgres@localhost:5450/snowops_acts?sslmode=disable" \
JWT_ACCESS_SECRET="dev-secret" \
go run ./cmd/acts-service
```

## API

`POST /contracts/{contract_id}/acts/generate-pdf`

Request (JSON):

```json
{
  "period_start": "2025-01-01",
  "period_end": "2025-01-31"
}
```

- Requires `Authorization: Bearer <jwt>`.
- Validates role and that contractor_id matches for contractor accounts.
- Ensures period lies within contract dates.
- Returns `application/pdf` attachment (`Content-Disposition: attachment; filename="act-..."`).
- Errors:
  - `400` invalid params, `403` permission, `404` contract missing,
  - `422` when no valid trips found for the requested window.

## Extending

- Set `ACTS_VALID_STATUSES` to include statuses approved by appeals (e.g. `OK,SUSPICIOUS_VOLUME`).
- Customize `ACTS_WORK_DESCRIPTION` with the official Russian wording from EPIC 9.
- Extend `internal/pdf/generator.go` to add multi-line trip details or QR codes if needed.
