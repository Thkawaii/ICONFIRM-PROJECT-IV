package controllers

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// ขั้นตอนการทำงาน 2 แบบในไฟล์นี้
//
// ── IT Controller / Engine (ของเดิม) ───────────────────────────────────────
// WH  : 1. อัปโหลดไฟล์ Planning WH (MC#, IT#, S/N#)
//       2. เลือก MC# → สแกน IT# และ S/N# → เทียบกับแผนของเครื่องนั้น
// MFG : 1. สแกน MC# จาก QR บน Kanban
//       2. ระบบแสดง IT# & S/N# ที่ WH จ่ายมา → MFG ตรวจว่าถูกต้องไหม
//       3. ถ่ายรูป & บันทึก
//
// ── CW / CV / SM / MP / PH (ขั้นตอนใหม่ — ยึด master_data) ──────────────────
// WH  : 1. เลือก MC# → สแกน Part# → ระบบเช็คว่าตรงตาม master_data ไหม
// MFG : 1. สแกน MC# จาก QR บน Kanban — เอา MC# และ Product Spec บน Kanban
//          ไปเทียบ master_data ว่ามี Product Spec นี้จริงไหม และ P/N ตรงกันไหม
//       2. ระบบแสดง Part# ที่ WH จ่าย → MFG ตรวจว่าถูกต้องไหม
//       3. สแกน S/N#
//       4. ถ่ายรูป & บันทึก
//
// ไฟล์ Planning MFG (ชีต Spec sheet_pcp) ไม่ใช่ตัวหลักของกลุ่มพาร์ทนี้แล้ว —
// ถ้าอัปโหลดไว้ ระบบจะเอา Product Spec ในไฟล์มาสอบทานกับ Kanban เพิ่มอีกชั้น
// ถ้าไม่มีไฟล์นี้ก็ข้ามการสอบทานนั้นไป
//
// ไม่ตรงขั้นไหนก็ตาม → ระบบแจ้ง "ข้อมูลไม่ถูกต้อง" พร้อมสาเหตุ และให้ติดต่อ WH
// ---------------------------------------------------------------------------

const (
	FlowVersionV2 = "V2"

	FlowSourceAllocation = "allocation"
	FlowSourceCWCVITS    = "cw_cv_its"

	FlowStatusPending  = "PENDING"
	FlowStatusIssued   = "ISSUED"
	FlowStatusMismatch = "MISMATCH"
	FlowStatusNoPlan   = "NO_PLAN"

	// ข้อความมาตรฐานที่แสดงให้ WH เห็นตอนสแกน
	FlowMsgSaved   = "บันทึกข้อมูลสำเร็จ"
	FlowMsgInvalid = "ข้อมูลไม่ถูกต้อง"
	// อ่าน QR มาไม่ครบ — คนละเรื่องกับข้อมูลไม่ตรง แค่ยิงซ้ำก็จบ
	FlowMsgScanIncomplete = "อ่านบาร์โค้ด Kanban มาได้ไม่ครบ กรุณายิงซ้ำอีกครั้ง"
)

// flowComponents: ลำดับรายการที่ WH ต้องจ่ายต่อ 1 เครื่อง
//
//	WHNeedsSerial  = WH สแกน S/N ด้วย (IT#)
//	MFGNeedsSerial = MFG สแกน S/N ตอนยืนยัน (พาร์ทที่มี S/N บนตัวของ)
var flowComponents = []struct {
	Code           string
	Source         string
	WHNeedsSerial  bool
	MFGNeedsSerial bool
	// AllocPartKeys / AllocSerialKeys = คอลัมน์ในชีต Engine_IT allocation
	AllocPartKeys   []string
	AllocSerialKeys []string
	// SpecPartKeys = คอลัมน์ในชีต CW_CV_ITS
	SpecPartKeys []string
}{
	{
		Code:            ComponentITC,
		Source:          FlowSourceAllocation,
		WHNeedsSerial:   true,
		MFGNeedsSerial:  false,
		AllocPartKeys:   []string{"IT Part no."},
		AllocSerialKeys: []string{"IT Serial no."},
	},
	{
		Code:            ComponentEN,
		Source:          FlowSourceAllocation,
		WHNeedsSerial:   true,
		MFGNeedsSerial:  false,
		AllocPartKeys:   []string{"Engine Part no."},
		AllocSerialKeys: []string{"Engine Serial no."},
		SpecPartKeys:    []string{"Engine Part No"},
	},
	{
		Code:           ComponentCW,
		Source:         FlowSourceCWCVITS,
		MFGNeedsSerial: true,
		SpecPartKeys:   []string{"CW Part No"},
	},
	{
		Code:           ComponentCV,
		Source:         FlowSourceCWCVITS,
		MFGNeedsSerial: true,
		SpecPartKeys:   []string{"CV Part No"},
	},
	{
		Code:           ComponentSM,
		Source:         FlowSourceCWCVITS,
		MFGNeedsSerial: true,
		SpecPartKeys:   []string{"SM Part No"},
	},
	{
		Code:           ComponentMP,
		Source:         FlowSourceCWCVITS,
		MFGNeedsSerial: true,
		SpecPartKeys:   []string{"Motor Propel Part No"},
	},
	{
		Code:           ComponentPH,
		Source:         FlowSourceCWCVITS,
		MFGNeedsSerial: true,
		SpecPartKeys:   []string{"Pump Hydrolics Part No"},
	},
}

func flowComponentIndex(code string) int {
	code = strings.ToUpper(strings.TrimSpace(code))
	for i, s := range flowComponents {
		if s.Code == code {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// อ่านข้อมูลที่ WH อัปโหลดไว้
// ---------------------------------------------------------------------------

// flowAllocIndex: แผน allocation รายเครื่อง key = เลขเครื่องแบบ normalize
func flowAllocIndex() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, row := range loadUploadRows(models.DatasetEngineITAlloc) {
		mc := pickField(row, "Machine S/N", extraColumnPrefix+"Machine S/N")
		key := NormalizeCodeValue(mc)
		if key == "" {
			continue
		}
		if _, ok := out[key]; !ok {
			out[key] = row
		}
	}
	return out
}

// flowSpecIndex: ตาราง CW_CV_ITS key = Product Spec แบบ normalize
func flowSpecIndex() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, row := range loadUploadRows(models.DatasetCWCVITS) {
		spec := pickField(row, "Product Spec", extraColumnPrefix+"Product Spec")
		key := NormalizeCodeValue(spec)
		if key == "" {
			continue
		}
		if _, ok := out[key]; !ok {
			out[key] = row
		}
	}
	return out
}

func flowAllocFor(alloc map[string]map[string]string, machineNo string) map[string]string {
	for _, v := range dedupeCodes(machineNo, ResolveMachineNo(machineNo), CurrentCodeOf(machineNo)) {
		if row, ok := alloc[NormalizeCodeValue(v)]; ok {
			return row
		}
	}
	return nil
}

// ชื่อไฟล์ที่อาจระบุ Product Spec ของเครื่องไว้ (ใช้บอกผู้ใช้ว่าต้องไปแก้ไฟล์ไหน)
const (
	PlanSourceWH        = "Planning WH"
	PlanSourceMFG       = "Planning MFG"
	PlanSourceDailyPlan = "Daily Plan"
)

// flowPlanSpecCodeOf: Product Spec ของเครื่องตามไฟล์ที่อัปโหลดไว้ + บอกว่ามาจากไฟล์ไหน
//
// ขั้นตอนใหม่ไม่ได้ยึดไฟล์พวกนี้เป็นตัวหลักแล้ว — CW / CV / SM / MP / PH อ่าน Product Spec
// จาก Kanban แล้วเทียบกับ master_data ตรง ๆ ค่าที่ได้จากที่นี่ใช้ "สอบทานเพิ่มอีกชั้น"
// เท่านั้น ถ้าไม่มีไฟล์ไหนระบุไว้เลยจะคืน "" ทั้งคู่ = ไม่ต้องเทียบขั้นนี้
func flowPlanSpecCodeOf(allocRow map[string]string, machineNo string) (string, string) {
	if v := pickField(allocRow, "Product Spec", extraColumnPrefix+"Product Spec"); v != "" {
		return v, PlanSourceWH
	}
	if v := lookupSpecSheetCode(machineNo); v != "" {
		return v, PlanSourceMFG
	}
	if plan := planForMachine(machineNo); plan != nil {
		if v := planValue(plan, "Spec Code", "Product Spec 1", "Product Spec", "Product Spec 2"); v != "" {
			return v, PlanSourceDailyPlan
		}
	}
	return "", ""
}

// flowSpecCodeOf: Product Spec ของเครื่อง (fallback = ค่าจาก Kanban ถ้ามี)
func flowSpecCodeOf(allocRow map[string]string, machineNo, fallback string) string {
	if v, _ := flowPlanSpecCodeOf(allocRow, machineNo); v != "" {
		return v
	}
	return strings.TrimSpace(fallback)
}

// lookupSpecSheetRow: แถวของเครื่องนี้ในชีต Planning MFG ที่อัปโหลดไว้
func lookupSpecSheetRow(machineNo string) map[string]string {
	idx := flowSpecSheetIndex()
	if len(idx) == 0 {
		return nil
	}
	for _, v := range dedupeCodes(machineNo, ResolveMachineNo(machineNo), CurrentCodeOf(machineNo)) {
		if row, ok := idx[NormalizeCodeValue(v)]; ok {
			return row
		}
	}
	return nil
}

// lookupSpecSheetCode: Product Spec ของเครื่อง จากชีต Planning MFG
func lookupSpecSheetCode(machineNo string) string {
	return pickField(lookupSpecSheetRow(machineNo),
		"Product Spec", extraColumnPrefix+"Product Spec")
}

// lookupITDevice: ชนิด IT ของเครื่อง เช่น IT(Satellite, iridium) / IT(Mobile4G, normal speed)
func lookupITDevice(machineNo string) string {
	return pickField(lookupSpecSheetRow(machineNo),
		"IT device", extraColumnPrefix+"IT device", "IT Device")
}

// ---------------------------------------------------------------------------
// รายการที่ WH ต้องจ่าย / MFG ต้องยืนยัน
// ---------------------------------------------------------------------------

type FlowItem struct {
	Component string `json:"component"`
	Label     string `json:"label"`
	Source    string `json:"source"`

	WHNeedsSerial  bool `json:"whNeedsSerial"`
	MFGNeedsSerial bool `json:"mfgNeedsSerial"`

	ExpectedPartNo   string `json:"expectedPartNo"`
	ExpectedSerialNo string `json:"expectedSerialNo"`

	// ExpectedWeightTons = น้ำหนักถ่วงในแถว master_data ของ Product Spec นี้
	// มีเฉพาะ Counter Weight — ใช้เทียบกับน้ำหนักที่ติดมากับ P/N บน Kanban
	ExpectedWeightTons string `json:"expectedWeightTons,omitempty"`

	Issued         bool   `json:"issued"`
	IssuedPartNo   string `json:"issuedPartNo"`
	IssuedSerialNo string `json:"issuedSerialNo"`
	IssuedBy       string `json:"issuedBy"`
	IssuedAt       string `json:"issuedAt"`
	IssueID        uint   `json:"issueId"`

	MFGConfirmed bool   `json:"mfgConfirmed"`
	MFGSerialNo  string `json:"mfgSerialNo"`
	MFGPhotoURL  string `json:"mfgPhotoUrl"`
	MFGAt        string `json:"mfgAt"`
	MFGID        uint   `json:"mfgId"`

	Status  string `json:"status"`
	Message string `json:"message"`
}

type FlowMachine struct {
	MachineNo string `json:"machineNo"`
	LotNo     string `json:"lotNo"`
	Customer  string `json:"customer"`
	MainLine  string `json:"mainLine"`
	SpecCode  string `json:"specCode"`
	// ITDevice = ชนิด IT ของเครื่อง (จาก Planning MFG) เช่น IT(Satellite, iridium)
	ITDevice string `json:"itDevice"`

	HasSpecRow bool `json:"hasSpecRow"`

	// PartSpecCode = Product Spec ที่ใช้หา P/N ของ CW / CV / SM / MP / PH จริง ๆ
	// ปกติเท่ากับ SpecCode แต่ถ้า MFG สแกน Kanban มา จะเป็นค่าจาก QR บน Kanban
	PartSpecCode   string `json:"partSpecCode"`
	HasPartSpecRow bool   `json:"hasPartSpecRow"`

	Total    int `json:"total"`
	Issued   int `json:"issued"`
	Assembly int `json:"assembled"`

	Items []FlowItem `json:"items"`
}

// flowIssuedIndex: ของที่ WH จ่ายแล้ว (PartCheck ที่ผูกกับเครื่อง) key = component ของเครื่องนั้น
func flowIssuedIndex(machineNo string) map[string]models.PartCheck {
	out := map[string]models.PartCheck{}
	keys := dedupeCodes(machineNo, ResolveMachineNo(machineNo), CurrentCodeOf(machineNo))
	if len(keys) == 0 {
		return out
	}
	var rows []models.PartCheck
	config.DB.Where("machine_no IN ?", keys).
		Where("match_status = ?", models.MatchStatusMatch).
		Order("checked_datetime asc").Find(&rows)
	for _, r := range rows {
		out[strings.ToUpper(strings.TrimSpace(r.PartType))] = r
	}
	return out
}

// flowAssemblyIndex: รายการที่ MFG ยืนยันแล้วของเครื่องนี้ (ขั้นตอนใหม่เท่านั้น)
func flowAssemblyIndex(machineNo string) map[string]models.MFGAssembly {
	out := map[string]models.MFGAssembly{}
	keys := dedupeCodes(machineNo, ResolveMachineNo(machineNo), CurrentCodeOf(machineNo))
	if len(keys) == 0 {
		return out
	}
	var rows []models.MFGAssembly
	config.DB.Where("machine_no IN ?", keys).
		Where("flow_version = ?", FlowVersionV2).
		Order("id asc").Find(&rows)
	for _, r := range rows {
		out[strings.ToUpper(strings.TrimSpace(r.Component))] = r
	}
	return out
}

// buildFlowMachine: ประกอบข้อมูลของเครื่อง 1 เครื่อง (แผน + ของที่จ่าย + ที่ประกอบแล้ว)
func buildFlowMachine(machineNo string, alloc map[string]map[string]string,
	specs map[string]map[string]string, specCodeHint string) FlowMachine {
	return buildFlowMachineWith(machineNo, alloc, specs, specCodeHint, "")
}

// buildFlowMachineWith: เหมือน buildFlowMachine แต่ระบุ Spec code จาก Kanban ได้
//
//	kanbanSpecCode != "" → ใช้ค่านี้หา P/N ของ CW / CV / SM / MP / PH (ชีต CW_CV_ITS)
//	แทน Product Spec ที่ resolve จาก MC# — IT Controller / Engine ไม่เปลี่ยน
//	เพราะ P/N + S/N ของสองตัวนั้นระบุรายเครื่องอยู่ในไฟล์ Planning WH อยู่แล้ว
func buildFlowMachineWith(machineNo string, alloc map[string]map[string]string,
	specs map[string]map[string]string, specCodeHint, kanbanSpecCode string) FlowMachine {

	machineNo = strings.ToUpper(strings.TrimSpace(machineNo))
	allocRow := flowAllocFor(alloc, machineNo)
	specCode := flowSpecCodeOf(allocRow, machineNo, specCodeHint)

	specRow := specs[NormalizeCodeValue(specCode)]

	// Spec code ที่ใช้กับ CW / CV / SM / MP / PH
	partSpecCode := specCode
	partSpecRow := specRow
	if strings.TrimSpace(kanbanSpecCode) != "" {
		partSpecCode = strings.TrimSpace(kanbanSpecCode)
		partSpecRow = specs[NormalizeCodeValue(partSpecCode)]
	}

	out := FlowMachine{
		MachineNo:      machineNo,
		LotNo:          pickField(allocRow, "Lot no."),
		Customer:       pickField(allocRow, "Customer name"),
		MainLine:       pickField(allocRow, "Main line"),
		SpecCode:       specCode,
		ITDevice:       lookupITDevice(machineNo),
		HasSpecRow:     specRow != nil,
		PartSpecCode:   partSpecCode,
		HasPartSpecRow: partSpecRow != nil,
	}

	issued := flowIssuedIndex(machineNo)
	assembled := flowAssemblyIndex(machineNo)

	for _, spec := range flowComponents {
		item := FlowItem{
			Component:      spec.Code,
			Label:          ComponentLabel(spec.Code),
			Source:         spec.Source,
			WHNeedsSerial:  spec.WHNeedsSerial,
			MFGNeedsSerial: spec.MFGNeedsSerial,
		}

		if spec.Source == FlowSourceAllocation {
			item.ExpectedPartNo = pickField(allocRow, spec.AllocPartKeys...)
			item.ExpectedSerialNo = pickField(allocRow, spec.AllocSerialKeys...)
		}
		// CW / CV / SM / MP / PH ใช้แถวตาม Spec code ของ Kanban (ถ้ามี)
		// Engine ยังใช้แถวตาม Product Spec ของแผนเหมือนเดิม
		itemSpecRow, itemSpecCode := specRow, specCode
		if spec.Source == FlowSourceCWCVITS {
			itemSpecRow, itemSpecCode = partSpecRow, partSpecCode
		}
		if keys := masterDataPartKeysOf(spec.Code); item.ExpectedPartNo == "" && len(keys) > 0 {
			item.ExpectedPartNo = pickField(itemSpecRow, keys...)
		}
		// Counter Weight เทียบน้ำหนักถ่วงด้วย ไม่ใช่แค่รหัส P/N
		if spec.Code == ComponentCW {
			item.ExpectedWeightTons = masterDataWeightTonsOf(itemSpecRow)
		}

		if chk, ok := issued[spec.Code]; ok {
			item.Issued = true
			item.IssueID = chk.ID
			item.IssuedPartNo = chk.PN
			item.IssuedSerialNo = chk.SN
			item.IssuedBy = chk.CheckedBy
			if !chk.CheckedDatetime.IsZero() {
				item.IssuedAt = chk.CheckedDatetime.Format(time.RFC3339)
			}
		}

		if asm, ok := assembled[spec.Code]; ok {
			item.MFGConfirmed = strings.EqualFold(asm.Status, models.MFGStatusMatched)
			item.MFGSerialNo = asm.SerialNo
			item.MFGPhotoURL = asm.PhotoURL
			item.MFGID = asm.ID
			if asm.CheckDate != nil {
				item.MFGAt = asm.CheckDate.Format(time.RFC3339)
			}
		}

		switch {
		case item.ExpectedPartNo == "" && item.ExpectedSerialNo == "":
			item.Status = FlowStatusNoPlan
			if spec.Source == FlowSourceAllocation {
				item.Message = "ยังไม่มีแผนจ่าย " + item.Label + " ของเครื่องนี้ในไฟล์ Planning WH"
			} else if strings.TrimSpace(itemSpecCode) == "" {
				item.Message = "ยังไม่รู้ Product Spec ของเครื่องนี้ — " + item.Label +
					" ต้องอ่าน Product Spec จาก Kanban แล้วเทียบกับ master_data"
			} else {
				item.Message = "ยังไม่มี P/N ของ " + item.Label + " ใน master_data สำหรับ Product Spec " + orDash(itemSpecCode)
			}
		case item.Issued:
			item.Status = FlowStatusIssued
			item.Message = "WH จ่ายแล้ว"
		default:
			item.Status = FlowStatusPending
			item.Message = "รอ WH จ่ายของ"
		}

		if item.Status != FlowStatusNoPlan {
			out.Total++
		}
		if item.Issued {
			out.Issued++
		}
		if item.MFGConfirmed {
			out.Assembly++
		}

		out.Items = append(out.Items, item)
	}

	return out
}

func orDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(ไม่ระบุ)"
	}
	return v
}

// ---------------------------------------------------------------------------
// API — ฝั่ง WH
// ---------------------------------------------------------------------------

// GetFlowMachines: GET /wh-flow/machines — รายชื่อ MC# ให้ WH เลือก
func GetFlowMachines(c *gin.Context) {
	alloc := flowAllocIndex()
	specs := flowSpecIndex()

	kw := strings.ToUpper(strings.TrimSpace(c.Query("keyword")))

	machines := make([]string, 0, len(alloc))
	for _, row := range alloc {
		if mc := pickField(row, "Machine S/N"); mc != "" {
			machines = append(machines, strings.ToUpper(mc))
		}
	}
	sort.Strings(machines)

	out := make([]FlowMachine, 0, len(machines))
	for _, mc := range machines {
		m := buildFlowMachine(mc, alloc, specs, "")
		if kw != "" &&
			!strings.Contains(strings.ToUpper(m.MachineNo), kw) &&
			!strings.Contains(strings.ToUpper(m.LotNo), kw) &&
			!strings.Contains(strings.ToUpper(m.Customer), kw) &&
			!strings.Contains(strings.ToUpper(m.SpecCode), kw) {
			continue
		}
		// รายการเครื่องไม่ต้องส่งรายละเอียดทุกชิ้น
		m.Items = nil
		out = append(out, m)
	}

	c.JSON(200, gin.H{"rows": out, "total": len(out)})
}

// GetFlowMachine: GET /wh-flow/machines/:machineNo — แผนของเครื่อง 1 เครื่อง
func GetFlowMachine(c *gin.Context) {
	machineNo := strings.TrimSpace(c.Param("machineNo"))
	if machineNo == "" {
		c.JSON(400, gin.H{"message": "กรุณาระบุ MC#"})
		return
	}
	alloc := flowAllocIndex()
	if flowAllocFor(alloc, machineNo) == nil {
		c.JSON(404, gin.H{
			"message": "ไม่พบเครื่อง " + strings.ToUpper(machineNo) + " ในไฟล์ Planning WH",
		})
		return
	}
	c.JSON(200, buildFlowMachine(machineNo, alloc, flowSpecIndex(), ""))
}

// FlowIssueRow: 1 แถวในตารางประวัติการจ่ายของ
type FlowIssueRow struct {
	ID        uint   `json:"id"`
	MachineNo string `json:"machineNo"`
	Component string `json:"component"`
	Label     string `json:"label"`
	PartNo    string `json:"partNo"`
	SerialNo  string `json:"serialNo"`
	IssuedBy  string `json:"issuedBy"`
	IssuedAt  string `json:"issuedAt"`
	LotNo     string `json:"lotNo"`
	Customer  string `json:"customer"`
	SpecCode  string `json:"specCode"`

	MFGConfirmed bool   `json:"mfgConfirmed"`
	MFGSerialNo  string `json:"mfgSerialNo"`
}

// GetFlowIssues: GET /wh-flow/issues — ของที่ WH จ่ายไปแล้วทั้งหมด (ใหม่สุดก่อน)
// ตารางนี้จะว่างตอนเริ่มต้น แล้วมีข้อมูลเมื่อ WH สแกนจ่ายของ
func GetFlowIssues(c *gin.Context) {
	known := map[string]bool{}
	for _, s := range flowComponents {
		known[s.Code] = true
	}

	var checks []models.PartCheck
	config.DB.Where("match_status = ?", models.MatchStatusMatch).
		Where("machine_no <> ?", "").
		Order("checked_datetime desc").Limit(2000).Find(&checks)

	// รายการที่ MFG ยืนยันแล้ว (ขั้นตอนใหม่) — key = MC#|component
	var assembled []models.MFGAssembly
	config.DB.Where("flow_version = ?", FlowVersionV2).Find(&assembled)
	confirmed := map[string]models.MFGAssembly{}
	for _, a := range assembled {
		key := NormalizeCodeValue(a.MachineNo) + "|" + strings.ToUpper(strings.TrimSpace(a.Component))
		confirmed[key] = a
	}

	alloc := flowAllocIndex()

	out := make([]FlowIssueRow, 0, len(checks))
	for _, r := range checks {
		comp := strings.ToUpper(strings.TrimSpace(r.PartType))
		if !known[comp] {
			continue
		}
		allocRow := flowAllocFor(alloc, r.MachineNo)
		if allocRow == nil {
			continue // ไม่ใช่รายการของขั้นตอนใหม่
		}
		row := FlowIssueRow{
			ID:        r.ID,
			MachineNo: r.MachineNo,
			Component: comp,
			Label:     ComponentLabel(comp),
			PartNo:    r.PN,
			SerialNo:  r.SN,
			IssuedBy:  r.CheckedBy,
			LotNo:     pickField(allocRow, "Lot no."),
			Customer:  pickField(allocRow, "Customer name"),
			SpecCode:  flowSpecCodeOf(allocRow, r.MachineNo, ""),
		}
		if !r.CheckedDatetime.IsZero() {
			row.IssuedAt = r.CheckedDatetime.Format(time.RFC3339)
		}
		if a, ok := confirmed[NormalizeCodeValue(r.MachineNo)+"|"+comp]; ok {
			row.MFGConfirmed = strings.EqualFold(a.Status, models.MFGStatusMatched)
			row.MFGSerialNo = a.SerialNo
		}
		out = append(out, row)
	}

	c.JSON(200, gin.H{"rows": out, "total": len(out)})
}

type FlowIssueRequest struct {
	MachineNo string `json:"machineNo" binding:"required"`
	// Component ว่างได้ — ระบบจะหาเองว่าเลขที่สแกนตรงกับรายการไหนของเครื่องนี้
	Component string `json:"component"`
	PartNo    string `json:"partNo"`
	SerialNo  string `json:"serialNo"`
}

// IssueFlowPart: POST /wh-flow/issue
// WH เลือก MC# แล้วสแกนของที่จะจ่าย — ระบบเทียบกับแผนก่อนบันทึก
func IssueFlowPart(c *gin.Context) {
	var req FlowIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	component := strings.ToUpper(strings.TrimSpace(req.Component))

	machineNo := strings.ToUpper(strings.TrimSpace(req.MachineNo))
	alloc := flowAllocIndex()
	if flowAllocFor(alloc, machineNo) == nil {
		c.JSON(404, gin.H{"message": "ไม่พบเครื่อง " + machineNo + " ในไฟล์ Planning WH"})
		return
	}

	machine := buildFlowMachine(machineNo, alloc, flowSpecIndex(), "")

	scannedPN := strings.TrimSpace(req.PartNo)
	scannedSN := strings.TrimSpace(req.SerialNo)

	if scannedPN == "" && scannedSN == "" {
		c.JSON(400, gin.H{"message": "ยังไม่มีค่าที่สแกน"})
		return
	}

	// ไม่ได้ระบุชนิดมา (WH เลือกแค่ MC# แล้วสแกนตามแผน)
	// → หาเองว่าเลขที่สแกนตรงกับรายการไหนในแผนของเครื่องนี้
	if component == "" {
		component = detectFlowComponent(machine, scannedPN, scannedSN)
		if component == "" {
			c.JSON(200, gin.H{
				"matched": false,
				"status":  FlowStatusMismatch,
				"message": FlowMsgInvalid,
				"detail":  flowExpectedSummary(machine),
			})
			return
		}
	}

	if flowComponentIndex(component) < 0 {
		c.JSON(400, gin.H{"message": "ชนิดของที่จ่ายไม่ถูกต้อง"})
		return
	}
	label := ComponentLabel(component)

	var item FlowItem
	for _, it := range machine.Items {
		if it.Component == component {
			item = it
			break
		}
	}

	if item.Status == FlowStatusNoPlan {
		c.JSON(200, gin.H{
			"matched": false,
			"status":  FlowStatusNoPlan,
			"message": FlowMsgInvalid,
			"detail":  item.Message,
		})
		return
	}

	// ต้องสแกนอะไรบ้าง ดูจากแผนที่ WH อัปโหลด ไม่ได้ตายตัวตามชนิดของ
	if item.ExpectedPartNo != "" && scannedPN == "" {
		c.JSON(400, gin.H{"message": label + " ต้องสแกน P/N ด้วย"})
		return
	}
	if item.ExpectedSerialNo != "" && scannedSN == "" {
		c.JSON(400, gin.H{"message": label + " ต้องสแกน S/N ด้วย"})
		return
	}

	// เทียบกับแผน — เทียบเฉพาะค่าที่แผนกำหนดไว้และมีการสแกนมา
	// ที่มาของค่าที่ใช้เทียบ — บอกให้ WH รู้ว่าต้องไปแก้ที่ไหนถ้าไม่ตรง
	expectedFrom := "ไฟล์ Planning WH"
	if item.Source == FlowSourceCWCVITS {
		expectedFrom = "master_data (Product Spec " + orDash(machine.SpecCode) + ")"
	}

	var problems []string
	if item.ExpectedPartNo != "" && scannedPN != "" && !SameCode(scannedPN, item.ExpectedPartNo) {
		problems = append(problems, expectedFrom+" กำหนด P/N "+item.ExpectedPartNo+" แต่สแกนได้ "+scannedPN)
	}
	if item.ExpectedSerialNo != "" && scannedSN != "" && !SameCode(scannedSN, item.ExpectedSerialNo) {
		problems = append(problems, expectedFrom+" กำหนด S/N# "+item.ExpectedSerialNo+" แต่สแกนได้ "+scannedSN)
	}

	if len(problems) > 0 {
		c.JSON(200, gin.H{
			"matched": false,
			"status":  FlowStatusMismatch,
			"message": FlowMsgInvalid,
			"detail":  strings.Join(problems, " · "),
		})
		return
	}

	// ของชิ้นนี้ถูกจ่ายให้เครื่องอื่นไปแล้วหรือยัง
	if owner := flowSerialOwner(component, scannedPN, scannedSN, machineNo); owner != "" {
		c.JSON(200, gin.H{
			"matched": false,
			"status":  FlowStatusMismatch,
			"message": FlowMsgInvalid,
			"detail":  label + " นี้ถูกจ่ายให้เครื่อง " + owner + " ไปแล้ว",
		})
		return
	}

	userID, name := lookupUserName(c)
	now := time.Now()

	check := models.PartCheck{
		PartType:        component,
		PN:              scannedPN,
		SN:              scannedSN,
		MachineNo:       machineNo,
		MatchStatus:     models.MatchStatusMatch,
		MatchMessage:    "ตรงตาม " + expectedFrom + " ของเครื่อง " + machineNo,
		CheckedBy:       name,
		CheckedDatetime: now,
		UserID:          userID,
	}

	// จ่ายซ้ำชิ้นเดิมของเครื่องเดิม → อัปเดตรายการเดิมแทนการสร้างใหม่
	if item.Issued && item.IssueID != 0 {
		check.ID = item.IssueID
		if err := config.DB.Model(&models.PartCheck{}).Where("id = ?", item.IssueID).
			Updates(map[string]interface{}{
				"pn":               check.PN,
				"sn":               check.SN,
				"match_status":     check.MatchStatus,
				"match_message":    check.MatchMessage,
				"checked_by":       check.CheckedBy,
				"checked_datetime": check.CheckedDatetime,
				"user_id":          check.UserID,
			}).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
	} else if err := config.DB.Create(&check).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	CreateAuditLog("WH_FLOW_ISSUE", check.ID, "issue", machineNo+"/"+component, userID, name)

	c.JSON(201, gin.H{
		"matched": true,
		"status":  FlowStatusIssued,
		"message": FlowMsgSaved,
		"machine": buildFlowMachine(machineNo, alloc, flowSpecIndex(), ""),
	})
}

// detectFlowComponent: เดาว่าเลขที่สแกนเป็นของรายการไหนในแผนของเครื่องนี้
// เทียบ P/N ก่อน ถ้าไม่เจอค่อยเทียบ S/N ("" = ไม่ตรงกับรายการใดเลย)
func detectFlowComponent(machine FlowMachine, partNo, serialNo string) string {
	for _, it := range machine.Items {
		if it.ExpectedPartNo != "" && SameCode(it.ExpectedPartNo, partNo) {
			return it.Component
		}
	}
	if strings.TrimSpace(serialNo) != "" {
		for _, it := range machine.Items {
			if it.ExpectedSerialNo != "" && SameCode(it.ExpectedSerialNo, serialNo) {
				return it.Component
			}
		}
	}
	return ""
}

// flowExpectedSummary: รายการที่ระบบคาดว่าจะได้รับของเครื่องนี้ (ใช้บอกตอนสแกนไม่ตรง)
//
//	IT Controller / Engine        → มาจากไฟล์ Planning WH
//	CW / CV / SM / MP / Pump HYD  → มาจาก master_data ตาม Product Spec ของเครื่อง
func flowExpectedSummary(machine FlowMachine) string {
	parts := make([]string, 0, len(machine.Items))
	for _, it := range machine.Items {
		if it.ExpectedPartNo == "" {
			continue
		}
		s := it.Label + ": " + it.ExpectedPartNo
		if it.ExpectedSerialNo != "" {
			s += " / S/N# " + it.ExpectedSerialNo
		}
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		if strings.TrimSpace(machine.SpecCode) == "" {
			return "เครื่องนี้ยังไม่รู้ Product Spec — กรุณาอัปโหลดไฟล์ Planning WH ที่มีคอลัมน์ Product Spec"
		}
		return "ไม่พบ P/N ของ Product Spec " + orDash(machine.SpecCode) +
			" ใน master_data — กรุณาอัปโหลด master_data ล่าสุด"
	}
	return "Part# นี้ไม่ตรงกับของที่เครื่องนี้ต้องใช้ (Product Spec " +
		orDash(machine.SpecCode) + ") — " + strings.Join(parts, " · ")
}

// flowSerialOwner: เลขของชิ้นนี้ถูกจ่ายให้เครื่องอื่นไปแล้วหรือไม่ ("" = ยังไม่ถูกจ่าย)
func flowSerialOwner(component, pn, sn, machineNo string) string {
	needle := strings.TrimSpace(sn)
	if needle == "" {
		needle = strings.TrimSpace(pn)
	}
	if needle == "" {
		return ""
	}
	variants := CodeVariants(needle)
	if len(variants) == 0 {
		variants = []string{needle}
	}

	var rows []models.PartCheck
	config.DB.Where("part_type = ?", component).
		Where("match_status = ?", models.MatchStatusMatch).
		Where("sn IN ? OR pn IN ?", variants, variants).
		Limit(50).Find(&rows)

	for _, r := range rows {
		owner := strings.TrimSpace(r.MachineNo)
		if owner == "" || SameCode(owner, machineNo) {
			continue
		}
		return owner
	}
	return ""
}

// CancelFlowIssue: DELETE /wh-flow/issue/:id — ยกเลิกของที่จ่ายผิด (ถ้า MFG ยังไม่ยืนยัน)
func CancelFlowIssue(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}
	var row models.PartCheck
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	asm := flowAssemblyIndex(row.MachineNo)
	if a, ok := asm[strings.ToUpper(row.PartType)]; ok && strings.EqualFold(a.Status, models.MFGStatusMatched) {
		c.JSON(400, gin.H{"message": "ยกเลิกไม่ได้ — MFG ยืนยันการประกอบไปแล้ว"})
		return
	}

	if err := config.DB.Delete(&models.PartCheck{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	CreateAuditLog("WH_FLOW_ISSUE", row.ID, "cancel", row.MachineNo+"/"+row.PartType, userID, name)

	c.JSON(200, gin.H{"deleted": true})
}

// ---------------------------------------------------------------------------
// API — ฝั่ง MFG
// ---------------------------------------------------------------------------

type FlowKanbanRequest struct {
	QRCode    string `json:"qrCode"`
	MachineNo string `json:"machineNo"`
}

// ScanFlowKanban: POST /mfg-flow/kanban
// MFG สแกน QR บน Kanban → ระบบคืน MC#, Product Spec และของที่ WH จ่ายมา
func ScanFlowKanban(c *gin.Context) {
	var req FlowKanbanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}

	raw := strings.TrimSpace(req.QRCode)
	machineNo := strings.ToUpper(strings.TrimSpace(req.MachineNo))
	specHint := ""
	itDeviceHint := ""
	customerHint := ""

	var q SpecQR
	if raw != "" {
		var ok bool
		if q, ok = ParseSpecQR(raw); ok {
			machineNo = q.MachineNo
			specHint = q.SpecCode
			itDeviceHint = q.ITDevice
			customerHint = q.Customer
		} else if machineNo == "" {
			// มี "," แต่ยังไม่ครบช่อง = เครื่องสแกนส่งมาไม่หมด
			// ห้ามตัดเศษข้อความไปใช้เป็น MC# เพราะจะกลายเป็น "ข้อมูลไม่ถูกต้อง" ทั้งที่แค่ยิงไม่ครบ
			if strings.Contains(raw, ",") {
				c.JSON(200, gin.H{
					"found":          false,
					"message":        FlowMsgScanIncomplete,
					"detail":         "QR บน Kanban ต้องมีครบทุกช่อง (MC#, Product Spec, ลูกค้า, P/N ...) แต่อ่านมาได้ไม่ครบ",
					"scanIncomplete": true,
				})
				return
			}
			// QR ที่มีแต่เลขเครื่อง
			machineNo = strings.ToUpper(strings.Fields(raw)[0])
		}
	}

	if machineNo == "" {
		c.JSON(200, gin.H{
			"found":   false,
			"message": FlowMsgInvalid,
			"detail":  "อ่าน MC# จาก QR ไม่ได้ — กรุณาสแกน QR บน Kanban อีกครั้ง",
		})
		return
	}

	specs := flowSpecIndex()
	alloc := flowAllocIndex()
	allocRow := flowAllocFor(alloc, machineNo)

	// เครื่องนี้ไม่อยู่ใน Planning WH — IT / Engine จะไม่มีแผนจ่าย แต่ CW / CV / SM / MP / PH
	// ไม่ได้พึ่งไฟล์นั้นแล้ว จึงไปต่อได้ถ้า Product Spec บน Kanban มีอยู่ใน master_data
	knownSpec := !specCodeValueEmpty(specHint) && specs[NormalizeCodeValue(specHint)] != nil
	if allocRow == nil && !knownSpec {
		c.JSON(200, gin.H{
			"found":     false,
			"machineNo": machineNo,
			"message":   FlowMsgInvalid,
			"detail": "ไม่พบเครื่อง " + machineNo + " ในไฟล์ Planning WH และอ่าน Product Spec " +
				"ที่มีอยู่ใน master_data จาก Kanban ไม่ได้",
		})
		return
	}

	// ลูกค้า / ประเทศบน Kanban ต้องตรงกับไฟล์ Planning WH ของเครื่องนี้ (ข้ามถ้าไม่มีไฟล์)
	customer := checkKanbanCustomerRow(machineNo, customerHint, allocRow)
	if customer.Blocked() {
		c.JSON(200, gin.H{
			"found":            false,
			"machineNo":        machineNo,
			"message":          customer.Message,
			"detail":           customer.Detail,
			"customerMismatch": true,
			"customerCheck":    customer,
		})
		return
	}

	// ขั้นที่ 1 ของโฟลว์ใหม่ — เอา MC# + Product Spec บน Kanban ไปเทียบ master_data
	// ไฟล์ Planning (WH / MFG) ที่อัปโหลดไว้จะถูกใช้สอบทานเพิ่มอีกชั้นเท่านั้น
	planSpec, planSource := flowPlanSpecCodeOf(allocRow, machineNo)
	specCheck := checkKanbanSpecCodeFrom(machineNo, specHint, planSpec, planSource, specs)
	if specCheck.Blocked() {
		c.JSON(200, gin.H{
			"found":        false,
			"machineNo":    machineNo,
			"message":      specCheck.Message,
			"detail":       specCheck.Detail,
			"specMismatch": true,
			"specCheck":    specCheck,
		})
		return
	}

	// ...และเทียบ P/N ที่ติดมากับ Kanban กับ P/N ใน master_data ของ Product Spec นั้น
	partSpecCode := kanbanSpecCodeOverride(specCheck)
	if partSpecCode == "" {
		partSpecCode = specCheck.Used
	}
	partCheck := checkKanbanPartNos(q, partSpecCode, specs[NormalizeCodeValue(partSpecCode)])
	if partCheck.Blocked() {
		c.JSON(200, gin.H{
			"found":        false,
			"machineNo":    machineNo,
			"message":      partCheck.Message,
			"detail":       partCheck.Detail,
			"partMismatch": true,
			"partCheck":    partCheck,
			"specCheck":    specCheck,
		})
		return
	}

	machine := buildFlowMachineWith(machineNo, alloc, specs, specHint,
		kanbanSpecCodeOverride(specCheck))
	if machine.ITDevice == "" {
		machine.ITDevice = itDeviceHint
	}
	if machine.Customer == "" {
		machine.Customer = customerHint
	}

	pending := 0
	for _, it := range machine.Items {
		if it.Status == FlowStatusPending {
			pending++
		}
	}

	c.JSON(200, gin.H{
		"found":      true,
		"machine":    machine,
		"qrCode":     raw,
		"pending":    pending,
		"ready":      pending == 0 && machine.Issued > 0,
		"waitingWH":  pending > 0,
		"messageWH":  "ยังมีของที่ WH ยังไม่จ่าย " + strconv.Itoa(pending) + " รายการ — กรุณาติดต่อ WH",
		"machineNo":  machineNo,
		"specCode":   machine.SpecCode,
		"hasSpecRow": machine.HasSpecRow,

		"partSpecCode":   machine.PartSpecCode,
		"hasPartSpecRow": machine.HasPartSpecRow,

		"customerCheck":    customer,
		"customerMismatch": false,

		"specCheck":    specCheck,
		"specMismatch": false,
		"specWarning":  specCheck.Warned(),

		"partCheck":    partCheck,
		"partMismatch": false,
	})
}

type FlowConfirmRequest struct {
	MachineNo string `json:"machineNo" binding:"required"`
	Component string `json:"component" binding:"required"`
	QRCode    string `json:"qrCode"`
	// SerialNo = S/N# ที่ MFG สแกนจากตัวของ (เฉพาะพาร์ทที่ต้องสแกน S/N)
	SerialNo string `json:"serialNo"`
	// Confirmed = MFG กดยืนยันว่าของตรงกับที่ WH จ่ายมา
	Confirmed bool `json:"confirmed"`
}

// ConfirmFlowAssembly: POST /mfg-flow/confirm
// MFG ยืนยันว่าของที่ WH จ่ายมาถูกต้อง → บันทึกเป็นรายการประกอบ (รอแนบรูปต่อ)
func ConfirmFlowAssembly(c *gin.Context) {
	var req FlowConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	component := strings.ToUpper(strings.TrimSpace(req.Component))
	idx := flowComponentIndex(component)
	if idx < 0 {
		c.JSON(400, gin.H{"message": "ชนิดของไม่ถูกต้อง"})
		return
	}
	spec := flowComponents[idx]
	label := ComponentLabel(component)

	machineNo := strings.ToUpper(strings.TrimSpace(req.MachineNo))
	alloc := flowAllocIndex()
	allocRow := flowAllocFor(alloc, machineNo)
	// IT Controller / Engine ยังต้องมีแผนจ่ายใน Planning WH
	// CW / CV / SM / MP / PH ไม่ได้พึ่งไฟล์นั้นแล้ว — ใช้ Product Spec บน Kanban + master_data
	if allocRow == nil && spec.Source == FlowSourceAllocation {
		c.JSON(200, gin.H{
			"matched": false,
			"status":  FlowStatusNoPlan,
			"message": FlowMsgInvalid,
			"detail":  "ไม่พบเครื่อง " + machineNo + " ในไฟล์ Planning WH",
		})
		return
	}

	// กันกรณียิง API ตรง ๆ โดยข้ามขั้นสแกน Kanban
	// CW / CV / SM / MP / PH ต้องมี QR จาก Kanban เสมอ — เอา Product Spec บน Kanban ไปเทียบ Master Data
	specs := flowSpecIndex()
	kanbanSpecCode := ""
	q, qrOK := ParseSpecQR(strings.TrimSpace(req.QRCode))
	if qrOK {
		customer := checkKanbanCustomerRow(machineNo, q.Customer, allocRow)
		if customer.Blocked() {
			c.JSON(200, gin.H{
				"matched":          false,
				"status":           FlowStatusMismatch,
				"message":          customer.Message,
				"detail":           customer.Detail,
				"customerMismatch": true,
				"customerCheck":    customer,
			})
			return
		}
	}
	if spec.Source == FlowSourceCWCVITS {
		if !qrOK {
			c.JSON(200, gin.H{
				"matched":      false,
				"status":       FlowStatusMismatch,
				"message":      FlowMsgInvalid,
				"detail":       "ต้องสแกน MC# จาก QR บน Kanban ก่อนยืนยัน " + label,
				"specMismatch": true,
			})
			return
		}
		planSpec, planSource := flowPlanSpecCodeOf(allocRow, machineNo)
		specCheck := checkKanbanSpecCodeFrom(machineNo, q.SpecCode, planSpec, planSource, specs)
		if specCheck.BlockedForPart() {
			msg, detail := partBlockMessage(specCheck, label)
			c.JSON(200, gin.H{
				"matched":      false,
				"status":       FlowStatusMismatch,
				"message":      msg,
				"detail":       detail,
				"specMismatch": true,
				"specCheck":    specCheck,
			})
			return
		}
		kanbanSpecCode = kanbanSpecCodeOverride(specCheck)

		// P/N ที่ติดมากับ Kanban ต้องตรงกับ master_data ของ Product Spec นั้น
		partCheck := checkKanbanPartNos(q, kanbanSpecCode, specs[NormalizeCodeValue(kanbanSpecCode)])
		if partCheck.Blocked() {
			c.JSON(200, gin.H{
				"matched":      false,
				"status":       FlowStatusMismatch,
				"message":      partCheck.Message,
				"detail":       partCheck.Detail,
				"partMismatch": true,
				"partCheck":    partCheck,
			})
			return
		}
	}

	machine := buildFlowMachineWith(machineNo, alloc, specs, "", kanbanSpecCode)
	if machine.Customer == "" && qrOK {
		machine.Customer = strings.TrimSpace(q.Customer)
	}
	var item FlowItem
	for _, it := range machine.Items {
		if it.Component == component {
			item = it
			break
		}
	}

	if !item.Issued {
		c.JSON(200, gin.H{
			"matched":  false,
			"status":   FlowStatusPending,
			"message":  FlowMsgInvalid,
			"detail":   "WH ยังไม่จ่าย " + label + " ของเครื่อง " + machineNo,
			"issuedBy": "",
		})
		return
	}

	// CW / CV / SM / MP / PH: P/N ที่ WH จ่าย ต้องตรงกับ P/N ของ Product Spec (จาก Kanban) ใน Master Data
	if spec.Source == FlowSourceCWCVITS && item.ExpectedPartNo != "" &&
		!SameCode(item.IssuedPartNo, item.ExpectedPartNo) {
		c.JSON(200, gin.H{
			"matched":      false,
			"status":       FlowStatusMismatch,
			"message":      FlowMsgInvalid,
			"detail":       label + " P/N " + orDash(item.IssuedPartNo) + " ที่ WH จ่าย ไม่ตรงกับ master_data ของ Product Spec " + orDash(machine.PartSpecCode) + " (P/N " + item.ExpectedPartNo + ")",
			"specMismatch": true,
			"issuedBy":     item.IssuedBy,
		})
		return
	}

	if !req.Confirmed {
		c.JSON(200, gin.H{
			"matched":  false,
			"status":   FlowStatusMismatch,
			"message":  FlowMsgInvalid,
			"detail":   label + " ไม่ตรงกับที่ WH จ่ายมา (" + orDash(item.IssuedPartNo) + ")",
			"issuedBy": item.IssuedBy,
		})
		return
	}

	serialNo := strings.TrimSpace(req.SerialNo)
	if spec.MFGNeedsSerial && serialNo == "" {
		c.JSON(400, gin.H{"message": label + " ต้องสแกน S/N# ก่อนบันทึก"})
		return
	}
	if !spec.MFGNeedsSerial && serialNo == "" {
		serialNo = item.IssuedSerialNo
	}

	// S/N ที่ MFG สแกน ต้องไม่ใช่ของเครื่องอื่น
	if spec.MFGNeedsSerial {
		if owner := flowAssemblySerialOwner(component, serialNo, machineNo); owner != "" {
			c.JSON(200, gin.H{
				"matched":  false,
				"status":   FlowStatusMismatch,
				"message":  FlowMsgInvalid,
				"detail":   label + " S/N# " + serialNo + " ถูกบันทึกกับเครื่อง " + owner + " ไปแล้ว",
				"issuedBy": item.IssuedBy,
			})
			return
		}
	}

	userID, name := lookupUserName(c)
	now := time.Now()

	row := models.MFGAssembly{
		FlowVersion:     FlowVersionV2,
		MachineNo:       machineNo,
		Component:       component,
		ComponentLabel:  label,
		PartNo:          item.IssuedPartNo,
		SerialNo:        serialNo,
		Country:         machine.Customer,
		QRCode:          strings.TrimSpace(req.QRCode),
		Status:          models.MFGStatusMatched,
		DateAssembly:    &now,
		CheckDate:       &now,
		CreatedBy:       name,
		CreatedDatetime: now,
		UpdatedDatetime: now,
		UserID:          userID,

		WHMatched:         true,
		WHCheckedBy:       item.IssuedBy,
		WHCheckedDatetime: &now,
	}
	if component == ComponentITC {
		row.ITControllerNo = item.IssuedPartNo
	}

	// ยืนยันซ้ำของชิ้นเดิม → อัปเดตรายการเดิม
	if item.MFGID != 0 {
		row.ID = item.MFGID
		if err := config.DB.Model(&models.MFGAssembly{}).Where("id = ?", item.MFGID).
			Updates(map[string]interface{}{
				"part_no":          row.PartNo,
				"serial_no":        row.SerialNo,
				"status":           row.Status,
				"qr_code":          row.QRCode,
				"check_date":       row.CheckDate,
				"created_by":       row.CreatedBy,
				"updated_datetime": row.UpdatedDatetime,
				"flow_version":     FlowVersionV2,
			}).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		config.DB.First(&row, item.MFGID)
	} else if err := config.DB.Create(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	CreateAuditLog("MFG_FLOW_CONFIRM", row.ID, "confirm", machineNo+"/"+component, userID, name)

	c.JSON(201, gin.H{
		"matched": true,
		"status":  models.MFGStatusMatched,
		"message": "ยืนยัน " + label + " ของเครื่อง " + machineNo + " แล้ว",
		"row":     row,
		"machine": buildFlowMachineWith(machineNo, alloc, specs, "", kanbanSpecCode),
	})
}

func flowAssemblySerialOwner(component, serial, machineNo string) string {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return ""
	}
	variants := CodeVariants(serial)
	if len(variants) == 0 {
		variants = []string{serial}
	}
	var rows []models.MFGAssembly
	config.DB.Where("component = ?", component).
		Where("flow_version = ?", FlowVersionV2).
		Where("serial_no IN ?", variants).
		Limit(50).Find(&rows)
	for _, r := range rows {
		owner := strings.TrimSpace(r.MachineNo)
		if owner == "" || SameCode(owner, machineNo) {
			continue
		}
		return owner
	}
	return ""
}