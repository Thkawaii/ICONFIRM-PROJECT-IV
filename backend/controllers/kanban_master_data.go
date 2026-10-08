package controllers

import (
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// master_data (ชีต CW_CV_ITS) — ทะเบียน P/N ของ CW / CV / SM / MP / PH ตาม Product Spec
//
// ขั้นตอนใหม่ของ CW / CV / SM / MP / PH ยึด master_data เป็นหลัก:
//
//	WH  : เลือก MC# → สแกน Part# → เทียบกับ P/N ใน master_data ของ Product Spec เครื่องนั้น
//	MFG : สแกน QR บน Kanban → เอา MC# + Product Spec บน Kanban ไปเทียบ master_data
//	      ว่ามี Product Spec นี้จริงไหม และค่าที่ติดมากับ Kanban ตรงกับ master_data ไหม
//	      · รหัส P/N    → คอลัมน์ P/N ของ component นั้น (เช่น CW Part No)
//	      · น้ำหนักถ่วง → คอลัมน์ Weight (Tons)
//	        เช่น "YN60C00942P1_4.3T" = P/N YN60C00942P1 + น้ำหนัก 4.3 ตัน
//
// ไฟล์ Planning MFG (ชีต Spec sheet_pcp) ไม่ใช่ตัวหลักแล้ว — ถ้าอัปโหลดไว้ระบบจะเอามา
// สอบทานเพิ่มอีกชั้น ถ้าไม่มีไฟล์นี้ก็ข้ามการสอบทานนั้นไป
// ---------------------------------------------------------------------------

// masterDataPartAliases: ชื่อคอลัมน์ P/N ในชีต master_data ที่เจอได้ (นอกเหนือจาก SpecPartKeys)
var masterDataPartAliases = map[string][]string{
	ComponentCW: {"CW Part No", "CW Part No.", "CW PartNo", "Counter Weight Part No"},
	ComponentCV: {"CV Part No", "CV Part No.", "CV PartNo", "Control Valve Part No"},
	ComponentSM: {"SM Part No", "SM Part No.", "SM PartNo", "Swing Motor Part No"},
	ComponentMP: {"Motor Propel Part No", "Motor Propel Part No.", "MP Part No", "MP PartNo"},
	ComponentPH: {"Pump Hydrolics Part No", "Pump Hydraulics Part No", "Pump Assy HYD Part No", "PH Part No"},
	ComponentEN: {"Engine Part No", "Engine Part No.", "Engine PartNo"},
}

// masterDataWeightKeys: คอลัมน์น้ำหนักถ่วง (ตัน) ในชีต master_data
var masterDataWeightKeys = []string{
	"Weight (Tons)", "Weight (Ton)", "Weight(Tons)", "Weight(tons)",
	"Weight Tons", "Weight (t)", "Weight",
}

// masterDataWeightTonsOf: น้ำหนักถ่วงเป็นตัน ในแถว master_data ของ Product Spec หนึ่ง
func masterDataWeightTonsOf(specRow map[string]string) string {
	if specRow == nil {
		return ""
	}
	keys := make([]string, 0, len(masterDataWeightKeys)*2)
	for _, k := range masterDataWeightKeys {
		keys = append(keys, k, extraColumnPrefix+k)
	}
	return pickField(specRow, keys...)
}

// masterDataPartKeysOf: คอลัมน์ทั้งหมดที่อาจเก็บ P/N ของ component นี้ใน master_data
// (รวมชื่อที่ถูกเติม prefix ตอนอัปโหลดคอลัมน์นอกแบบฟอร์ม)
func masterDataPartKeysOf(component string) []string {
	component = strings.ToUpper(strings.TrimSpace(component))

	var base []string
	if idx := flowComponentIndex(component); idx >= 0 {
		base = append(base, flowComponents[idx].SpecPartKeys...)
	}
	base = append(base, masterDataPartAliases[component]...)

	out := make([]string, 0, len(base)*2)
	seen := map[string]bool{}
	for _, k := range base {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k, extraColumnPrefix+k)
	}
	return out
}

// masterDataPartNoOf: P/N ของ component นี้ ในแถว master_data ของ Product Spec หนึ่ง
func masterDataPartNoOf(specRow map[string]string, component string) string {
	if specRow == nil {
		return ""
	}
	return pickField(specRow, masterDataPartKeysOf(component)...)
}

// splitKanbanPartNo: แยกค่าบน Kanban ออกเป็น "รหัส P/N" กับ "ส่วนขยายท้าย"
//
//	"YN60C00942P1_4.3T" → ("YN60C00942P1", "4.3T")   ← CW บน Kanban ต่อท้ายด้วยน้ำหนักถ่วง
//	"YN60C00942P1"      → ("YN60C00942P1", "")
func splitKanbanPartNo(s string) (string, string) {
	s = strings.TrimSpace(unwrapExcelText(s))
	if i := strings.IndexAny(s, "_#"); i > 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

// partNoBase: เหลือเฉพาะรหัส P/N (ตัดส่วนขยายท้ายและสัญลักษณ์ออก)
func partNoBase(s string) string {
	code, _ := splitKanbanPartNo(s)
	return NormalizeCodeValue(code)
}

// SamePartNo: P/N สองค่าเป็นตัวเดียวกันไหม (ยอมให้ต่างกันแค่ส่วนขยายท้ายรหัส)
func SamePartNo(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if SameCode(a, b) {
		return true
	}
	ba, bb := partNoBase(a), partNoBase(b)
	if ba == "" || bb == "" {
		return false
	}
	return ba == bb
}

// weightUnitAliases: หน่วยน้ำหนักที่เขียนได้หลายแบบ → หน่วยมาตรฐาน
//
// ค่าว่าง (ไม่เขียนหน่วย) ถือเป็น "ตัน" เพราะคอลัมน์ใน master_data คือ Weight (Tons)
var weightUnitAliases = map[string]string{
	"": "T",

	"T": "T", "TON": "T", "TONS": "T", "TONNE": "T", "TONNES": "T", "MT": "T",

	"KG": "KG", "KGS": "KG", "KGM": "KG", "KILOGRAM": "KG", "KILOGRAMS": "KG",

	"G": "G", "GRAM": "G", "GRAMS": "G",

	"LB": "LB", "LBS": "LB", "POUND": "LB", "POUNDS": "LB",
}

// normalizeNumberSeparators: จัดการ "," ในตัวเลขให้ถูกความหมาย
//
//	"6,200"   → "6200"   ← คั่นหลักพัน (หลัง "," มีเลข 3 ตัวพอดี)
//	"4,3"     → "4.3"    ← จุดทศนิยมแบบยุโรป
//	"6,200.5" → "6200.5"
//
// สำคัญกับ Kanban มาก เพราะถ้าอ่าน "6,200" เป็น 6.2 จะกลายเป็นน้ำหนักคนละตัว
func normalizeNumberSeparators(s string) string {
	for {
		i := strings.Index(s, ",")
		if i < 0 {
			return s
		}

		digits := 0
		for j := i + 1; j < len(s) && s[j] >= '0' && s[j] <= '9'; j++ {
			digits++
		}
		prevIsDigit := i > 0 && s[i-1] >= '0' && s[i-1] <= '9'

		if digits == 3 && prevIsDigit {
			s = s[:i] + s[i+1:] // คั่นหลักพัน → ตัดทิ้ง
			continue
		}
		s = s[:i] + "." + s[i+1:] // นอกนั้นถือเป็นจุดทศนิยม
	}
}

// parseWeight: แยกค่าน้ำหนักออกเป็น "ตัวเลข" กับ "หน่วย"
//
//	"4.3T"     → 4.3, "T"
//	"4.3 Tons" → 4.3, "T"
//	"6.2KG"    → 6.2, "KG"
//	"6.2"      → 6.2, "T"    (ไม่เขียนหน่วย = ตัน ตามคอลัมน์ Weight (Tons))
//	"6.2XYZ"   → 6.2, "XYZ"  (หน่วยที่ไม่รู้จัก เก็บไว้ตามที่เขียนมา จะได้จับได้ว่าไม่ตรง)
//	"" / "-"   → ไม่ใช่ตัวเลข (ok = false)
func parseWeight(s string) (float64, string, bool) {
	s = strings.ToUpper(strings.TrimSpace(unwrapExcelText(s)))
	s = normalizeNumberSeparators(s)

	var b strings.Builder
	rest := ""
	for i, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			rest = s[i:] // ตัวอักษรหลังตัวเลข = หน่วย
			break
		}
	}

	num := strings.Trim(b.String(), ".")
	if num == "" {
		return 0, "", false
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, "", false
	}

	// เหลือเฉพาะตัวอักษรของหน่วย (ตัดเว้นวรรค จุด วงเล็บ ออก)
	var u strings.Builder
	for _, r := range rest {
		if r >= 'A' && r <= 'Z' {
			u.WriteRune(r)
		}
	}

	unit := u.String()
	if std, ok := weightUnitAliases[unit]; ok {
		return v, std, true
	}
	return v, unit, true
}

// parseTons: อ่านน้ำหนักเป็นตัวเลข (ไม่สนหน่วย)
func parseTons(s string) (float64, bool) {
	v, _, ok := parseWeight(s)
	return v, ok
}

// weightToKG: อัตราแปลงหน่วย → กิโลกรัม (หน่วยที่ไม่อยู่ในตารางนี้ = แปลงไม่ได้)
var weightToKG = map[string]float64{
	"T":  1000,
	"KG": 1,
	"G":  0.001,
	"LB": 0.45359237,
}

// weightInKG: อ่านน้ำหนักแล้วแปลงเป็นกิโลกรัม
//
//	"6.2T"   → 6200
//	"6200KG" → 6200
//	"6.2"    → 6200   (ไม่เขียนหน่วย = ตัน)
//
// ok = false เมื่ออ่านตัวเลขไม่ได้ หรือเขียนหน่วยที่ระบบไม่รู้จัก
func weightInKG(s string) (float64, bool) {
	v, unit, ok := parseWeight(s)
	if !ok {
		return 0, false
	}
	rate, known := weightToKG[unit]
	if !known {
		return 0, false
	}
	return v * rate, true
}

// SameWeightNumber: ตัวเลขน้ำหนักเท่ากันไหม โดยไม่สนหน่วยเลย
//
// ใช้แยกแยะเคส "ตัวเลขเดียวกันแต่คนละหน่วย" (6.2KG vs 6.2T = คนละน้ำหนัก 1000 เท่า)
// ออกจากเคส "ตัวเลขผิดไปเลย" เพื่อให้ข้อความแจ้งเตือนชัดขึ้น
func SameWeightNumber(a, b string) bool {
	fa, _, oka := parseWeight(a)
	fb, _, okb := parseWeight(b)
	if !oka || !okb {
		return false
	}
	return nearlySameNumber(fa, fb)
}

// SameTons: น้ำหนักถ่วงสองค่าเป็นน้ำหนักเดียวกันไหม
//
// แปลงหน่วยให้ก่อนเทียบ จึงเทียบข้ามหน่วยได้
//
//	4.3T = 4.3 = 4.30 = "4.3 Tons"   ← เขียนคนละแบบ หน่วยเดียวกัน
//	6.2T = 6200KG                     ← คนละหน่วย แต่เป็นน้ำหนักเดียวกัน
//	6.2T ≠ 6.2KG                      ← ต่างกัน 1000 เท่า คนละน้ำหนักจริง ๆ
//
// หน่วยที่ระบบไม่รู้จัก (พิมพ์มาแปลก ๆ) จะแปลงไม่ได้ → ตกไปเทียบแบบตรงตัว
// คือต้องเขียนหน่วยเหมือนกันเป๊ะและตัวเลขเท่ากัน
func SameTons(a, b string) bool {
	ka, oka := weightInKG(a)
	kb, okb := weightInKG(b)
	if oka && okb {
		return nearlySameKG(ka, kb)
	}

	fa, ua, oa := parseWeight(a)
	fb, ub, ob := parseWeight(b)
	if !oa || !ob || ua != ub {
		return false
	}
	return nearlySameNumber(fa, fb)
}

// nearlySameKG: ยอมคลาดเคลื่อนได้ไม่ถึง 5 กก. (= 0.005 ตัน กติกาเดิม)
func nearlySameKG(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 5
}

// nearlySameNumber: เทียบตัวเลขดิบ ยอมคลาดเคลื่อนได้ไม่ถึง 0.005
func nearlySameNumber(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.005
}

// ---------------------------------------------------------------------------
// เทียบค่าที่ติดมากับ QR บน Kanban กับ master_data
// ---------------------------------------------------------------------------

const (
	KanbanPartCheckMatch    = "MATCH"
	KanbanPartCheckMismatch = "MISMATCH"
	KanbanPartCheckSkip     = "SKIP"
	KanbanPartCheckNoMaster = "NO_MASTER"
)

type KanbanPartItem struct {
	Component string `json:"component"`
	Label     string `json:"label"`
	// Field = ช่องที่เทียบ เช่น "P/N" หรือ "Weight (Tons)"
	Field      string `json:"field"`
	QR         string `json:"qr"`
	MasterData string `json:"masterData"`
	OK         bool   `json:"ok"`
}

type KanbanPartCheckResult struct {
	State    string           `json:"state"`
	SpecCode string           `json:"specCode"`
	Matched  bool             `json:"matched"`
	Message  string           `json:"message"`
	Detail   string           `json:"detail"`
	Items    []KanbanPartItem `json:"items"`
}

// Blocked: ค่าบน Kanban ไม่ตรงกับ master_data จริง ๆ → ห้ามประกอบต่อ
// (ไม่มีข้อมูลให้เทียบ = ไม่บล็อก ปล่อยให้ขั้นตอนอื่นจัดการ)
func (r KanbanPartCheckResult) Blocked() bool { return r.State == KanbanPartCheckMismatch }

// kanbanPartFields: ช่องใน QR บน Kanban ที่มีค่าตรงกับคอลัมน์ใน master_data
//
//	YN15435865,YN15-0TD6LG111001,Indonesia,YN02B10084F1,YN12B20016F1,800mm HD grouser shoe,
//	YN60C00942P1_4.3T,...
//	^ MC#      ^ Product Spec    ^ ลูกค้า  ^ Boom P/N    ^ Arm P/N     ^ Shoe
//	                                                                   ^ CW (ช่องที่ 7)
//
// ค่า CW บน Kanban เขียนติดกันสองอย่าง — "รหัส P/N" _ "น้ำหนักถ่วง"
// จึงต้องแยกไปเทียบคนละคอลัมน์:
//
//	YN60C00942P1 → คอลัมน์ CW Part No
//	4.3T         → คอลัมน์ Weight (Tons)
//
// Boom / Arm / Shoe ไม่มีคอลัมน์ใน master_data จึงยังไม่เอามาเทียบ
var kanbanPartFields = []struct {
	Component string
	QRValue   func(SpecQR) string
	// HasWeight = ค่าหลังขีดล่างเป็นน้ำหนักถ่วง (ตัน) ที่ต้องเทียบกับ Weight (Tons)
	HasWeight bool
}{
	{ComponentCW, func(q SpecQR) string { return q.CWPN }, true},
}

// checkKanbanPartNos: เทียบค่าที่ติดมากับ Kanban (รหัส P/N และน้ำหนักถ่วง)
// กับคอลัมน์ที่ตรงกันใน master_data ของ Product Spec นั้น
//
//	specRow = แถว master_data ของ Product Spec ที่อ่านได้จาก Kanban (nil = ไม่มีในทะเบียน)
func checkKanbanPartNos(q SpecQR, specCode string, specRow map[string]string) KanbanPartCheckResult {
	res := KanbanPartCheckResult{SpecCode: strings.TrimSpace(unwrapExcelText(specCode))}

	if specRow == nil {
		res.State = KanbanPartCheckNoMaster
		res.Message = "ไม่พบ Product Spec นี้ใน master_data"
		res.Detail = "Product Spec " + orDash(res.SpecCode) +
			" ไม่มีใน master_data จึงยังเทียบ P/N บน Kanban ไม่ได้"
		return res
	}

	var bad []string
	add := func(component, field, qv, mv string, ok bool) {
		it := KanbanPartItem{
			Component:  component,
			Label:      ComponentLabel(component),
			Field:      field,
			QR:         qv,
			MasterData: mv,
			OK:         ok,
		}
		if !ok {
			bad = append(bad, it.Label+" "+field+": Kanban = "+it.QR+" / master_data = "+it.MasterData)
		}
		res.Items = append(res.Items, it)
	}

	for _, f := range kanbanPartFields {
		raw := strings.TrimSpace(unwrapExcelText(f.QRValue(q)))
		if raw == "" {
			continue
		}
		qrPartNo, qrWeight := splitKanbanPartNo(raw)

		// รหัส P/N → คอลัมน์ P/N ของ component นั้น
		if mv := masterDataPartNoOf(specRow, f.Component); mv != "" && qrPartNo != "" {
			add(f.Component, "P/N", qrPartNo, mv, SamePartNo(qrPartNo, mv))
		}

		// น้ำหนักถ่วงที่ต่อท้าย → คอลัมน์ Weight (Tons)
		//
		// Kanban ของ CW ต้องเขียนน้ำหนักถ่วงต่อท้ายเสมอ (เช่น YN60C00942P1_4.3T)
		// ถ้าไม่มี ถือว่าข้อมูลบน Kanban ไม่ครบ — ไม่ใช่เรื่องที่ข้ามได้
		// เพราะ CW รหัสเดียวกันมีได้หลายน้ำหนัก ถ้าไม่เทียบจะจับของผิดน้ำหนักไม่ได้เลย
		if f.HasWeight {
			if mw := masterDataWeightTonsOf(specRow); mw != "" {
				switch {
				case qrWeight == "":
					add(f.Component, "Weight (Tons)", "(ไม่มีน้ำหนักถ่วงต่อท้าย P/N)", mw, false)

				case SameTons(qrWeight, mw):
					add(f.Component, "Weight (Tons)", qrWeight, mw, true)

				// ตัวเลขเท่ากันแต่คนละหน่วย (เช่น 6.2KG กับ 6.2T = ต่างกัน 1000 เท่า)
				// แยกข้อความออกมา เพราะคนอ่านมักมองผ่านว่า "ก็ 6.2 เหมือนกัน"
				case SameWeightNumber(qrWeight, mw):
					add(f.Component, "Weight — ตัวเลขเท่ากันแต่คนละหน่วย", qrWeight, mw, false)

				default:
					add(f.Component, "Weight (Tons)", qrWeight, mw, false)
				}
			}
		}
	}

	if len(res.Items) == 0 {
		res.State = KanbanPartCheckSkip
		res.Message = "ไม่มีค่าบน Kanban ที่เทียบกับ master_data ได้"
		res.Detail = "QR บน Kanban ไม่มีช่อง P/N / น้ำหนักถ่วง ที่ตรงกับคอลัมน์ใน master_data จึงข้ามการตรวจข้อนี้"
		return res
	}

	if len(bad) > 0 {
		res.State = KanbanPartCheckMismatch
		res.Message = FlowMsgInvalid
		res.Detail = "ข้อมูลบน Kanban ไม่ตรงกับ master_data ของ Product Spec " + orDash(res.SpecCode) +
			" — " + strings.Join(bad, " · ") + " กรุณาติดต่อ WH"
		return res
	}

	res.State = KanbanPartCheckMatch
	res.Matched = true
	res.Message = "ข้อมูลบน Kanban ตรงกับ master_data"
	res.Detail = "Product Spec " + orDash(res.SpecCode) + " — ตรวจแล้ว " +
		strconv.Itoa(len(res.Items)) + " รายการ"
	return res
}