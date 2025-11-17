package model

import (
	"time"

	"github.com/google/uuid"
)

type Contract struct {
	ID             uuid.UUID
	ContractorID   uuid.UUID
	CustomerOrgID  uuid.UUID
	Name           string
	PricePerM3     float64
	BudgetTotal    float64
	StartAt        time.Time
	EndAt          time.Time
	Contractor     Organization
	Customer       Organization
}
