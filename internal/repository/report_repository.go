package repository

import (
    "context"
    "strings"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "github.com/nurpe/snowops-acts/internal/model"
)

type ReportRepository struct {
    db *gorm.DB
}

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

func (r *ReportRepository) ListLandfillPolygonIDs(ctx context.Context, landfillID uuid.UUID) ([]uuid.UUID, error) {
    var polygonIDs []uuid.UUID
    err := r.db.WithContext(ctx).Raw(`
        SELECT id
        FROM polygons
        WHERE organization_id = ?
        ORDER BY name ASC
    `, landfillID).Scan(&polygonIDs).Error
    if err == nil {
        return polygonIDs, nil
    }

    errLower := strings.ToLower(err.Error())
    if !strings.Contains(errLower, "organization_id") {
        return nil, err
    }

    polygonIDs = nil
    if err := r.db.WithContext(ctx).Raw(`
        SELECT id
        FROM polygons
        ORDER BY name ASC
    `).Scan(&polygonIDs).Error; err != nil {
        return nil, err
    }
    return polygonIDs, nil
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
			ae.polygon_id AS id,
			COALESCE(p.name, 'Unknown') AS name,
			COUNT(*) AS trip_count
		FROM anpr_events ae
		LEFT JOIN polygons p ON p.id = ae.polygon_id
		WHERE ae.contractor_id = ?
			AND ae.polygon_id IS NOT NULL
			AND ae.matched_snow = true
			AND COALESCE(ae.event_time, ae.created_at) >= ?
			AND COALESCE(ae.event_time, ae.created_at) < ?
		GROUP BY ae.polygon_id, p.name
		ORDER BY name ASC
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
			COALESCE(org.name, 'Unknown') AS name,
			COUNT(*) AS trip_count
		FROM anpr_events ae
		LEFT JOIN organizations org ON org.id = ae.contractor_id
		WHERE ae.polygon_id = ANY(?)
			AND ae.contractor_id IS NOT NULL
			AND ae.matched_snow = true
			AND COALESCE(ae.event_time, ae.created_at) >= ?
			AND COALESCE(ae.event_time, ae.created_at) < ?
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
			COALESCE(ae.event_time, ae.created_at) AS event_time,
			COALESCE(ae.normalized_plate, ae.raw_plate) AS plate,
			ae.polygon_id,
			p.name AS polygon_name,
			ae.contractor_id,
			org.name AS contractor_name,
			ae.snow_volume_m3
		FROM anpr_events ae
		LEFT JOIN polygons p ON p.id = ae.polygon_id
		LEFT JOIN organizations org ON org.id = ae.contractor_id
		WHERE ae.contractor_id = ?
			AND ae.polygon_id = ?
			AND ae.matched_snow = true
			AND COALESCE(ae.event_time, ae.created_at) >= ?
			AND COALESCE(ae.event_time, ae.created_at) < ?
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
			COALESCE(ae.event_time, ae.created_at) AS event_time,
			COALESCE(ae.normalized_plate, ae.raw_plate) AS plate,
			ae.polygon_id,
			p.name AS polygon_name,
			ae.contractor_id,
			org.name AS contractor_name,
			ae.snow_volume_m3
		FROM anpr_events ae
		LEFT JOIN polygons p ON p.id = ae.polygon_id
		LEFT JOIN organizations org ON org.id = ae.contractor_id
		WHERE ae.polygon_id = ANY(?)
			AND ae.contractor_id = ?
			AND ae.matched_snow = true
			AND COALESCE(ae.event_time, ae.created_at) >= ?
			AND COALESCE(ae.event_time, ae.created_at) < ?
		ORDER BY event_time ASC
	`

	var rows []model.TripDetail
	if err := r.db.WithContext(ctx).Raw(query, polygonIDs, contractorID, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
