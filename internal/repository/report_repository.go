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

const cameraLandfillNameExpr = `
	CASE LOWER(ae.camera_id)
		WHEN 'shahovskoye' THEN 'шаховское'
		WHEN 'yakor' THEN 'якорь'
		WHEN 'solnechniy' THEN 'солнечный'
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

func (r *ReportRepository) ListLandfills(ctx context.Context) ([]model.TripGroup, error) {
	var rows []model.TripGroup
	if err := r.db.WithContext(ctx).Raw(`
        SELECT id, name, 0 AS trip_count
        FROM organizations
        WHERE type = 'LANDFILL'
          AND name NOT ILIKE 'TEST%'
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

func (r *ReportRepository) EventCountsByLandfill(
	ctx context.Context,
	contractorID uuid.UUID,
	from, to time.Time,
) ([]model.TripGroup, error) {
	query := `
		SELECT
			lf.id AS id,
			lf.name AS name,
			COUNT(*) AS trip_count
		FROM anpr_events ae
		JOIN organizations lf
		  ON lf.type = 'LANDFILL'
		 AND LOWER(lf.name) = ` + cameraLandfillNameExpr + `
		WHERE ae.contractor_id = ?
			AND ae.matched_snow = true
			AND ae.event_time >= ?
			AND ae.event_time < ?
		GROUP BY lf.id, lf.name
		ORDER BY lf.name ASC
	`

	var rows []model.TripGroup
	if err := r.db.WithContext(ctx).Raw(query, contractorID, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ReportRepository) EventCountsByContractor(
	ctx context.Context,
	landfillID uuid.UUID,
	from, to time.Time,
) ([]model.TripGroup, error) {
	query := `
		SELECT
			ae.contractor_id AS id,
			org.name AS name,
			COUNT(*) AS trip_count
		FROM anpr_events ae
		JOIN organizations lf
		  ON lf.type = 'LANDFILL'
		 AND LOWER(lf.name) = ` + cameraLandfillNameExpr + `
		JOIN organizations org ON org.id = ae.contractor_id
		WHERE lf.id = ?
			AND org.type = 'CONTRACTOR'
			AND org.name NOT ILIKE 'TEST%'
			AND ae.matched_snow = true
			AND ae.event_time >= ?
			AND ae.event_time < ?
		GROUP BY ae.contractor_id, org.name
		ORDER BY name ASC
	`

	var rows []model.TripGroup
	if err := r.db.WithContext(ctx).Raw(query, landfillID, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ReportRepository) ListEventsByLandfill(
	ctx context.Context,
	contractorID uuid.UUID,
	landfillID uuid.UUID,
	from, to time.Time,
) ([]model.TripDetail, error) {
	query := `
		SELECT
			ae.event_time AS event_time,
			COALESCE(ae.normalized_plate, ae.raw_plate) AS plate,
			lf.id AS polygon_id,
			lf.name AS polygon_name,
			ae.contractor_id,
			org.name AS contractor_name,
			ae.snow_volume_m3
		FROM anpr_events ae
		JOIN organizations lf
		  ON lf.type = 'LANDFILL'
		 AND LOWER(lf.name) = ` + cameraLandfillNameExpr + `
		LEFT JOIN organizations org ON org.id = ae.contractor_id
		WHERE ae.contractor_id = ?
			AND lf.id = ?
			AND ae.matched_snow = true
			AND ae.event_time >= ?
			AND ae.event_time < ?
		ORDER BY event_time ASC
	`

	var rows []model.TripDetail
	if err := r.db.WithContext(ctx).Raw(query, contractorID, landfillID, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ReportRepository) ListEventsByContractor(
	ctx context.Context,
	contractorID uuid.UUID,
	landfillID uuid.UUID,
	from, to time.Time,
) ([]model.TripDetail, error) {
	query := `
		SELECT
			ae.event_time AS event_time,
			COALESCE(ae.normalized_plate, ae.raw_plate) AS plate,
			lf.id AS polygon_id,
			lf.name AS polygon_name,
			ae.contractor_id,
			org.name AS contractor_name,
			ae.snow_volume_m3
		FROM anpr_events ae
		JOIN organizations lf
		  ON lf.type = 'LANDFILL'
		 AND LOWER(lf.name) = ` + cameraLandfillNameExpr + `
		JOIN organizations org ON org.id = ae.contractor_id
		WHERE lf.id = ?
			AND ae.contractor_id = ?
			AND org.type = 'CONTRACTOR'
			AND org.name NOT ILIKE 'TEST%'
			AND ae.matched_snow = true
			AND ae.event_time >= ?
			AND ae.event_time < ?
		ORDER BY event_time ASC
	`

	var rows []model.TripDetail
	if err := r.db.WithContext(ctx).Raw(query, landfillID, contractorID, from, to).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
