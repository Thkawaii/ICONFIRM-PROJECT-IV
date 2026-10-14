package models

import "time"

const (
	DatasetPlanning = "planning"
	DatasetWH1      = "wh1"
	DatasetWH2      = "wh2"
	DatasetEngine   = "engine"

	// DatasetDailyPlan: Daily plan (ชีต Specification sheet) — ใช้เทียบกับ QR ที่ MFG สแกน
	DatasetDailyPlan = "daily_plan"

	// DatasetEngineITAlloc: ชีต "Engine_IT allocation" — แผนจ่าย Engine / IT# รายเครื่อง (MC#)
	DatasetEngineITAlloc = "engine_it_allocation"

	// DatasetCWCVITS: ชีต "CW_CV_ITS" — P/N ของ CW / CV / SM / MP / PH / Engine ตาม Product Spec
	DatasetCWCVITS = "cw_cv_its"

	// DatasetSpecSheet: ชีต "Specification sheet" ในไฟล์แผนผลิตรายเดือน (Planning PCP)
	// — สเปกรายเครื่อง ผูก MC# เข้ากับ Product Spec (spec code)
	// ไฟล์รูปแบบเดิมที่มีชีตชื่อ "Spec sheet_pcp" ก็ยังอ่านได้
	DatasetSpecSheet = "spec_sheet"

	// DatasetPCPDailyPlan: ชีต "Daily plan" ในไฟล์แผนผลิตรายเดือนไฟล์เดียวกัน
	//
	// แยกตารางจาก DatasetSpecSheet เพราะสองชีตนี้เปลี่ยนคนละจังหวะ — สเปกเครื่องนิ่ง
	// แต่แผนจ่ายถูกแก้บ่อย (ดูคำว่า Draft.00 ในชื่อไฟล์) ถ้าเก็บรวมตารางเดียว
	// อัปแผนรอบใหม่ทีจะทับสเปกไปด้วย
	//
	// และที่สำคัญกว่า: แยกแล้วจึงรู้ได้ว่าสองชีต "ไม่ตรงกัน" ซึ่งเป็นสัญญาณว่าคนแก้ไม่ครบ
	// ถ้ารวมกัน ระบบจะเลือกค่าหนึ่งแล้วจบ ความขัดแย้งหายไปโดยไม่มีใครรู้
	//
	// ไม่อยู่ใน UploadableDatasets เพราะไม่ได้อัปแยก — เขียนพร้อมกันตอนอัป Planning PCP
	DatasetPCPDailyPlan = "pcp_daily_plan"
)

// UploadableDatasets: ชุดข้อมูลที่ยังเปิดให้อัปโหลดในหน้า "อัพโหลดข้อมูล"
// ชุดอื่น (Planning / Daily Plan / WH1 / WH2 / Engine) ยังอ่านได้ แต่ไม่ให้อัปโหลดใหม่แล้ว
//
// Planning WH (Engine_IT allocation) ถูกถอดออกจากรายการนี้แล้ว
// P/N + S/N ของ IT Controller ไม่ได้มาจากไฟล์นั้นอีกต่อไป — ย้ายไปเทียบกับ
// ทะเบียนใบอนุญาตนำเข้าแทน (ดู wh_mfg_flow.go) ข้อมูลเก่าที่อัปไว้แล้วยังอ่านได้
// เพื่อให้เครื่องที่ค้างอยู่ในสายการผลิตเดินงานต่อจนจบได้
var UploadableDatasets = []string{
	DatasetSpecSheet,
	DatasetCWCVITS,
}

func IsUploadableDataset(dataset string) bool {
	for _, d := range UploadableDatasets {
		if d == dataset {
			return true
		}
	}
	return false
}

type UploadDataRow struct {
	ID uint `gorm:"primaryKey;index:idx_ud_ds_order,priority:2"`

	Dataset string `gorm:"size:20;index;not null;index:idx_ud_ds_order,priority:1"`

	MachineNo string `gorm:"size:100;index"`
	LotNo     string `gorm:"size:100;index"`
	OrderNo   string `gorm:"size:100;index"`
	PartsNo   string `gorm:"size:100;index"`
	KCMOrder  string `gorm:"size:100"`
	WorkOrder string `gorm:"size:100"`

	DataJSON string `gorm:"type:text"`

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
