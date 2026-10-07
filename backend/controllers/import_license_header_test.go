package controllers

import (
	"testing"

	"iconfirm/models"
)

// หัวคอลัมน์จริงของไฟล์ "บัญชีแสดงหมายเลขเครื่องใบอนุญาตนำเข้า"
// สังเกตว่า "วันที่ออกใบอนุญาต" มีสองครั้ง ครั้งที่สองเป็นของใบนำออก
var importSheetHeaderRow = []string{
	"ลำดับ", "PART NO.", "ตราอักษร", "แบบ/รุ่น",
	"เลขใบอนุญาตนำเข้า ", "วันที่ออกใบอนุญาต",
	"เลขอินวอยซ์นำเข้า", "เลขใบขนสินค้าขาเข้า", "จำนวน (เครื่อง )",
	"หมายเลขเครื่อง", "หมายเลขการผลิต",
	"เลขใบอนุญาตนำออก", "วันที่ออกใบอนุญาต", "ส่งออกไปประเทศ",
}

func TestFindImportLicenseHeaderSplitsExportIssueDate(t *testing.T) {
	rows := [][]string{
		{"บัญชีแสดงหมายเลขเครื่องนำเข้า CONTROLLER "},
		{},
		importSheetHeaderRow,
		{"1", "YN22E00849FA", "JRC MOBILITY", "JRN-260K",
			"E05036903281", "05/08/2026", "TQ80110", "A0130690818250", "1",
			"878270000101", "300234031527390", "E05046900716", "09/09/2026", "Indonesia"},
	}

	idx, headers := findImportLicenseHeader(rows)
	if idx != 2 {
		t.Fatalf("หาหัวตารางได้แถว %d ต้องการแถว 2", idx)
	}
	if headers[5] != "วันที่ออกใบอนุญาต" {
		t.Fatalf("คอลัมน์ 6 = %q ต้องเป็นวันที่ออกใบนำเข้า", headers[5])
	}
	// คอลัมน์ 13 ชื่อซ้ำกับคอลัมน์ 6 แต่อยู่หลังเลขใบอนุญาตนำออก จึงเป็นของใบนำออก
	if headers[12] != importExportIssueDateKey {
		t.Fatalf("คอลัมน์ 13 = %q ต้องถูกแยกเป็น %q", headers[12], importExportIssueDateKey)
	}

	// อ่านค่าลงรุ่นข้อมูลได้ครบทั้งใบนำเข้าและใบนำออก
	it := newImportItemFromRow(headers, rows[3])
	if it.LicenseNo != "E05036903281" {
		t.Fatalf("เลขใบนำเข้า = %q", it.LicenseNo)
	}
	if it.ExportLicenseNo != "E05046900716" {
		t.Fatalf("เลขใบนำออก = %q", it.ExportLicenseNo)
	}
	if it.IssueDate == nil || it.IssueDate.Format("2006-01-02") != "2026-08-05" {
		t.Fatalf("วันที่ออกใบนำเข้า = %v ต้องเป็น 2026-08-05", it.IssueDate)
	}
	if it.ExportIssueDate == nil || it.ExportIssueDate.Format("2006-01-02") != "2026-09-09" {
		t.Fatalf("วันที่ออกใบนำออก = %v ต้องเป็น 2026-09-09", it.ExportIssueDate)
	}
	if it.ExportCountry != "Indonesia" {
		t.Fatalf("ประเทศ = %q", it.ExportCountry)
	}
}

// ชีตฝั่ง Export (Serial Allocation) มีหมายเลขเครื่องและคอลัมน์ที่รู้จักครบ 3
// แต่ไม่มีเลขใบอนุญาตนำเข้า ต้องไม่ถูกดูดเข้ามาเป็นบัญชีใบอนุญาตนำเข้า
func TestFindImportLicenseHeaderRejectsExportSheet(t *testing.T) {
	rows := [][]string{
		{"IT Controller Serial Allocation"},
		{"Item", "Date Ass'y", "Machine No", "IT Controller Serial No.", "Manufacture Serial",
			"Part no.", "Telephone", "Mail address", "Country", "Classification",
			"Invoice date", "Invoice no.", "Export Entry",
			"IMPORT  License (in Invoice)", "EXPORT  License (in Invoice)"},
		{"1399", "2022-09-19", "YN15431832", "IT LESS", "IT LESS", "IT LESS", "IT LESS",
			"IT LESS", "INDONESIA", "IRIDIUM", "", "", "", "", ""},
	}
	if idx, _ := findImportLicenseHeader(rows); idx >= 0 {
		t.Fatalf("ชีต Export ถูกมองเป็นชีต Import ที่แถว %d", idx)
	}
}

// newImportItemFromRow: อ่านแถวเดียวตามหัวคอลัมน์ แบบเดียวกับตอนอัปโหลดจริง
func newImportItemFromRow(headers, row []string) models.LicenseItem {
	var it models.LicenseItem
	for col, header := range headers {
		if col >= len(row) {
			break
		}
		if setter, ok := importLicenseColumns[header]; ok {
			setter(&it, row[col])
		}
	}
	return it
}
