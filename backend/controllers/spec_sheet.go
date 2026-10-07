package controllers

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"regexp"
	"strings"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Specification sheet (Daily plan) → ตาราง Planning
//
// ชีต "Specification sheet_20 Draft.00" / "Specification sheet_30" เก็บข้อมูลเครื่องละ 1 บล็อก 5 แถว:
//
//	     A        B             C                  D                 E                  F               G                H                     I                J
//	+0  LOT NO.  Machine No.   Main line          Brand             Other piping       Boom            Counter Weight   Cold region spec      Other option     Remark
//	+1           (Spec code)                      Destination       Base Machine Spec  Piping, boom    Lower ATT        Auto Greasing System  Additional ATT
//	+2                                            Purpose           Cab base           Arm             Lever            Seat                  Radio
//	+3           (Customer)    Stand by shipping  Upp,Lower Spec    Cab                Piping,arm      Multi control    Paint                 Engine start key
//	+4                                            Front ATT piping  Cab guard          Shoe            Air conditioner  Front ATT             IT device        Cold region spec(HYD oil)
//
// ระบบอ่านหัวตาราง 5 แถวเพื่อรู้ว่าช่องไหนคือฟิลด์อะไร แล้วแปลงแต่ละบล็อกเป็น 1 แถวของตาราง Planning
// ---------------------------------------------------------------------------

const (
	specFieldSpecCode = "Spec Code"
	specFieldCustomer = "Country Name"
)

// specHeaderToPlanning: หัวคอลัมน์ในชีต (normalize แล้ว) → ชื่อคอลัมน์มาตรฐานของตาราง Planning
var specHeaderToPlanning = map[string]string{
	"lotno":                "LOT NO.",
	"machineno":            "Machine",
	"mainline":             "Main line",
	"standbyshipping":      "Stand by shipping",
	"brand":                "Brand",
	"destination":          "Destination",
	"purpose":              "Purpose",
	"upplowerspec":         "Upp,lower spec.",
	"frontattpiping":       "Front ATT piping",
	"otherpiping":          "Other piping",
	"basemachinespec":      "Base machine spec.",
	"cabbase":              "Cab base",
	"cab":                  "Cab",
	"cabguard":             "Cab guard",
	"boom":                 "Boom",
	"pipingboom":           "Piping, boom",
	"arm":                  "Arm",
	"pipingarm":            "Piping, arm",
	"shoe":                 "Shoe",
	"counterweight":        "Counter weight",
	"loweratt":             "Lower ATT",
	"lever":                "Lever",
	"multicontrol":         "Multi control",
	"airconditioner":       "Air conditioner",
	"coldregionspec":       "Cold region spec.",
	"autogreasingsystem":   "Auto greasing system",
	"seat":                 "Seat",
	"paint":                "Paint",
	"frontatt":             "Front ATT",
	"otheroption":          "Other option",
	"additionalatt":        "Additional ATT",
	"radio":                "Radio",
	"enginestartkey":       "Engine start key",
	"itdevice":             "IT device",
	"coldregionspechydoil": "Cold region spec(HYD oil)",
	"remark":               "Remark",
	"remarks":              "Remark",
}

// specSheetColumns: ลำดับคอลัมน์ของตาราง Planning ที่ได้จาก Specification sheet
var specSheetColumns = []string{
	"LOT NO.", "Machine", specFieldSpecCode, specFieldCustomer, "Main line", "Stand by shipping",
	"Brand", "Destination", "Purpose", "Upp,lower spec.", "Base machine spec.",
	"Front ATT piping", "Other piping", "Cab base", "Cab", "Cab guard",
	"Boom", "Piping, boom", "Arm", "Piping, arm", "Shoe", "Counter weight",
	"Lower ATT", "Lever", "Multi control", "Air conditioner",
	"Cold region spec.", "Auto greasing system", "Seat", "Paint",
	"Front ATT", "Other option", "Additional ATT", "Radio", "Engine start key",
	"IT device", "Cold region spec(HYD oil)", "Remark",
}

// isSpecSheetTable: แถวหัวตารางนี้มาจากการแปลง Specification sheet หรือไม่
func isSpecSheetTable(header []string) bool {
	if len(header) != len(specSheetColumns) {
		return false
	}
	for i, l := range specSheetColumns {
		if header[i] != l {
			return false
		}
	}
	return true
}

var errNoSpecSheet = errors.New("ไม่พบชีต Specification sheet ในไฟล์นี้ — Daily Plan ต้องมีชีตที่ขึ้นต้นด้วย LOT NO. / Machine No. (เครื่องละ 5 แถว)")

var specMachineNoRe = regexp.MustCompile(`^[A-Z]{1,4}[0-9]{5,}[A-Z0-9]*$`)

func cellAt(rows [][]string, r, c int) string {
	if r < 0 || r >= len(rows) || c < 0 || c >= len(rows[r]) {
		return ""
	}
	return strings.TrimSpace(unwrapExcelText(rows[r][c]))
}

// findSpecSheetHeader: หาแถวหัวตาราง (แถวที่ A = LOT NO. และ B = Machine No.) และตำแหน่งฟิลด์ใน 5 แถว
// คืน -1 ถ้าชีตนี้ไม่ใช่ Specification sheet
func findSpecSheetHeader(rows [][]string) (int, map[[2]int]string) {
	limit := 40
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		if normalizeHeader(cellAt(rows, i, 0)) != "lotno" {
			continue
		}
		if normalizeHeader(cellAt(rows, i, 1)) != "machineno" {
			continue
		}
		layout := map[[2]int]string{}
		hits := 0
		for off := 0; off < 5; off++ {
			if i+off >= len(rows) {
				break
			}
			for c := range rows[i+off] {
				if label, ok := specHeaderToPlanning[normalizeHeader(cellAt(rows, i+off, c))]; ok {
					layout[[2]int{off, c}] = label
					hits++
				}
			}
		}
		// ต้องเจอแถวหัวครบพอสมควร (กันชนกับตารางอื่นที่มี LOT NO. / Machine No.)
		if hits < 15 {
			continue
		}
		// ช่องที่ไม่มีหัวในชีต: B+1 = Spec code, B+3 = ลูกค้า/ประเทศ
		layout[[2]int{1, 1}] = specFieldSpecCode
		layout[[2]int{3, 1}] = specFieldCustomer
		return i, layout
	}
	return -1, nil
}

// specSheetBlocks: แปลงบล็อกเครื่องในชีตเป็นแถวข้อมูล (map ชื่อคอลัมน์ → ค่า)
func specSheetBlocks(rows [][]string) []map[string]string {
	headerIdx, layout := findSpecSheetHeader(rows)
	if headerIdx < 0 {
		return nil
	}
	var out []map[string]string
	for r := headerIdx + 5; r < len(rows); r++ {
		mc := strings.ToUpper(cellAt(rows, r, 1))
		if !specMachineNoRe.MatchString(mc) {
			continue
		}
		data := map[string]string{}
		for pos, label := range layout {
			v := cellAt(rows, r+pos[0], pos[1])
			if v == "0" { // ช่องว่างที่เป็นสูตรใน Excel แสดงผลเป็น 0
				v = ""
			}
			if v != "" {
				data[label] = v
			}
		}
		data["Machine"] = mc
		out = append(out, data)
		r += 4 // ข้ามแถวที่เหลือของบล็อกนี้
	}
	return out
}

// specBlocksToTable: สร้างตารางปกติ (หัวคอลัมน์ + แถวข้อมูล) ให้ขั้นตอนอัปโหลดเดิมใช้ต่อได้
func specBlocksToTable(blocks []map[string]string) [][]string {
	table := make([][]string, 0, len(blocks)+1)
	header := append([]string{}, specSheetColumns...)
	table = append(table, header)
	for _, b := range blocks {
		row := make([]string, len(specSheetColumns))
		for i, label := range specSheetColumns {
			row[i] = b[label]
		}
		table = append(table, row)
	}
	return table
}

// readPlanningSpecSheets: ถ้าไฟล์มีชีต Specification sheet (20T / 30T ...) จะรวมทุกชีตเป็นตารางเดียว
// ok=false = ไม่ใช่ไฟล์รูปแบบนี้ (ให้ใช้วิธีอ่านเดิม)
func readPlanningSpecSheets(fileHeader *multipart.FileHeader) ([][]string, bool, error) {
	sheets, err := readAllUploadedSheets(fileHeader)
	if err != nil {
		return nil, false, err
	}
	var blocks []map[string]string
	seen := map[string]bool{}
	found := false
	for _, sh := range sheets {
		b := specSheetBlocks(sh.rows)
		if b == nil {
			continue
		}
		found = true
		for _, row := range b {
			key := NormalizeCodeValue(row["Machine"])
			if seen[key] {
				continue
			}
			seen[key] = true
			blocks = append(blocks, row)
		}
	}
	if !found {
		return nil, false, nil
	}
	return specBlocksToTable(blocks), true, nil
}

// ---------------------------------------------------------------------------
// QR บน Specification sheet ที่ MFG สแกน
//
//	YN15438324,YN15-0QD7BG131001,Singapore,YN02B10321F1,YN12B10983F1,600mm HD grouser shoe,
//	YN60C00942P1_4.3T,0.93m3 w/REINF bucket(SEA T),Southeast Asia A(less EGR),IT(Mobile4G  normal speed),,,,sk200-10, sea-a
//
// ลำดับ: 0 Machine No · 1 Spec code · 2 ลูกค้า · 3 Boom P/N · 4 Arm P/N · 5 Shoe · 6 C/W P/N
//
//	7 Front ATT (bucket) · 8 Destination (Engine type) · 9 IT device · 10-12 (ว่าง) · 13+ Model
//
// ค่าที่มี "," ในชีต ใน QR จะถูกแทนด้วยช่องว่าง จึงเทียบแบบตัดสัญลักษณ์/ช่องว่างออก (NormalizeCodeValue)
// ---------------------------------------------------------------------------

type SpecQR struct {
	Raw         string `json:"raw"`
	MachineNo   string `json:"machineNo"`
	SpecCode    string `json:"specCode"`
	Customer    string `json:"customer"`
	BoomPN      string `json:"boomPN"`
	ArmPN       string `json:"armPN"`
	Shoe        string `json:"shoe"`
	CWPN        string `json:"cwPN"`
	FrontATT    string `json:"frontATT"`
	Destination string `json:"destination"`
	ITDevice    string `json:"itDevice"`
	Model       string `json:"model"`
}

// LooksLikeSpecQR: ข้อความที่สแกนเป็น QR ของ Specification sheet หรือไม่
func LooksLikeSpecQR(raw string) bool {
	return strings.Count(raw, ",") >= 5
}

func ParseSpecQR(raw string) (SpecQR, bool) {
	raw = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, "\r", " "), "\n", " "))
	if !LooksLikeSpecQR(raw) {
		return SpecQR{}, false
	}
	f := strings.Split(raw, ",")
	get := func(i int) string {
		if i < len(f) {
			return strings.TrimSpace(f[i])
		}
		return ""
	}
	q := SpecQR{
		Raw:         raw,
		MachineNo:   strings.ToUpper(get(0)),
		SpecCode:    get(1),
		Customer:    get(2),
		BoomPN:      get(3),
		ArmPN:       get(4),
		Shoe:        get(5),
		CWPN:        get(6),
		FrontATT:    get(7),
		Destination: get(8),
		ITDevice:    get(9),
	}
	// Model อาจมี "," อยู่ในค่า (เช่น "SK200-10, SEA-A") → รวมทุกช่องที่เหลือหลังช่องว่าง
	if len(f) > 10 {
		start := 10
		if len(f) > 13 {
			start = 13
		}
		parts := []string{}
		for _, p := range f[start:] {
			if t := strings.TrimSpace(p); t != "" || len(parts) > 0 {
				parts = append(parts, t)
			}
		}
		q.Model = strings.TrimSpace(strings.Join(parts, ", "))
	}
	if q.MachineNo == "" {
		return q, false
	}
	return q, true
}

const (
	SpecStateMatch    = "MATCH"
	SpecStateMismatch = "MISMATCH"
	SpecStateNoPlan   = "NO_PLAN"
	SpecStateNoQR     = "NO_QR"
)

type SpecCompareItem struct {
	Field string `json:"field"`
	Label string `json:"label"`
	QR    string `json:"qr"`
	Plan  string `json:"plan"`
	OK    bool   `json:"ok"`
}

type SpecCheckResult struct {
	State     string            `json:"state"`
	MachineNo string            `json:"machineNo"`
	Matched   bool              `json:"matched"`
	Message   string            `json:"message"`
	Detail    string            `json:"detail"`
	Items     []SpecCompareItem `json:"items"`
	// ข้อมูลใน QR ที่ไม่มีในชีต (แสดงอย่างเดียว ไม่นำมาเทียบ)
	Info []SpecCompareItem `json:"info,omitempty"`
}

// specCompareFields: ฟิลด์ที่เทียบระหว่าง QR กับ Specification sheet
var specCompareFields = []struct {
	field string
	label string
	qr    func(SpecQR) string
	plan  []string
}{
	{"specCode", "Spec code", func(q SpecQR) string { return q.SpecCode }, []string{specFieldSpecCode, "Product Spec 1"}},
	{"customer", "ลูกค้า / ประเทศ", func(q SpecQR) string { return q.Customer }, []string{specFieldCustomer}},
	{"model", "Model (Base machine spec.)", func(q SpecQR) string { return q.Model }, []string{"Base machine spec."}},
	{"destination", "Engine type (Destination)", func(q SpecQR) string { return q.Destination }, []string{"Destination"}},
	{"shoe", "Shoe", func(q SpecQR) string { return q.Shoe }, []string{"Shoe"}},
	{"frontATT", "Bucket (Front ATT)", func(q SpecQR) string { return q.FrontATT }, []string{"Front ATT"}},
	{"itDevice", "IT device", func(q SpecQR) string { return q.ITDevice }, []string{"IT device"}},
}

// loadSpecPlans: ข้อมูล Daily Plan ล่าสุด (อ่านตรงจากฐานข้อมูล) key = เลขเครื่องแบบ normalize
func loadSpecPlans() map[string]map[string]string {
	out := map[string]map[string]string{}
	if config.DB == nil {
		return out
	}
	var rows []models.UploadDataRow
	config.DB.Select("id", "machine_no", "data_json").
		Where("dataset = ?", models.DatasetDailyPlan).Order("id asc").Find(&rows)
	for _, r := range rows {
		m := map[string]string{}
		if json.Unmarshal([]byte(r.DataJSON), &m) != nil {
			continue
		}
		mc := r.MachineNo
		if mc == "" {
			mc = machineFromRow(m)
		}
		if key := NormalizeCodeValue(mc); key != "" {
			if _, ok := out[key]; !ok {
				out[key] = m
			}
		}
	}
	return out
}

func specPlanFor(plans map[string]map[string]string, machineNo string) map[string]string {
	if p, ok := plans[NormalizeCodeValue(machineNo)]; ok {
		return p
	}
	if old := ResolveMachineNo(machineNo); old != machineNo {
		if p, ok := plans[NormalizeCodeValue(old)]; ok {
			return p
		}
	}
	if cur := CurrentCodeOf(machineNo); cur != machineNo {
		if p, ok := plans[NormalizeCodeValue(cur)]; ok {
			return p
		}
	}
	return nil
}

// CheckSpecQR: เทียบ QR กับ Specification sheet → MATCH = ประกอบได้
func CheckSpecQR(plans map[string]map[string]string, rawQR string) SpecCheckResult {
	q, ok := ParseSpecQR(rawQR)
	if !ok {
		return SpecCheckResult{
			State:   SpecStateNoQR,
			Message: "ยังไม่ได้สแกน QR ของ Specification sheet",
			Detail:  "กรุณาสแกน QR บน Specification sheet ของเครื่อง",
		}
	}
	res := SpecCheckResult{MachineNo: q.MachineNo}
	res.Info = []SpecCompareItem{
		{Field: "boomPN", Label: "Boom P/N", QR: q.BoomPN, OK: true},
		{Field: "armPN", Label: "Arm P/N", QR: q.ArmPN, OK: true},
		{Field: "cwPN", Label: "Counterweight P/N", QR: q.CWPN, OK: true},
	}

	plan := specPlanFor(plans, q.MachineNo)
	if plan == nil {
		res.State = SpecStateNoPlan
		res.Message = "ข้อมูลการประกอบไม่ถูกต้อง"
		res.Detail = "ไม่พบ Machine No. " + q.MachineNo + " ในตาราง Daily Plan — กรุณาอัปโหลด Daily Plan ล่าสุด"
		return res
	}

	var bad []string
	for _, f := range specCompareFields {
		qv := strings.TrimSpace(f.qr(q))
		if qv == "" {
			continue // QR ไม่มีค่านี้ → ไม่เทียบ
		}
		pv := pickField(plan, f.plan...)
		item := SpecCompareItem{Field: f.field, Label: f.label, QR: qv, Plan: pv}
		item.OK = pv != "" && NormalizeCodeValue(qv) == NormalizeCodeValue(pv)
		if !item.OK {
			bad = append(bad, f.label)
		}
		res.Items = append(res.Items, item)
	}

	if len(bad) == 0 {
		res.State = SpecStateMatch
		res.Matched = true
		res.Message = "ข้อมูลตรงกับ Specification sheet"
		res.Detail = "QR ของเครื่อง " + q.MachineNo + " ตรงกับ Specification sheet ทุกรายการ"
		return res
	}
	res.State = SpecStateMismatch
	res.Message = "ข้อมูลไม่ตรงกับ Specification sheet"
	parts := make([]string, 0, len(bad))
	for _, it := range res.Items {
		if it.OK {
			continue
		}
		plan := it.Plan
		if plan == "" {
			plan = "(ไม่มีในชีต)"
		}
		parts = append(parts, it.Label+": QR = "+it.QR+" / ชีต = "+plan)
	}
	res.Detail = "เครื่อง " + q.MachineNo + " ไม่ตรง " + strings.Join(parts, " · ")
	return res
}

func specResultJSON(r SpecCheckResult) string {
	b, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(b)
}

// CheckMFGSpecQR: POST /mfg-assembly/spec-check {qrCode}
// ตรวจ QR กับ Specification sheet ทันทีหลังสแกน (ยังไม่บันทึก) — ให้ MFG รู้ผลก่อนสแกน IT Controller
func CheckMFGSpecQR(c *gin.Context) {
	var req struct {
		QRCode string `json:"qrCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}
	c.JSON(200, CheckSpecQR(loadSpecPlans(), req.QRCode))
}
