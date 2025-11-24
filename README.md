# Snowops Acts Service

The Acts service implements EPIC 9 – automated creation of contractor acceptance certificates (форма Р-1) without manual data entry on the frontend. The endpoint validates user roles, aggregates trips for the requested period, persists audit data (act + act_trip) and returns a ready-to-print PDF.

## Features

- Role-aware generation (`AKIMAT_ADMIN`, `KGU_ZKH_ADMIN`, `CONTRACTOR_ADMIN`, `LANDFILL_ADMIN`).
- Data source: `contracts`, `tickets`, `trips`, `organizations`; persists into `act` and `act_trip`.
- Filters trips by contract, date range, status whitelist and removes trips already included in other acts.
- For LANDFILL_SERVICE contracts: aggregates trips by polygons (from `contract_polygons`).
- Calculates totals, VAT, budget overrun notice and writes audit logs in a single transaction.
- Generates PDF form R‑1 (UTF-8, Noto Sans font) with requisites, totals, warning banner and signature lines.
- **Workflow подтверждения актов LANDFILL**: акты по приёму снега создаются со статусом `PENDING_APPROVAL` и требуют подтверждения от LANDFILL.

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

Все эндпоинты требуют `Authorization: Bearer <jwt>`.

### Генерация актов

#### `POST /contracts/{contract_id}/acts/generate-pdf`

Генерация акта по контракту (вывоз или приём снега).

**Request (JSON):**
```json
{
  "period_start": "2025-01-01",
  "period_end": "2025-01-31"
}
```

**Доступ:**
- `AKIMAT_ADMIN`, `KGU_ZKH_ADMIN` — для всех контрактов
- `CONTRACTOR_ADMIN` — только для своих контрактов (CONTRACTOR_SERVICE)
- `LANDFILL_ADMIN` — только для своих контрактов (LANDFILL_SERVICE)

**Особенности:**
- Для CONTRACTOR_SERVICE: агрегирует рейсы по тикетам контракта
- Для LANDFILL_SERVICE: агрегирует рейсы по полигонам контракта (из `contract_polygons`)
- Для LANDFILL_SERVICE: акт создаётся со статусом `PENDING_APPROVAL` (требует подтверждения)
- Для CONTRACTOR_SERVICE: акт создаётся со статусом `GENERATED`

**Ответ:** `application/pdf` attachment (`Content-Disposition: attachment; filename="act-..."`)

**Ошибки:**
- `400` invalid params, `403` permission, `404` contract missing,
- `422` when no valid trips found for the requested window.

### Управление актами LANDFILL

#### `GET /acts/landfill`
Список актов для LANDFILL организации.

**Query параметры:**
- `status` (опционально) — фильтр по статусу: `GENERATED`, `PENDING_APPROVAL`, `APPROVED`, `REJECTED`

**Доступ:** только `LANDFILL_ADMIN`, `LANDFILL_USER`

**Ответ:**
```json
{
  "data": [
    {
      "id": "uuid",
      "contract_id": "uuid",
      "landfill_id": "uuid",
      "act_number": "AKT-...",
      "act_date": "2025-01-15T00:00:00Z",
      "period_start": "2025-01-01T00:00:00Z",
      "period_end": "2025-01-31T00:00:00Z",
      "total_volume_m3": 1250.8,
      "price_per_m3": 500.00,
      "amount_wo_vat": 625400.00,
      "vat_rate": 12.00,
      "vat_amount": 75048.00,
      "amount_with_vat": 700448.00,
      "status": "PENDING_APPROVAL",
      "rejection_reason": null,
      "approved_by_org_id": null,
      "approved_by_user_id": null,
      "approved_at": null,
      "created_by_org_id": "uuid",
      "created_by_user_id": "uuid",
      "created_at": "2025-01-15T10:30:00Z"
    }
  ]
}
```

#### `GET /acts/landfill/:id`
Просмотр акта по ID.

**Доступ:** только `LANDFILL_ADMIN`, `LANDFILL_USER` (только свои акты)

**Ответ:** 200 OK с объектом акта (формат как в списке выше)

#### `PUT /acts/landfill/:id/approve`
Подтвердить акт.

**Доступ:** только `LANDFILL_ADMIN`

**Request (JSON, опционально):**
```json
{
  "comment": "Акт подтверждён"
}
```

**Ответ:**
```json
{
  "message": "act approved"
}
```

**Ошибки:**
- `400` если акт не в статусе `PENDING_APPROVAL`
- `403` если акт не принадлежит организации пользователя
- `404` если акт не найден

#### `PUT /acts/landfill/:id/reject`
Отклонить акт с указанием причины.

**Доступ:** только `LANDFILL_ADMIN`

**Request (JSON):**
```json
{
  "reason": "Несоответствие объёмов по полигону №2"
}
```

**Ответ:**
```json
{
  "message": "act rejected"
}
```

**Ошибки:**
- `400` если акт не в статусе `PENDING_APPROVAL` или не указана причина
- `403` если акт не принадлежит организации пользователя
- `404` если акт не найден

## Extending

- Set `ACTS_VALID_STATUSES` to include statuses approved by appeals (e.g. `OK,SUSPICIOUS_VOLUME`).
- Customize `ACTS_WORK_DESCRIPTION` with the official Russian wording from EPIC 9.
- Extend `internal/pdf/generator.go` to add multi-line trip details or QR codes if needed.
