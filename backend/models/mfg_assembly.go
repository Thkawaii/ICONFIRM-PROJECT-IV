package models

import "time"

const (
	MFGStatusMatched       = "MATCHED"
	MFGStatusNotMatched    = "NOT_MATCHED"
	MFGStatusDuplicate     = "DUPLICATE"
	MFGStatusRetiredFormat = "RETIRED_FORMAT"
)

type MFGAssembly struct {
	ID uint `gorm:"primaryKey"`

	DateAssembly *time.Time `gorm:"column:date_assembly"`

	MachineNo string `gorm:"column:machine_no;size:60;index"`

	ITControllerNo string `gorm:"column:no;size:40;index"`

	PartNo string `gorm:"column:part_no;size:100"`

	SerialNo string `gorm:"column:serial_no;size:100;index"`

	Component string `gorm:"column:component;size:10;index"`

	Country string `gorm:"column:country;size:100"`

	CheckDate *time.Time `gorm:"column:check_date"`

	Status string `gorm:"column:status;size:20;index"`

	RetiredDetail string `gorm:"column:retired_detail;size:255"`

	// FlowVersion = "V2" คือรายการที่บันทึกจากขั้นตอนใหม่
	// (WH จ่ายของตามแผน allocation → MFG สแกน Kanban แล้วยืนยัน)
	// รายการ V2 ใช้ผลตรวจที่บันทึกไว้ตอนยืนยัน ไม่ถูกคำนวณใหม่จากตาราง Planning เดิม
	FlowVersion string `gorm:"column:flow_version;size:10;index"`

	// QR บน Specification sheet ที่ MFG สแกน + ผลเทียบกับตาราง Planning (Specification sheet)
	QRCode     string `gorm:"column:qr_code;type:text"`
	SpecState  string `gorm:"column:spec_state;size:20"`
	SpecDetail string `gorm:"column:spec_detail;type:text"`

	WHMatched         bool       `gorm:"column:wh_matched;index"`
	WHLicenseNo       string     `gorm:"column:wh_license_no;size:50"`
	WHInvoiceNo       string     `gorm:"column:wh_invoice_no;size:50"`
	WHProductionNo    string     `gorm:"column:wh_production_no;size:30"`
	WHModel           string     `gorm:"column:wh_model;size:50"`
	WHCheckedBy       string     `gorm:"column:wh_checked_by;size:100"`
	WHCheckedDatetime *time.Time `gorm:"column:wh_checked_datetime"`

	ComponentLabel string `gorm:"-"`

	// SpecCode / ITDevice: เติมตอนอ่าน จากไฟล์ Planning MFG (ใช้ในตาราง QA)
	SpecCode      string `gorm:"-"`
	ITDevice      string `gorm:"-"`
	WHRequired    bool   `gorm:"-"`
	WHPartType    string `gorm:"-"`
	WHMatchStatus string `gorm:"-"`
	WHMessage     string `gorm:"-"`

	PlanITControllerNo string `gorm:"-"`
	PlanState          string `gorm:"-"`
	PlanComponent      string `gorm:"-"`
	PlanComponentLabel string `gorm:"-"`
	PlanMatched        bool   `gorm:"-"`
	PlanMessage        string `gorm:"-"`
	PlanDetail         string `gorm:"-"`
	PlanOwnerMachineNo string `gorm:"-"`

	CreatedBy       string    `gorm:"column:created_by;size:100"`
	CreatedDatetime time.Time `gorm:"column:created_datetime"`
	UpdatedDatetime time.Time `gorm:"column:updated_datetime"`

	PhotoURL string `gorm:"column:photo_url;size:255"`

	UserID uint
	User   User
}
