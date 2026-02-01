package repository

import (
    "context"
    "fmt"
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

func (r *ReportRepository) TripCountsByPolygon(
    ctx context.Context,
    contractorID uuid.UUID,
    from, to time.Time,
    statuses []string,
) ([]model.TripGroup, error) {
    baseQuery := `
        SELECT
            tr.polygon_id AS id,
            COALESCE(p.name, 'Unknown') AS name,
            COUNT(*) AS trip_count
        FROM trips tr
        JOIN tickets t ON t.id = tr.ticket_id
        LEFT JOIN polygons p ON p.id = tr.polygon_id
        WHERE t.contractor_id = ?
            AND tr.entry_at >= ?
            AND tr.entry_at < ?
            AND tr.polygon_id IS NOT NULL
    `
    args := []interface{}{contractorID, from, to}
    baseQuery, args = appendStatusFilter(baseQuery, args, statuses)
    baseQuery += " GROUP BY tr.polygon_id, p.name ORDER BY name ASC"

    var rows []model.TripGroup
    if err := r.db.WithContext(ctx).Raw(baseQuery, args...).Scan(&rows).Error; err != nil {
        return nil, err
    }
    return rows, nil
}

func (r *ReportRepository) TripCountsByContractor(
    ctx context.Context,
    polygonIDs []uuid.UUID,
    from, to time.Time,
    statuses []string,
) ([]model.TripGroup, error) {
    if len(polygonIDs) == 0 {
        return []model.TripGroup{}, nil
    }

    baseQuery := `
        SELECT
            t.contractor_id AS id,
            COALESCE(org.name, 'Unknown') AS name,
            COUNT(*) AS trip_count
        FROM trips tr
        JOIN tickets t ON t.id = tr.ticket_id
        LEFT JOIN organizations org ON org.id = t.contractor_id
        WHERE tr.polygon_id = ANY(?)
            AND tr.entry_at >= ?
            AND tr.entry_at < ?
    `
    args := []interface{}{polygonIDs, from, to}
    baseQuery, args = appendStatusFilter(baseQuery, args, statuses)
    baseQuery += " GROUP BY t.contractor_id, org.name ORDER BY name ASC"

    var rows []model.TripGroup
    if err := r.db.WithContext(ctx).Raw(baseQuery, args...).Scan(&rows).Error; err != nil {
        return nil, err
    }
	return rows, nil
}

func (r *ReportRepository) ListTripsByPolygon(
	ctx context.Context,
	contractorID uuid.UUID,
	polygonID uuid.UUID,
	from, to time.Time,
	statuses []string,
) ([]model.TripDetail, error) {
	baseQuery := `
		SELECT
			tr.id,
			tr.entry_at,
			tr.exit_at,
			tr.status,
			tr.polygon_id,
			p.name AS polygon_name,
			t.contractor_id,
			org.name AS contractor_name,
			tr.vehicle_plate_number,
			tr.detected_plate_number,
			tr.detected_volume_entry,
			tr.detected_volume_exit,
			tr.total_volume_m3
		FROM trips tr
		JOIN tickets t ON t.id = tr.ticket_id
		LEFT JOIN polygons p ON p.id = tr.polygon_id
		LEFT JOIN organizations org ON org.id = t.contractor_id
		WHERE t.contractor_id = ?
			AND tr.polygon_id = ?
			AND tr.entry_at >= ?
			AND tr.entry_at < ?
	`
	args := []interface{}{contractorID, polygonID, from, to}
	baseQuery, args = appendStatusFilter(baseQuery, args, statuses)
	baseQuery += " ORDER BY tr.entry_at ASC"

	var rows []model.TripDetail
	if err := r.db.WithContext(ctx).Raw(baseQuery, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ReportRepository) ListTripsByContractor(
	ctx context.Context,
	contractorID *uuid.UUID,
	polygonIDs []uuid.UUID,
	from, to time.Time,
	statuses []string,
) ([]model.TripDetail, error) {
	if len(polygonIDs) == 0 {
		return []model.TripDetail{}, nil
	}

	baseQuery := `
		SELECT
			tr.id,
			tr.entry_at,
			tr.exit_at,
			tr.status,
			tr.polygon_id,
			p.name AS polygon_name,
			t.contractor_id,
			org.name AS contractor_name,
			tr.vehicle_plate_number,
			tr.detected_plate_number,
			tr.detected_volume_entry,
			tr.detected_volume_exit,
			tr.total_volume_m3
		FROM trips tr
		JOIN tickets t ON t.id = tr.ticket_id
		LEFT JOIN polygons p ON p.id = tr.polygon_id
		LEFT JOIN organizations org ON org.id = t.contractor_id
		WHERE tr.polygon_id = ANY(?)
			AND tr.entry_at >= ?
			AND tr.entry_at < ?
	`
	args := []interface{}{polygonIDs, from, to}
	if contractorID == nil {
		baseQuery += " AND t.contractor_id IS NULL"
	} else {
		baseQuery += " AND t.contractor_id = ?"
		args = append(args, *contractorID)
	}
	baseQuery, args = appendStatusFilter(baseQuery, args, statuses)
	baseQuery += " ORDER BY tr.entry_at ASC"

	var rows []model.TripDetail
	if err := r.db.WithContext(ctx).Raw(baseQuery, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func appendStatusFilter(baseQuery string, args []interface{}, statuses []string) (string, []interface{}) {
    if len(statuses) == 0 {
        return baseQuery, args
    }

    placeholders := make([]string, len(statuses))
    for i := range statuses {
        placeholders[i] = "?"
    }
    baseQuery += fmt.Sprintf(" AND tr.status IN (%s)", strings.Join(placeholders, ","))
    for _, status := range statuses {
        args = append(args, status)
    }
    return baseQuery, args
}
