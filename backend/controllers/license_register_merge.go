package controllers

import (
	"strconv"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// ทะเบียนใบอนุญาต (ชีต "ต่ออายุ") — ซิงก์แบบเดียวกับทะเบียนอะไหล่ / Export License
//
// ใช้ร่วมกันทั้งการอัปชีตทะเบียนโดยตรง (/license-renewal/upload)
// และการอัป Import / Export ที่มีชีตทะเบียนติดมาด้วย (AbsorbLedgerSheet / ประวัติต่ออายุ)
//
//   - เทียบรายแถวด้วยคีย์
//       มีเลข Export License         → ใช้เลขนั้น (1 ใบนำออก = 1 แถว)
//       ยังไม่มี Export License      → ใช้ เลขใบนำเข้า + ประเทศ + NO.
//   - ไฟล์ชื่อเดิม (ชื่อเดียวกัน) = ซิงก์กับไฟล์ แถวที่หายจากไฟล์จะถูกลบ
//   - ไฟล์ชื่อใหม่ = เพิ่มข้อมูล ไม่ลบแถวของไฟล์อื่น
//   - แถวเดิมที่คีย์ตรงกับไฟล์ จะถูกอัปเดตตามไฟล์ และย้ายไปอยู่ในชื่อไฟล์ล่าสุด
//   - แถวที่กรอกเองในระบบ (manual_entry) ไม่ถูกลบโดยการซิงก์ ถ้าคีย์ตรงกับไฟล์ ไฟล์ชนะ
//   - ลำดับ: ไฟล์ใหม่ต่อท้าย และแถวที่กรอกเองอยู่ท้ายสุดของทะเบียนเสมอ
// ---------------------------------------------------------------------------

// registerRowKey คีย์ที่ใช้เทียบแถวทะเบียน
func registerRowKey(r *models.LicenseRenewal) string {
	if exp := NormalizeCodeValue(r.ExportLicenseNo); exp != "" {
		return "E|" + exp
	}
	return "I|" + NormalizeCodeValue(r.ImportLicenseNo) + "|" +
		NormalizeCodeValue(r.Country) + "|" + NormalizeCodeValue(r.GroupNo)
}

// clampRegisterRow ตัดค่าที่ยาวเกินคอลัมน์ — ค่าเดียวที่ยาวเกินทำให้ทั้งไฟล์บันทึกไม่ได้
func clampRegisterRow(r *models.LicenseRenewal) {
	r.GroupNo = clampRunes(r.GroupNo, 50)
	r.ITControllerModel = clampRunes(r.ITControllerModel, 60)
	r.ImportLicenseNo = clampRunes(r.ImportLicenseNo, 60)
	r.ExportLicenseNo = clampRunes(r.ExportLicenseNo, 60)
	r.Country = clampRunes(r.Country, 120)
	r.FileName = clampRunes(r.FileName, 255)
}

// registerUpdateFields คอลัมน์ที่อัปเดตเมื่อแถวเดิมถูกจับคู่กับแถวในไฟล์
func registerUpdateFields(r *models.LicenseRenewal) map[string]interface{} {
	return map[string]interface{}{
		"group_no":            r.GroupNo,
		"it_controller_model": r.ITControllerModel,
		"import_license_no":   r.ImportLicenseNo,
		"export_license_no":   r.ExportLicenseNo,
		"country":             r.Country,
		"note":                r.Note,
		"total":               r.Total,
		"issue_date":          r.IssueDate,
		"expire_date":         r.ExpireDate,
		"stock":               r.Stock,
		"remain":              r.Remain,
		"has_remain":          r.HasRemain,
		"email_date":          r.EmailDate,
		"payment_date":        r.PaymentDate,
		"received_date":       r.ReceivedDate,
		"extra_json":          r.ExtraJSON,
		"manual_entry":        false,
		"file_name":           r.FileName,
		"upload_date":         r.UploadDate,
		"user_id":             r.UserID,
		"sort_order":          r.SortOrder,
	}
}

// registerMergeResult สรุปผลการซิงก์ทะเบียนหนึ่งครั้ง
type registerMergeResult struct {
	Created int
	Updated int
	Deleted int
}

// mergeLicenseRegister รวมแถวจากไฟล์เข้ากับทะเบียนที่มีอยู่แล้ว (ดูกติกาที่หัวไฟล์)
func mergeLicenseRegister(fileRows []models.LicenseRenewal, fileName string, userID uint, now time.Time) (registerMergeResult, error) {
	var res registerMergeResult
	if len(fileRows) == 0 {
		return res, nil
	}

	var existing []models.LicenseRenewal
	if err := config.DB.Order("sort_order asc, id asc").Find(&existing).Error; err != nil {
		return res, err
	}

	// จัดกลุ่มแถวเดิมตามคีย์ (เรียงตามลำดับเดิม) เพื่อจับคู่แบบ "ซ้ำได้" ตามลำดับการปรากฏ
	byKey := map[string][]int{}
	for i := range existing {
		k := registerRowKey(&existing[i])
		byKey[k] = append(byKey[k], i)
	}

	sortBase := uploadSortBase(func() *gorm.DB {
		return config.DB.Model(&models.LicenseRenewal{})
	}, fileName)

	claimed := make([]bool, len(existing))
	occurrence := map[string]int{}
	var toCreate, toUpdate []models.LicenseRenewal
	for i := range fileRows {
		row := fileRows[i]
		row.ID = 0
		row.FileName = fileName
		row.UploadDate = now
		row.UserID = userID
		row.ManualEntry = false
		row.SortOrder = sortBase + int64(i)
		clampRegisterRow(&row)

		k := registerRowKey(&row)
		j := occurrence[k]
		occurrence[k]++
		if idxs := byKey[k]; j < len(idxs) {
			idx := idxs[j]
			claimed[idx] = true
			row.ID = existing[idx].ID
			toUpdate = append(toUpdate, row)
			continue
		}
		toCreate = append(toCreate, row)
	}

	// ลบเฉพาะแถวของไฟล์ชื่อเดียวกันที่ไม่อยู่ในไฟล์รอบนี้แล้ว (แถวที่กรอกเองไม่ลบ)
	fileKey := normFileName(fileName)
	var deleteIDs []uint
	for i := range existing {
		r := &existing[i]
		if claimed[i] || r.ManualEntry || fileKey == "" {
			continue
		}
		if normFileName(r.FileName) == fileKey {
			deleteIDs = append(deleteIDs, r.ID)
		}
	}

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, part := range chunkSlice(deleteIDs, dbInListChunk) {
			if err := tx.Where("id IN ?", part).Delete(&models.LicenseRenewal{}).Error; err != nil {
				return err
			}
		}
		for i := range toUpdate {
			r := toUpdate[i]
			if err := tx.Model(&models.LicenseRenewal{}).Where("id = ?", r.ID).
				Updates(registerUpdateFields(&r)).Error; err != nil {
				return err
			}
		}
		for _, part := range chunkSlice(toCreate, 200) {
			if err := tx.Create(&part).Error; err != nil {
				return err
			}
		}
		return repositionManualRegisterRows(tx)
	})
	if err != nil {
		return res, err
	}

	res.Created = len(toCreate)
	res.Updated = len(toUpdate)
	res.Deleted = len(deleteIDs)
	return res, nil
}

// repositionManualRegisterRows ย้ายแถวที่กรอกเองไปไว้ท้ายสุดของทะเบียน
// เพราะระบบใช้ "แถวสุดท้ายของโซ่" ตัดสินว่าปิดงานแล้วหรือยัง
func repositionManualRegisterRows(tx *gorm.DB) error {
	var manual []models.LicenseRenewal
	if err := tx.Where("manual_entry IS TRUE").Order("sort_order asc, id asc").
		Find(&manual).Error; err != nil {
		return err
	}
	if len(manual) == 0 {
		return nil
	}
	var maxSort *int64
	if err := tx.Model(&models.LicenseRenewal{}).Where("manual_entry IS NOT TRUE").
		Select("MAX(sort_order)").Scan(&maxSort).Error; err != nil {
		return err
	}
	base := int64(0)
	if maxSort != nil {
		base = *maxSort + 1
	}
	moves := make([]rowReposition, 0, len(manual))
	for i, m := range manual {
		moves = append(moves, rowReposition{id: m.ID, sort: base + int64(i)})
	}
	return applyRepositions(tx, &models.LicenseRenewal{}, moves, "")
}

// registerSyncMessage ข้อความสรุปผลสำหรับผู้ใช้
func registerSyncMessage(res registerMergeResult, manual int64) string {
	msg := "อัปเดตทะเบียนใบอนุญาตแล้ว · เพิ่มใหม่ " + strconv.Itoa(res.Created) +
		" · อัปเดต " + strconv.Itoa(res.Updated) +
		" · ลบ " + strconv.Itoa(res.Deleted)
	if manual > 0 {
		msg += " (เก็บแถวที่กรอกในระบบไว้ " + strconv.FormatInt(manual, 10) + " แถว)"
	}
	return msg
}
