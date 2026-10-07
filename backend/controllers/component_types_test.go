package controllers

import (
	"encoding/json"
	"testing"

	"iconfirm/models"
)

func TestDetectComponentType(t *testing.T) {
	cases := map[string]string{
		"CV2411001":    ComponentCV,
		"SM2411001":    ComponentSM,
		"SW2411001":    ComponentSM,
		"MP2411001":    ComponentMP,
		"PH2411001":    ComponentPH,
		"PA2411001":    ComponentPH,
		"CW2411001":    ComponentCW,
		"878250022801": ComponentITC,
		"":             "",
		"ZZ999":        "",
	}
	for in, want := range cases {
		if got := DetectComponentType(in); got != want {
			t.Errorf("DetectComponentType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComponentNeedsLicense(t *testing.T) {
	if !ComponentNeedsLicense(ComponentITC) {
		t.Error("IT Controller ต้องเทียบใบอนุญาต")
	}
	for _, code := range []string{ComponentCV, ComponentSM, ComponentMP, ComponentPH, ComponentCW, ComponentEN} {
		if ComponentNeedsLicense(code) {
			t.Errorf("%s ไม่ต้องเทียบใบอนุญาต", code)
		}
	}
}

func TestEveryComponentNeedsWHScan(t *testing.T) {
	for _, code := range AllComponentCodes() {
		if !ComponentNeedsWHScan(code) {
			t.Errorf("%s: ต้องบังคับให้ WH สแกนก่อนประกอบ", code)
		}
	}
	if ComponentNeedsWHScan("") {
		t.Error("ชนิดพาร์ทว่างไม่ควรบังคับ WH")
	}
	if ComponentNeedsWHScan("ZZ") {
		t.Error("ชนิดพาร์ทที่ไม่รู้จักไม่ควรบังคับ WH")
	}
}

func TestMFGStatusForAllPartsWaitsForWH(t *testing.T) {
	for _, code := range []string{ComponentITC, ComponentCV, ComponentSM, ComponentMP, ComponentPH, ComponentEN, ComponentCW} {
		if got := mfgStatusFor(code, false, PlanStateMatch, false); got != models.MFGStatusNotMatched {
			t.Errorf("%s: WH ยังไม่สแกน status = %q, want NOT_MATCHED", code, got)
		}
		if got := mfgStatusFor(code, false, PlanStateMatch, true); got != models.MFGStatusMatched {
			t.Errorf("%s: WH สแกนแล้ว status = %q, want MATCHED", code, got)
		}
		if bad := mfgStatusFor(code, false, PlanStateMismatch, true); bad != models.MFGStatusNotMatched {
			t.Errorf("%s: ผิดแผนต้องเป็น NOT_MATCHED", code)
		}
		if dup := mfgStatusFor(code, true, PlanStateMatch, true); dup != models.MFGStatusDuplicate {
			t.Errorf("%s: ซ้ำต้องเป็น DUPLICATE", code)
		}
	}
}

func TestPlanComponentsFilled(t *testing.T) {
	one := map[string]string{"IT Controller No": "878250022801"}
	if got := planComponentsFilled(one); len(got) != 1 {
		t.Errorf("กรอกชนิดเดียว = %v, want 1", got)
	}

	two := map[string]string{
		"IT Controller No": "878250022801",
		"Swing Motor No":   "SW2411001",
	}
	if got := planComponentsFilled(two); len(got) != 2 {
		t.Errorf("ITC + SM = %v, want 2", got)
	}

	full := map[string]string{
		"IT Controller No": "878250022801",
		"Swing Motor No":   "SW2411001",
		"Pump Assy HYD No": "PH2411001",
		"Motor Propel No":  "MP2411001",
		"Control Valve No": "CV2411001",
		"CW No":            "CW2411001",
		"Engine":           "J05E-0007",
	}
	if got := planComponentsFilled(full); len(got) != 7 {
		t.Errorf("กรอกครบทุกชนิด = %v, want 7", got)
	}

	if got := planComponentsFilled(map[string]string{}); len(got) != 0 {
		t.Errorf("ไม่กรอกเลย = %v, want 0", got)
	}
}

func TestMFGScanSwingMotor(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "mfg@kobelco.com", "mfg07", "MFG", "MFG")

	data, _ := json.Marshal(map[string]string{
		"Machine No":     "LX10400691",
		"Swing Motor No": "SW2411002",
		"Country Name":   "Vietnam",
	})
	db.Create(&models.UploadDataRow{
		Dataset:   models.DatasetPlanning,
		MachineNo: "LX10400691",
		DataJSON:  string(data),
	})

	body := `{"machineNo":"LX10400691","serialNo":"SW2411002"}`

	c, rec := newContext("POST", body, u.ID, u.Username)
	ScanMFGAssembly(c)
	mustStatus(t, rec, 201)
	resp := decodeJSON(t, rec)

	if resp["component"] != ComponentSM {
		t.Errorf("component = %v, want SM", resp["component"])
	}
	if resp["status"] != models.MFGStatusNotMatched {
		t.Fatalf("status = %v, want NOT_MATCHED (WH ยังไม่ได้สแกน Swing Motor)", resp["status"])
	}
	if resp["whMissing"] != true {
		t.Errorf("whMissing = %v, want true", resp["whMissing"])
	}

	db.Create(&models.PartCheck{
		PartType:    ComponentSM,
		SN:          "SW2411002",
		MatchStatus: models.MatchStatusMatch,
		CheckedBy:   "WH",
	})

	c2, rec2 := newContext("POST", body, u.ID, u.Username)
	ScanMFGAssembly(c2)
	mustStatus(t, rec2, 201)
	resp2 := decodeJSON(t, rec2)

	if resp2["status"] != models.MFGStatusMatched {
		t.Fatalf("หลัง WH สแกนแล้ว status = %v, want MATCHED", resp2["status"])
	}
	if resp2["whMissing"] != false {
		t.Errorf("whMissing = %v, want false", resp2["whMissing"])
	}
}

func TestMFGScanSwingMotorMismatch(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "mfg@kobelco.com", "mfg07", "MFG", "MFG")

	for _, tc := range []struct{ mc, sm string }{
		{"LX10400690", "SW2411001"},
		{"LX10400691", "SW2411002"},
	} {
		data, _ := json.Marshal(map[string]string{
			"Machine No":     tc.mc,
			"Swing Motor No": tc.sm,
		})
		db.Create(&models.UploadDataRow{
			Dataset:   models.DatasetPlanning,
			MachineNo: tc.mc,
			DataJSON:  string(data),
		})
	}

	body := `{"machineNo":"LX10400690","serialNo":"SW2411002"}`
	c, rec := newContext("POST", body, u.ID, u.Username)
	ScanMFGAssembly(c)

	mustStatus(t, rec, 201)
	resp := decodeJSON(t, rec)

	if resp["status"] != models.MFGStatusNotMatched {
		t.Fatalf("status = %v, want NOT_MATCHED", resp["status"])
	}
	if resp["plannedITControllerNo"] != "SW2411001" {
		t.Errorf("planned = %v, want SW2411001", resp["plannedITControllerNo"])
	}
}

func TestPlannedNoOfReadsRealColumn(t *testing.T) {
	plan := map[string]string{"CW No": "CW2411001"}
	if got := PlannedNoOf(plan, ComponentCW); got != "CW2411001" {
		t.Errorf("อ่านคอลัมน์ CW No ไม่ได้: %q", got)
	}
}

func TestPlannedNoOfFallsBackToExtraColumn(t *testing.T) {
	for _, code := range []struct{ comp, key, val string }{
		{ComponentCW, extraColumnPrefix + "CW No", "CW2411001"},
		{ComponentSM, extraColumnPrefix + "Swing Motor No", "SW2411004"},
		{ComponentCV, extraColumnPrefix + "Control Valve No", "CV2411005"},
		{ComponentMP, extraColumnPrefix + "Motor Propel No", "MP2411006"},
		{ComponentPH, extraColumnPrefix + "Pump Assy HYD No", "PH2411007"},
	} {
		plan := map[string]string{code.key: code.val}
		if got := PlannedNoOf(plan, code.comp); got != code.val {
			t.Errorf("%s: อ่านจาก extra column ไม่ได้ = %q, want %q", code.comp, got, code.val)
		}
	}
}

func TestPlanningDatasetHasCWNoColumn(t *testing.T) {
	for _, ds := range []string{"planning"} {
		found := false
		for _, c := range udDatasets[ds].Columns {
			if c.Label == "CW No" {
				found = true
			}
		}
		if !found {
			t.Errorf("dataset %s ต้องมีคอลัมน์ CW No", ds)
		}
	}
}

func TestUploadDatasetsHaveNoAliasCollision(t *testing.T) {
	for name, ds := range udDatasets {
		owner := map[string]string{}
		for _, c := range ds.Columns {
			if len(c.Aliases) == 0 {
				t.Errorf("%s: คอลัมน์ %q ไม่มี alias", name, c.Label)
				continue
			}
			key := c.Aliases[0]
			if prev, dup := owner[key]; dup {
				t.Errorf("%s: %q กับ %q ใช้ alias ตัวแรกซ้ำกัน (%q) ตัวหลังจะอ่านค่าไม่ได้",
					name, prev, c.Label, key)
				continue
			}
			owner[key] = c.Label
		}
	}
}

func TestUploadDatasetsHaveNoDuplicateLabel(t *testing.T) {
	for name, ds := range udDatasets {
		seen := map[string]bool{}
		for _, c := range ds.Columns {
			if seen[c.Label] {
				t.Errorf("%s: คอลัมน์ %q ถูกประกาศซ้ำ", name, c.Label)
			}
			seen[c.Label] = true
		}
	}
}
