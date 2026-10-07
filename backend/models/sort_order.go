package models

import "gorm.io/gorm"

// SortOrderBlock: ช่วงลำดับต่อไฟล์/ต่อแถวที่เพิ่มเอง (ต้องตรงกับ controllers.uploadSortBlock)
const SortOrderBlock int64 = 1_000_000

// แถวที่ไม่ได้มาจากการอัปโหลดไฟล์ (เพิ่มเอง / ระบบสร้าง) ให้ต่อท้ายตามลำดับที่เพิ่ม
func fillSortOrder(tx *gorm.DB, model interface{}, id uint, sortOrder int64) error {
	if sortOrder != 0 || id == 0 {
		return nil
	}
	return tx.Model(model).Where("id = ?", id).UpdateColumn("sort_order", int64(id)*SortOrderBlock).Error
}


func (m *UploadDataRow) AfterCreate(tx *gorm.DB) error {
	return fillSortOrder(tx, &UploadDataRow{}, m.ID, m.SortOrder)
}

func (m *LicenseItem) AfterCreate(tx *gorm.DB) error {
	return fillSortOrder(tx, &LicenseItem{}, m.ID, m.SortOrder)
}
