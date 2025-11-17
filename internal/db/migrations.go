package db

import (
	"fmt"

	"gorm.io/gorm"
)

var migrationStatements = []string{
	`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`,
	`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`,
	`CREATE TABLE IF NOT EXISTS act (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		contract_id UUID NOT NULL REFERENCES contracts(id),
		contractor_id UUID NOT NULL REFERENCES organizations(id),
		act_number VARCHAR(64) NOT NULL,
		act_date DATE NOT NULL,
		period_start DATE NOT NULL,
		period_end DATE NOT NULL,
		total_volume_m3 NUMERIC(18,3) NOT NULL,
		price_per_m3 NUMERIC(18,4) NOT NULL,
		amount_wo_vat NUMERIC(18,2) NOT NULL,
		vat_rate NUMERIC(5,2) NOT NULL,
		vat_amount NUMERIC(18,2) NOT NULL,
		amount_with_vat NUMERIC(18,2) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'GENERATED',
		created_by_org_id UUID NOT NULL REFERENCES organizations(id),
		created_by_user_id UUID NOT NULL REFERENCES users(id),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_act_number ON act (act_number);`,
	`CREATE INDEX IF NOT EXISTS idx_act_contract_id ON act (contract_id);`,
	`CREATE TABLE IF NOT EXISTS act_trip (
		act_id UUID NOT NULL REFERENCES act(id) ON DELETE CASCADE,
		trip_id UUID NOT NULL REFERENCES trips(id),
		PRIMARY KEY (act_id, trip_id)
	);`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_act_trip_trip_id ON act_trip(trip_id);`,
}

func runMigrations(db *gorm.DB) error {
	for i, stmt := range migrationStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}
