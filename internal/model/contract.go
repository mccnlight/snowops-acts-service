package model

import (
	"time"

	"github.com/google/uuid"
)

type Contract struct {
	ID            uuid.UUID
	ContractorID  uuid.UUID
	LandfillID    *uuid.UUID
	ContractType  string // "CONTRACTOR_SERVICE" или "LANDFILL_SERVICE"
	CustomerOrgID uuid.UUID
	Name          string
	PricePerM3    float64
	BudgetTotal   float64
	StartAt       time.Time
	EndAt         time.Time
	Contractor    Organization
	Customer      Organization
}
