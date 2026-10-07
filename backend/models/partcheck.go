package models

import "time"

type PartCheck struct {
	ID uint `gorm:"primaryKey"`

	PartType string `gorm:"size:10"`

	PN string `gorm:"size:100"`

	SN string `gorm:"size:100"`

	MachineNo string `gorm:"size:30;index"`

	ProductionNo string `gorm:"size:30"`

	LicenseNo string `gorm:"size:50;index"`
	InvoiceNo string `gorm:"size:50;index"`

	MatchStatus string `gorm:"size:20;index"`

	MatchMessage string `gorm:"size:255"`

	ImportLicenseItemID *uint

	CheckedBy string `gorm:"size:100"`

	CheckedDatetime time.Time

	ExpectedPN  string `gorm:"-"`
	MatchDetail string `gorm:"-"`

	UserID uint
	User   User
}
