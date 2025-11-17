package model

import (
	"time"

	"github.com/google/uuid"
)

type Act struct {
	ID              uuid.UUID
	ContractID      uuid.UUID
	ContractorID    uuid.UUID
	ActNumber       string
	ActDate         time.Time
	PeriodStart     time.Time
	PeriodEnd       time.Time
	TotalVolumeM3   float64
	PricePerM3      float64
	AmountWoVAT     float64
	VATRate         float64
	VATAmount       float64
	AmountWithVAT   float64
	Status          string
	CreatedByOrgID  uuid.UUID
	CreatedByUserID uuid.UUID
	CreatedAt       time.Time
}

type TripForAct struct {
	ID       uuid.UUID
	VolumeM3 float64
	EntryAt  time.Time
}

type ActDocument struct {
	Act             Act
	Contract        Contract
	WorkDescription string
	PaidBefore      float64
	BudgetExceeded  bool
}
