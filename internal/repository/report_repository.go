package repository

import (
    "context"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "github.com/nurpe/snowops-acts/internal/model"
)

type ReportRepository struct {
    db *gorm.DB
}

const cameraPolygonNameExpr = `
    CASE LOWER(ae.camera_id)
        WHEN 'shahovskoye' THEN 'Шаховское'
        WHEN 'yakor' THEN 'Якорь'
        WHEN 'solnechniy' THEN 'Солнечный'
        ELSE NULL
    END
`

func NewReportRepository(db *gorm.DB) *ReportRepository {
    return &ReportRepository{db: db}
}

func (r *ReportRepository) GetOrganization(ctx context.Context, id uuid.UUID) (*model.Organization, error) {
    var org model.Organization
    if err := r.db.WithContext(ctx).Raw(`
        SELECT id, name, type, bin, head_full_name, address, phone
        FROM organizations
        WHERE id = ?
        LIMIT 1
    `, id).Scan(&org).Error; err != nil {
        return nil, err
    }
    if org.ID == uuid.Nil {
        return nil, gorm.ErrRecordNotFound
    }
    return &org, nil
}

func (r *ReportRepository) GetPolygon(ctx context.Context, id uuid.UUID) (uuid.UUID, string, error) {
	var row struct {
		ID   uuid.UUID
		Name string
	}
	if err := r.db.WithContext(ctx).Raw(`
        SELECT id, name
        FROM polygons
        WHERE id = ?
        LIMIT 1
    `, id).Scan(&row).Error; err != nil {
		return uuid.Nil, "", err
	}
	if row.ID == uuid.Nil {
		return uuid.Nil, "", gorm.ErrRecordNotFound
	}
	return row.ID, row.Name, nil
}

func (r *ReportRepository) ListPolygons(ctx context.Context) ([]model.TripGroup, error) {
    var rows []model.TripGroup
    if err := r.db.WithContext(ctx).Raw(`
        SELECT id, name, 0 AS trip_count
        FROM polygons
        ORDER BY name ASC
    `).Scan(&rows).Error; err != nil {
        return nil, err
    }
    return rows, nil
}

func (r *ReportRepository) ListContractors(ctx context.Context) ([]model.TripGroup, error) {
    var rows []model.TripGroup
    if err := r.db.WithContext(ctx).Raw(`
        SELECT id, name, 0 AS trip_count
        FROM organizations
        WHERE type = 'CONTRACTOR'
          AND name NOT ILIKE 'TEST%'
        ORDER BY name ASC
    `).Scan(&rows).Error; err != nil {
        return nil, err
    }
    return rows, nil
}

func (r *ReportRepository) EventCountsByPolygon(
	ctx context.Context,
	contractorID uuid.UUID,
	from, to time.Time,
) ([]model.TripGroup, error) {
	query := `
		SELECT
			p.id AS id,
			p.name AS name,
			COUNT(*) AS trip_count
		FROM anpr_events ae
		JOIN polygons p ON p.name = ` + cameraPolygonNameExpr + `
		WHERE ae.contractor_id = ?
			AND ae.matched_snow = true
			AND ae.created_at >= ?
			AND ae.created_at < ?
		GROUP BY p.id, p.name
		ORDER BY p.name ASC
	`
	var rows []model.TripGroup
	if err := r.db.WithContext(ctx).Raw(query, contractorID, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ReportRepository) EventCountsByContractor(
	ctx context.Context,
	polygonIDs []uuid.UUID,
	from, to time.Time,
) ([]model.TripGroup, error) {
	if len(polygonIDs) == 0 {
		return []model.TripGroup{}, nil
	}

	query := `
		SELECT
			ae.contractor_id AS id,
			org.name AS name,
			COUNT(*) AS trip_count
		FROM anpr_events ae
		JOIN polygons p ON p.name = ` + cameraPolygonNameExpr + `
		JOIN organizations org ON org.id = ae.contractor_id
		WHERE p.id = ANY(?)
			AND org.type = 'CONTRACTOR'
			AND org.name NOT ILIKE 'TEST%'
			AND ae.matched_snow = true
			AND ae.created_at >= ?
			AND ae.created_at < ?
		GROUP BY ae.contractor_id, org.name
		ORDER BY name ASC
	`

	var rows []model.TripGroup
	if err := r.db.WithContext(ctx).Raw(query, polygonIDs, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ReportRepository) ListEventsByPolygon(
	ctx context.Context,
	contractorID uuid.UUID,
	polygonID uuid.UUID,
	from, to time.Time,
) ([]model.TripDetail, error) {
	query := `
		SELECT
			ae.created_at AS event_time,
			COALESCE(ae.normalized_plate, ae.raw_plate) AS plate,
			p.id AS polygon_id,
			p.name AS polygon_name,
			ae.contractor_id,
			org.name AS contractor_name,
			ae.snow_volume_m3
		FROM anpr_events ae
		JOIN polygons p ON p.name = ` + cameraPolygonNameExpr + `
		LEFT JOIN organizations org ON org.id = ae.contractor_id
		WHERE ae.contractor_id = ?
			AND p.id = ?
			AND ae.matched_snow = true
			AND ae.created_at >= ?
			AND ae.created_at < ?
		ORDER BY event_time ASC
	`

	var rows []model.TripDetail
	if err := r.db.WithContext(ctx).Raw(query, contractorID, polygonID, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ReportRepository) ListEventsByContractor(
	ctx context.Context,
	contractorID uuid.UUID,
	polygonIDs []uuid.UUID,
	from, to time.Time,
) ([]model.TripDetail, error) {
	if len(polygonIDs) == 0 {
		return []model.TripDetail{}, nil
	}

	query := `
		SELECT
			ae.created_at AS event_time,
			COALESCE(ae.normalized_plate, ae.raw_plate) AS plate,
			p.id AS polygon_id,
			p.name AS polygon_name,
			ae.contractor_id,
			org.name AS contractor_name,
			ae.snow_volume_m3
		FROM anpr_events ae
		JOIN polygons p ON p.name = ` + cameraPolygonNameExpr + `
		JOIN organizations org ON org.id = ae.contractor_id
		WHERE p.id = ANY(?)
			AND ae.contractor_id = ?
			AND org.type = 'CONTRACTOR'
			AND org.name NOT ILIKE 'TEST%'
			AND ae.matched_snow = true
			AND ae.created_at >= ?
			AND ae.created_at < ?
		ORDER BY event_time ASC
	`

	var rows []model.TripDetail
	if err := r.db.WithContext(ctx).Raw(query, polygonIDs, contractorID, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
