# Snowops Acts Service

Сервис формирует Excel-акты по ивентам вывоза снега. Для фронтенда используется один эндпоинт: `POST /acts/export`.

## Быстрый старт для фронтенда

1. Получить JWT в auth-сервисе.
2. Отправить `POST /acts/export` с `mode`, `target_id`, `period_start`, `period_end`.
3. Принять ответ как бинарный файл (`.xlsx`) и сохранить/скачать.

## Эндпоинт

### `POST /acts/export`

- Заголовки:
  - `Authorization: Bearer <jwt>`
  - `Content-Type: application/json`
- Тело запроса:

```json
{
  "mode": "contractor",
  "target_id": "UUID",
  "period_start": "2025-12-01",
  "period_end": "2026-02-28"
}
```

## Параметры

- `mode`:
  - `contractor` — акт по подрядчику, группировка по полигонам (landfill).
  - `landfill` — акт по полигону, группировка по подрядчикам.
- `target_id`:
  - для `contractor`: `organizations.id` подрядчика (`type = CONTRACTOR`)
  - для `landfill`: `organizations.id` полигона (`type = LANDFILL`)
- `period_start`, `period_end`:
  - даты периода, поддерживаются `YYYY-MM-DD` и RFC3339.

## Что приходит в ответ

- HTTP `200`
- `Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
- `Content-Disposition: attachment; filename="acts-...xlsx"`
- Тело ответа — бинарный Excel файл.

## Структура Excel

- Лист 1: `Сводка`
  - тип отчета, организация, период, общее количество рейсов, общий объем снега
  - таблица по группам (количество рейсов и объем)
- Остальные листы: по каждой группе
  - для `contractor`: по каждому полигону
  - для `landfill`: по каждому подрядчику
  - строки ивентов: дата, номер машины, полигон, подрядчик, объем снега

Даже если данных нет, файл все равно формируется: листы остаются, значения будут нулевые/пустые.

## Источник данных и правила

Сервис читает из `anpr_events` и `organizations`.

- учитываются только `matched_snow = true`
- период фильтруется по `event_time`
- полигон определяется через `camera_id` в `anpr_events`:
  - `shahovskoye` -> `Шаховское`
  - `yakor` -> `Якорь`
  - `solnechniy` -> `Солнечный`
- подрядчики берутся из `organizations` (`type = CONTRACTOR`), тестовые (`name ILIKE 'TEST%'`) исключаются

## Ошибки API

- `400` — некорректные входные данные
- `401` — нет/невалидный токен
- `403` — нет прав
- `404` — организация не найдена
- `500` — внутренняя ошибка

## Пример (fetch)

```js
const response = await fetch('https://snowops-acts-service.onrender.com/acts/export', {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    mode: 'landfill',
    target_id: landfillOrgId,
    period_start: '2026-02-01',
    period_end: '2026-02-04',
  }),
});

if (!response.ok) {
  const text = await response.text();
  throw new Error(`Export failed: ${response.status} ${text}`);
}

const blob = await response.blob();
const url = URL.createObjectURL(blob);
const a = document.createElement('a');
a.href = url;
a.download = 'acts.xlsx';
a.click();
URL.revokeObjectURL(url);
```

## Конфигурация сервиса

| Переменная | Описание |
| --- | --- |
| `APP_ENV` | окружение (`development` / `production`) |
| `HTTP_HOST`, `HTTP_PORT` | адрес и порт HTTP |
| `DB_DSN` | строка подключения к Postgres |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME` | настройки пула БД |
| `JWT_ACCESS_SECRET` | секрет проверки JWT (должен совпадать с auth-сервисом) |
