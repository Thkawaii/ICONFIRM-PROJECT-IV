package controllers

import (
	"encoding/json"
	"testing"

	"iconfirm/models"

	"gorm.io/gorm"
)

func seedComponentPlan(t *testing.T, db *gorm.DB, machineNo string, fields map[string]string) {
	t.Helper()

	data := map[string]string{"Machine No": machineNo}
	for k, v := range fields {
		data[k] = v
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	row := models.UploadDataRow{
		Dataset:   models.DatasetPlanning,
		MachineNo: machineNo,
		DataJSON:  string(b),
		FileName:  "test-plan",
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}
}

func seedWHCheck(t *testing.T, db *gorm.DB, partType, pn, sn, machineNo string) {
	t.Helper()
	row := models.PartCheck{
		PartType:    partType,
		PN:          pn,
		SN:          sn,
		MachineNo:   machineNo,
		MatchStatus: models.MatchStatusMatch,
		CheckedBy:   "WH",
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed part check: %v", err)
	}
}

var whGateCases = []struct {
	component string
	planKey   string
	serial    string
}{
	{ComponentCV, "Control Valve No", "CV2411001"},
	{ComponentSM, "Swing Motor No", "SW2411001"},
	{ComponentMP, "Motor Propel No", "MP2411001"},
	{ComponentPH, "Pump Assy HYD No", "PH2411001"},
	{ComponentCW, "CW No", "CW2411001"},
}

func TestAssemblyBlockedUntilWHScans(t *testing.T) {
	for _, tc := range whGateCases {
		t.Run(tc.component, func(t *testing.T) {
			db := newTestDB(t)
			u := makeUser(t, db, "mfg@kobelco.com", "mfg07", "MFG", "MFG")

			seedComponentPlan(t, db, "LX10400690", map[string]string{tc.planKey: tc.serial})

			body := `{"machineNo":"LX10400690","serialNo":"` + tc.serial + `"}`

			c, rec := newContext("POST", body, u.ID, u.Username)
			ScanMFGAssembly(c)
			mustStatus(t, rec, 201)

			resp := decodeJSON(t, rec)
			if resp["component"] != tc.component {
				t.Fatalf("component = %v, want %s", resp["component"], tc.component)
			}
			if resp["status"] != models.MFGStatusNotMatched {
				t.Fatalf("ยังไม่ได้สแกน WH: status = %v, want NOT_MATCHED", resp["status"])
			}
			if resp["whMissing"] != true {
				t.Errorf("whMissing = %v, want true", resp["whMissing"])
			}
			if resp["plannedState"] != PlanStateMatch {
				t.Errorf("plannedState = %v, want MATCH (ตรงแผน แต่ติดที่ WH)", resp["plannedState"])
			}

			seedWHCheck(t, db, tc.component, "", tc.serial, "")

			c2, rec2 := newContext("POST", body, u.ID, u.Username)
			ScanMFGAssembly(c2)
			mustStatus(t, rec2, 201)

			resp2 := decodeJSON(t, rec2)
			if resp2["status"] != models.MFGStatusMatched {
				t.Fatalf("หลัง WH สแกน: status = %v, want MATCHED", resp2["status"])
			}
			if resp2["whMatched"] != true {
				t.Errorf("whMatched = %v, want true", resp2["whMatched"])
			}

			var count int64
			db.Model(&models.MFGAssembly{}).Where("machine_no = ?", "LX10400690").Count(&count)
			if count != 1 {
				t.Fatalf("แถว MFG = %d, want 1 (ต้องแก้แถวเดิม)", count)
			}
		})
	}
}

func TestEngineAssemblyBlockedUntilWHScans(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "mfg@kobelco.com", "mfg07", "MFG", "MFG")

	seedComponentPlan(t, db, "LX10400690", map[string]string{"Engine": "EN2411001"})

	body := `{"machineNo":"LX10400690","serialNo":"EN2411001","partType":"EN"}`

	c, rec := newContext("POST", body, u.ID, u.Username)
	ScanMFGAssembly(c)
	mustStatus(t, rec, 201)

	if got := decodeJSON(t, rec)["status"]; got != models.MFGStatusNotMatched {
		t.Fatalf("Engine ที่ WH ยังไม่รับ: status = %v, want NOT_MATCHED", got)
	}

	seedWHCheck(t, db, ComponentEN, "EN2411001", "HIST001", "LX10400690")

	c2, rec2 := newContext("POST", body, u.ID, u.Username)
	ScanMFGAssembly(c2)
	mustStatus(t, rec2, 201)

	if got := decodeJSON(t, rec2)["status"]; got != models.MFGStatusMatched {
		t.Fatalf("Engine หลัง WH รับแล้ว: status = %v, want MATCHED", got)
	}
}

func TestFindWHPartCheckMatchesSNAndPN(t *testing.T) {
	db := newTestDB(t)

	seedWHCheck(t, db, ComponentCV, "", "CV2411001", "")
	seedWHCheck(t, db, ComponentEN, "EN2411001", "HIST001", "LX1")
	seedWHCheck(t, db, ComponentITC, "YN22E00849FA", "KQ300", "878250022802")

	if findWHPartCheck(ComponentCV, "CV2411001") == nil {
		t.Error("ต้องหาเจอจากคอลัมน์ sn")
	}
	if findWHPartCheck(ComponentEN, "EN2411001") == nil {
		t.Error("ต้องหาเจอจากคอลัมน์ pn")
	}
	if findWHPartCheck(ComponentEN, "HIST001") == nil {
		t.Error("Engine ต้องหาเจอจากทั้ง pn และ sn")
	}
	if findWHPartCheck(ComponentITC, "878250022802") == nil {
		t.Error("ITC ต้องหาเจอจากคอลัมน์ machine_no")
	}

	if findWHPartCheck(ComponentSM, "CV2411001") != nil {
		t.Error("ต้องไม่ข้ามชนิดพาร์ท")
	}
	if findWHPartCheck(ComponentCV, "") != nil {
		t.Error("เลขว่างต้องไม่เจอ")
	}
	if findWHPartCheck(ComponentCV, "NOPE") != nil {
		t.Error("เลขที่ไม่มีต้องไม่เจอ")
	}
}

func TestFindWHPartCheckIgnoresFailedScans(t *testing.T) {
	db := newTestDB(t)

	db.Create(&models.PartCheck{
		PartType:    ComponentCV,
		SN:          "CV2411001",
		MatchStatus: models.MatchStatusNotFound,
		CheckedBy:   "WH",
	})

	if findWHPartCheck(ComponentCV, "CV2411001") != nil {
		t.Error("การสแกนที่ไม่ผ่านต้องไม่นับว่า WH ยืนยันแล้ว")
	}
	if latestWHPartCheckAnyStatus(ComponentCV, "CV2411001") == nil {
		t.Error("ต้องยังดึงผลสแกนล่าสุดมาบอกเหตุผลได้")
	}
}

func TestEnrichMFGWithWHUsesComponent(t *testing.T) {
	db := newTestDB(t)

	seedComponentPlan(t, db, "LX10400690", map[string]string{"Control Valve No": "CV2411001"})
	seedWHCheck(t, db, ComponentCV, "", "CV2411001", "")

	row := models.MFGAssembly{
		MachineNo:      "LX10400690",
		ITControllerNo: "CV2411001",
		Component:      ComponentCV,
	}
	enrichMFGWithWH(&row)

	if !row.WHMatched {
		t.Fatal("WHMatched = false, want true")
	}
	if !row.WHRequired {
		t.Error("WHRequired = false, want true")
	}
	if row.ComponentLabel != "Control Valve" {
		t.Errorf("ComponentLabel = %q, want Control Valve", row.ComponentLabel)
	}

	missing := models.MFGAssembly{
		MachineNo:      "LX10400690",
		ITControllerNo: "CV9999999",
		Component:      ComponentCV,
	}
	enrichMFGWithWH(&missing)
	if missing.WHMatched {
		t.Error("พาร์ทที่ WH ยังไม่รับ ต้องได้ WHMatched = false")
	}
}

func TestGetMFGAssembliesReportsWHGate(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "mfg@kobelco.com", "mfg07", "MFG", "MFG")

	seedComponentPlan(t, db, "LX10400690", map[string]string{"Control Valve No": "CV2411001"})
	db.Create(&models.MFGAssembly{
		MachineNo:      "LX10400690",
		ITControllerNo: "CV2411001",
		Component:      ComponentCV,
		Status:         models.MFGStatusMatched,
	})

	c, rec := newContext("GET", "", u.ID, u.Username)
	GetMFGAssemblies(c)
	mustStatus(t, rec, 200)

	var rows []models.MFGAssembly
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Status != models.MFGStatusNotMatched {
		t.Fatalf("status = %q, want NOT_MATCHED (WH ยังไม่ยืนยัน)", rows[0].Status)
	}
	if !rows[0].WHRequired {
		t.Error("WHRequired = false, want true")
	}
	if rows[0].ComponentLabel != "Control Valve" {
		t.Errorf("ComponentLabel = %q", rows[0].ComponentLabel)
	}
}

func TestMFGListDoesNotAutoMatchAfterWHScans(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "mfg@kobelco.com", "mfg07", "MFG", "MFG")

	seedComponentPlan(t, db, "MC-002", map[string]string{"Swing Motor No": "SM-0002"})

	body := `{"machineNo":"MC-002","serialNo":"SM-0002","partType":"SM"}`

	c, rec := newContext("POST", body, u.ID, u.Username)
	ScanMFGAssembly(c)
	mustStatus(t, rec, 201)
	if got := decodeJSON(t, rec)["status"]; got != models.MFGStatusNotMatched {
		t.Fatalf("ตอนสแกน: status = %v, want NOT_MATCHED", got)
	}

	seedWHCheck(t, db, ComponentSM, "", "SM-0002", "")

	c2, rec2 := newContext("GET", "", u.ID, u.Username)
	GetMFGAssemblies(c2)
	mustStatus(t, rec2, 200)

	var rows []models.MFGAssembly
	if err := json.Unmarshal(rec2.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Status != models.MFGStatusNotMatched {
		t.Fatalf("หลัง WH สแกน แต่ MFG ยังไม่สแกนซ้ำ: status = %q, want NOT_MATCHED",
			rows[0].Status)
	}
	if !rows[0].WHMatched {
		t.Error("WHMatched = false, want true (ต้องบอกได้ว่า WH รับเข้าคลังแล้ว)")
	}

	c3, rec3 := newContext("POST", body, u.ID, u.Username)
	ScanMFGAssembly(c3)
	mustStatus(t, rec3, 201)
	if got := decodeJSON(t, rec3)["status"]; got != models.MFGStatusMatched {
		t.Fatalf("MFG สแกนซ้ำ: status = %v, want MATCHED", got)
	}

	c4, rec4 := newContext("GET", "", u.ID, u.Username)
	GetMFGAssemblies(c4)
	mustStatus(t, rec4, 200)

	rows = nil
	if err := json.Unmarshal(rec4.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != models.MFGStatusMatched {
		t.Fatalf("หลัง MFG สแกนซ้ำ: rows = %d status = %q, want 1 / MATCHED",
			len(rows), rows[0].Status)
	}
}
