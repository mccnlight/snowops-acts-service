package db

import (
	"fmt"

	"gorm.io/gorm"
)

var migrationStatements = []string{
	`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`,
	`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'act_status') THEN
			CREATE TYPE act_status AS ENUM ('GENERATED', 'PENDING_APPROVAL', 'APPROVED', 'REJECTED');
		END IF;
	END
	$$;`,
	`CREATE TABLE IF NOT EXISTS act (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		contract_id UUID NOT NULL REFERENCES contracts(id),
		contractor_id UUID REFERENCES organizations(id),
		landfill_id UUID REFERENCES organizations(id),
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
		status act_status NOT NULL DEFAULT 'GENERATED',
		rejection_reason TEXT,
		approved_by_org_id UUID REFERENCES organizations(id),
		approved_by_user_id UUID REFERENCES users(id),
		approved_at TIMESTAMPTZ,
		created_by_org_id UUID NOT NULL REFERENCES organizations(id),
		created_by_user_id UUID NOT NULL REFERENCES users(id),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'act' AND column_name = 'landfill_id') THEN
			ALTER TABLE act ADD COLUMN landfill_id UUID REFERENCES organizations(id);
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'act' AND column_name = 'status') THEN
			ALTER TABLE act ADD COLUMN status act_status NOT NULL DEFAULT 'GENERATED';
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'act' AND column_name = 'rejection_reason') THEN
			ALTER TABLE act ADD COLUMN rejection_reason TEXT;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'act' AND column_name = 'approved_by_org_id') THEN
			ALTER TABLE act ADD COLUMN approved_by_org_id UUID REFERENCES organizations(id);
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'act' AND column_name = 'approved_by_user_id') THEN
			ALTER TABLE act ADD COLUMN approved_by_user_id UUID REFERENCES users(id);
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'act' AND column_name = 'approved_at') THEN
			ALTER TABLE act ADD COLUMN approved_at TIMESTAMPTZ;
		END IF;
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'act' AND column_name = 'contractor_id' AND is_nullable = 'NO') THEN
			ALTER TABLE act ALTER COLUMN contractor_id DROP NOT NULL;
		END IF;
	END
	$$;`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uq_act_number ON act (act_number);`,
	`CREATE INDEX IF NOT EXISTS idx_act_contract_id ON act (contract_id);`,
	`CREATE INDEX IF NOT EXISTS idx_act_contractor_id ON act (contractor_id) WHERE contractor_id IS NOT NULL;`,
	`CREATE INDEX IF NOT EXISTS idx_act_landfill_id ON act (landfill_id) WHERE landfill_id IS NOT NULL;`,
	`CREATE INDEX IF NOT EXISTS idx_act_status ON act (status);`,
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
