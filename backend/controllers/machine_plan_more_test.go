package controllers

import (
	"strconv"
	"strings"
	"testing"

	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

func TestMFGFinalMessageNamesTheBlockingPart(t *testing.T) {
	res := MFGPlanResult{State: PlanStateMatch, Component: ComponentCV, Label: "Control Valve"}

	got := mfgFinalMessage(models.MFGStatusNotMatched, res, "")
	for _, want := range []string{"Control Valve", "WH"} {
		if !strings.Contains(got, want) {
			t.Errorf("ข้อความขาด %q: %s", want, got)
		}
	}

	if ok := mfgFinalMessage(models.MFGStatusMatched, res, ""); ok != "ข้อมูลตรง บันทึกรายการสำเร็จ" {
		t.Errorf("MATCHED message = %q", ok)
	}
}

func TestMFGPlanDetailMentionsPartLabel(t *testing.T) {
	res := MFGPlanResult{
		State:      PlanStateMismatch,
		Component:  ComponentCW,
		Label:      "Counter Weight",
		PlannedITC: "CW2411001",
		ScannedITC: "CW2411099",
	}
	detail := mfgPlanDetail("LX10400690", res)
	for _, want := range []string{"Counter Weight", "CW2411001", "CW2411099", "LX10400690"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail ขาด %q: %s", want, detail)
		}
	}
}

func TestPlannedNoOfUnknownComponent(t *testing.T) {
	plan := map[string]string{"IT Controller No": "878250022802"}
	if got := PlannedNoOf(plan, "ZZ"); got != "" {
		t.Errorf("ชนิดที่ไม่รู้จัก = %q, want empty", got)
	}
	if got := PlannedNoOf(nil, ComponentITC); got != "" {
		t.Errorf("plan nil = %q, want empty", got)
	}
}

func TestPlanValueTreatsDashAsEmpty(t *testing.T) {
	plan := map[string]string{"Control Valve No": "-", "Swing Motor No": "  SW1  "}
	if got := PlannedNoOf(plan, ComponentCV); got != "" {
		t.Errorf(`"-" ต้องถือว่าว่าง แต่ได้ %q`, got)
	}
	if got := PlannedNoOf(plan, ComponentSM); got != "SW1" {
		t.Errorf("ค่าที่มีช่องว่าง = %q, want SW1", got)
	}
}

func TestComponentLabelFallsBackToCode(t *testing.T) {
	if got := ComponentLabel(ComponentPH); got != "Pump Assy HYD" {
		t.Errorf("PH label = %q", got)
	}
	if got := ComponentLabel("ZZ"); got != "ZZ" {
		t.Errorf("label ที่ไม่รู้จัก = %q, want ZZ", got)
	}
}

func TestIsKnownComponent(t *testing.T) {
	for _, code := range AllComponentCodes() {
		if !IsKnownComponent(code) {
			t.Errorf("%s ต้องเป็นชนิดที่รู้จัก", code)
		}
		if !IsKnownComponent(strings.ToLower(code)) {
			t.Errorf("%s ต้องรับตัวพิมพ์เล็กได้", code)
		}
	}
	if IsKnownComponent("ZZ") {
		t.Error("ZZ ไม่ควรเป็นชนิดที่รู้จัก")
	}
}

func TestDetectComponentFromPlanBeatsPrefix(t *testing.T) {
	plan := map[string]string{"Control Valve No": "SW2411001"}
	if got := DetectComponentFromPlan(plan, "SW2411001"); got != ComponentCV {
		t.Errorf("แผนต้องชนะการเดาจากคำนำหน้า = %q, want CV", got)
	}
	if got := DetectComponentFromPlan(nil, "SW2411001"); got != "" {
		t.Errorf("plan nil = %q, want empty", got)
	}
}

func TestUpdateMFGAssemblyKeepsWHGate(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "mfg@kobelco.com", "mfg07", "MFG", "MFG")

	seedComponentPlan(t, db, "LX10400690", map[string]string{"Control Valve No": "CV2411001"})

	row := models.MFGAssembly{
		MachineNo:      "LX10400690",
		ITControllerNo: "CV2411001",
		Component:      ComponentCV,
		Status:         models.MFGStatusMatched,
	}
	db.Create(&row)

	body := `{"machineNo":"LX10400690","itControllerNo":"CV2411001"}`
	c, rec := newContext("PATCH", body, u.ID, u.Username)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(row.ID), 10)}}
	UpdateMFGAssembly(c)
	mustStatus(t, rec, 200)

	var got models.MFGAssembly
	db.First(&got, row.ID)
	if got.Status != models.MFGStatusNotMatched {
		t.Fatalf("status = %q, want NOT_MATCHED (WH ยังไม่ยืนยัน)", got.Status)
	}
	if got.Component != ComponentCV {
		t.Errorf("component = %q, want CV", got.Component)
	}
}
