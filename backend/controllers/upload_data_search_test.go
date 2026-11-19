package controllers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// ค้นหาในตาราง Planning PCP ด้วย Main line / Stand by shipping
//
// กรอก 25-Sep-26 → ได้ทุกแถวที่จ่ายวันนั้น ไม่ว่าไฟล์จะเขียนวันไว้แบบไหน
// ---------------------------------------------------------------------------

func pcpSearchRow(machine, country, mainLine, standBy string) models.UploadDataRow {
	data := map[string]string{
		"LOT NO.":           "7.154",
		"Machine":           machine,
		"Country Name":      country,
		"Main line":         mainLine,
		"Stand by shipping": standBy,
	}
	b, _ := json.Marshal(data)
	row := models.UploadDataRow{
		Dataset:  models.DatasetSpecSheet,
		DataJSON: string(b),
		FileName: "plan.xlsx",
	}
	fillUploadDataKeys(&row, models.DatasetSpecSheet, data)
	return row
}

func matchesPCPKeyword(row models.UploadDataRow, keyword string) bool {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	return uploadRowMatchesKeyword(models.DatasetSpecSheet, row, needle, parseLicenseDate(keyword))
}

func TestSearchPlanningPCPByMainLineDate(t *testing.T) {
	hit := pcpSearchRow("YN15438295", "Indonesia", "25-Sep-26", "28-Sep-26")
	other := pcpSearchRow("YN15438312", "Indonesia", "30-Sep-26", "02-Oct-26")

	// วันเดียวกันพิมพ์คนละแบบต้องได้ผลเหมือนกัน
	for _, kw := range []string{"25-Sep-26", "25-sep-26", "2026-09-25", "25/09/2026", "25 Sep 2026"} {
		if !matchesPCPKeyword(hit, kw) {
			t.Errorf("ค้น %q ต้องเจอแถวที่ Main line = 25-Sep-26", kw)
		}
		if matchesPCPKeyword(other, kw) {
			t.Errorf("ค้น %q ต้องไม่เจอแถวที่ Main line = 30-Sep-26", kw)
		}
	}

	// ค้นด้วยวัน Stand by shipping ได้ด้วย
	if !matchesPCPKeyword(hit, "28-Sep-26") {
		t.Error("ต้องค้นด้วยวัน Stand by shipping ได้")
	}
}

// ไฟล์แต่ละไฟล์เขียนวันที่คนละรูปแบบ — คนค้นไม่ต้องรู้ว่าเก็บไว้แบบไหน
func TestSearchPlanningPCPDateStoredInAnyFormat(t *testing.T) {
	for _, stored := range []string{"25-Sep-26", "25-Sep-2026", "2026-09-25", "25/09/2026", "46290"} {
		row := pcpSearchRow("YN15438295", "Indonesia", stored, "")
		if !matchesPCPKeyword(row, "25-Sep-26") {
			t.Errorf("เก็บไว้เป็น %q แล้วค้น 25-Sep-26 ต้องเจอ", stored)
		}
	}
}

// ช่องอื่นก็ยังค้นได้ตามเดิม (และได้มากกว่าเดิม — ประเทศ, LOT)
func TestSearchPlanningPCPByText(t *testing.T) {
	row := pcpSearchRow("YN15438295", "Malaysia", "25-Sep-26", "28-Sep-26")
	for _, kw := range []string{"malaysia", "YN154", "7.154"} {
		if !matchesPCPKeyword(row, kw) {
			t.Errorf("ค้น %q ต้องเจอ", kw)
		}
	}
	if matchesPCPKeyword(row, "Thailand") {
		t.Error("ค้นคำที่ไม่มีในแถว ต้องไม่เจอ")
	}
}

func TestGetUploadDataFiltersByMainLineDate(t *testing.T) {
	db := newTestDB(t)

	rows := []models.UploadDataRow{
		pcpSearchRow("YN15438295", "Indonesia", "25-Sep-26", "28-Sep-26"),
		pcpSearchRow("LG03405125", "Ariya", "2026-09-25", ""),
		pcpSearchRow("YN15438312", "Indonesia", "30-Sep-26", "02-Oct-26"),
	}
	for i := range rows {
		rows[i].SortOrder = int64(i)
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("สร้างข้อมูลทดสอบไม่สำเร็จ: %v", err)
		}
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/?dataset=spec_sheet&keyword=25-Sep-26", nil)
	GetUploadData(c)

	if rec.Code != 200 {
		t.Fatalf("ขอรายการไม่สำเร็จ: %d %s", rec.Code, rec.Body.String())
	}

	var out struct {
		Total int                    `json:"total"`
		Rows  []models.UploadDataRow `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("อ่านผลลัพธ์ไม่สำเร็จ: %v (%s)", err, rec.Body.String())
	}

	if out.Total != 2 {
		t.Fatalf("วันที่ 25-Sep-26 มี 2 เครื่อง (เขียนวันคนละแบบ) ได้ %d", out.Total)
	}
	got := map[string]bool{}
	for _, r := range out.Rows {
		got[r.MachineNo] = true
	}
	if !got["YN15438295"] || !got["LG03405125"] {
		t.Fatalf("ต้องได้ทั้งสองเครื่องที่จ่ายวันนั้น ได้ %v", got)
	}
	if got["YN15438312"] {
		t.Error("เครื่องที่จ่าย 30-Sep-26 ต้องไม่ติดมา")
	}
}
