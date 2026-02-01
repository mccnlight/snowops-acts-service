package model

import (
	"time"

	"github.com/google/uuid"
)

type ReportMode string

const (
	ReportModeContractor ReportMode = "CONTRACTOR"
	ReportModeLandfill   ReportMode = "LANDFILL"
)

type TripGroup struct {
	ID        uuid.UUID
	Name      string
	TripCount int64
	Trips     []TripDetail
}

type TripDetail struct {
	ID                  uuid.UUID
	EntryAt             time.Time
	ExitAt              *time.Time
	Status              string
	PolygonID           *uuid.UUID
	PolygonName         *string
	ContractorID        *uuid.UUID
	ContractorName      *string
	VehiclePlateNumber  *string
	DetectedPlateNumber *string
	DetectedVolumeEntry *float64
	DetectedVolumeExit  *float64
	TotalVolumeM3       *float64
}

type ActReport struct {
	Mode        ReportMode
	Target      Organization
	PeriodStart time.Time
	PeriodEnd   time.Time
	TotalTrips  int64
	Groups      []TripGroup
}
