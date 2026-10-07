package controllers

import (
	"encoding/json"
	"testing"

	"iconfirm/models"
)

func TestQAScanKeyNormalises(t *testing.T) {
	cases := map[string]string{
		"CV-2411 001":       "CV2411001",
		"cv2411001":         "CV2411001",
		`="878250022802"`:   "878250022802",
		"  SW/2411.001  ":   "SW2411001",
		"-":                 "",
		"":                  "",
		"MP_2411_001":       "MP2411001",
		"878250022802.0000": "8782500228020000",
	}
	for in, want := range cases {
		if got := qaScanKey(in); got != want {
			t.Errorf("qaScanKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQAPartScanSummaryCountsPlannedParts(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "qa@kobelco.com", "qa07", "QA", "QA")

	seedComponentPlan(t, db, "LX10400690", map[string]string{
		"IT Controller No": "878250022802",
		"Control Valve No": "CV2411001",
		"Swing Motor No":   "SW2411001",
		"Country Name":     "Indonesia",
	})

	seedWHCheck(t, db, ComponentCV, "", "CV2411001", "")

	db.Create(&models.MFGAssembly{
		MachineNo:      "LX10400690",
		ITControllerNo: "CV2411001",
		Component:      ComponentCV,
		Status:         models.MFGStatusMatched,
	})

	c, rec := newContext("GET", "", u.ID, u.Username)
	GetQAPartScanSummary(c)
	mustStatus(t, rec, 200)

	var resp QAPartScanSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Components) != len(qaScanComponentOrder) {
		t.Errorf("components = %d, want %d", len(resp.Components), len(qaScanComponentOrder))
	}
	if resp.Machines != 1 {
		t.Errorf("machines = %d, want 1", resp.Machines)
	}
	if len(resp.Units) != 3 {
		t.Fatalf("units = %d, want 3 (เฉพาะพาร์ทที่แผนกำหนด)", len(resp.Units))
	}

	byComp := map[string]QAScanUnit{}
	for _, u := range resp.Units {
		byComp[u.Component] = u
	}

	cv, ok := byComp[ComponentCV]
	if !ok {
		t.Fatal("ไม่พบหน่วย CV")
	}
	if !cv.Scanned {
		t.Error("CV: scanned = false, want true")
	}
	if !cv.Assembled {
		t.Error("CV: assembled = false, want true")
	}
	if cv.PlannedNo != "CV2411001" {
		t.Errorf("CV: plannedNo = %q", cv.PlannedNo)
	}

	sm, ok := byComp[ComponentSM]
	if !ok {
		t.Fatal("ไม่พบหน่วย SM")
	}
	if sm.Scanned {
		t.Error("SM: ยังไม่ได้สแกน แต่ scanned = true")
	}
	if sm.Assembled {
		t.Error("SM: ยังไม่ได้ประกอบ แต่ assembled = true")
	}

	if _, ok := byComp[ComponentMP]; ok {
		t.Error("แผนไม่ได้กำหนด MP จึงไม่ควรมีหน่วยนี้")
	}
}

func TestQAPartScanSummaryPullsEngineFromEngineDataset(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "qa@kobelco.com", "qa07", "QA", "QA")

	seedComponentPlan(t, db, "LX10400690", map[string]string{"IT Controller No": "878250022802"})

	engine, _ := json.Marshal(map[string]string{
		"Machine No": "LX10400690",
		"ENGINE":     "EN2411001",
		"History":    "HIST001",
	})
	db.Create(&models.UploadDataRow{
		Dataset:   models.DatasetEngine,
		MachineNo: "LX10400690",
		DataJSON:  string(engine),
	})

	c, rec := newContext("GET", "", u.ID, u.Username)
	GetQAPartScanSummary(c)
	mustStatus(t, rec, 200)

	var resp QAPartScanSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	found := false
	for _, u := range resp.Units {
		if u.Component == ComponentEN {
			found = true
			if u.PlannedNo != "EN2411001" {
				t.Errorf("Engine plannedNo = %q, want EN2411001", u.PlannedNo)
			}
		}
	}
	if !found {
		t.Error("Engine ต้องถูกดึงมาจากไฟล์ Engine")
	}
}

func TestQAPartScanSummaryEmpty(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "qa@kobelco.com", "qa07", "QA", "QA")
	_ = db

	c, rec := newContext("GET", "", u.ID, u.Username)
	GetQAPartScanSummary(c)
	mustStatus(t, rec, 200)

	var resp QAPartScanSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Units) != 0 {
		t.Errorf("units = %d, want 0", len(resp.Units))
	}
	if resp.GeneratedAt == "" {
		t.Error("generatedAt ต้องไม่ว่าง")
	}
}
