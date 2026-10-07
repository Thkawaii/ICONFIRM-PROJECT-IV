package controllers

import (
	"strings"

	"iconfirm/models"
)

const (
	PlanStateMatch       = "MATCH"
	PlanStateMismatch    = "MISMATCH"
	PlanStateNoScan      = "NO_SCAN"
	PlanStateNoPlan      = "NO_PLAN"
	PlanStateNoITC       = "NO_ITC_PLAN"
	PlanStateNotInMaster = "NOT_IN_MASTER"

	// IT Controller: MFG สแกน QR บน Specification sheet แล้วเทียบกับตาราง Planning
	PlanStateSpecMismatch = "SPEC_MISMATCH"
	PlanStateNoQR         = "NO_QR"
)

var planITCKeys = []string{
	"IT Controller No", "IT Controller No.", "ITControllerNo",
	extraColumnPrefix + "IT Controller No", extraColumnPrefix + "IT Controller No.",
}

var planCountryKeys = []string{"Country Name", "Destination", "Country"}

var planDeviceKeys = []string{"IT device", "IT Device", "ITDevice"}

func planValue(plan map[string]string, keys ...string) string {
	if plan == nil {
		return ""
	}
	v := strings.TrimSpace(unwrapExcelText(pickField(plan, keys...)))
	if v == "-" {
		return ""
	}
	return v
}

func PlannedITCOf(plan map[string]string) string { return planValue(plan, planITCKeys...) }

func plannedCountryOf(plan map[string]string) string { return planValue(plan, planCountryKeys...) }

func plannedDeviceOf(plan map[string]string) string { return planValue(plan, planDeviceKeys...) }

func loadMachinePlans() map[string]map[string]string {
	return machineIndex()
}

func planForMachine(machineNo string) map[string]string {
	machineNo = strings.TrimSpace(machineNo)
	if machineNo == "" {
		return nil
	}
	return machineIndex()[machineNo]
}

type MFGPlanResult struct {
	State        string `json:"state"`
	Component    string `json:"component"`
	Label        string `json:"componentLabel"`
	PlannedITC   string `json:"plannedITControllerNo"`
	ScannedITC   string `json:"scannedITControllerNo"`
	OwnerMachine string `json:"ownerMachineNo"`

	Message string `json:"message"`
	Detail  string `json:"detail"`
}

func (r MFGPlanResult) OK() bool { return r.State == PlanStateMatch }

type mfgPlanResolver struct {
	planByMachine map[string]map[string]string

	machineByCode map[string]string

	itcOwner  map[string]string
	masterITC map[string]bool

	// specPlans: ตาราง Planning (Specification sheet) ล่าสุด สำหรับเทียบกับ QR
	specPlans map[string]map[string]string
}

func newMFGPlanResolver() *mfgPlanResolver {
	r := &mfgPlanResolver{
		planByMachine: loadMachinePlans(),
		machineByCode: map[string]string{},
		itcOwner:      map[string]string{},
		masterITC:     map[string]bool{},
		specPlans:     loadSpecPlans(),
	}

	for mc, plan := range r.planByMachine {
		if key := NormalizeCodeValue(mc); key != "" {
			if _, ok := r.machineByCode[key]; !ok {
				r.machineByCode[key] = mc
			}
		}
		if itc := PlannedITCOf(plan); itc != "" {
			key := NormalizeCodeValue(itc)
			if _, ok := r.itcOwner[key]; !ok {
				r.itcOwner[key] = mc
			}
		}
	}

	// ไม่มีทะเบียนกลางให้เทียบ masterITC จึงว่างเสมอ

	return r
}

func (r *mfgPlanResolver) planOf(machineNo string) map[string]string {
	machineNo = strings.TrimSpace(machineNo)
	if machineNo == "" {
		return nil
	}
	if plan, ok := r.planByMachine[machineNo]; ok {
		return plan
	}

	if old := ResolveMachineNo(machineNo); old != machineNo {
		if plan, ok := r.planByMachine[old]; ok {
			return plan
		}
		if key, ok := r.machineByCode[NormalizeCodeValue(old)]; ok {
			return r.planByMachine[key]
		}
	}

	if key, ok := r.machineByCode[NormalizeCodeValue(machineNo)]; ok {
		return r.planByMachine[key]
	}
	return nil
}

func (r *mfgPlanResolver) evaluate(machineNo, scanned string) MFGPlanResult {
	return r.evaluateComponent(machineNo, scanned, "")
}

func (r *mfgPlanResolver) evaluateComponent(machineNo, scanned, component string) MFGPlanResult {
	machineNo = strings.TrimSpace(machineNo)
	scanned = strings.TrimSpace(scanned)

	plan := r.planOf(machineNo)

	component = strings.ToUpper(strings.TrimSpace(component))

	if component != "" {
		scanned = ResolveComponentSerial(component, scanned)
	} else {
		scanned = ResolveScannedCode(scanned)
	}

	if component == "" {
		component = DetectComponentFromPlan(plan, scanned)
	}
	if component == "" {
		component = DetectComponentType(scanned)
	}

	if component != "" {
		scanned = ResolveComponentSerial(component, scanned)
	}

	res := MFGPlanResult{
		Component:  component,
		Label:      ComponentLabel(component),
		ScannedITC: scanned,
	}
	if component != "" {
		res.PlannedITC = PlannedNoOf(plan, component)
	} else {
		res.PlannedITC = PlannedITCOf(plan)
	}

	switch {
	case scanned == "":
		res.State = PlanStateNoScan

	case component == "":

		res.State = PlanStateNotInMaster

	case component == ComponentITC && !r.masterITC[NormalizeCodeValue(scanned)]:
		res.State = PlanStateNotInMaster

	case plan == nil:
		res.State = PlanStateNoPlan

	case res.PlannedITC == "":
		res.State = PlanStateNoITC

	case !SameCode(scanned, res.PlannedITC):
		res.State = PlanStateMismatch
		res.OwnerMachine = r.ownerOf(component, scanned)

	default:
		res.State = PlanStateMatch
	}

	res.Message = mfgPlanMessage(res)
	res.Detail = mfgPlanDetail(machineNo, res)
	return res
}

// evaluateWithSpec: ตรวจรายการที่ MFG สแกน
//   - Kanban อย่างเดียว (ไม่มีพาร์ท): ผลขึ้นกับ QR บน Specification sheet เทียบกับ Daily Plan อย่างเดียว
//   - IT Controller (รายการรูปแบบเดิม): ตรวจ P/N-S/N ในทะเบียนตามเดิม แต่ "ตรงแผน" ดูจาก QR
//   - พาร์ทอื่น: ใช้วิธีเดิม
func (r *mfgPlanResolver) evaluateWithSpec(machineNo, scanned, component, qrCode string) (MFGPlanResult, SpecCheckResult) {
	spec := CheckSpecQR(r.specPlans, qrCode)

	var res MFGPlanResult
	if strings.TrimSpace(scanned) == "" && strings.TrimSpace(qrCode) != "" {
		// Scan Kanban ขั้นตอนเดียว
		res = MFGPlanResult{}
	} else {
		res = r.evaluateComponent(machineNo, scanned, component)
		if res.Component != ComponentITC {
			return res, spec
		}
		switch res.State {
		case PlanStateNoITC, PlanStateMatch, PlanStateNoPlan:
			// ใช้ผลจาก QR แทน
		default:
			// NO_SCAN / NOT_IN_MASTER / MISMATCH → คงผลเดิม
			return res, spec
		}
	}

	if spec.MachineNo != "" && strings.TrimSpace(machineNo) != "" &&
		!SameCode(spec.MachineNo, machineNo) &&
		!SameCode(ResolveMachineNo(spec.MachineNo), ResolveMachineNo(machineNo)) {
		spec.State = SpecStateMismatch
		spec.Matched = false
		spec.Message = "Kanban ไม่ใช่ของเครื่องนี้"
		spec.Detail = "Kanban เป็นของเครื่อง " + spec.MachineNo + " แต่บันทึกเป็นเครื่อง " + machineNo
	}

	switch spec.State {
	case SpecStateMatch:
		res.State = PlanStateMatch
	case SpecStateNoPlan:
		res.State = PlanStateNoPlan
	case SpecStateMismatch:
		res.State = PlanStateSpecMismatch
	default:
		res.State = PlanStateNoQR
	}
	res.Message = mfgPlanMessage(res)
	if spec.Detail != "" {
		res.Detail = spec.Detail
	} else {
		res.Detail = mfgPlanDetail(machineNo, res)
	}
	return res, spec
}

func (r *mfgPlanResolver) ownerOf(component, serial string) string {
	if component == ComponentITC {
		if mc, ok := r.itcOwner[NormalizeCodeValue(serial)]; ok {
			return mc
		}
	}
	for mc, plan := range r.planByMachine {
		if v := PlannedNoOf(plan, component); v != "" && SameCode(v, serial) {
			return mc
		}
	}
	return ""
}

func mfgPlanMessage(res MFGPlanResult) string {
	switch res.State {
	case PlanStateNoScan:
		return "ยังไม่ได้สแกนหมายเลขพาร์ท"

	case PlanStateNotInMaster, PlanStateNoPlan:
		return "ไม่พบข้อมูล กรุณาติดต่อ ADMIN"

	case PlanStateMismatch, PlanStateNoITC:
		return "ข้อมูลไม่ตรง"

	case PlanStateSpecMismatch:
		return "ข้อมูลไม่ตรงกับ Daily Plan"

	case PlanStateNoQR:
		return "ยังไม่ได้ Scan Kanban"

	default:
		return "ข้อมูลตรง"
	}
}

func mfgPlanDetail(machineNo string, res MFGPlanResult) string {
	mc := machineNo
	if mc == "" {
		mc = "(ไม่ระบุ)"
	}

	part := res.Label
	if part == "" {
		part = "พาร์ท"
	}

	switch res.State {
	case PlanStateNoScan:
		return "ต้องสแกนทั้ง Machine No. และหมายเลขพาร์ท จึงจะยืนยันได้ว่าประกอบตรงแผน"

	case PlanStateNotInMaster:
		if res.Component == "" {
			return "หมายเลข " + res.ScannedITC + " ไม่ตรงกับพาร์ทชนิดใดในแผนของเครื่อง " + mc +
				" และไม่รู้จักคำนำหน้า (รองรับ CV / SM / MP / PH / CW และเลข IT Controller)"
		}
		return "ไม่พบ " + part + " " + res.ScannedITC + " ในทะเบียน Master Data"

	case PlanStateNoPlan:
		return "ไม่พบแผนประกอบของเครื่อง " + mc +
			" ใน Master Data (Planning / WH1 / WH2 / Engine)"

	case PlanStateNoITC:
		return "แผนของเครื่อง " + mc + " ไม่ได้กำหนด " + part + " ไว้ แต่มีการสแกน " + res.ScannedITC

	case PlanStateMismatch:
		d := "เครื่อง " + mc + " ต้องใช้ " + part + " " + res.PlannedITC +
			" แต่สแกนได้ " + res.ScannedITC
		if res.OwnerMachine != "" {
			d += " (เลขนี้เป็นของเครื่อง " + res.OwnerMachine + ")"
		}
		return d

	default:
		return part + " ตรงกับแผนประกอบใน Master Data"
	}
}

func mfgStatusFromPlan(duplicate bool, planState string, whMatched bool) string {
	return mfgStatusFor(ComponentITC, duplicate, planState, whMatched)
}

func mfgStatusFor(component string, duplicate bool, planState string, whMatched bool) string {
	switch {
	case planState != PlanStateMatch:
		return models.MFGStatusNotMatched
	case duplicate:
		return models.MFGStatusDuplicate
	case !ComponentNeedsWHScan(component):
		return models.MFGStatusMatched
	case whMatched:
		return models.MFGStatusMatched
	default:
		return models.MFGStatusNotMatched
	}
}

func mfgDisplayStatus(saved, computed string) string {
	if computed != models.MFGStatusMatched {
		return computed
	}
	if strings.EqualFold(strings.TrimSpace(saved), models.MFGStatusMatched) {
		return models.MFGStatusMatched
	}
	return models.MFGStatusNotMatched
}

func mfgFinalMessage(status string, res MFGPlanResult, licenseNo string) string {
	switch status {
	case models.MFGStatusDuplicate:
		return "รายการนี้เคยบันทึกไปแล้ว"

	case models.MFGStatusMatched:
		return "ข้อมูลตรง บันทึกรายการสำเร็จ"

	default:
		if res.State == PlanStateMatch {

			if label := strings.TrimSpace(res.Label); label != "" {
				return "ข้อมูลตรง แต่ต้องให้ WH สแกน " + label + " ก่อนจึงจะประกอบได้"
			}
			return "ข้อมูลตรง แต่ต้องให้ WH สแกนก่อนจึงจะประกอบได้"
		}
		return res.Message
	}
}
