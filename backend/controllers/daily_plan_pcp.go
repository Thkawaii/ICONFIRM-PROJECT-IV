package controllers

import (
	"encoding/json"
	"mime/multipart"
	"regexp"
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// ชีต "Daily plan" ในไฟล์แผนผลิตรายเดือน (Planning PCP)
//
// เป็นตารางแนวนอนปกติ หัวตารางอยู่ราวแถว 10 คนละโครงสร้างกับชีต
// Specification sheet (ที่เป็นบล็อกเครื่องละหลายแถว — ดู spec_sheet.go)
//
//	C/S  C/F  B  A  Upper  LOT NO.  Machine  Customer  Q'ty  Main line  Stand by shipping  Pitch time
//
// ฟิลด์ที่ระบบใช้จริง
//
//	Machine    = MC# เช่น YN15438295 · YC14406079
//	Customer   = ประเทศปลายทาง เช่น Indonesia · ANZ-AU
//	Main line  = วันที่จ่ายของ (WH ใช้ตัวนี้กรองว่าวันนี้ต้องสแกนกี่เครื่อง)
//
// ---------------------------------------------------------------------------
// ช่อง Main line เขียนได้ 2 แบบ และต้องรองรับทั้งคู่
//
// แบบที่ 1 — ลงวันที่ทุกแถว (พบในชีต 30T)
//
//	YC14406079  ANZ-AU  1  25-Sep-26
//	YC14406080  ANZ-AU  1  30-Sep-26
//	LC14405987  Indonesia  1  30-Sep-26
//
// แบบที่ 2 — ลงวันที่แถวแรกของกลุ่ม แล้วบอกจำนวนเครื่องในกลุ่มไว้ใต้วันที่ (พบในชีต 20T)
//
//	YN15438295  Indonesia  1  25-Sep-26   ← วันที่ของทั้งกลุ่ม
//	YN15438296  Indonesia  1  (6 MC)      ← บอกว่ากลุ่มนี้มี 6 เครื่อง
//	LG03405125  Ariya      1              ← ว่าง = ใช้วันที่ของกลุ่มเดิม
//	YN15438309  Indonesia  1
//	YN15438310  Indonesia  1
//	YN15438311  Indonesia  1
//	YN15438312  Indonesia  1  30-Sep-26   ← ขึ้นกลุ่มใหม่
//
// สองแบบนี้บอกเรื่องเดียวกัน ต่างกันแค่วิธีเขียน จึงอ่านด้วยกติกาเดียว:
// ไล่จากบนลงล่าง จำวันที่ล่าสุดที่เจอไว้ แล้วเติมให้ทุกแถวที่ตามมาจนกว่าจะเจอวันใหม่
//
// ข้อความ "(6 MC)" เป็นคำอธิบายของคน ไม่ใช่วันที่ — ต้องข้าม ไม่ใช่ล้างวันที่ทิ้ง
// ถ้าตีความผิดเป็น "ไม่มีวันที่" เครื่องเกือบทั้งชีตจะไม่มีวันจ่าย แล้ว WH
// จะเปิดหน้าจอมาเจอรายการว่าง ทั้งที่แผนมีของให้จ่ายอยู่
//
// จำนวนในวงเล็บใช้ตรวจทานว่านับเครื่องในกลุ่มได้ครบตามที่ไฟล์บอกหรือไม่
// ไม่ได้ใช้ตัดกลุ่ม เพราะไฟล์จริงมีแถวว่างคั่นและแถวที่ไม่ใช่เครื่องปนอยู่
// ---------------------------------------------------------------------------

// dailyPlanColumns: คอลัมน์ของตารางที่แปลงเสร็จแล้ว
var dailyPlanColumns = []string{
	"LOT NO.", "Machine", "Country Name", "Q'ty", "Main line", "Stand by shipping",
}

// dailyPlanGroupCountRe: ข้อความบอกจำนวนเครื่องในกลุ่ม เช่น "(6 MC)" "(11 MC)" "( 6 mc )"
var dailyPlanGroupCountRe = regexp.MustCompile(`^\(\s*(\d+)\s*MC\s*\)$`)

// dailyPlanMachineRe: รูปแบบเลขเครื่อง เช่น YN15438295 · LG03405125 · YC14406079
var dailyPlanMachineRe = regexp.MustCompile(`^[A-Z]{1,4}[0-9]{5,}[A-Z0-9]*$`)

// dailyPlanGroupCount: แถวนี้เป็นข้อความบอกจำนวนเครื่องในกลุ่มหรือไม่
//
// คืน (จำนวน, true) ถ้าใช่ — ผู้เรียกต้อง "ข้าม" ช่องนี้ ไม่ใช่ถือว่าไม่มีวันที่
func dailyPlanGroupCount(s string) (int, bool) {
	t := strings.ToUpper(strings.TrimSpace(unwrapExcelText(s)))
	m := dailyPlanGroupCountRe.FindStringSubmatch(t)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// dailyPlanRow = หนึ่งเครื่องในแผนจ่ายรายวัน
type dailyPlanRow struct {
	Lot       string
	Machine   string
	Country   string
	Qty       string
	MainLine  string // วันที่จ่าย (เติมลงมาจากหัวกลุ่มแล้ว)
	StandBy   string
	GroupSize int // จำนวนเครื่องที่ไฟล์บอกไว้ในวงเล็บ (0 = ไฟล์ไม่ได้บอก)
}

// dailyPlanHeader = ตำแหน่งคอลัมน์ที่หาเจอในชีต
type dailyPlanHeader struct {
	row      int
	lot      int
	machine  int
	customer int
	qty      int
	mainLine int
	standBy  int
}

// findDailyPlanHeader: หาแถวหัวตารางของชีต Daily plan
//
// ยึดจาก Machine + Main line เป็นหลัก เพราะสองช่องนี้มีแน่นอนทุกไฟล์
// ส่วน LOT NO. / Customer / Q'ty บางไฟล์สะกดต่างกันหรือไม่มี จึงไม่บังคับ
//
// คืน ok=false ถ้าชีตนี้ไม่ใช่ Daily plan
func findDailyPlanHeader(rows [][]string) (dailyPlanHeader, bool) {
	h := dailyPlanHeader{row: -1, lot: -1, machine: -1, customer: -1, qty: -1, mainLine: -1, standBy: -1}

	limit := 40
	if len(rows) < limit {
		limit = len(rows)
	}

	for i := 0; i < limit; i++ {
		// ชีต Specification sheet ก็มีคำว่า Machine No. กับ Main line เหมือนกัน
		// ถ้าไม่กันไว้ มันจะถูกอ่านเป็น Daily plan ไปด้วย แล้วได้ประเทศว่างเปล่า
		// ทั้งคอลัมน์ เพราะชีตนั้นเก็บประเทศไว้คนละที่ (ในบล็อก ไม่ใช่หัวตาราง)
		//
		// ลายเซ็นของ Specification sheet คือ A = LOT NO. และ B = Machine No. ติดกัน
		// ส่วน Daily plan เลขเครื่องอยู่กลางตาราง (คอลัมน์ G) ไม่เคยอยู่ช่อง B
		if normalizeHeader(cellAt(rows, i, 0)) == "lotno" &&
			normalizeHeader(cellAt(rows, i, 1)) == "machineno" {
			continue
		}

		cur := dailyPlanHeader{row: i, lot: -1, machine: -1, customer: -1, qty: -1, mainLine: -1, standBy: -1}
		for c := range rows[i] {
			switch normalizeHeader(cellAt(rows, i, c)) {
			case "lotno", "lot":
				if cur.lot < 0 {
					cur.lot = c
				}
			case "machine", "machineno", "machinesn", "mcno":
				if cur.machine < 0 {
					cur.machine = c
				}
			case "customer", "customername", "country", "countryname":
				if cur.customer < 0 {
					cur.customer = c
				}
			case "qty", "quantity":
				if cur.qty < 0 {
					cur.qty = c
				}
			case "mainline":
				// ชีต 30T มีคอลัมน์ชื่อ "Mainline" ซ้ำอีกช่องท้ายตารางที่เก็บตัวเลข
				// ไม่ใช่วันที่ จึงเอาช่องแรกที่เจอเท่านั้น
				if cur.mainLine < 0 {
					cur.mainLine = c
				}
			case "standbyshipping", "standby":
				if cur.standBy < 0 {
					cur.standBy = c
				}
			}
		}
		if cur.machine >= 0 && cur.mainLine >= 0 {
			return cur, true
		}
	}
	return h, false
}

// dailyPlanSheetRows: แปลงชีต Daily plan หนึ่งชีตเป็นรายการเครื่อง
//
// คืน nil ถ้าชีตนี้ไม่ใช่ Daily plan
func dailyPlanSheetRows(rows [][]string) []dailyPlanRow {
	h, ok := findDailyPlanHeader(rows)
	if !ok {
		return nil
	}

	var out []dailyPlanRow

	// วันที่ล่าสุดที่เจอ — ใช้เติมให้แถวที่ช่องวันที่ว่าง (รูปแบบที่ 2)
	curMain := ""
	curStandBy := ""
	curGroup := 0

	for i := h.row + 1; i < len(rows); i++ {
		machine := strings.TrimSpace(unwrapExcelText(cellAt(rows, i, h.machine)))

		// อัปเดตวันที่ของกลุ่มก่อนเสมอ แม้แถวนี้จะไม่ใช่เครื่อง
		// เพราะไฟล์จริงมีแถวคั่นที่ลงวันที่ไว้แต่ไม่มีเลขเครื่อง
		if h.mainLine >= 0 {
			raw := cellAt(rows, i, h.mainLine)
			if n, isCount := dailyPlanGroupCount(raw); isCount {
				// "(6 MC)" = คำอธิบายจำนวนเครื่องในกลุ่ม ไม่ใช่วันที่ → ข้าม
				curGroup = n
			} else if v := strings.TrimSpace(unwrapExcelText(raw)); v != "" {
				// เจอวันที่ใหม่ = ขึ้นกลุ่มใหม่
				curMain = v
				curGroup = 0
			}
		}
		if h.standBy >= 0 {
			raw := cellAt(rows, i, h.standBy)
			if _, isCount := dailyPlanGroupCount(raw); !isCount {
				if v := strings.TrimSpace(unwrapExcelText(raw)); v != "" {
					curStandBy = v
				}
			}
		}

		if machine == "" || !dailyPlanMachineRe.MatchString(strings.ToUpper(machine)) {
			continue
		}

		out = append(out, dailyPlanRow{
			Lot:       strings.TrimSpace(unwrapExcelText(cellAt(rows, i, h.lot))),
			Machine:   machine,
			Country:   strings.TrimSpace(unwrapExcelText(cellAt(rows, i, h.customer))),
			Qty:       strings.TrimSpace(unwrapExcelText(cellAt(rows, i, h.qty))),
			MainLine:  curMain,
			StandBy:   curStandBy,
			GroupSize: curGroup,
		})
	}
	return out
}

// dailyPlanMachineKey: คีย์ของแถวแผนจ่าย = MC# แบบ normalize
func dailyPlanMachineKey(r dailyPlanRow) string {
	return NormalizeCodeValue(r.Machine)
}

// mergeLatestDailyPlanRow: เครื่องเดียวกันเจอซ้ำ — ช่องที่แถวล่าสุดกรอกมาชนะ
//
// ดูเหตุผลของกติกา "เอาอันล่าสุด" ใน plan_latest_row.go
func mergeLatestDailyPlanRow(prev, latest dailyPlanRow) dailyPlanRow {
	out := prev
	out.Machine = latest.Machine
	if v := strings.TrimSpace(latest.Lot); v != "" {
		out.Lot = v
	}
	if v := strings.TrimSpace(latest.Country); v != "" {
		out.Country = v
	}
	if v := strings.TrimSpace(latest.Qty); v != "" {
		out.Qty = v
	}
	if v := strings.TrimSpace(latest.MainLine); v != "" {
		out.MainLine = v
	}
	if v := strings.TrimSpace(latest.StandBy); v != "" {
		out.StandBy = v
	}
	if latest.GroupSize > 0 {
		out.GroupSize = latest.GroupSize
	}
	return out
}

// keepLatestDailyPlanRows: ยุบแถวของเครื่องเดียวกันให้เหลือแถวเดียว เอาค่าจากแถวล่างสุด
func keepLatestDailyPlanRows(list []dailyPlanRow) []dailyPlanRow {
	return keepLatestByKey(list, dailyPlanMachineKey, mergeLatestDailyPlanRow)
}

// readDailyPlanSheets: อ่านชีต Daily plan ทุกชีตในไฟล์ (20T / 30T / ชีตเดียวก็ได้)
//
// ไฟล์แผนผลิตบางเดือนมีทั้ง 20T และ 30T บางเดือนมีชุดเดียว — รวมเป็นตารางเดียวกัน
// เครื่องเดียวกันที่เขียนซ้ำ (ในชีตเดียวหรือข้ามชีต) ยุบเหลือแถวเดียว
// โดยเอาค่าจากแถวล่างสุดที่เจอ — ไม่ใช่แถวแรก (ดู plan_latest_row.go)
//
// ok=false = ไฟล์นี้ไม่มีชีต Daily plan เลย
func readDailyPlanSheets(fileHeader *multipart.FileHeader) ([]dailyPlanRow, bool, error) {
	sheets, err := readAllUploadedSheets(fileHeader)
	if err != nil {
		return nil, false, err
	}

	var all []dailyPlanRow
	found := false

	for _, sh := range sheets {
		list := dailyPlanSheetRows(sh.rows)
		if list == nil {
			continue
		}
		found = true
		all = append(all, list...)
	}
	if !found {
		return nil, false, nil
	}
	return keepLatestDailyPlanRows(all), true, nil
}

// dailyPlanByMachine: จัดรายการเป็น map ตามเลขเครื่อง (normalize แล้ว)
//
// ใช้เติมประเทศและวันที่จ่ายให้ตาราง Planning PCP ที่มาจาก Specification sheet
func dailyPlanByMachine(list []dailyPlanRow) map[string]dailyPlanRow {
	out := make(map[string]dailyPlanRow, len(list))
	for _, r := range list {
		key := NormalizeCodeValue(r.Machine)
		if key == "" {
			continue
		}
		out[key] = r
	}
	return out
}

// dailyPlanToTable: แปลงเป็นตารางพร้อมหัวคอลัมน์ สำหรับเก็บลง upload_data
func dailyPlanToTable(list []dailyPlanRow) [][]string {
	table := [][]string{append([]string{}, dailyPlanColumns...)}
	for _, r := range list {
		table = append(table, []string{
			r.Lot, r.Machine, r.Country, r.Qty, r.MainLine, r.StandBy,
		})
	}
	return table
}

// ---------------------------------------------------------------------------
// ประกอบตาราง Planning PCP จากไฟล์แผนผลิตรายเดือนรูปแบบใหม่
//
// ไฟล์รูปแบบใหม่ไม่มีชีตชื่อ "Spec sheet_pcp" แล้ว มีแต่
//
//	Specification sheet_xxx  → สเปกรายเครื่อง (บล็อกเครื่องละหลายแถว)
//	Daily plan_xxx           → แผนจ่ายรายวัน (ตารางแนวนอน)
//
// สองชีตนี้เล่าคนละด้านของเครื่องเดียวกัน จึงรวมเป็นตารางเดียวโดยใช้ MC# เป็นกุญแจ
//
// ชีต Specification sheet มีวันที่ Main line ครบทุกเครื่องอยู่แล้ว ส่วนชีต Daily plan
// เป็นตัวยืนยันและเป็นที่มาของชื่อประเทศ (คอลัมน์ Customer) ถ้าไฟล์ไหนมีแค่ชีตเดียว
// ก็ยังใช้ได้ — เติมเท่าที่มี
// ---------------------------------------------------------------------------

// pcpExtraColumns: คอลัมน์ที่เติมเข้าตาราง Planning PCP เพื่อให้ WH ใช้งานได้
//
// ต้องมีสามช่องนี้ ไม่งั้นหน้า WH เลือกเครื่องตามวันจ่ายไม่ได้
var pcpExtraColumns = []string{"Country Name", "Main line", "Stand by shipping"}

// specBlockToPCPRow: แปลงบล็อกเครื่องจาก Specification sheet เป็นแถวของตาราง Planning PCP
//
// ชื่อคอลัมน์สองฝั่งไม่ตรงกันเป๊ะ ต้องแมปเอง:
//
//	Spec Code → Product Spec   (ชื่อเดียวกันคนละชีตเรียกคนละอย่าง)
//	Remark    → Note1
//
// ช่องที่ Specification sheet ไม่มี (Domestic/Exp, Selling comp, KCM Order)
// ปล่อยว่าง ไม่เดาแทน
func specBlockToPCPRow(b map[string]string, dp map[string]dailyPlanRow) []string {
	get := func(k string) string { return strings.TrimSpace(b[k]) }

	country := get(specFieldCustomer)
	mainLine := get("Main line")
	standBy := get("Stand by shipping")

	// ชีต Daily plan ชนะเมื่อ Specification sheet ไม่ได้กรอกช่องนั้นไว้
	if r, ok := dp[NormalizeCodeValue(get("Machine"))]; ok {
		if country == "" {
			country = r.Country
		}
		if mainLine == "" {
			mainLine = r.MainLine
		}
		if standBy == "" {
			standBy = r.StandBy
		}
	}

	return []string{
		get("LOT NO."),
		get("Machine"),
		get(specFieldSpecCode), // → Product Spec
		"",                     // Domestic/Exp — ไม่มีในไฟล์รูปแบบนี้
		"",                     // Selling comp
		"",                     // KCM Order
		get("Brand"),
		get("Destination"),
		get("Base machine spec."),
		get("Boom"),
		get("Arm"),
		get("Shoe"),
		get("Counter weight"),
		get("Front ATT"),
		get("Cab"),
		get("Cab guard"),
		get("IT device"),
		get("Remark"), // → Note1
		country,
		mainLine,
		standBy,
	}
}

// readPlanningPCPWorkbook: อ่านไฟล์แผนผลิตรายเดือนเป็นตาราง Planning PCP
//
// ok=false = ไฟล์นี้ไม่มีชีต Specification sheet (ให้ผู้เรียกไปลองวิธีอ่านแบบเดิม)
func readPlanningPCPWorkbook(fileHeader *multipart.FileHeader) ([][]string, bool, error) {
	sheets, err := readAllUploadedSheets(fileHeader)
	if err != nil {
		return nil, false, err
	}

	// แผนจ่ายรายวันก่อน เอาไว้เติมช่องที่ Specification sheet ไม่ได้กรอก
	// เครื่องซ้ำเอาแถวล่างสุดของไฟล์ (ดู plan_latest_row.go)
	var plan []dailyPlanRow
	for _, sh := range sheets {
		plan = append(plan, dailyPlanSheetRows(sh.rows)...)
	}
	dp := dailyPlanByMachine(keepLatestDailyPlanRows(plan))

	var blocks []map[string]string
	found := false
	for _, sh := range sheets {
		list := specSheetBlocks(sh.rows)
		if list == nil {
			continue
		}
		found = true
		blocks = append(blocks, list...)
	}
	if !found {
		return nil, false, nil
	}

	header := append(append([]string{}, specPCPColumns...), pcpExtraColumns...)
	table := [][]string{header}
	for _, b := range keepLatestMachineBlocks(blocks) {
		table = append(table, specBlockToPCPRow(b, dp))
	}
	return table, true, nil
}

// ---------------------------------------------------------------------------
// เขียนชีต Daily plan ลงตารางของตัวเอง
//
// เรียกต่อท้ายการบันทึก Planning PCP — ผู้ใช้อัปไฟล์ครั้งเดียว ระบบเขียนสองตาราง
//
// ใช้วิธีลบทิ้งแล้วเขียนใหม่ทั้งชุด ต่างจากตารางอื่นที่ค่อย ๆ เทียบทีละแถว
// เพราะแผนจ่ายไม่มีการสแกนผูกไว้ ไม่มีอะไรให้ล็อก และแผนรอบใหม่ย่อมแทนรอบเก่าทั้งก้อน
// (ชื่อไฟล์เขียน Draft.00 ไว้ แปลว่าทั้งแผนถูกแก้พร้อมกันเป็นรอบ ๆ)
//
// ไม่คืน error ขึ้นไปทำให้การอัปโหลดล้ม — สเปกเครื่องบันทึกไปแล้วและใช้งานได้
// ถ้าชีต Daily plan มีปัญหา ควรบอกเป็นคำเตือน ไม่ใช่ย้อนงานที่สำเร็จแล้วทิ้ง
// ---------------------------------------------------------------------------

func saveDailyPlanDataset(c *gin.Context, dataset, fileName string, userID uint, now time.Time) (int, error) {
	if dataset != models.DatasetSpecSheet {
		return 0, nil
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return 0, nil
	}

	list, ok, err := readDailyPlanSheets(fileHeader)
	if err != nil {
		return 0, err
	}
	if !ok || len(list) == 0 {
		// ไฟล์ที่มีแต่ Specification sheet ก็ใช้ได้ ไม่ใช่ความผิดพลาด
		// แต่ต้องไม่ล้างแผนเดิมทิ้ง ไม่งั้นอัปไฟล์ที่ขาดชีตไปหนึ่งครั้ง
		// แผนจ่ายทั้งเดือนจะหายไปเฉย ๆ
		return 0, nil
	}

	rows := make([]models.UploadDataRow, 0, len(list))
	for i, r := range list {
		data := map[string]string{
			"LOT NO.":           r.Lot,
			"Machine":           r.Machine,
			"Country Name":      r.Country,
			"Q'ty":              r.Qty,
			"Main line":         r.MainLine,
			"Stand by shipping": r.StandBy,
		}
		jsonBytes, _ := json.Marshal(data)
		row := models.UploadDataRow{
			Dataset:    models.DatasetPCPDailyPlan,
			DataJSON:   string(jsonBytes),
			FileName:   fileName,
			UploadDate: now,
			UserID:     userID,
			SortOrder:  int64(i),
		}
		fillUploadDataKeys(&row, models.DatasetPCPDailyPlan, data)
		rows = append(rows, row)
	}

	// แถวเก่าที่ต้องถูกแทนที่ — ดูสองเงื่อนไข ไม่ใช่เงื่อนไขเดียว
	//
	//  1) แถวที่มาจากไฟล์ชื่อเดียวกัน → แผนรอบใหม่แทนรอบเก่าทั้งก้อน
	//     (รวมแถวที่ถูกลบออกจากไฟล์ไปแล้ว ซึ่งต้องหายไปจากระบบด้วย)
	//
	//  2) แถวของ "เครื่องเดียวกัน" ที่ไฟล์รอบนี้มี → แทนที่แม้มาจากไฟล์ชื่ออื่น
	//     เพราะชื่อไฟล์แผนเปลี่ยนทุกรอบ (Draft.00 → Draft.01) ถ้าดูแต่ชื่อไฟล์
	//     เครื่องเดิมจะมีสองแถวจากสองไฟล์ แล้วหน้า WH หยิบแถวเก่าไปแสดง
	//     ประเทศและวันจ่ายของรอบก่อน — ซึ่งคือ Machine No. เดิมต้อง UPDATE ไม่ใช่เพิ่มแถว
	//
	// ไม่ลบทั้งตาราง เพราะเครื่องของไฟล์อื่นที่ยังไม่ถูกแทนที่ต้องอยู่ครบ
	// (ตารางนี้ไม่มีการสแกนผูกไว้ จึงลบแล้วเขียนใหม่ได้ ไม่มีอะไรต้องล็อก)
	newMachines := make(map[string]bool, len(list))
	for _, r := range list {
		if k := NormalizeCodeValue(r.Machine); k != "" {
			newMachines[k] = true
		}
	}

	var oldRows []models.UploadDataRow
	config.DB.Select("id", "machine_no", "data_json", "file_name").
		Where("dataset = ?", models.DatasetPCPDailyPlan).Find(&oldRows)

	var staleIDs []uint
	for _, r := range oldRows {
		if sameUploadFile(r.FileName, fileName) {
			staleIDs = append(staleIDs, r.ID)
			continue
		}
		mc := r.MachineNo
		if mc == "" {
			m := map[string]string{}
			if json.Unmarshal([]byte(r.DataJSON), &m) == nil {
				mc = pickField(m, "Machine", extraColumnPrefix+"Machine")
			}
		}
		if k := NormalizeCodeValue(mc); k != "" && newMachines[k] {
			staleIDs = append(staleIDs, r.ID)
		}
	}

	err = config.DB.Transaction(func(tx *gorm.DB) error {
		for _, part := range chunkSlice(staleIDs, dbInsertBatch) {
			if err := tx.Where("id IN ?", part).Delete(&models.UploadDataRow{}).Error; err != nil {
				return err
			}
		}
		return tx.CreateInBatches(&rows, 1000).Error
	})
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

// dailyPlanIndex: แผนจ่ายรายวัน key = เลขเครื่องแบบ normalize
//
// เครื่องเดียวกันเจอหลายแถว (แถวซ้ำที่ค้างจากไฟล์รอบก่อน) เอา "แถวล่าสุด" เสมอ
// loadUploadRows เรียงตาม id จากน้อยไปมาก แถวท้ายคือแถวที่อัปเข้ามาหลังสุด
//
// ของเดิมเอาแถวแรกที่เจอ = แถวเก่าสุด ทำให้หน้า WH เห็นประเทศและวันจ่ายของรอบก่อน
// ทั้งที่อัปแผนใหม่ไปแล้ว
func dailyPlanIndex() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, row := range loadUploadRows(models.DatasetPCPDailyPlan) {
		key := NormalizeCodeValue(pickField(row, "Machine", extraColumnPrefix+"Machine"))
		if key == "" {
			continue
		}
		out[key] = row
	}
	return out
}
