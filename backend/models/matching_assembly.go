package models

import "time"

type MatchingAssembly struct {
	ID uint `gorm:"primaryKey"`

	MachineNo string `gorm:"column:machine_no;size:30;index"`

	ITControllerSN string `gorm:"column:it_controller_sn;size:100;index"`

	Country string `gorm:"column:country;size:100"`

	Classification string `gorm:"column:classification;size:100"`

	AssemblyPartsNo string `gorm:"column:assembly_parts_no;size:100;index"`

	AssemblyPartsName string `gorm:"column:assembly_parts_name;size:150"`

	CreatedBy       string    `gorm:"column:created_by;size:100"`
	CreatedDatetime time.Time `gorm:"column:created_datetime"`
	UpdatedDatetime time.Time `gorm:"column:updated_datetime"`

	UserID uint
	User   User
}
