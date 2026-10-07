package models

import "time"

const (
	LicenseItemPending   = "PENDING"
	LicenseItemConfirmed = "CONFIRMED"
)

const (
	MatchStatusMatch       = "MATCH"
	MatchStatusNotFound    = "NOT_FOUND"
	MatchStatusWrongInv    = "WRONG_INVOICE"
	MatchStatusWrongPart   = "WRONG_PART"
	MatchStatusWrongProd   = "WRONG_PRODNO"
	MatchStatusDuplicate   = "DUPLICATE"
	MatchStatusNotRequired = "NOT_REQUIRED"

	MatchStatusRetiredFormat = "RETIRED_FORMAT"
)

// LicenseItem = เครื่องหนึ่งเครื่องในบัญชีแสดงหมายเลขเครื่อง
//
// ชื่อเดิมคือ ImportLicenseItem (ตาราง import_license_items) ซึ่งสื่อไม่ตรง
// เพราะไฟล์บัญชีแสดงหมายเลขเครื่องมีทั้งเลขใบอนุญาตนำเข้าและเลขใบอนุญาตนำออก
// อยู่ในแถวเดียวกัน ตารางนี้จึงเป็นทะเบียนเครื่อง ไม่ใช่ทะเบียนใบนำเข้า
type LicenseItem struct {
	ID uint `gorm:"primaryKey"`

	Brand string `gorm:"size:100"`
	Model string `gorm:"size:50;index"`

	LicenseNo string `gorm:"size:50;index"`

	IssueDate *time.Time `gorm:"index"`

	ExpireDate *time.Time `gorm:"index"`

	InvoiceNo     string `gorm:"size:50;index"`
	DeclarationNo string `gorm:"size:50"`

	Qty int

	MachineNo string `gorm:"size:30;uniqueIndex;not null"`

	ProductionNo string `gorm:"size:30;index"`

	Remark        string `gorm:"type:text"`
	ExportCountry string `gorm:"size:100"`

	// ---- ใบอนุญาตนำออกที่ใช้ส่งเครื่องนี้ออก ----
	// บัญชีใบอนุญาตนำเข้าของจริงมีสองคอลัมน์นี้ต่อท้าย คู่กับ ExportCountry
	// (เลขใบอนุญาตนำออก · วันที่ออกใบอนุญาต · ส่งออกไปประเทศ)
	ExportLicenseNo string     `gorm:"size:60;index"`
	ExportIssueDate *time.Time `gorm:"index"`

	ExtraJSON string `gorm:"type:text" json:"extra_json"`

	ConfirmStatus     string `gorm:"size:20;index;default:PENDING"`
	ConfirmedBy       string `gorm:"size:100"`
	ConfirmedDatetime *time.Time

	Completed   bool   `gorm:"index;not null;default:false"`
	CompletedBy string `gorm:"size:100"`
	CompletedAt *time.Time

	FileName   string `gorm:"size:255"`
	UploadDate time.Time

	// SortOrder = ลำดับการแสดงผล ตามลำดับแถวในไฟล์ Excel ล่าสุดที่อัปโหลด
	SortOrder int64 `gorm:"column:sort_order;index;default:0"`

	UserID uint
	User   User

	// Locked = ถูกสแกนผ่านไปแล้ว แก้ไข/ลบไม่ได้ (คำนวณตอนอ่าน ไม่ได้เก็บในฐานข้อมูล)
	Locked     bool   `gorm:"-"`
	LockReason string `gorm:"-"`
}

const ImportLicenseValidityMonths = 6

// ApplyRemarkStatus ตั้งสถานะ "เสร็จสิ้น" จากช่องหมายเหตุ (Remark) ในไฟล์ Excel
//
// ผู้ใช้พิมพ์คำว่า "เสร็จสิ้น" / "เสร็จแล้ว" / "completed" ลงในช่องหมายเหตุของไฟล์
// พออัปโหลดเข้าระบบ แถวนั้นต้องขึ้นสถานะเสร็จสิ้นทันที ไม่ต้องมากดทีละรายการในหน้าเว็บ
// ใช้ตัวตัดสินตัวเดียวกับฝั่ง Export (NoteMeansCompleted) เพื่อให้สองฝั่งตีความเหมือนกันเป๊ะ
// รวมถึงกรณีปฏิเสธ เช่น "ยังไม่เสร็จ" ที่ต้องไม่ถือว่าเสร็จ
func (m *LicenseItem) ApplyRemarkStatus(by string, now time.Time) {
	if NoteMeansCompleted(m.Remark) {
		m.Completed = true
		if m.CompletedBy == "" {
			m.CompletedBy = by
		}
		if m.CompletedAt == nil {
			t := now
			m.CompletedAt = &t
		}
		return
	}
	// ลบคำว่าเสร็จสิ้นออกจากไฟล์แล้วอัปโหลดใหม่ = ย้อนสถานะกลับเป็นยังไม่เสร็จ
	m.Completed = false
	m.CompletedBy = ""
	m.CompletedAt = nil
}

func (m *LicenseItem) FillExpireDate() {
	if m.ExpireDate != nil || m.IssueDate == nil {
		return
	}
	exp := m.IssueDate.AddDate(0, ImportLicenseValidityMonths, 0)
	m.ExpireDate = &exp
}

// TableName: ตาราง license_items (ชื่อเดิม import_license_items)
//
// การเปลี่ยนชื่อตารางไม่ย้ายข้อมูลให้เอง — config.renameLegacyTables() เป็นคน
// ALTER TABLE ... RENAME TO ให้ตอนระบบเริ่มทำงาน ถ้าไม่มีขั้นตอนนั้น AutoMigrate
// จะสร้างตารางใหม่ที่ว่างเปล่า แล้วข้อมูลเดิมจะกลายเป็นตารางกำพร้า
func (LicenseItem) TableName() string { return "license_items" }

// ---------------------------------------------------------------------------
// ฝั่งใบอนุญาตนำออกของเครื่องนี้
//
// เครื่องหนึ่งเครื่องรู้ทั้งใบนำเข้าและใบนำออกของตัวเอง เพราะไฟล์บัญชีแสดงหมายเลข
// เครื่องมีทั้งสองคอลัมน์อยู่ในแถวเดียวกัน จึงไม่ต้องมีตารางใบนำออกแยกอีกตาราง
// ---------------------------------------------------------------------------

// ExportEffectiveExpireDate: วันหมดอายุของใบอนุญาตนำออกที่ส่งเครื่องนี้ออก
//
// ใบนำออกมีอายุ 1 เดือนนับจากวันที่ออกใบ นับแบบ clamp สิ้นเดือน
// (ออกวันที่ 31 ม.ค. → หมดอายุ 28/29 ก.พ. ไม่ใช่ 3 มี.ค.)
func (m *LicenseItem) ExportEffectiveExpireDate() *time.Time {
	if m.ExportIssueDate == nil {
		return nil
	}
	exp := AddMonthsClamped(*m.ExportIssueDate, ExportLicenseValidityMonths)
	return &exp
}

// ExportLeadTimeDate: วันครบกำหนดยื่นขอต่ออายุใบนำออกใบนี้ต่อ กสทช.
func (m *LicenseItem) ExportLeadTimeDate() *time.Time {
	exp := m.ExportEffectiveExpireDate()
	if exp == nil {
		return nil
	}
	lead := SubtractBusinessDays(*exp, ExportLicenseLeadDays)
	return &lead
}
