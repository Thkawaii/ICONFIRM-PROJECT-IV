package controllers

import (
	"testing"
	"time"

	"iconfirm/mailer"
)

// TestLinkImportExportRefsFillsFromImportSide: ไฟล์อัปโหลดใบนำเข้าเองมีคอลัมน์ท้ายไฟล์บอกว่า
// ใบนำเข้านี้ตอนนำออกจะใช้ใบอนุญาตนำออกเลขไหน (ImportRow.ExportLicenseNo) — ถ้าฝั่งใบนำออก
// ไม่ได้กรอกเลขใบนำเข้าอ้างอิงไว้ตอนอัปโหลด (ExportRow.ImportLicenseNo ว่าง) ต้องยังจับคู่ได้
// จากฝั่งใบนำเข้าแทน
func TestLinkImportExportRefsFillsFromImportSide(t *testing.T) {
	expiry := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)

	report := mailer.WeeklyReport{
		Import: []mailer.ImportRow{
			{LicenseNo: "E05036900659", ExpiryDate: &expiry, ExportLicenseNo: "E05046900719"},
		},
		Export: []mailer.ExportRow{
			{ExportLicenseNo: "E05046900719"}, // ไม่ได้กรอก ImportLicenseNo ไว้ตอนอัปโหลด
		},
	}

	linkImportExportRefs(&report)

	got := report.Export[0]
	if got.ImportLicenseNo != "E05036900659" {
		t.Fatalf("ต้องเติมเลขใบนำเข้าอ้างอิงจากฝั่งใบนำเข้า ได้ %q", got.ImportLicenseNo)
	}
	if got.ImportExpiryDate == nil || !got.ImportExpiryDate.Equal(expiry) {
		t.Fatalf("ต้องเติมวันหมดอายุใบนำเข้าที่อ้างอิงด้วย ได้ %v", got.ImportExpiryDate)
	}
}

// TestLinkImportExportRefsKeepsExportSideWhenAlreadySet: ถ้าไฟล์อัปโหลดใบนำออกกรอกเลขใบนำเข้า
// อ้างอิงไว้เองอยู่แล้ว ต้องไม่ถูกฝั่งใบนำเข้าเขียนทับ
func TestLinkImportExportRefsKeepsExportSideWhenAlreadySet(t *testing.T) {
	fromExport := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fromImport := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)

	report := mailer.WeeklyReport{
		Import: []mailer.ImportRow{
			{LicenseNo: "IMP-FROM-IMPORT-SIDE", ExpiryDate: &fromImport, ExportLicenseNo: "EXP-001"},
		},
		Export: []mailer.ExportRow{
			{ExportLicenseNo: "EXP-001", ImportLicenseNo: "IMP-FROM-EXPORT-SIDE", ImportExpiryDate: &fromExport},
		},
	}

	linkImportExportRefs(&report)

	if got := report.Export[0].ImportLicenseNo; got != "IMP-FROM-EXPORT-SIDE" {
		t.Fatalf("ถ้าฝั่งใบนำออกกรอกเลขอ้างอิงไว้แล้ว ไม่ควรถูกเขียนทับ ได้ %q", got)
	}
}

// TestLinkImportExportRefsNoMatchLeavesEmpty: ไม่มีใบนำเข้าใบไหนประกาศเลขใบนำออกนี้ไว้เลย
// ต้องปล่อยว่างไว้ตามเดิม ไม่เดาสุ่ม
func TestLinkImportExportRefsNoMatchLeavesEmpty(t *testing.T) {
	report := mailer.WeeklyReport{
		Import: []mailer.ImportRow{
			{LicenseNo: "IL-UNRELATED"},
		},
		Export: []mailer.ExportRow{
			{ExportLicenseNo: "EXP-002"},
		},
	}

	linkImportExportRefs(&report)

	if got := report.Export[0].ImportLicenseNo; got != "" {
		t.Fatalf("ไม่ควรจับคู่มั่ว ได้ %q", got)
	}
}
