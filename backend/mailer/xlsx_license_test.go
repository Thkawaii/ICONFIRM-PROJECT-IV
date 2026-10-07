package mailer

import (
	"strconv"
	"testing"
)

// ชีต Import license ต้องมีหัวคอลัมน์ตามลำดับที่กำหนดไว้ทุกตัวอักษร
func TestImportSheetHeaderOrder(t *testing.T) {
	want := []string{
		"ลำดับ",
		"เลขที่ใบอนุญาต",
		"เลขอินวอยซ์นำเข้า",
		"เลขใบขนสินค้าขาเข้า",
		"ตราอักษร",
		"แบบ/รุ่น",
		"จำนวน",
		"วันหมดอายุ",
		"คงเหลือ",
		"สถานะ",
	}
	assertHeaders(t, "Import license", importLicenseColumns, want)
}

// ชีต Export license ต้องมีหัวคอลัมน์ตามลำดับที่กำหนดไว้ทุกตัวอักษร
func TestExportSheetHeaderOrder(t *testing.T) {
	want := []string{
		"ลำดับ",
		"เลขที่ใบอนุญาต",
		"จำนวน",
		"วันหมดอายุ",
		"คงเหลือ",
		"สถานะ",
		"กำหนดยื่น กสทช.",
		"สถานะการยื่น",
	}
	assertHeaders(t, "Export license", exportLicenseColumns, want)
}

func assertHeaders(t *testing.T, sheet string, got []xlsxColumn, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ชีต %s มี %d คอลัมน์ ต้องการ %d", sheet, len(got), len(want))
	}
	for i, c := range got {
		if c.Title != want[i] {
			t.Fatalf("ชีต %s คอลัมน์ที่ %d = %q ต้องการ %q", sheet, i+1, c.Title, want[i])
		}
	}
}

// แต่ละชีตต้องมีหนึ่งแถวต่อหนึ่งใบอนุญาต ไม่จับคู่ข้ามประเภทกันอีกแล้ว
func TestSheetsHaveOneRowPerLicense(t *testing.T) {
	r := sampleReport()

	imports := importLicenseRows(r)
	if len(imports) != len(r.Import) {
		t.Fatalf("ชีตนำเข้ามี %d แถว ต้องการ %d", len(imports), len(r.Import))
	}
	if imports[0].Cells[1] != "IL-2026-0148" || imports[1].Cells[1] != "IL-2026-0163" {
		t.Fatalf("เลขใบนำเข้าเรียงผิด: %v / %v", imports[0].Cells, imports[1].Cells)
	}

	exports := exportLicenseRows(r)
	if len(exports) != len(r.Export) {
		t.Fatalf("ชีตนำออกมี %d แถว ต้องการ %d", len(exports), len(r.Export))
	}
	if exports[0].Cells[1] != "EX-2026-0091" || exports[1].Cells[1] != "EX-2026-0133" {
		t.Fatalf("เลขใบนำออกเรียงผิด: %v / %v", exports[0].Cells, exports[1].Cells)
	}
}

// ชีตนำเข้าต้องมีเลขอินวอยซ์ เลขใบขน ตราอักษร และแบบ/รุ่น ครบทุกช่อง
func TestImportSheetCarriesInvoiceAndBrand(t *testing.T) {
	r := sampleReport()
	r.Import[0].DeclarationNo = "A0123-00045"
	r.Import[0].Brand = "KOBELCO"

	row := importLicenseRows(r)[0]
	if row.Cells[2] != "KCMT-25-0912" {
		t.Fatalf("เลขอินวอยซ์นำเข้า = %q", row.Cells[2])
	}
	if row.Cells[3] != "A0123-00045" {
		t.Fatalf("เลขใบขนสินค้าขาเข้า = %q", row.Cells[3])
	}
	if row.Cells[4] != "KOBELCO" {
		t.Fatalf("ตราอักษร = %q", row.Cells[4])
	}
	if row.Cells[5] != "SK75-8" {
		t.Fatalf("แบบ/รุ่น = %q", row.Cells[5])
	}
}

// ลำดับคอลัมน์แรกต้องนับต่อเนื่อง 1..N และทุกแถวต้องมีจำนวนช่องเท่ากับหัวคอลัมน์
func TestSheetRowsNumberingAndWidth(t *testing.T) {
	r := sampleReport()

	for _, c := range []struct {
		name    string
		rows    []xlsxRow
		columns []xlsxColumn
	}{
		{"Import license", importLicenseRows(r), importLicenseColumns},
		{"Export license", exportLicenseRows(r), exportLicenseColumns},
	} {
		for i, row := range c.rows {
			if len(row.Cells) != len(c.columns) {
				t.Fatalf("ชีต %s แถว %d มี %d ช่อง ต้องการ %d", c.name, i+1, len(row.Cells), len(c.columns))
			}
			if want := strconv.Itoa(i + 1); row.Cells[0] != want {
				t.Fatalf("ชีต %s ลำดับแถว %d = %q ต้องการ %q", c.name, i+1, row.Cells[0], want)
			}
		}
	}
}

// ใบนำเข้าที่ไม่มีวันหมดอายุต้องไม่แสดงคงเหลือเป็น "วันนี้"
func TestImportSheetNoDateLabel(t *testing.T) {
	r := sampleReport()
	r.Import = append(r.Import, ImportRow{LicenseNo: "IL-NODATE", Status: StatusNoDate})

	for _, row := range importLicenseRows(r) {
		if row.Cells[1] != "IL-NODATE" {
			continue
		}
		if row.Cells[8] != "ยังไม่ระบุวันที่" {
			t.Fatalf("คงเหลือของใบที่ไม่มีวันที่ = %q", row.Cells[8])
		}
		return
	}
	t.Fatal("ไม่พบแถว IL-NODATE")
}
