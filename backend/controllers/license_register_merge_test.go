package controllers

import (
	"testing"
	"time"

	"iconfirm/config"
	"iconfirm/models"
)

func regLine(imp, exp string, total, stock, remain int) models.LicenseRenewal {
	return models.LicenseRenewal{
		ImportLicenseNo: imp,
		ExportLicenseNo: exp,
		Country:         "THAILAND",
		GroupNo:         "Completed 01",
		Total:           total,
		Stock:           stock,
		Remain:          remain,
		HasRemain:       true,
	}
}

func regMerge(t *testing.T, file string, rows ...models.LicenseRenewal) registerMergeResult {
	t.Helper()
	res, err := mergeLicenseRegister(rows, file, 1, time.Now())
	if err != nil {
		t.Fatalf("merge %s: %v", file, err)
	}
	return res
}

func regAll(t *testing.T) []models.LicenseRenewal {
	t.Helper()
	var rows []models.LicenseRenewal
	config.DB.Order("sort_order asc, id asc").Find(&rows)
	return rows
}

func regFind(rows []models.LicenseRenewal, exp string) *models.LicenseRenewal {
	for i := range rows {
		if NormalizeCodeValue(rows[i].ExportLicenseNo) == NormalizeCodeValue(exp) {
			return &rows[i]
		}
	}
	return nil
}

// ไฟล์ใหม่ต้องไม่ลบทะเบียนของไฟล์อื่น (ตัวอย่างจากผู้ใช้: IMP-001 / IMP-002 ต้องยังอยู่หลังอัป IMP-003)
func TestRegisterNewFileAccumulatesAcrossFiles(t *testing.T) {
	newTestDB(t)

	regMerge(t, "Export-มกรา.xlsx",
		regLine("IMP-001", "E001", 100, 60, 40),
		regLine("IMP-002", "E002", 50, 20, 30))
	res := regMerge(t, "Export-กุมภา.xlsx",
		regLine("IMP-003", "E003", 80, 10, 70))

	if res.Created != 1 || res.Deleted != 0 || res.Updated != 0 {
		t.Fatalf("result = %+v, want created=1 only", res)
	}
	rows := regAll(t)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (IMP-001, IMP-002 must survive)", len(rows))
	}
	if regFind(rows, "E001") == nil || regFind(rows, "E002") == nil || regFind(rows, "E003") == nil {
		t.Fatalf("missing rows after new-file upload: %+v", rows)
	}
}

// ไฟล์ชื่อเดิม: แก้ค่า → อัปเดต, ลบออกจากไฟล์ → ลบ, เพิ่มแถวต่ออายุ → เพิ่ม
func TestRegisterSameFileSyncsEditDeleteAdd(t *testing.T) {
	newTestDB(t)

	regMerge(t, "Export-มกรา.xlsx",
		regLine("IMP-001", "E001", 100, 60, 40),
		regLine("IMP-002", "E002", 50, 20, 30))
	regMerge(t, "Export-กุมภา.xlsx", regLine("IMP-003", "E003", 80, 10, 70))

	res := regMerge(t, "Export-มกรา.xlsx",
		regLine("IMP-001", "E001", 100, 50, 50), // แก้ STOCK/REMAIN
		regLine("IMP-001", "E004", 100, 10, 10)) // ต่ออายุเพิ่มในไฟล์เดิม

	if res.Updated != 1 || res.Created != 1 || res.Deleted != 1 {
		t.Fatalf("result = %+v, want updated=1 created=1 deleted=1 (E002 removed)", res)
	}
	rows := regAll(t)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (E001, E004 from Jan; E003 from Feb)", len(rows))
	}
	if e := regFind(rows, "E001"); e == nil || e.Stock != 50 || e.Remain != 50 {
		t.Fatalf("E001 not updated: %+v", e)
	}
	if regFind(rows, "E002") != nil {
		t.Fatalf("E002 should be deleted (removed from same file)")
	}
	if regFind(rows, "E003") == nil {
		t.Fatalf("E003 from other file must not be touched")
	}
}

// ไฟล์ชื่อใหม่ที่มีใบเดิมอยู่แล้ว → อัปเดตแถวเดิม ไม่สร้างซ้ำ
func TestRegisterNewFileSameKeyUpdatesNotDuplicates(t *testing.T) {
	newTestDB(t)

	regMerge(t, "Export-มกรา.xlsx", regLine("IMP-001", "E001", 100, 60, 40))
	res := regMerge(t, "Export-กุมภา.xlsx", regLine("IMP-001", "E001", 100, 45, 55))

	if res.Created != 0 || res.Updated != 1 {
		t.Fatalf("result = %+v, want updated=1 created=0", res)
	}
	rows := regAll(t)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 (no duplicate)", len(rows))
	}
	if rows[0].Stock != 45 || rows[0].FileName != "Export-กุมภา.xlsx" {
		t.Fatalf("row = %+v, want stock 45 and file moved to new file", rows[0])
	}
}

// แถวที่กรอกเองต้องไม่ถูกลบเมื่ออัปไฟล์ และต้องอยู่ท้ายสุดของทะเบียน
func TestRegisterKeepsManualRowsLast(t *testing.T) {
	db := newTestDB(t)

	manual := regLine("IMP-009", "E009", 10, 5, 5)
	manual.ManualEntry = true
	db.Create(&manual)

	regMerge(t, "Export-มกรา.xlsx", regLine("IMP-001", "E001", 100, 60, 40))
	regMerge(t, "Export-กุมภา.xlsx", regLine("IMP-002", "E002", 50, 20, 30))

	rows := regAll(t)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (manual row kept)", len(rows))
	}
	last := rows[len(rows)-1]
	if !last.ManualEntry || NormalizeCodeValue(last.ExportLicenseNo) != "E009" {
		t.Fatalf("manual row must be last, got last = %+v", last)
	}
}

// แถวที่กรอกเอง ถ้าไฟล์มีคีย์เดียวกัน → ไฟล์ชนะ (กลายเป็นแถวจากไฟล์ ไม่ซ้ำ)
func TestRegisterFileWinsOverManualSameKey(t *testing.T) {
	db := newTestDB(t)

	manual := regLine("IMP-001", "E001", 100, 60, 40)
	manual.ManualEntry = true
	db.Create(&manual)

	regMerge(t, "Export-มกรา.xlsx", regLine("IMP-001", "E001", 100, 50, 50))

	rows := regAll(t)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 (no duplicate with manual)", len(rows))
	}
	if rows[0].ManualEntry || rows[0].Stock != 50 {
		t.Fatalf("row = %+v, want file value and manual flag cleared", rows[0])
	}
}

// แถวที่ไม่มี Export License ที่มีคีย์ซ้ำกันในไฟล์เดียว ต้องแยกเป็นคนละแถวและอัปเดตได้ถูกตัว
func TestRegisterDuplicateKeysInFileStayDistinct(t *testing.T) {
	newTestDB(t)

	a := regLine("IMP-001", "", 60, 0, 0)
	b := regLine("IMP-001", "", 40, 0, 0)
	regMerge(t, "Export-มกรา.xlsx", a, b)

	a.Total = 70
	res := regMerge(t, "Export-มกรา.xlsx", a, b)
	if res.Created != 0 || res.Updated != 2 || res.Deleted != 0 {
		t.Fatalf("result = %+v, want updated=2 and no duplicates", res)
	}
	rows := regAll(t)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
}
