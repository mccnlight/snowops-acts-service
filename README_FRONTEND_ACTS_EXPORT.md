# Frontend Guide: Export Acts (Excel/PDF)

Короткая инструкция для фронтенда по выгрузке актов.

## База

- Service: `snowops-acts-service`
- Auth: `Authorization: Bearer <JWT>`
- Форматы:
  - Excel: `POST /acts/export`
  - PDF: `POST /acts/export/pdf`

## Важные правила

- `target_id` — это **`organizations.id`**, а не `polygon_id` из ANPR.
- `mode`:
  - `landfill` -> `target_id` должен быть `organizations.type = LANDFILL`
  - `contractor` -> `target_id` должен быть `organizations.type = CONTRACTOR`
- `period_start`, `period_end`: `YYYY-MM-DD` или RFC3339.

## Тело запроса (одно и то же для Excel/PDF)

```json
{
  "mode": "landfill",
  "target_id": "UUID",
  "period_start": "2026-01-10",
  "period_end": "2026-01-11"
}
```

## Ответы

### Excel (`POST /acts/export`)

- `200 OK`
- `Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
- `Content-Disposition: attachment; filename="acts-...xlsx"`

### PDF (`POST /acts/export/pdf`)

- `200 OK`
- `Content-Type: application/pdf`
- `Content-Disposition: attachment; filename="acts-...pdf"`

## Пример fetch (Excel)

```js
const response = await fetch('http://localhost:7010/acts/export', {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    mode: 'landfill',
    target_id: landfillOrgId,
    period_start: '2026-01-10',
    period_end: '2026-01-11',
  }),
});

if (!response.ok) throw new Error(await response.text());
const blob = await response.blob();
const url = URL.createObjectURL(blob);
const a = document.createElement('a');
a.href = url;
a.download = 'acts.xlsx';
a.click();
URL.revokeObjectURL(url);
```

## Пример fetch (PDF)

```js
const response = await fetch('http://localhost:7010/acts/export/pdf', {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    mode: 'landfill',
    target_id: landfillOrgId,
    period_start: '2026-01-10',
    period_end: '2026-01-11',
  }),
});

if (!response.ok) throw new Error(await response.text());
const blob = await response.blob();
const url = URL.createObjectURL(blob);
const a = document.createElement('a');
a.href = url;
a.download = 'acts.pdf';
a.click();
URL.revokeObjectURL(url);
```

## Insomnia (быстрые шаги)

1. Method: `POST`
2. URL: `http://localhost:7010/acts/export/pdf` (или `/acts/export`)
3. Headers:
   - `Authorization: Bearer <JWT>`
   - `Content-Type: application/json`
4. Body: JSON из примера выше
5. Нажать `Send` и сохранить файл из response.

## Частые ошибки

- `404 not found`:
  - неверный `target_id` (организация не найдена), или
  - не тот URL/порт.
- `400 invalid mode`:
  - mode должен быть только `landfill` или `contractor`.
- `401`:
  - невалидный/просроченный токен.
- `403`:
  - у роли нет доступа к выбранной организации.

## Кириллица в PDF

Если в PDF появляются некорректные символы, укажи шрифт с кириллицей:

- env: `PDF_FONT_PATH`
- пример (Windows): `C:\Windows\Fonts\arial.ttf`
