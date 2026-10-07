package controllers

import (
	"testing"

	"iconfirm/models"
)

func renewalLink(licenseType, oldNo, newNo string) models.LicenseRenewalHistory {
	return models.LicenseRenewalHistory{
		LicenseType:  licenseType,
		OldLicenseNo: oldNo,
		NewLicenseNo: newNo,
	}
}

// EXP-001 → EXP-002 → EXP-003 → EXP-004
// ใบปัจจุบัน = EXP-004 · ต่ออายุ 3 ครั้ง (ใบต้นฉบับไม่นับ)
func TestBuildLicenseChain(t *testing.T) {
	rows := []models.LicenseRenewalHistory{
		renewalLink(models.LicenseTypeExport, "EXP-001", "EXP-002"),
		renewalLink(models.LicenseTypeExport, "EXP-002", "EXP-003"),
		renewalLink(models.LicenseTypeExport, "EXP-003", "EXP-004"),
	}

	// ไล่จากเลขใบไหนในโซ่ก็ต้องได้ผลเดียวกัน
	for _, from := range []string{"EXP-001", "EXP-002", "EXP-003", "EXP-004", "exp004"} {
		c := buildLicenseChain(rows, models.LicenseTypeExport, from)
		if c.CurrentNo != "EXP-004" {
			t.Errorf("จาก %s: ใบปัจจุบัน = %q, want EXP-004", from, c.CurrentNo)
		}
		if c.OriginalNo != "EXP-001" {
			t.Errorf("จาก %s: ใบต้นฉบับ = %q, want EXP-001", from, c.OriginalNo)
		}
		if c.RenewalCount != 3 {
			t.Errorf("จาก %s: จำนวนครั้งที่ต่ออายุ = %d, want 3", from, c.RenewalCount)
		}
		if len(c.Steps) != 4 {
			t.Fatalf("จาก %s: steps = %d, want 4 (Original + 3 ครั้ง)", from, len(c.Steps))
		}
		if c.Steps[0].Label != "Original" || c.Steps[0].Round != 0 {
			t.Errorf("แถวแรกต้องเป็นใบต้นฉบับ: %+v", c.Steps[0])
		}
		if c.Steps[3].OldLicenseNo != "EXP-003" || c.Steps[3].NewLicenseNo != "EXP-004" {
			t.Errorf("การต่ออายุครั้งสุดท้าย = %+v", c.Steps[3])
		}
	}
}

// Import กับ Export ต้องไม่เชื่อมถึงกัน แม้เลขใบจะซ้ำกัน
func TestLicenseChainSeparatesImportExport(t *testing.T) {
	rows := []models.LicenseRenewalHistory{
		renewalLink(models.LicenseTypeExport, "L-001", "L-002"),
		renewalLink(models.LicenseTypeExport, "L-002", "L-003"),
		renewalLink(models.LicenseTypeImport, "L-001", "IMP-900"),
	}

	exp := buildLicenseChain(rows, models.LicenseTypeExport, "L-001")
	if exp.CurrentNo != "L-003" || exp.RenewalCount != 2 {
		t.Fatalf("โซ่ฝั่ง Export = %+v", exp)
	}

	imp := buildLicenseChain(rows, models.LicenseTypeImport, "L-001")
	if imp.CurrentNo != "IMP-900" || imp.RenewalCount != 1 {
		t.Fatalf("โซ่ฝั่ง Import รั่วข้ามประเภท = %+v", imp)
	}
}

// ใบที่ยังไม่เคยต่ออายุ — Renewal Count = 0 และมีแต่แถว Original
func TestLicenseChainWithoutRenewal(t *testing.T) {
	c := buildLicenseChain(nil, models.LicenseTypeExport, "EXP-100")
	if c.CurrentNo != "EXP-100" || c.OriginalNo != "EXP-100" {
		t.Fatalf("chain = %+v", c)
	}
	if c.RenewalCount != 0 || len(c.Steps) != 1 {
		t.Fatalf("ใบที่ยังไม่ต่ออายุ: count = %d, steps = %d", c.RenewalCount, len(c.Steps))
	}
}

// ข้อมูลที่วนลูป (A→B และ B→A) ต้องไม่ทำให้ระบบไล่ไม่จบ
func TestLicenseChainLoopSafe(t *testing.T) {
	rows := []models.LicenseRenewalHistory{
		renewalLink(models.LicenseTypeExport, "A", "B"),
		renewalLink(models.LicenseTypeExport, "B", "A"),
	}
	c := buildLicenseChain(rows, models.LicenseTypeExport, "A")
	if c.RenewalCount > 2 {
		t.Fatalf("ไล่โซ่ไม่จบ: %+v", c)
	}
}

func TestNormalizeLicenseType(t *testing.T) {
	cases := map[string]string{
		"Import":         models.LicenseTypeImport,
		"IMPORT LICENSE": models.LicenseTypeImport,
		"imp":            models.LicenseTypeImport,
		"ใบอนุญาตนำเข้า": models.LicenseTypeImport,
		"Export": models.LicenseTypeExport,
		"EXP":    models.LicenseTypeExport,
		"ใบอนุญาตนำออก": models.LicenseTypeExport,
		"": "",
		"อย่างอื่นที่ไม่ใช่": "",
	}
	for in, want := range cases {
		if got := models.NormalizeLicenseType(in); got != want {
			t.Errorf("NormalizeLicenseType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLicenseRowMatchesFilter(t *testing.T) {
	imp := LicenseOverviewRow{LicenseType: models.LicenseTypeImport, Status: models.LicenseStatusValid}
	expNear := LicenseOverviewRow{LicenseType: models.LicenseTypeExport, Status: models.LicenseStatusExpiring}
	expOld := LicenseOverviewRow{LicenseType: models.LicenseTypeExport, Status: models.LicenseStatusExpired}

	checks := []struct {
		filter string
		row    LicenseOverviewRow
		want   bool
	}{
		{LicenseFilterAll, imp, true},
		{LicenseFilterImport, imp, true},
		{LicenseFilterImport, expNear, false},
		{LicenseFilterExport, expNear, true},
		{LicenseFilterActive, imp, true},
		{LicenseFilterActive, expNear, true},
		{LicenseFilterActive, expOld, false},
		{LicenseFilterNearExpiry, expNear, true},
		{LicenseFilterNearExpiry, imp, false},
		{LicenseFilterExpired, expOld, true},
	}
	for _, c := range checks {
		if got := licenseRowMatchesFilter(c.row, c.filter); got != c.want {
			t.Errorf("filter %s กับสถานะ %s = %v, want %v", c.filter, c.row.Status, got, c.want)
		}
	}
}
