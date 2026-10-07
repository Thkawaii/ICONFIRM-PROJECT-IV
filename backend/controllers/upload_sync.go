package controllers

import (
	"strings"

	"iconfirm/models"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// อัปโหลด "ไฟล์เดิม" (ชื่อไฟล์เดียวกัน) = ซิงก์กับไฟล์
//
//   - แถวที่มีในไฟล์         → เพิ่ม / อัปเดต / เหมือนเดิม (ดู upload_rematch.go)
//   - แถวที่ถูกลบออกจากไฟล์   → ลบออกจากระบบอัตโนมัติ ยกเว้นแถวที่สแกนผ่านแล้ว (ล็อก)
//   - ลำดับในระบบ            → เรียงตามลำดับแถวในไฟล์ล่าสุด
//
// อัปโหลด "ไฟล์ใหม่" (ชื่อไฟล์อื่น) = เพิ่มข้อมูล ไม่ลบแถวของไฟล์อื่น
// แถวของไฟล์ใหม่ต่อท้ายข้อมูลเดิม
// ---------------------------------------------------------------------------

// uploadSortBlock: แต่ละไฟล์ใช้ช่วงลำดับของตัวเอง (รองรับไฟล์ละไม่เกิน 1,000,000 แถว)
const uploadSortBlock = models.SortOrderBlock

func normFileName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func sameUploadFile(a, b string) bool {
	na := normFileName(a)
	return na != "" && na == normFileName(b)
}

// uploadSortBase หาลำดับเริ่มต้นของไฟล์นี้
//   - เคยอัปไฟล์ชื่อนี้แล้ว → ใช้ช่วงเดิม (แถวอยู่ตำแหน่งเดิมในตาราง)
//   - ไฟล์ใหม่             → ช่วงถัดจากข้อมูลล่าสุด (ต่อท้าย)
func uploadSortBase(scope func() *gorm.DB, fileName string) int64 {
	var minSort *int64
	scope().Where("file_name <> '' AND LOWER(TRIM(file_name)) = ?", normFileName(fileName)).
		Select("MIN(sort_order)").Scan(&minSort)
	if minSort != nil && *minSort > 0 {
		return (*minSort / uploadSortBlock) * uploadSortBlock
	}
	var maxSort *int64
	scope().Select("MAX(sort_order)").Scan(&maxSort)
	if maxSort == nil || *maxSort < 0 {
		return uploadSortBlock
	}
	return (*maxSort/uploadSortBlock + 1) * uploadSortBlock
}

type syncDelete struct {
	id     uint
	label  string
	locked bool
	reason string
}

func countSyncDeletes(list []syncDelete) (deletable, lockedKept int) {
	for _, d := range list {
		if d.locked {
			lockedKept++
		} else {
			deletable++
		}
	}
	return
}

func syncDeleteIDs(list []syncDelete) []uint {
	var ids []uint
	for _, d := range list {
		if !d.locked {
			ids = append(ids, d.id)
		}
	}
	return ids
}
