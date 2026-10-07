package models

import "time"

type AuditLog struct {
	ID uint `gorm:"primaryKey"`

	SourceTable string

	SourceID uint

	Action string

	ResultStatus string

	ActionDatetime time.Time

	UserID uint

	Name string

	User User
}
