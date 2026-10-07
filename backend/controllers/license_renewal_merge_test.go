package controllers

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

// ชีตต่ออายุที่ merge คอลัมน์ NO. และเลขใบนำเข้าข้ามหลายแถว (เช่น Completed 01)
// ต้องได้ค่าครบทุกแถว ไม่ใช่มีเฉพาะแถวแรก
func TestFillVerticalMergesFillsGroupAndImportNo(t *testing.T) {
	xl := excelize.NewFile()
	defer xl.Close()
	sheet := xl.GetSheetName(0)

	_ = xl.SetSheetRow(sheet, "A1", &[]interface{}{"NO.", "IMPORT LICENSE NO.", "EXPORT LICENSE NO.", "STOCK", "REMAIN"})
	_ = xl.SetSheetRow(sheet, "A2", &[]interface{}{"Completed 01", "IMP-1", "EXP-1", 10, 40})
	_ = xl.SetSheetRow(sheet, "C3", &[]interface{}{"EXP-2", 20, 20})
	_ = xl.SetSheetRow(sheet, "C4", &[]interface{}{"EXP-3", 20, -10})
	if err := xl.MergeCell(sheet, "A2", "A4"); err != nil {
		t.Fatal(err)
	}
	if err := xl.MergeCell(sheet, "B2", "B4"); err != nil {
		t.Fatal(err)
	}

	rows, err := readSheetAllRows(xl, sheet)
	if err != nil {
		t.Fatal(err)
	}
	rows = fillVerticalMerges(xl, sheet, rows)

	for _, i := range []int{1, 2, 3} {
		if got := rows[i][0]; got != "Completed 01" {
			t.Errorf("row %d NO. = %q, want Completed 01", i+1, got)
		}
		if got := rows[i][1]; got != "IMP-1" {
			t.Errorf("row %d import = %q, want IMP-1", i+1, got)
		}
	}
	// ค่าที่ไม่ได้ merge ต้องไม่ถูกแตะ
	if rows[3][2] != "EXP-3" || rows[3][4] != "-10" {
		t.Errorf("unmerged cells changed: %v", rows[3])
	}
}

func TestRenewalIntHandlesExcelDisplayFormats(t *testing.T) {
	cases := map[string]int{
		"":          0,
		"-":         0,
		" -   ":     0,
		"10":        10,
		"-10":       -10,
		"(10)":      -10,
		"10-":       -10,
		"1,234.00 ": 1234,
		"\u221210":  -10,
		"55.0":      55,
		"abc":       0,
	}
	for in, want := range cases {
		if got := renewalInt(in); got != want {
			t.Errorf("renewalInt(%q) = %d, want %d", in, got, want)
		}
	}
}
