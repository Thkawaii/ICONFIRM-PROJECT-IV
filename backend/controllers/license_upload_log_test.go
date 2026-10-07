package controllers

import "testing"

func TestLicenseUploadLogLine(t *testing.T) {
	line := licenseUploadLogLine("import.xlsx", 178, 170, 5, 3, 2)
	want := "import.xlsx | total=178 new=170 update=5 delete=3 error=2"
	if line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}

	if got := licenseUploadLogFileName(line); got != "import.xlsx" {
		t.Errorf("fileName = %q, want import.xlsx", got)
	}

	counts, ok := licenseUploadLogCounts(line)
	if !ok {
		t.Fatal("อ่านตัวเลขจากบรรทัดสรุปไม่ได้")
	}
	for key, want := range map[string]int{
		"total": 178, "new": 170, "update": 5, "delete": 3, "error": 2,
	} {
		if counts[key] != want {
			t.Errorf("%s = %d, want %d", key, counts[key], want)
		}
	}
}

func TestLicenseUploadLogLegacyLine(t *testing.T) {
	// log รูปแบบเก่าที่เก็บแค่ชื่อไฟล์ — ต้องยังอ่านชื่อไฟล์ได้ และบอกว่าไม่มีตัวเลข
	const legacy = "export-2024.xlsx"
	if got := licenseUploadLogFileName(legacy); got != legacy {
		t.Errorf("fileName = %q, want %q", got, legacy)
	}
	if _, ok := licenseUploadLogCounts(legacy); ok {
		t.Error("log รูปแบบเก่าไม่ควรมีตัวเลข")
	}
}

func TestLicenseUploadLogLineLength(t *testing.T) {
	longName := ""
	for i := 0; i < 200; i++ {
		longName += "abcdefg"
	}
	if got := len(licenseUploadLogLine(longName, 1, 1, 0, 0, 0)); got > licenseUploadLogMaxLen {
		t.Fatalf("ความยาว = %d, ต้องไม่เกิน %d", got, licenseUploadLogMaxLen)
	}
}

func TestApplyUploadLogDetail(t *testing.T) {
	row := LicenseUploadLogRow{Key: "import"}
	applyUploadLogDetail(&row, "import.xlsx | total=10 new=8 update=1 delete=0 error=1")
	if !row.HasCounts {
		t.Fatal("ต้องอ่านตัวเลขได้")
	}
	if row.FileName != "import.xlsx" {
		t.Errorf("fileName = %q", row.FileName)
	}
	if row.Total != 10 || row.Added != 8 || row.Updated != 1 || row.Errors != 1 {
		t.Errorf("ตัวเลขไม่ตรง: %+v", row)
	}

	// มีชื่อไฟล์จากตารางอยู่แล้ว → ไม่ทับด้วยชื่อจาก log
	keep := LicenseUploadLogRow{FileName: "จากตาราง.xlsx"}
	applyUploadLogDetail(&keep, "อื่น.xlsx | total=1 new=1 update=0 delete=0 error=0")
	if keep.FileName != "จากตาราง.xlsx" {
		t.Errorf("fileName = %q, want จากตาราง.xlsx", keep.FileName)
	}

	// log ว่าง → ไม่เปลี่ยนอะไร
	empty := LicenseUploadLogRow{}
	applyUploadLogDetail(&empty, "   ")
	if empty.HasCounts || empty.Detail != "" {
		t.Errorf("log ว่างไม่ควรเปลี่ยนค่า: %+v", empty)
	}
}
