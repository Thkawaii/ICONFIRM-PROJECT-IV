package models

import "time"

type ColumnAlias struct {
	ID uint `gorm:"primaryKey" json:"id"`

	Scope string `gorm:"column:table;size:40;index;not null" json:"table"`

	Source string `gorm:"column:new;size:150;not null" json:"new"`

	Target string `gorm:"column:old;size:150;not null" json:"old"`

	Kind string `gorm:"size:20" json:"kind"`

	Note       string    `gorm:"size:255" json:"note"`
	UploadDate time.Time `json:"upload_date"`
	UserID     uint      `json:"user_id"`
}

type CodeAlias struct {
	ID uint `gorm:"primaryKey" json:"id"`

	ComponentType string `gorm:"size:50;index" json:"component_type"`

	Kind string `gorm:"size:20;index" json:"kind"`

	FromCode string `gorm:"column:new;size:150;not null" json:"new"`

	ToOld string `gorm:"column:old;size:150;index;not null" json:"old"`

	Note       string    `gorm:"size:255" json:"note"`
	UploadDate time.Time `json:"upload_date"`
	UserID     uint      `json:"user_id"`
}

func (CodeAlias) TableName() string {
	return "change_format_parts"
}
