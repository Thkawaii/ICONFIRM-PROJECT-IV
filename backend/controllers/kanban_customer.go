package controllers

import "strings"

// ---------------------------------------------------------------------------
// ตรวจ "ลูกค้า / ประเทศ" ตอน MFG สแกน Kanban
//
// QR บน Kanban ช่องที่ 3 (index 2) = ชื่อลูกค้า / ประเทศปลายทาง เช่น "Singapore"
//
//	YN15438324,YN15-0QD7BG131001,Singapore,YN02B10321F1,...
//	            ^ Spec code       ^ ลูกค้า / ประเทศ
//
// ไฟล์ Planning WH (ชีต Engine_IT allocation) มีคอลัมน์ "Customer name" ของเครื่องนั้น
//
// ถ้าสองค่านี้ไม่ตรงกัน = Kanban ที่ถืออยู่เป็นของคนละประเทศกับแผนจ่ายของ WH
// → ห้ามประกอบต่อ ต้องให้ WH ตรวจก่อน
//
// ถ้าฝั่งใดฝั่งหนึ่งไม่มีค่า (QR ไม่มีช่องลูกค้า / Planning WH เว้นว่าง) จะ "ข้าม" การตรวจ
// ไม่บล็อก เพราะไม่มีข้อมูลพอจะบอกว่าผิด
// ---------------------------------------------------------------------------

const (
	CustomerCheckMatch    = "MATCH"
	CustomerCheckMismatch = "MISMATCH"
	CustomerCheckNoQR     = "NO_QR"
	CustomerCheckNoPlan   = "NO_PLAN"
)

// allocCustomerKeys: คอลัมน์ชื่อลูกค้า / ประเทศ ในไฟล์ Planning WH
var allocCustomerKeys = []string{
	"Customer name", "Customer Name", "Customer",
	extraColumnPrefix + "Customer name", extraColumnPrefix + "Customer Name",
	"Country Name", "Country",
}

type CustomerCheckResult struct {
	State     string `json:"state"`
	MachineNo string `json:"machineNo"`
	QR        string `json:"qr"`
	Plan      string `json:"plan"`
	Matched   bool   `json:"matched"`
	Message   string `json:"message"`
	Detail    string `json:"detail"`
}

// Blocked: ไม่ตรงกันจริง ๆ (ขาดข้อมูลฝั่งใดฝั่งหนึ่ง = ไม่บล็อก)
func (r CustomerCheckResult) Blocked() bool { return r.State == CustomerCheckMismatch }

// customerNoiseWords: คำที่ไม่ได้บอกว่าเป็นลูกค้า/ประเทศไหน (คำต่อท้ายนิติบุคคล ชื่อกลุ่มบริษัท)
var customerNoiseWords = map[string]bool{
	"CO": true, "LTD": true, "LTDA": true, "PTE": true, "PVT": true, "INC": true,
	"CORP": true, "CORPORATION": true, "COMPANY": true, "LIMITED": true,
	"SDN": true, "BHD": true, "GMBH": true, "PLC": true, "LLC": true,
	"KOBELCO": true, "KCM": true, "KCMSA": true, "KCSA": true,
	"CONSTRUCTION": true, "MACHINERY": true, "MACHINERIES": true, "EQUIPMENT": true,
	"THE": true, "AND": true, "FOR": true, "CUSTOMER": true, "DEST": true,
	"DESTINATION": true, "EXPORT": true, "STOCK": true,
}

// normalizeCustomerName: เหลือเฉพาะ A-Z 0-9 (ตัดเว้นวรรค วงเล็บ จุด ขีด ออก)
func normalizeCustomerName(s string) string {
	s = strings.ToUpper(strings.TrimSpace(unwrapExcelText(s)))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// customerValueEmpty: ค่าที่ถือว่า "ไม่ได้ระบุ"
func customerValueEmpty(s string) bool {
	switch normalizeCustomerName(s) {
	case "", "NA", "TBD", "TBA", "NONE", "NIL", "UNKNOWN", "XXX":
		return true
	}
	return false
}

// customerTokens: คำสำคัญของชื่อลูกค้า (ตัดคำต่อท้ายนิติบุคคล และคำสั้นกว่า 3 ตัวอักษรออก)
func customerTokens(s string) []string {
	s = strings.ToUpper(strings.TrimSpace(unwrapExcelText(s)))
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 3 || customerNoiseWords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// SameCustomerName: ชื่อลูกค้า / ประเทศ 2 ค่าถือว่าเป็นที่เดียวกันไหม
//
// เทียบแบบยืดหยุ่น เพราะ QR มักเขียนสั้น ("Singapore") ส่วนในชีตมักเขียนเต็ม
// ("KOBELCO (SINGAPORE) PTE LTD") — จะถือว่าตรงเมื่อ
//   - ตัดสัญลักษณ์แล้วเท่ากัน
//   - ค่าหนึ่งเป็นส่วนหนึ่งของอีกค่า
//   - มีคำสำคัญร่วมกันอย่างน้อย 1 คำ
func SameCustomerName(a, b string) bool {
	na, nb := normalizeCustomerName(a), normalizeCustomerName(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	if len(na) >= 3 && strings.Contains(nb, na) {
		return true
	}
	if len(nb) >= 3 && strings.Contains(na, nb) {
		return true
	}
	for _, t := range customerTokens(a) {
		if strings.Contains(nb, t) {
			return true
		}
	}
	for _, t := range customerTokens(b) {
		if strings.Contains(na, t) {
			return true
		}
	}
	return false
}

// checkKanbanCustomer: เทียบลูกค้า / ประเทศ บน Kanban กับไฟล์ Planning WH
func checkKanbanCustomer(machineNo, qrCustomer, planCustomer string) CustomerCheckResult {
	res := CustomerCheckResult{
		MachineNo: strings.ToUpper(strings.TrimSpace(machineNo)),
		QR:        strings.TrimSpace(unwrapExcelText(qrCustomer)),
		Plan:      strings.TrimSpace(unwrapExcelText(planCustomer)),
	}
	mc := res.MachineNo
	if mc == "" {
		mc = "(ไม่ระบุ)"
	}

	switch {
	case customerValueEmpty(res.QR):
		res.State = CustomerCheckNoQR
		res.Message = "ไม่พบชื่อลูกค้า / ประเทศบน Kanban"
		res.Detail = "QR บน Kanban ไม่มีช่องลูกค้า / ประเทศ จึงข้ามการตรวจข้อนี้"

	case customerValueEmpty(res.Plan):
		res.State = CustomerCheckNoPlan
		res.Message = "ไฟล์ Planning WH ไม่มีชื่อลูกค้า / ประเทศ"
		res.Detail = "เครื่อง " + mc + " ไม่มีค่าในคอลัมน์ Customer name ของไฟล์ Planning WH จึงข้ามการตรวจข้อนี้"

	case SameCustomerName(res.QR, res.Plan):
		res.State = CustomerCheckMatch
		res.Matched = true
		res.Message = "ลูกค้า / ประเทศตรงกับ Planning WH"
		res.Detail = "เครื่อง " + mc + " ปลายทาง " + res.Plan

	default:
		res.State = CustomerCheckMismatch
		res.Message = FlowMsgInvalid
		res.Detail = "ลูกค้า / ประเทศไม่ตรงกัน — Kanban ของเครื่อง " + mc +
			" ระบุ \"" + res.QR + "\" แต่ Planning WH ระบุ \"" + res.Plan + "\" กรุณาติดต่อ WH"
	}
	return res
}

// checkKanbanCustomerRow: เทียบกับแถว Planning WH ของเครื่องนั้นที่หามาแล้ว
func checkKanbanCustomerRow(machineNo, qrCustomer string, allocRow map[string]string) CustomerCheckResult {
	return checkKanbanCustomer(machineNo, qrCustomer, pickField(allocRow, allocCustomerKeys...))
}

// checkKanbanCustomerForMachine: หาแถว Planning WH ของเครื่องให้เอง (ใช้ตอนที่ยังไม่ได้โหลด index)
func checkKanbanCustomerForMachine(machineNo, qrCustomer string) CustomerCheckResult {
	return checkKanbanCustomerRow(machineNo, qrCustomer,
		flowAllocFor(flowAllocIndex(), machineNo))
}
