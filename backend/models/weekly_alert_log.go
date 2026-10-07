package models

import "time"

const (
	WeeklyAlertAuto = "AUTO"

	WeeklyAlertManual = "MANUAL"
)

const (
	WeeklyAlertSent    = "SENT"
	WeeklyAlertFailed  = "FAILED"
	WeeklyAlertSkipped = "SKIPPED"
)

type WeeklyAlertLog struct {
	ID uint `gorm:"primaryKey"`

	WeekKey string `gorm:"size:16;index"`

	Mode string `gorm:"size:10;index"`

	Status string `gorm:"size:10;index"`

	Subject    string `gorm:"size:255"`
	Recipients string `gorm:"size:512"`

	ItemCount   int
	ExpiredCnt  int
	ExpiringCnt int
	LeadCnt     int

	Provider string `gorm:"size:16"`

	Error string `gorm:"size:1000"`

	TriggeredBy string `gorm:"size:100"`

	SentAt time.Time `gorm:"index"`
}
