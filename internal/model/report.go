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
	Trips     []TripDetail `gorm:"-"`
}

type TripDetail struct {
	EventTime      time.Time
	Plate          *string
	PolygonID      *uuid.UUID
	PolygonName    *string
	ContractorID   *uuid.UUID
	ContractorName *string
	SnowVolumeM3   *float64
}

type ActReport struct {
	Mode        ReportMode
	Target      Organization
	PeriodStart time.Time
	PeriodEnd   time.Time
	TotalTrips  int64
	Groups      []TripGroup
}
