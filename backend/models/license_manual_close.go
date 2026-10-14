package models

import "time"

// ---------------------------------------------------------------------------
// LicenseManualClose — บันทึกว่า "คนกดอนุมัติปิดใบนี้เอง"
//
// ใบอนุญาตนำออกจะเสร็จสิ้นเองได้ทางเดียวคือ REMAIN = 0 (ของถูกนำออกหมดแล้ว)
// ส่วนใบที่ REMAIN ยังไม่เป็น 0 แต่หน้างานรู้ว่าจบแล้ว — เช่น ship ของไปแล้ว
// ไม่ได้ต่ออายุ ของเลยค้างอยู่ในระบบ — ระบบจะไม่ปิดให้เอง ต้องมีคนกดอนุมัติ
//
// เก็บเป็นตารางแยกแทนที่จะเขียนทับคอลัมน์ completed ของ license_items เพราะ
//
//   - ต้องรู้ว่าใครปิด ปิดเมื่อไหร่ ด้วยเหตุผลอะไร (ตรวจย้อนหลังได้)
//   - ต้องยกเลิกได้โดยไม่ไปแตะข้อมูลเครื่องรายตัว
//   - ถ้าภายหลังมีการอัปโหลดทะเบียนใหม่ทับ ข้อมูลการอนุมัติต้องไม่หายไปด้วย
//
// ผูกกับ "เลขใบปัจจุบัน" ของโซ่ (CurrentLicenseNo) ตัวเดียวกับที่หน้า
// License Overview ใช้เป็นคีย์ ไม่ใช่เลขใบตั้งต้น เพราะผู้ใช้เห็นและกดจากเลขนี้
// ---------------------------------------------------------------------------

type LicenseManualClose struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// LicenseType = IMPORT / EXPORT (models.LicenseTypeImport / LicenseTypeExport)
	LicenseType string `gorm:"column:license_type;size:16;index:idx_manual_close_key,unique" json:"licenseType"`

	// LicenseNo = เลขใบปัจจุบันของโซ่ ตรงกับ CurrentLicenseNo ในหน้า Overview
	LicenseNo string `gorm:"column:license_no;size:64;index:idx_manual_close_key,unique" json:"licenseNo"`

	// RemainAtClose = ยอดคงเหลือ ณ ตอนที่กดปิด เก็บไว้ดูย้อนหลังว่าปิดทั้งที่ค้างเท่าไร
	RemainAtClose int `gorm:"column:remain_at_close" json:"remainAtClose"`

	// Note = เหตุผลที่ปิด เช่น "ship ไปแล้ว ไม่ได้ต่ออายุ"
	Note string `gorm:"column:note;size:255" json:"note"`

	ClosedBy string    `gorm:"column:closed_by;size:128" json:"closedBy"`
	ClosedAt time.Time `gorm:"column:closed_at" json:"closedAt"`
}

func (LicenseManualClose) TableName() string { return "license_manual_closes" }

// LicenseManualCloseKey: คีย์ที่ใช้จับคู่กับแถวในหน้า Overview
func LicenseManualCloseKey(licenseType, licenseNo string) string {
	return licenseType + "|" + licenseNo
}
