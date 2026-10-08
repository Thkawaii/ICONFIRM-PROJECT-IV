package models

import "time"

// DataStatus — บอกว่าแถวนี้เป็นข้อมูลที่ "ถูกต้อง" หรือ "ไม่ถูกต้อง"
const (
	AuditStatusValid   = "VALID"   // ข้อมูลถูกต้อง — ระบบบันทึกให้ตามปกติ
	AuditStatusInvalid = "INVALID" // ข้อมูลไม่ถูกต้อง — ระบบไม่รับ แต่เก็บไว้ตรวจย้อนหลัง
)

// AuditLog = ตารางประวัติการใช้งานระบบ (ตรวจสอบย้อนหลังได้)
//
// เก็บ 2 แบบในตารางเดียวกัน แยกด้วยคอลัมน์ DataStatus
//
//	VALID   → ทำรายการสำเร็จ เช่น จ่ายของ / ยืนยันประกอบ / แก้ไข / ลบ
//	INVALID → สแกนหรือกรอกข้อมูลผิด ระบบปฏิเสธไป แต่ยัง "เก็บของที่ผิดไว้"
//	          เพื่อให้ย้อนกลับมาดูได้ว่าใครสแกนอะไรผิด เมื่อไหร่ และที่ถูกต้องคืออะไร
//
// เวลาเป็น INVALID จะเก็บคู่กันไว้ในแถวเดียว อ่านแล้วเข้าใจทันที
//
//	WrongValue   = ค่าที่สแกน/กรอกเข้ามาจริง  (ของที่ผิด)
//	CorrectValue = ค่าที่ถูกต้องตามแผน/Master Data (ของที่ควรจะเป็น)
//	Reason       = ผิดเพราะอะไร
//
// ตัวอย่างแถว INVALID
//
//	source_table = WH_FLOW_ISSUE       machine_no    = YN30100003
//	action       = issue_invalid       component     = ITC
//	field_name   = P/N                 wrong_value   = YN22E00851X9
//	data_status  = INVALID             correct_value = YN22E00851F1
//	reason       = ไฟล์ Planning WH กำหนด P/N YN22E00851F1 แต่สแกนได้ YN22E00851X9
type AuditLog struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// ทำกับตาราง/ขั้นตอนไหน เช่น WH_FLOW_ISSUE, MFG_FLOW_CONFIRM, MFG_ASSEMBLY, USER
	SourceTable string `gorm:"size:64;index" json:"sourceTable"`

	// id ของแถวที่เกี่ยวข้อง (0 = ไม่มีแถวถูกบันทึก เช่น กรณีสแกนผิด)
	SourceID uint `json:"sourceId"`

	// ทำอะไร เช่น issue, confirm, scan_create, delete, issue_invalid
	Action string `gorm:"size:64;index" json:"action"`

	// สรุปผลสั้น ๆ ของรายการนั้น (ของเดิมที่มีอยู่แล้ว)
	ResultStatus string `json:"resultStatus"`

	// VALID / INVALID — แถวเก่าก่อนมีคอลัมน์นี้จะว่าง ให้ถือว่า VALID
	DataStatus string `gorm:"size:16;index" json:"dataStatus"`

	// บริบทของรายการ — ไว้กรองหาตอนตรวจย้อนหลัง
	MachineNo string `gorm:"size:64;index" json:"machineNo"`
	Component string `gorm:"size:32" json:"component"`

	// ข้อมูลที่ผิด + ข้อมูลที่ถูก (ใช้ตอน DataStatus = INVALID)
	FieldName    string `gorm:"size:64" json:"fieldName"`     // ผิดที่ช่องไหน เช่น P/N, S/N#, Product Spec
	WrongValue   string `gorm:"size:255" json:"wrongValue"`   // ค่าที่สแกนได้จริง
	CorrectValue string `gorm:"size:255" json:"correctValue"` // ค่าที่ถูกต้อง
	Reason       string `gorm:"size:500" json:"reason"`       // สาเหตุที่ผิด

	ActionDatetime time.Time `gorm:"index" json:"actionDatetime"`

	UserID uint   `json:"userId"`
	Name   string `json:"name"`

	User User `json:"-"`
}

// Invalid: แถวนี้เป็นข้อมูลที่ไม่ถูกต้องหรือไม่
func (a AuditLog) Invalid() bool { return a.DataStatus == AuditStatusInvalid }
