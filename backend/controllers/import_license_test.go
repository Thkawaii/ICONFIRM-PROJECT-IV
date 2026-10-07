package controllers

import (
	"testing"
	"time"

	"iconfirm/config"
	"iconfirm/models"
)

func seedLicenseItem(t *testing.T, machineNo, invoiceNo, prodNo, licenseNo, country, confirm string) models.LicenseItem {
	t.Helper()
	if confirm == "" {
		confirm = models.LicenseItemPending
	}
	item := models.LicenseItem{
		MachineNo:     machineNo,
		InvoiceNo:     invoiceNo,
		ProductionNo:  prodNo,
		LicenseNo:     licenseNo,
		ExportCountry: country,
		ConfirmStatus: confirm,
	}
	if err := config.DB.Create(&item).Error; err != nil {
		t.Fatalf("seed license item: %v", err)
	}
	return item
}

func TestMatchImportLicense(t *testing.T) {
	db := newTestDB(t)
	_ = db
	seedLicenseItem(t, "878250022802", "TQ60610", "111122223333444", "E05036901604", "Indonesia", "")

	t.Run("match by machine no", func(t *testing.T) {
		status, _, item := matchImportLicense("878250022802", "", "")
		if status != models.MatchStatusMatch {
			t.Fatalf("status = %q, want MATCH", status)
		}
		if item == nil || item.LicenseNo != "E05036901604" {
			t.Fatalf("item license mismatch: %+v", item)
		}
	})

	t.Run("not found", func(t *testing.T) {
		status, _, item := matchImportLicense("999999999999", "", "")
		if status != models.MatchStatusNotFound {
			t.Fatalf("status = %q, want NOT_FOUND", status)
		}
		if item != nil {
			t.Fatalf("item should be nil, got %+v", item)
		}
	})

	t.Run("empty code", func(t *testing.T) {
		status, _, _ := matchImportLicense("   ", "", "")
		if status != models.MatchStatusNotFound {
			t.Fatalf("status = %q, want NOT_FOUND for empty", status)
		}
	})

	t.Run("wrong invoice", func(t *testing.T) {
		status, _, item := matchImportLicense("878250022802", "OTHER-INV", "")
		if status != models.MatchStatusWrongInv {
			t.Fatalf("status = %q, want WRONG_INVOICE", status)
		}
		if item == nil {
			t.Fatal("expected item even on wrong invoice")
		}
	})

	t.Run("invoice case-insensitive match", func(t *testing.T) {
		status, _, _ := matchImportLicense("878250022802", "tq60610", "")
		if status != models.MatchStatusMatch {
			t.Fatalf("status = %q, want MATCH (case-insensitive invoice)", status)
		}
	})

	t.Run("wrong production no", func(t *testing.T) {
		status, _, _ := matchImportLicense("878250022802", "TQ60610", "000000000000000")
		if status != models.MatchStatusWrongProd {
			t.Fatalf("status = %q, want WRONG_PRODNO", status)
		}
	})

	t.Run("match by production no", func(t *testing.T) {
		status, _, _ := matchImportLicense("111122223333444", "", "")
		if status != models.MatchStatusMatch {
			t.Fatalf("status = %q, want MATCH by prod no", status)
		}
	})
}

func TestMatchImportLicenseDuplicate(t *testing.T) {
	newTestDB(t)
	seedLicenseItem(t, "878250022999", "TQ60610", "", "E050", "Malaysia", models.LicenseItemConfirmed)

	status, _, item := matchImportLicense("878250022999", "", "")
	if status != models.MatchStatusDuplicate {
		t.Fatalf("status = %q, want DUPLICATE", status)
	}
	if item == nil {
		t.Fatal("expected item on duplicate")
	}
}

func TestMatchImportLicenseCodeAlias(t *testing.T) {
	newTestDB(t)
	seedLicenseItem(t, "878250022802", "TQ60610", "", "E05036901604", "Indonesia", "")

	alias := models.CodeAlias{
		ComponentType: "import_license",
		Kind:          "machine",
		FromCode:      "NEW-878-250-022-802",
		ToOld:         "878250022802",
	}
	if err := config.DB.Create(&alias).Error; err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	status, _, item := matchImportLicense("NEW-878-250-022-802", "", "")
	if status != models.MatchStatusMatch {
		t.Fatalf("status = %q, want MATCH via alias", status)
	}
	if item == nil || item.MachineNo != "878250022802" {
		t.Fatalf("alias did not resolve to standard row: %+v", item)
	}
}

func TestImportLicenseFillExpireDate(t *testing.T) {
	issue := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	m := models.LicenseItem{IssueDate: &issue}
	m.FillExpireDate()

	if m.ExpireDate == nil {
		t.Fatal("ExpireDate ต้องถูกเติมให้")
	}
	want := issue.AddDate(0, 6, 0)
	if !m.ExpireDate.Equal(want) {
		t.Errorf("ExpireDate = %v, want %v (ออก + 6 เดือน)", m.ExpireDate, want)
	}

	custom := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	m2 := models.LicenseItem{IssueDate: &issue, ExpireDate: &custom}
	m2.FillExpireDate()
	if !m2.ExpireDate.Equal(custom) {
		t.Errorf("ExpireDate ถูกเขียนทับ = %v, want %v", m2.ExpireDate, custom)
	}

	m3 := models.LicenseItem{}
	m3.FillExpireDate()
	if m3.ExpireDate != nil {
		t.Error("ไม่มี IssueDate ต้องไม่เติม ExpireDate")
	}
}
