package model

import "github.com/google/uuid"

type Organization struct {
	ID           uuid.UUID
	Name         string
	Type         string
	BIN          string
	HeadFullName string
	Address      string
	Phone        string
}
