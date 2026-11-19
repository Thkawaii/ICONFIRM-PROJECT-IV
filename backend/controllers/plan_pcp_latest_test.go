package controllers

import (
	"encoding/json"
	"testing"

	"iconfirm/models"
)

// ---------------------------------------------------------------------------
// กติกา "หนึ่งเครื่อง = หนึ่งแถว เอาอันล่าสุด" ของแผน Planning PCP
//
//	ไฟล์เขียนเครื่องเดียวกันซ้ำ          → เก็บแถวล่างสุด
//	อัปไฟล์ใหม่ MC# เดิมแต่ค่าเปลี่ยน     → UPDATE แถวเดิม
//	อัปไฟล์ใหม่ MC# ใหม่                → INSERT
//	แถวซ้ำที่ค้างในระบบจากไฟล์รอบก่อน     → ยุบทิ้งให้เหลือแถวเดียว
// ---------------------------------------------------------------------------

// ชีต Daily plan ที่คนทำแผนพิมพ์เครื่องเดิมซ้ำ แล้วแก้ประเทศ/วันที่ไว้แถวล่างสุด
func TestDailyPlanKeepsLatestRowPerMachine(t *testing.T) {
	rows := [][]string{
		{"LOT NO.", "Machine", "Customer", "Q'ty", "Main line", "Stand by shipping"},
		{"7.154", "YN15437980", "Laos(X)", "1", "21-Jul-26", "22-Jul-26"},
		{"7.154", "YN15437980", "Laos(X)", "1", "21-Jul-26", "22-Jul-26"},
		{"7.154", "YN15437980", "Malaysia", "1", "22-Jul-26", "24-Jul-26"},
	}

	list := keepLatestDailyPlanRows(dailyPlanSheetRows(rows))
	if len(list) != 1 {
		t.Fatalf("เครื่องเดียวกัน 3 แถวต้องยุบเหลือ 1 แถว ได้ %d แถว", len(list))
	}
	got := list[0]
	if got.Machine != "YN15437980" {
		t.Errorf("Machine = %q ต้องเป็น YN15437980", got.Machine)
	}
	if got.Country != "Malaysia" {
		t.Errorf("Country = %q ต้องเป็น Malaysia (แถวล่างสุด)", got.Country)
	}
	if got.MainLine != "22-Jul-26" {
		t.Errorf("Main line = %q ต้องเป็น 22-Jul-26 (แถวล่างสุด)", got.MainLine)
	}
	if got.StandBy != "24-Jul-26" {
		t.Errorf("Stand by shipping = %q ต้องเป็น 24-Jul-26 (แถวล่างสุด)", got.StandBy)
	}
}

// แถวล่างสุดชนะทุกช่องที่กรอกมา ช่องที่เว้นว่างยังใช้ค่าเดิมต่อ
func TestKeepLatestMachineBlocksMergesBlankCells(t *testing.T) {
	blocks := []map[string]string{
		{
			"Machine":      "YN15437980",
			"Country Name": "Laos(X)",
			"Main line":    "21-Jul-26",
			"IT device":    "YN02B10321F1",
		},
		{
			"Machine":      "YN15437980",
			"Country Name": "Malaysia",
			"Main line":    "22-Jul-26",
		},
	}

	out := keepLatestMachineBlocks(blocks)
	if len(out) != 1 {
		t.Fatalf("ต้องยุบเหลือ 1 บล็อก ได้ %d", len(out))
	}
	if out[0]["Country Name"] != "Malaysia" {
		t.Errorf("Country Name = %q ต้องเป็น Malaysia", out[0]["Country Name"])
	}
	if out[0]["Main line"] != "22-Jul-26" {
		t.Errorf("Main line = %q ต้องเป็น 22-Jul-26", out[0]["Main line"])
	}
	// บล็อกล่างไม่ได้กรอก IT device ต้องไม่ทำให้ของเดิมหาย
	if out[0]["IT device"] != "YN02B10321F1" {
		t.Errorf("IT device = %q ต้องยังเป็น YN02B10321F1", out[0]["IT device"])
	}
}

func specSheetUploadRow(id uint, fileName, machine, country, mainLine string) models.UploadDataRow {
	data := map[string]string{
		"LOT NO.":      "7.154",
		"Machine":      machine,
		"Country Name": country,
		"Main line":    mainLine,
	}
	b, _ := json.Marshal(data)
	return models.UploadDataRow{
		ID:        id,
		Dataset:   models.DatasetSpecSheet,
		DataJSON:  string(b),
		FileName:  fileName,
		MachineNo: machine,
	}
}

// MC# เดิม → UPDATE แถวล่าสุด · MC# ใหม่ → INSERT · แถวซ้ำที่ค้าง → ยุบทิ้ง
func TestPlanningPCPUpdatesSameMachineAndCollapsesDuplicates(t *testing.T) {
	newTestDB(t)

	// ในระบบมีเครื่องเดียวกันค้างอยู่ 3 แถว จากไฟล์รอบก่อน
	existing := []models.UploadDataRow{
		specSheetUploadRow(1, "plan Draft.00.xlsx", "YN15437980", "Laos(X)", "21-Jul-26"),
		specSheetUploadRow(2, "plan Draft.00.xlsx", "YN15437980", "Laos(X)", "21-Jul-26"),
		specSheetUploadRow(3, "plan Draft.00.xlsx", "YN15437980", "Laos(X)", "21-Jul-26"),
	}

	// ไฟล์ใหม่ (ชื่อคนละชื่อ): เครื่องเดิมเปลี่ยนประเทศ/วันที่ และมีเครื่องใหม่เพิ่ม
	fileRows := []map[string]string{
		{"LOT NO.": "7.154", "Machine": "YN15437980", "Country Name": "Malaysia", "Main line": "22-Jul-26"},
		{"LOT NO.": "7.155", "Machine": "YN15437981", "Country Name": "Indonesia", "Main line": "23-Jul-26"},
	}

	ds := udDatasets[models.DatasetSpecSheet]
	plan := planUploadDataRows(models.DatasetSpecSheet, ds, existing,
		func(i int) map[string]string { return fileRows[i] }, len(fileRows))

	if plan[0].action != udActionUpdate {
		t.Fatalf("MC# เดิมที่ค่าเปลี่ยนต้อง UPDATE (action=%d) ไม่ใช่เพิ่มแถวใหม่", plan[0].action)
	}
	if plan[0].existing == nil || plan[0].existing.ID != 3 {
		t.Fatalf("ต้องอัปเดตแถวล่าสุดของเครื่องนั้น (id=3) ได้ %v", plan[0].existing)
	}
	if plan[1].action != udActionInsert {
		t.Fatalf("MC# ใหม่ต้อง INSERT (action=%d)", plan[1].action)
	}

	deletes := uploadDataSyncDeletes(models.DatasetSpecSheet, existing, plan, "plan Draft.01.xlsx")

	mergedIDs := map[uint]bool{}
	for _, d := range deletes {
		if d.merged {
			mergedIDs[d.id] = true
		}
	}
	if len(mergedIDs) != 2 || !mergedIDs[1] || !mergedIDs[2] {
		t.Fatalf("แถวซ้ำ id 1 และ 2 ต้องถูกยุบทิ้ง ได้ %v", mergedIDs)
	}

	gone, _ := countSyncDeletes(deletes)
	if gone != 0 {
		t.Errorf("ไม่มีแถวที่หายไปจากไฟล์ แต่ได้นับว่าลบ %d แถว", gone)
	}
	merged, _ := countSyncMerges(deletes)
	if merged != 2 {
		t.Errorf("ต้องยุบแถวซ้ำ 2 แถว ได้ %d", merged)
	}
}

// ไฟล์เดียวกันเขียนเครื่องซ้ำ → เก็บแถวล่างสุดแถวเดียว ไม่เพิ่มแถวซ้ำเข้าระบบ
func TestPlanningPCPKeepsOnlyLastRowOfMachineInFile(t *testing.T) {
	newTestDB(t)

	fileRows := []map[string]string{
		{"Machine": "YN15437980", "Country Name": "Laos(X)", "Main line": "21-Jul-26"},
		{"Machine": "YN15437980", "Country Name": "Laos(X)", "Main line": "21-Jul-26"},
		{"Machine": "YN15437980", "Country Name": "Malaysia", "Main line": "22-Jul-26"},
	}

	ds := udDatasets[models.DatasetSpecSheet]
	plan := planUploadDataRows(models.DatasetSpecSheet, ds, nil,
		func(i int) map[string]string { return fileRows[i] }, len(fileRows))

	inserts := 0
	for _, d := range plan {
		if d.action == udActionInsert {
			inserts++
		}
	}
	if inserts != 1 {
		t.Fatalf("เครื่องเดียวกัน 3 แถวต้องเข้าระบบ 1 แถว ได้ %d แถว", inserts)
	}
	if plan[2].action != udActionInsert {
		t.Fatalf("แถวที่เข้าระบบต้องเป็นแถวล่างสุด (action=%d)", plan[2].action)
	}
}
