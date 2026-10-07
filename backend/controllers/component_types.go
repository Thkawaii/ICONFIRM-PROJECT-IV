package controllers

import (
	"strings"

	"iconfirm/models"
)

const (
	ComponentITC = "ITC"
	ComponentCV  = "CV"
	ComponentSM  = "SM"
	ComponentMP  = "MP"
	ComponentPH  = "PH"
	ComponentEN  = "EN"
	ComponentCW  = "CW"
)

type ComponentSpec struct {
	Code  string
	Label string

	PlanKeys []string

	Prefixes []string

	NeedsLicense bool

	NeedsWHScan bool

	// MasterType = ชนิดพาร์ทในทะเบียนพาร์ท (Master Data) สำหรับเทียบตอน WH สแกน
	MasterType string
}

var componentSpecs = []ComponentSpec{
	{
		Code:  ComponentITC,
		Label: "IT Controller",
		PlanKeys: []string{
			"IT Controller No", "IT Controller No.", "ITControllerNo",
		},

		Prefixes:     nil,
		NeedsLicense: true,
		NeedsWHScan:  true,
		MasterType:   "it_controller",
	},
	{
		Code:        ComponentCV,
		Label:       "Control Valve",
		PlanKeys:    []string{"Control Valve No", "Control Valve No.", "ControlValveNo", extraColumnPrefix + "Control Valve No"},
		Prefixes:    []string{"CV"},
		NeedsWHScan: true,
		MasterType:  "control_valve",
	},
	{
		Code:        ComponentSM,
		Label:       "Swing Motor",
		PlanKeys:    []string{"Swing Motor No", "Swing Motor No.", "SwingMotorNo", extraColumnPrefix + "Swing Motor No"},
		Prefixes:    []string{"SM", "SW"},
		NeedsWHScan: true,
		MasterType:  "swing_motor",
	},
	{
		Code:        ComponentMP,
		Label:       "Motor Propel",
		PlanKeys:    []string{"Motor Propel No", "Motor Propel No.", "MotorPropelNo", extraColumnPrefix + "Motor Propel No"},
		Prefixes:    []string{"MP"},
		NeedsWHScan: true,
		MasterType:  "motor_propel",
	},
	{
		Code:        ComponentPH,
		Label:       "Pump Assy HYD",
		PlanKeys:    []string{"Pump Assy HYD No", "Pump Assy HYD No.", "PumpAssyHYDNo", extraColumnPrefix + "Pump Assy HYD No"},
		Prefixes:    []string{"PH", "PA"},
		NeedsWHScan: true,
		MasterType:  "pump_assy_hyd",
	},
	{
		Code:  ComponentCW,
		Label: "Counter Weight",
		PlanKeys: []string{
			"CW No", "CW no", "CW No.", "CWNo",
			"CounterWeight No", "Counter Weight No", "CW Part No", "CW part no",
			extraColumnPrefix + "CW No", extraColumnPrefix + "CW no",
			extraColumnPrefix + "CounterWeight No", extraColumnPrefix + "Counter Weight No",
		},
		Prefixes:    []string{"CW"},
		NeedsWHScan: true,
	},
	{
		Code:        ComponentEN,
		Label:       "Engine",
		PlanKeys:    []string{"Engine", "ENGINE"},
		Prefixes:    nil,
		NeedsWHScan: true,
	},
}

var componentByCode = func() map[string]ComponentSpec {
	m := map[string]ComponentSpec{}
	for _, s := range componentSpecs {
		m[s.Code] = s
	}
	return m
}()

func ComponentLabel(code string) string {
	if s, ok := componentByCode[strings.ToUpper(strings.TrimSpace(code))]; ok {
		return s.Label
	}
	return code
}

func IsKnownComponent(code string) bool {
	_, ok := componentByCode[strings.ToUpper(strings.TrimSpace(code))]
	return ok
}

func ComponentNeedsLicense(code string) bool {
	s, ok := componentByCode[strings.ToUpper(strings.TrimSpace(code))]
	return ok && s.NeedsLicense
}

func ComponentNeedsWHScan(code string) bool {
	s, ok := componentByCode[strings.ToUpper(strings.TrimSpace(code))]
	return ok && s.NeedsWHScan
}

func AllComponentCodes() []string {
	out := make([]string, 0, len(componentSpecs))
	for _, s := range componentSpecs {
		out = append(out, s.Code)
	}
	return out
}

// MasterTypeOf: ชนิดพาร์ทในทะเบียนพาร์ท (Master Data) ของ component นี้ ("" = ไม่มีในทะเบียน)
func MasterTypeOf(code string) string {
	s, ok := componentByCode[strings.ToUpper(strings.TrimSpace(code))]
	if !ok {
		return ""
	}
	return s.MasterType
}

// ComponentLabelByMasterType: แปลงชนิดพาร์ทในทะเบียนกลับเป็นชื่อที่ใช้แสดง
func ComponentLabelByMasterType(masterType string) string {
	masterType = strings.ToLower(strings.TrimSpace(masterType))
	for _, s := range componentSpecs {
		if s.MasterType != "" && s.MasterType == masterType {
			return s.Label
		}
	}
	return ""
}

func PlannedNoOf(plan map[string]string, code string) string {
	s, ok := componentByCode[strings.ToUpper(strings.TrimSpace(code))]
	if !ok {
		return ""
	}
	return planValue(plan, s.PlanKeys...)
}

func looksLikeITControllerNo(s string) bool {
	return looks12Digit(s)
}

func DetectComponentType(serial string) string {
	serial = strings.ToUpper(strings.TrimSpace(serial))
	if serial == "" {
		return ""
	}

	if looksLikeITControllerNo(serial) {
		return ComponentITC
	}

	for _, s := range componentSpecs {
		for _, p := range s.Prefixes {
			if strings.HasPrefix(serial, p) {
				return s.Code
			}
		}
	}

	return ""
}

func DetectComponentFromPlan(plan map[string]string, serial string) string {
	serial = strings.TrimSpace(serial)
	if serial == "" || plan == nil {
		return ""
	}
	for _, s := range componentSpecs {
		if v := planValue(plan, s.PlanKeys...); v != "" && strings.EqualFold(v, serial) {
			return s.Code
		}
	}
	return ""
}

func planComponentsFilled(data map[string]string) []string {
	var filled []string
	for _, s := range componentSpecs {
		if planValue(data, s.PlanKeys...) != "" {
			filled = append(filled, s.Label)
		}
	}
	return filled
}

func engineRowFor(pn, sn string) (map[string]string, bool) {
	pn = strings.TrimSpace(pn)
	sn = strings.TrimSpace(sn)
	if pn == "" || sn == "" {
		return nil, false
	}

	for _, row := range loadUploadRows(models.DatasetEngine) {
		engine := strings.TrimSpace(pickField(row, "ENGINE", "Engine"))
		history := strings.TrimSpace(pickField(row, "History", "Engine History"))

		if engine == "" && history == "" {
			continue
		}

		if (SameCode(engine, pn) && SameCode(history, sn)) ||
			(SameCode(history, pn) && SameCode(engine, sn)) {
			return row, true
		}
	}

	return nil, false
}

func engineRowByValue(v string) (map[string]string, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, false
	}
	for _, row := range loadUploadRows(models.DatasetEngine) {
		engine := strings.TrimSpace(pickField(row, "ENGINE", "Engine"))
		history := strings.TrimSpace(pickField(row, "History", "Engine History"))
		if SameCode(engine, v) || SameCode(history, v) {
			return row, true
		}
	}
	return nil, false
}
