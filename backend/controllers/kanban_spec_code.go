package controllers

import "strings"

// ---------------------------------------------------------------------------
// ตรวจ "Spec code" ตอน MFG สแกน Kanban — ใช้กับ CW / CV / SM / MP / PH เท่านั้น
//
// QR บน Kanban ช่องที่ 2 (index 1) = Spec code ของเครื่องนั้น
//
//	YN15438295,YN15-0TD6BG111001,Indonesia,YN02B10321F1,...
//	^ MC#      ^ Spec code       ^ ลูกค้า / ประเทศ
//
// P/N ของ CW / CV / SM / MP / PH ผูกกับ Spec code (ชีต CW_CV_ITS = master_data)
// ไม่ได้ผูกกับเลขเครื่อง เดิมระบบหา Spec code จากไฟล์ Planning MFG (MC# → Product Spec)
// ซึ่งถ้าไฟล์กรอกผิด จะดึง P/N ผิดชุดมาทั้งสายโดยไม่มีอะไรจับได้
//
// ขั้นตอนใหม่: Spec code บน Kanban เป็นตัวหลัก เพราะติดมากับตัวเครื่องจริงหน้างาน
// แล้วเอาไปเทียบกับ master_data ส่วนไฟล์ Planning MFG เป็นแค่การสอบทานเพิ่มอีกชั้น
// ถ้าไม่ได้อัปโหลดไฟล์นั้นไว้ก็ข้ามไป:
//
//	QR ไม่มี Spec code      → ใช้ Product Spec ตามแผน (ถ้ามี) — CW/CV/SM/MP/PH ยืนยันไม่ได้
//	QR ≠ แผน                → บล็อก (Kanban คนละ spec กับแผนของเครื่องนี้ = หยิบใบผิด)
//	QR ไม่มีใน master_data  → เตือน ไม่บล็อก IT/Engine แต่ CW/CV/SM/MP/PH ยืนยันไม่ได้
//	                          เพราะไม่มี P/N ให้เทียบ
//	แผนเว้นว่าง / ไม่มีไฟล์ → ใช้ค่าจาก QR ได้เลย ไม่บล็อก (ไม่ต้องเทียบขั้นนี้)
// ---------------------------------------------------------------------------

const (
	SpecCodeCheckMatch       = "MATCH"
	SpecCodeCheckMismatch    = "MISMATCH"
	SpecCodeCheckNotInMaster = "NOT_IN_MASTER"
	SpecCodeCheckNoQR        = "NO_QR"
	SpecCodeCheckNoPlan      = "NO_PLAN"
)

type SpecCodeCheckResult struct {
	State     string `json:"state"`
	MachineNo string `json:"machineNo"`
	// QR = Spec code บน Kanban, Plan = Product Spec ตามไฟล์ Planning (ถ้ามีอัปโหลดไว้)
	QR   string `json:"qr"`
	Plan string `json:"plan"`
	// PlanSource = ไฟล์ที่ค่า Plan มาจาก (\"\" = ไม่มีไฟล์ไหนระบุไว้ จึงไม่ได้เทียบขั้นนี้)
	PlanSource string `json:"planSource"`
	// Used = ค่าที่ระบบใช้หา P/N ของ CW / CV / SM / MP / PH จริง ๆ
	Used     string `json:"used"`
	InMaster bool   `json:"inMaster"`
	Matched  bool   `json:"matched"`
	Message  string `json:"message"`
	Detail   string `json:"detail"`
}

// Blocked: ห้ามประกอบต่อ ต้องให้ WH ตรวจก่อน (Kanban คนละ spec กับแผน = หยิบใบผิด)
func (r SpecCodeCheckResult) Blocked() bool {
	return r.State == SpecCodeCheckMismatch
}

// Warned: ทำงานต่อได้ แต่ต้องเตือน — Spec code ถูก แต่ยังไม่มีรุ่นนี้ใน Master Data
// (CW / CV / SM / MP / PH ของเครื่องนี้จะหา P/N ไม่เจอ ส่วน IT / Engine ยังจ่ายได้ตามปกติ)
func (r SpecCodeCheckResult) Warned() bool {
	return r.State == SpecCodeCheckNotInMaster
}

// specCodeValueEmpty: ค่าที่ถือว่า "ไม่ได้ระบุ"
func specCodeValueEmpty(s string) bool {
	switch NormalizeCodeValue(s) {
	case "", "NA", "TBD", "TBA", "NONE", "NIL", "UNKNOWN", "XXX":
		return true
	}
	return false
}

// checkKanbanSpecCode: เทียบ Spec code บน Kanban กับแผน และกับ master_data (ชีต CW_CV_ITS)
//
//	qrSpec   = ช่องที่ 2 ของ QR บน Kanban
//	planSpec = Product Spec ของเครื่องนี้ตามไฟล์ที่อัปโหลดไว้ (ว่าง = ไม่ต้องเทียบขั้นนี้)
//	specs    = ตาราง master_data (key = Product Spec แบบ normalize)
func checkKanbanSpecCode(machineNo, qrSpec, planSpec string,
	specs map[string]map[string]string) SpecCodeCheckResult {
	return checkKanbanSpecCodeFrom(machineNo, qrSpec, planSpec, "", specs)
}

// checkKanbanSpecCodeFrom: เหมือน checkKanbanSpecCode แต่ระบุได้ว่าค่า planSpec มาจากไฟล์ไหน
// เพื่อให้ข้อความบอกผู้ใช้ได้ว่าต้องไปแก้ไฟล์ไหน (\"\" = ไม่ระบุ)
func checkKanbanSpecCodeFrom(machineNo, qrSpec, planSpec, planSource string,
	specs map[string]map[string]string) SpecCodeCheckResult {

	res := SpecCodeCheckResult{
		MachineNo:  strings.ToUpper(strings.TrimSpace(machineNo)),
		QR:         strings.TrimSpace(unwrapExcelText(qrSpec)),
		Plan:       strings.TrimSpace(unwrapExcelText(planSpec)),
		PlanSource: strings.TrimSpace(planSource),
	}
	planFrom := "แผน"
	if res.PlanSource != "" {
		planFrom = "ไฟล์ " + res.PlanSource
	}
	mc := res.MachineNo
	if mc == "" {
		mc = "(ไม่ระบุ)"
	}

	inMaster := func(code string) bool {
		if specCodeValueEmpty(code) {
			return false
		}
		return specs[NormalizeCodeValue(code)] != nil
	}

	// QR ไม่มี Spec code → ใช้ค่าตามแผนเหมือนเดิม (CW/CV/SM/MP/PH จะยืนยันไม่ได้)
	if specCodeValueEmpty(res.QR) {
		res.State = SpecCodeCheckNoQR
		res.Used = res.Plan
		res.InMaster = inMaster(res.Plan)
		res.Message = "ไม่พบ Product Spec บน Kanban"
		res.Detail = "QR บน Kanban ไม่มีช่อง Product Spec จึงใช้ค่าตามแผนของเครื่อง " + mc +
			" — CW / CV / SM / MP / PH ต้องอ่าน Product Spec จาก Kanban เท่านั้น"
		return res
	}

	res.Used = res.QR
	res.InMaster = inMaster(res.QR)

	// Kanban คนละ spec กับแผนของเครื่องนี้ (สอบทานเพิ่มเมื่อมีไฟล์แผนอยู่ในระบบ)
	if !specCodeValueEmpty(res.Plan) && !SameCode(res.QR, res.Plan) {
		res.State = SpecCodeCheckMismatch
		res.Message = FlowMsgInvalid
		res.Detail = "Product Spec ไม่ตรงกัน — Kanban ของเครื่อง " + mc +
			" ระบุ \"" + res.QR + "\" แต่" + planFrom + "ระบุ \"" + res.Plan + "\" กรุณาติดต่อ WH"
		return res
	}

	// ไม่มีแถวใน master_data → CW / CV / SM / MP / PH หา P/N ไม่เจอ แต่ไม่บล็อก IT / Engine
	if !res.InMaster {
		res.State = SpecCodeCheckNotInMaster
		res.Message = "ไม่พบ Product Spec นี้ใน master_data"
		res.Detail = "Product Spec \"" + res.QR + "\" ของเครื่อง " + mc +
			" ไม่มีใน master_data (ชีต CW_CV_ITS) — CW / CV / SM / MP / PH ของเครื่องนี้จะยังจ่ายไม่ได้" +
			" กรุณาอัปโหลด master_data ล่าสุด"
		return res
	}

	if specCodeValueEmpty(res.Plan) {
		res.State = SpecCodeCheckNoPlan
		res.Matched = true
		res.Message = "ใช้ Product Spec จาก Kanban"
		res.Detail = "ไม่มีไฟล์แผนที่ระบุ Product Spec ของเครื่อง " + mc +
			" — ใช้ \"" + res.QR + "\" จาก Kanban เทียบกับ master_data"
		return res
	}

	res.State = SpecCodeCheckMatch
	res.Matched = true
	res.Message = "Product Spec ตรงกับ master_data"
	res.Detail = "เครื่อง " + mc + " — Product Spec " + res.QR + " (สอบทานกับ" + planFrom + "แล้ว)"
	return res
}

// kanbanSpecCodeOverride: ค่าที่จะใช้แทน Product Spec ตามแผน สำหรับ CW / CV / SM / MP / PH
// ("" = ใช้ค่าตามแผนเหมือนเดิม)
func kanbanSpecCodeOverride(res SpecCodeCheckResult) string {
	if res.State == SpecCodeCheckNoQR || res.Blocked() {
		return ""
	}
	return res.Used
}

// BlockedForPart: เงื่อนไขของ CW / CV / SM / MP / PH — Product Spec บน Kanban ต้องมีอยู่ใน Master Data
// (นอกจากกรณี Blocked() ที่ไม่ตรงกับแผน) ไม่มี Spec บน Kanban หรือไม่มีใน Master Data = ห้ามยืนยัน
// IT Controller / Engine ไม่ใช้ฟังก์ชันนี้ จึงทำงานต่อได้ตามปกติ
func (r SpecCodeCheckResult) BlockedForPart() bool {
	switch r.State {
	case SpecCodeCheckMismatch, SpecCodeCheckNotInMaster, SpecCodeCheckNoQR:
		return true
	}
	return false
}

// partBlockMessage: ข้อความบอกสาเหตุตอนบล็อก CW / CV / SM / MP / PH
func partBlockMessage(r SpecCodeCheckResult, label string) (message, detail string) {
	switch r.State {
	case SpecCodeCheckNotInMaster:
		return FlowMsgInvalid, "Product Spec \"" + r.QR + "\" บน Kanban ไม่มีใน master_data (ชีต CW_CV_ITS) — " +
			"ยืนยัน " + label + " ไม่ได้ กรุณาติดต่อ WH / ADMIN"
	case SpecCodeCheckNoQR:
		return FlowMsgInvalid, "ไม่พบ Product Spec บน Kanban — " + label +
			" ต้องเทียบ Product Spec จาก Kanban กับ master_data กรุณาสแกน Kanban ใหม่"
	}
	return r.Message, r.Detail
}
