package controllers

import (
	"encoding/json"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"
)

// ---------------------------------------------------------------------------
// ค้นหาในตารางข้อมูลที่อัปโหลด (Planning PCP ฯลฯ)
//
// ของเดิมค้นได้แค่ 4 ช่องที่มีคอลัมน์จริงในฐานข้อมูล (machine_no · lot_no ·
// order_no · parts_no) ส่วนที่เหลือเก็บรวมไว้ใน data_json ก้อนเดียว จึงค้นไม่ได้
// รวมถึง Main line และ Stand by shipping ที่คนหน้างานใช้ไล่ของตามวันจ่าย
//
// ตัวนี้ค้นได้ทุกช่องที่ตารางแสดง และแยกการค้น "วันที่" ออกมาเป็นพิเศษ
//
//	กรอก 25-Sep-26   → ทุกแถวที่ Main line หรือ Stand by shipping ตรงวันนั้น
//	กรอก 2026-09-25  → ได้ผลเหมือนกัน
//	กรอก 25/09/2026  → ได้ผลเหมือนกัน
//
// ทำไมต้องเทียบเป็น "วัน" ไม่ใช่เทียบตัวอักษร: ไฟล์แผนแต่ละไฟล์เขียนวันที่คนละแบบ
// (25-Sep-26 · 2026-09-25 · 25/09/2026 · เลข serial ของ Excel) ระบบเก็บไว้ตามที่
// ไฟล์เขียนมา ถ้าเทียบตัวอักษร คนต้องพิมพ์ให้ตรงรูปแบบที่เก็บไว้เป๊ะ ๆ ซึ่งคนใช้งาน
// ไม่มีทางรู้ว่าเก็บไว้แบบไหน — ใช้ parseLicenseDate ตัวเดียวกับที่ WH ใช้กรอง
// วันจ่ายอยู่แล้ว (ดู flowPlanDate) จะได้ตีความวันเหมือนกันทั้งระบบ
//
// กรองในหน่วยความจำ ไม่ได้กรองด้วย SQL เพราะเทียบวันแบบนี้ SQL ทำไม่ได้
// (ค่าใน data_json เป็นข้อความคนละรูปแบบ) ตารางพวกนี้เป็นแผนรายเดือนขนาดไม่ใหญ่
// และหน้า Export ก็อ่านทั้งตารางขึ้นมาอยู่แล้ว
// ---------------------------------------------------------------------------

// udDatasetDateFields: ช่องที่เป็นวันที่ของแต่ละชุดข้อมูล (ค้นด้วยวันที่ได้)
var udDatasetDateFields = map[string][]string{
	models.DatasetSpecSheet:     {"Main line", "Stand by shipping"},
	models.DatasetPCPDailyPlan:  {"Main line", "Stand by shipping"},
	models.DatasetDailyPlan:     {"Main line", "Stand by shipping"},
	models.DatasetEngineITAlloc: {"Main line"},
}

// udSearchesAllFields: ชุดข้อมูลนี้ค้นหาแบบไล่ทุกช่อง (และค้นด้วยวันที่) ได้หรือไม่
func udSearchesAllFields(dataset string) bool {
	return len(udDatasetDateFields[dataset]) > 0
}

// searchUploadRowIDs: id ของแถวที่เข้าคำค้น เรียงตามลำดับที่แสดงในตาราง
func searchUploadRowIDs(dataset, keyword string) []uint {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle == "" {
		return nil
	}
	// คำค้นเป็นวันที่หรือไม่ — แปลครั้งเดียว ไม่ใช่แปลซ้ำทุกแถว
	day := parseLicenseDate(keyword)

	var rows []models.UploadDataRow
	config.DB.Select("id", "machine_no", "lot_no", "order_no", "parts_no", "data_json").
		Where("dataset = ?", dataset).
		Order("sort_order asc, id asc").
		Find(&rows)

	out := make([]uint, 0, 64)
	for _, r := range rows {
		if uploadRowMatchesKeyword(dataset, r, needle, day) {
			out = append(out, r.ID)
		}
	}
	return out
}

// uploadRowMatchesKeyword: แถวนี้เข้าคำค้นหรือไม่ — เข้าข้อใดข้อหนึ่งก็พอ
//
//	ก. มีคำค้นอยู่ในช่องใดช่องหนึ่งของแถว (ไม่สนตัวพิมพ์เล็กใหญ่)
//	   ค้น Malaysia · YN154 · 7.154 · Sep ได้ด้วยข้อนี้
//	ข. คำค้นเป็นวันที่ และตรงกับ "วัน" ในช่องวันที่ของแถว
//
// ข้อ ข. เป็นการเพิ่มผลลัพธ์ ไม่ใช่มาแทนข้อ ก. เพราะคำค้นบางคำเป็นได้ทั้งสองอย่าง
// (เช่น 25000 เป็นเลข serial วันที่ของ Excel ก็ได้ เป็นเลขออเดอร์ก็ได้)
func uploadRowMatchesKeyword(dataset string, row models.UploadDataRow, needle string, day *time.Time) bool {
	data := map[string]string{}
	_ = json.Unmarshal([]byte(row.DataJSON), &data)

	for _, v := range []string{row.MachineNo, row.LotNo, row.OrderNo, row.PartsNo} {
		if v != "" && strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}
	for _, v := range data {
		if v != "" && strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}

	if day == nil {
		return false
	}
	want := startOfDay(*day)
	for _, f := range udDatasetDateFields[dataset] {
		raw := pickField(data, f, extraColumnPrefix+f)
		if raw == "" {
			continue
		}
		if d := parseLicenseDate(raw); d != nil && startOfDay(*d).Equal(want) {
			return true
		}
	}
	return false
}

// pageOfIDs: ตัดเอาเฉพาะหน้าที่ขอ
func pageOfIDs(ids []uint, page, limit int) []uint {
	if limit <= 0 || page <= 0 {
		return nil
	}
	start := (page - 1) * limit
	if start >= len(ids) {
		return nil
	}
	end := start + limit
	if end > len(ids) {
		end = len(ids)
	}
	return ids[start:end]
}

// loadUploadRowsByID: อ่านแถวเต็ม (พร้อมชื่อผู้อัป) ตามลำดับ id ที่ส่งมา
func loadUploadRowsByID(ids []uint) []models.UploadDataRow {
	if len(ids) == 0 {
		return nil
	}
	var rows []models.UploadDataRow
	config.DB.Preload("User").Where("id IN ?", ids).Find(&rows)

	// SQL ไม่รับประกันลำดับของ IN → เรียงกลับตามลำดับที่กรองมา
	byID := make(map[uint]models.UploadDataRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	out := make([]models.UploadDataRow, 0, len(ids))
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			out = append(out, r)
		}
	}
	return out
}
