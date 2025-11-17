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

type ActRepository struct {
	db *gorm.DB
}

func NewActRepository(db *gorm.DB) *ActRepository {
	return &ActRepository{db: db}
}

func (r *ActRepository) GetContract(ctx context.Context, id uuid.UUID) (*model.Contract, error) {
	var row struct {
		ID             uuid.UUID
		ContractorID   uuid.UUID
		CustomerOrgID  uuid.UUID
		Name           string
		PricePerM3     float64
		BudgetTotal    float64
		StartAt        time.Time
		EndAt          time.Time
		ContractorName string
		ContractorType string
		ContractorBIN  string
		ContractorHead string
		ContractorAddr string
		ContractorPhone string
		CustomerName   string
		CustomerType   string
		CustomerBIN    string
		CustomerHead   string
		CustomerAddr   string
		CustomerPhone  string
	}

	err := r.db.WithContext(ctx).Raw(`
		SELECT
			c.id,
			c.contractor_id,
			c.created_by_org AS customer_org_id,
			c.name,
			c.price_per_m3,
			c.budget_total,
			c.start_at,
			c.end_at,
			contractor.name AS contractor_name,
			contractor.type AS contractor_type,
			contractor.bin AS contractor_bin,
			contractor.head_full_name AS contractor_head,
			contractor.address AS contractor_addr,
			contractor.phone AS contractor_phone,
			customer.name AS customer_name,
			customer.type AS customer_type,
			customer.bin AS customer_bin,
			customer.head_full_name AS customer_head,
			customer.address AS customer_addr,
			customer.phone AS customer_phone
		FROM contracts c
		JOIN organizations contractor ON contractor.id = c.contractor_id
		JOIN organizations customer ON customer.id = c.created_by_org
		WHERE c.id = ?
	`, id).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}

	return &model.Contract{
		ID:            row.ID,
		ContractorID:  row.ContractorID,
		CustomerOrgID: row.CustomerOrgID,
		Name:          row.Name,
		PricePerM3:    row.PricePerM3,
		BudgetTotal:   row.BudgetTotal,
		StartAt:       row.StartAt,
		EndAt:         row.EndAt,
		Contractor: model.Organization{
			ID:           row.ContractorID,
			Name:         row.ContractorName,
			Type:         row.ContractorType,
			BIN:          row.ContractorBIN,
			HeadFullName: row.ContractorHead,
			Address:      row.ContractorAddr,
			Phone:        row.ContractorPhone,
		},
		Customer: model.Organization{
			ID:           row.CustomerOrgID,
			Name:         row.CustomerName,
			Type:         row.CustomerType,
			BIN:          row.CustomerBIN,
			HeadFullName: row.CustomerHead,
			Address:      row.CustomerAddr,
			Phone:        row.CustomerPhone,
		},
	}, nil
}

func (r *ActRepository) ListTripsForPeriod(
	ctx context.Context,
	contractID uuid.UUID,
	from, to time.Time,
	statuses []string,
) ([]model.TripForAct, error) {
	baseQuery := `
		SELECT
			tr.id,
			COALESCE(tr.detected_volume_entry, 0) AS volume_m3,
			tr.entry_at
		FROM trips tr
		JOIN tickets t ON t.id = tr.ticket_id
		WHERE t.contract_id = ?
			AND tr.entry_at >= ?
			AND tr.entry_at < ?
			AND NOT EXISTS (
				SELECT 1 FROM act_trip at WHERE at.trip_id = tr.id
			)
	`
	args := []interface{}{contractID, from, to}
	var filters []string
	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i := range statuses {
			placeholders[i] = "?"
		}
		filters = append(filters, fmt.Sprintf("tr.status IN (%s)", strings.Join(placeholders, ",")))
		for _, status := range statuses {
			args = append(args, status)
		}
	}

	if len(filters) > 0 {
		baseQuery += " AND " + strings.Join(filters, " AND ")
	}
	baseQuery += " ORDER BY tr.entry_at ASC"

	var trips []model.TripForAct
	if err := r.db.WithContext(ctx).Raw(baseQuery, args...).Scan(&trips).Error; err != nil {
		return nil, err
	}
	return trips, nil
}

func (r *ActRepository) SumActs(ctx context.Context, contractID uuid.UUID) (float64, error) {
	var total float64
	if err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(amount_wo_vat), 0) FROM act WHERE contract_id = ?
	`, contractID).Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *ActRepository) CreateAct(ctx context.Context, act model.Act, tripIDs []uuid.UUID) (*model.Act, error) {
	var saved model.Act
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Raw(`
			INSERT INTO act (
				contract_id,
				contractor_id,
				act_number,
				act_date,
				period_start,
				period_end,
				total_volume_m3,
				price_per_m3,
				amount_wo_vat,
				vat_rate,
				vat_amount,
				amount_with_vat,
				status,
				created_by_org_id,
				created_by_user_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			RETURNING
				id,
				contract_id,
				contractor_id,
				act_number,
				act_date,
				period_start,
				period_end,
				total_volume_m3,
				price_per_m3,
				amount_wo_vat,
				vat_rate,
				vat_amount,
				amount_with_vat,
				status,
				created_by_org_id,
				created_by_user_id,
				created_at
		`,
			act.ContractID,
			act.ContractorID,
			act.ActNumber,
			act.ActDate,
			act.PeriodStart,
			act.PeriodEnd,
			act.TotalVolumeM3,
			act.PricePerM3,
			act.AmountWoVAT,
			act.VATRate,
			act.VATAmount,
			act.AmountWithVAT,
			act.Status,
			act.CreatedByOrgID,
			act.CreatedByUserID,
		).Scan(&saved).Error
		if err != nil {
			return err
		}

		for _, tripID := range tripIDs {
			if err := tx.Exec(`
				INSERT INTO act_trip (act_id, trip_id)
				VALUES (?, ?)
			`, saved.ID, tripID).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}
