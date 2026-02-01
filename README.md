# Snowops Acts Service

Сервис формирует Excel‑акты вывоза снега. Фронтенд вызывает один эндпоинт и получает `.xlsx` файл.

## Что делает сервис

- Формирует акт за период (`period_start`..`period_end`).
- Два режима:
  - `contractor`: акт по подрядчику, группировка по полигонам.
  - `landfill`: акт по полигону (организации‑полигону), группировка по подрядчикам.
- Excel содержит несколько листов:
  - `Summary`: итоги по всем рейсам + сводка по группам.
  - по одному листу на каждую группу (полигон или подрядчик).
- Даже если рейсов нет — акт всё равно оформляется (листы есть, строки пустые).
- Только чтение БД: без записи и без отдельных таблиц актов.

## Конфигурация

| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `APP_ENV` | `development` / `production` | `development` |
| `HTTP_HOST`, `HTTP_PORT` | адрес/порт HTTP | `0.0.0.0`, `7093` |
| `DB_DSN` | строка подключения Postgres | `postgres://postgres:postgres@localhost:5450/snowops_acts?sslmode=disable` |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME` | пул соединений | `20`, `10`, `1h` |
| `JWT_ACCESS_SECRET` | секрет проверки JWT | обязательно |
| `ACTS_VALID_STATUSES` | статусы `trips.status`, которые включаются в акт | `OK` |

## Источники данных

Сервис читает из:

- `organizations`: `id`, `name`, `type`, `bin`, `head_full_name`, `address`, `phone`
- `polygons`: `id`, `name` *(если есть `organization_id`, используется для полигона; если нет — берутся все полигоны)*
- `tickets`: `id`, `contractor_id`
- `trips`:  
  `id`, `ticket_id`, `polygon_id`, `entry_at`, `exit_at`, `status`,  
  `vehicle_plate_number`, `detected_plate_number`,  
  `detected_volume_entry`, `detected_volume_exit`, `total_volume_m3`

**Объём (Volume M3):**
1) если есть `total_volume_m3` — используем его;  
2) иначе `detected_volume_entry`;  
3) иначе `detected_volume_exit`.

## API

Все эндпоинты требуют `Authorization: Bearer <jwt>`.

### `POST /acts/export`

Exports an Excel report.

**Запрос (JSON):**
```json
{
  "mode": "landfill",
  "target_id": "UUID",
  "period_start": "2025-12-01",
  "period_end": "2026-02-28"
}
```

**Поля:**
- `mode`: `landfill` или `contractor`.
- `target_id`:
  - `landfill`: `organizations.id` организации‑полигона.
  - `contractor`: `organizations.id` подрядчика.
- `period_start`, `period_end`: ISO (`YYYY-MM-DD`) или RFC3339.

**Права доступа:**
- `contractor`: `AKIMAT_*`, `KGU_*`, `CONTRACTOR_ADMIN` (только свой `org_id`).
- `landfill`: `AKIMAT_*`, `KGU_*`, `LANDFILL_*` (только свой `org_id`).

**Ответ:**
- `200` и файл Excel (`Content-Disposition: attachment; filename="acts-...xlsx"`).

**Ошибки:**
- `400` некорректные входные данные.
- `403` нет прав.
- `404` организация не найдена.

## Локальный запуск

```bash
cd deploy
docker compose up -d

cd ..
APP_ENV=development \
DB_DSN="postgres://postgres:postgres@localhost:5450/snowops_acts?sslmode=disable" \
JWT_ACCESS_SECRET="dev-secret" \
go run ./cmd/acts-service
```

## Render (деплой)

- `DB_DSN` должен указывать на основную SnowOps базу.
- `JWT_ACCESS_SECRET` должен совпадать с auth‑сервисом.
- `ACTS_VALID_STATUSES` — список статусов для отчёта.

## Пример запроса (PowerShell)

```powershell
$body = @{
  mode = "contractor"
  target_id = "UUID"
  period_start = "2025-12-01"
  period_end = "2026-02-28"
} | ConvertTo-Json

Invoke-WebRequest `
  -Uri "https://snowops-acts-service.onrender.com/acts/export" `
  -Method POST `
  -Headers @{ Authorization = "Bearer <jwt>"; "Content-Type" = "application/json" } `
  -Body $body `
  -OutFile ".\\acts.xlsx" `
  -UseBasicParsing
```

## Пример запроса (curl)

```bash
curl -L -X POST "https://snowops-acts-service.onrender.com/acts/export" \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "contractor",
    "target_id": "UUID",
    "period_start": "2025-12-01",
    "period_end": "2026-02-28"
  }' \
  -o acts.xlsx
```
