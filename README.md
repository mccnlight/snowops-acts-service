# Snowops Acts Service

This service generates Excel act reports for trips. The frontend calls a single endpoint and receives an `.xlsx` file.

## What it does

- Builds reports for a selected period (`period_start`..`period_end`).
- Two report modes:
  - `contractor`: trips grouped by polygons (landfills) for a contractor.
  - `landfill`: trips grouped by contractors for a landfill organization (polygons are matched by `polygons.organization_id`).
- Returns an Excel file with multiple sheets:
  - `Summary`: total trips + list of groups with counts.
  - One sheet per group with the group trip count and per-trip rows (including snow volume).
- Read-only: no database writes, no act tables.

## Configuration

| Variable | Description | Default |
| --- | --- | --- |
| `APP_ENV` | `development` / `production` | `development` |
| `HTTP_HOST`, `HTTP_PORT` | HTTP bind | `0.0.0.0`, `7093` |
| `DB_DSN` | Postgres DSN | `postgres://postgres:postgres@localhost:5450/snowops_acts?sslmode=disable` |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME` | Pool tuning | `20`, `10`, `1h` |
| `JWT_ACCESS_SECRET` | JWT secret for role extraction | required |
| `ACTS_VALID_STATUSES` | Comma-separated trip statuses included in reports | `OK` |

## Data sources

The service reads from existing tables:

- `trips` (entry time, status, polygon_id)
- `tickets` (contractor_id)
- `polygons` (name, organization_id)
- `organizations` (id, name)

## API

All endpoints require `Authorization: Bearer <jwt>`.

### `POST /acts/export`

Exports an Excel report.

**Request (JSON):**
```json
{
  "mode": "landfill",
  "target_id": "UUID",
  "period_start": "2025-12-01",
  "period_end": "2026-02-28"
}
```

**Fields:**
- `mode`: `landfill` or `contractor`.
- `target_id`:
  - `landfill`: organization id that owns polygons (`polygons.organization_id`).
  - `contractor`: organization id of the contractor.
- `period_start`, `period_end`: ISO dates (`YYYY-MM-DD`) or RFC3339.

**Access rules:**
- `contractor` mode: `AKIMAT_*`, `KGU_*`, or `CONTRACTOR_ADMIN` (only own org).
- `landfill` mode: `AKIMAT_*`, `KGU_*`, or `LANDFILL_*` (only own org).

**Response:**
- `200` with `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` and `Content-Disposition: attachment; filename="acts-...xlsx"`.

**Errors:**
- `400` invalid input.
- `403` permission denied.
- `404` target organization not found.
- `422` no trips for the selected period.

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

## Notes for deployment (Render)

- Provide `DB_DSN` pointing to the shared SnowOps database.
- Set `JWT_ACCESS_SECRET` to the same value as the auth service.
- Set `ACTS_VALID_STATUSES` if you need extra trip statuses.
