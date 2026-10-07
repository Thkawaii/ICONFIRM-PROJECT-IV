package models

import (
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// LicenseRenewalHistory = ประวัติการต่ออายุใบอนุญาต (ตาราง license_renewal_history)
//
// 1 แถว = การต่ออายุ 1 ครั้ง คือ "เลขใบเดิม → เลขใบใหม่"
//
//	EXP-001 → EXP-002   (ครั้งที่ 1)
//	EXP-002 → EXP-003   (ครั้งที่ 2)
//	EXP-003 → EXP-004   (ครั้งที่ 3)  → ใบปัจจุบันคือ EXP-004, ต่ออายุ 3 ครั้ง
//
// ระบบไม่เก็บ RenewalCount ไว้ในฐานข้อมูล แต่คำนวณจากการไล่โซ่ทุกครั้งที่อ่าน
// เพื่อไม่ให้ตัวเลขเพี้ยนเวลามีการอัปโหลดเพิ่ม
//
// คีย์ของการจับคู่คือ LicenseType + LicenseNo เสมอ — Import กับ Export
// ห้ามเชื่อมข้ามประเภทกัน แม้เลขใบจะบังเอิญซ้ำกัน
// ---------------------------------------------------------------------------

const (
	LicenseTypeImport = "IMPORT"
	LicenseTypeExport = "EXPORT"
)

// NormalizeLicenseType: แปลงค่าที่ผู้ใช้พิมพ์ในไฟล์เป็นรหัสประเภทมาตรฐาน
// ("" = ไม่ใช่ประเภทที่รู้จัก → ต้องแจ้ง error ไม่ใช่เดา)
func NormalizeLicenseType(v string) string {
	s := strings.ToUpper(strings.TrimSpace(v))
	s = strings.NewReplacer(" ", "", "-", "", "_", "", ".", "").Replace(s)
	switch {
	case s == "":
		return ""
	case strings.HasPrefix(s, "IMPORT"), strings.HasPrefix(s, "IMP"), strings.Contains(s, "นำเข้า"):
		return LicenseTypeImport
	case strings.HasPrefix(s, "EXPORT"), strings.HasPrefix(s, "EXP"), strings.Contains(s, "นำออก"), strings.Contains(s, "ส่งออก"):
		return LicenseTypeExport
	}
	return ""
}

// LicenseTypeLabel: ชื่อประเภทที่แสดงให้ผู้ใช้เห็น
func LicenseTypeLabel(t string) string {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case LicenseTypeImport:
		return "Import License (ใบอนุญาตนำเข้า)"
	case LicenseTypeExport:
		return "Export License (ใบอนุญาตนำออก)"
	}
	return t
}

type LicenseRenewalHistory struct {
	ID uint `gorm:"primaryKey"`

	// LicenseType + OldLicenseNo + NewLicenseNo = คีย์กันข้อมูลซ้ำ
	// อัปโหลดไฟล์เดิมซ้ำจะไม่สร้างประวัติซ้ำ
	LicenseType  string `gorm:"column:license_type;size:10;index;uniqueIndex:ux_lrh_type_old_new,priority:1;not null"`
	OldLicenseNo string `gorm:"column:old_license_no;size:60;index;uniqueIndex:ux_lrh_type_old_new,priority:2;not null"`
	NewLicenseNo string `gorm:"column:new_license_no;size:60;index;uniqueIndex:ux_lrh_type_old_new,priority:3;not null"`

	// RenewalNo = ครั้งที่ต่ออายุตามที่ระบุในไฟล์ (ถ้ามี) — ค่าที่แสดงจริงคำนวณจากโซ่
	RenewalNo int `gorm:"column:renewal_no;default:0"`

	RenewalDate   *time.Time `gorm:"column:renewal_date;index"`
	OldExpireDate *time.Time `gorm:"column:old_expire_date"`
	NewExpireDate *time.Time `gorm:"column:new_expire_date;index"`

	Remark string `gorm:"column:remark;type:text"`

	// GroupNo = คอลัมน์ NO. ของชีต "ต่ออายุ" ที่การต่ออายุครั้งนี้มาจาก เช่น "Completed 01" หรือ "117"
	// เก็บไว้ที่นี่ด้วยเพื่อให้รู้สถานะ "ปิดงานแล้ว" ได้แม้ตารางทะเบียนจะถูกล้างไป
	GroupNo string `gorm:"column:group_no;size:50;index"`

	FileName   string    `gorm:"column:file_name;size:255;index"`
	UploadedBy string    `gorm:"column:uploaded_by;size:100"`
	UploadedAt time.Time `gorm:"column:uploaded_at;index"`

	SortOrder int64 `gorm:"column:sort_order;index;default:0"`

	UserID uint
	User   User

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (LicenseRenewalHistory) TableName() string { return "license_renewal_history" }

// สถานะของใบอนุญาตในหน้า License Overview
//
// ใช้สถานะเดิมของระบบ (VALID / EXPIRING / EXPIRED / NO_DATE) ต่อไป
// แล้วเพิ่มสถานะของ "ใบเก่าที่ถูกต่ออายุไปแล้ว" ซึ่งต้องไม่ถูกนับ countdown อีก
const (
	LicenseStatusRenewed   = "RENEWED"   // ใบเดิมที่ถูกต่ออายุไปแล้ว (ประวัติ)
	LicenseStatusCompleted = "COMPLETED" // ปิดงานแล้ว
)
