package controllers

import (
	"errors"
	"mime/multipart"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// ไฟล์ Renewal (ต่ออายุ) — อัปโหลดแยกจาก Import / Export
//
// รูปแบบที่ระบบอ่านได้ 2 แบบ:
//
//	แบบที่ 1 — ตารางต่ออายุโดยตรง (แนะนำ)
//	  License Type | Old License No. | New License No. | Renewal Date | New Expiry Date
//	  Export       | EXP-001         | EXP-002         | 20/09/2026   | 20/09/2027
//
//	แบบที่ 2 — ชีต "ต่ออายุ" เดิมของ KCMSA
//	  NO. | IT CONTROLLER MODEL | IMPORT LICENSE | ... | EXPORT LICENSE | DATE | EXPIRE DATE | ...
//	  ใบนำออกหลายใบที่อยู่ใต้ใบนำเข้าเดียวกัน = การต่ออายุต่อเนื่องกัน
//	  ระบบจะจับคู่ใบที่อยู่ติดกันตามลำดับวันที่ให้เอง (ใบก่อนหน้า → ใบถัดไป)
//
// ทั้งสองแบบให้ผลลัพธ์เป็นแถวใน license_renewal_history เหมือนกัน
// การอัปโหลดเป็นแบบ "เพิ่ม/อัปเดต" ไม่ล้างของเดิม และไม่แตะข้อมูล License เดิม
// ---------------------------------------------------------------------------

var errNoRenewalHistorySheet = errors.New("รูปแบบข้อมูลที่อัพโหลดไม่ถูกต้อง")

// renewalHistoryHeaders: หัวคอลัมน์ (normalize แล้ว) → ชื่อช่องภายใน
var renewalHistoryHeaders = map[string]string{
	"licensetype": "type",
	"type":        "type",
	"ประเภทใบอนุญาต":  "type",
	"ประเภท":          "type",
	"oldlicenseno":    "oldNo",
	"oldlicense":      "oldNo",
	"previouslicense": "oldNo",
	"เลขใบเดิม":       "oldNo",
	"เลขใบอนุญาตเดิม": "oldNo",
	"newlicenseno":    "newNo",
	"newlicense":      "newNo",
	"currentlicense":  "newNo",
	"เลขใบใหม่":       "newNo",
	"เลขใบอนุญาตใหม่": "newNo",
	"renewalno":       "renewalNo",
	"renewalround":    "renewalNo",
	"ครั้งที่":        "renewalNo",
	"renewaldate":     "renewalDate",
	"dateofrenewal":   "renewalDate",
	"วันที่ต่ออายุ":   "renewalDate",
	"oldexpirydate":   "oldExpire",
	"oldexpiredate":   "oldExpire",
	"วันหมดอายุเดิม":  "oldExpire",
	"newexpirydate":   "newExpire",
	"newexpiredate":   "newExpire",
	"expirydate":      "newExpire",
	"วันหมดอายุใหม่":  "newExpire",
	"remark":   "remark",
	"note":     "remark",
	"หมายเหตุ": "remark",
}

// findRenewalHistoryHeader: หาแถวหัวตารางของไฟล์ Renewal แบบที่ 1
func findRenewalHistoryHeader(rows [][]string) (int, map[int]string) {
	limit := 20
	if len(rows) < limit {
		limit = len(rows)
	}
	// หัวคอลัมน์ที่จับคู่ใน Format Settings (scope renewal_license) ใช้ได้ด้วย
	aliasRev := map[string]string{}
	if config.DB != nil {
		aliasRev = loadColumnAliasReverse("renewal_license")
	}
	resolve := func(cell string) (string, bool) {
		n := normalizeHeader(cell)
		if k, ok := renewalHistoryHeaders[n]; ok {
			return k, true
		}
		if t, ok := aliasRev[n]; ok {
			if k, ok := renewalHistoryHeaders[t]; ok {
				return k, true
			}
		}
		return "", false
	}
	for i := 0; i < limit; i++ {
		layout := map[int]string{}
		taken := map[string]bool{}
		for c := 0; c < len(rows[i]); c++ {
			key, ok := resolve(cellAt(rows, i, c))
			if !ok || taken[key] {
				continue
			}
			taken[key] = true
			layout[c] = key
		}
		// ต้องมีทั้งเลขใบเดิมและเลขใบใหม่จึงจะใช่ตารางนี้
		if taken["oldNo"] && taken["newNo"] {
			return i, layout
		}
	}
	return -1, nil
}

// RenewalUploadProblem = ปัญหา 1 ข้อที่เจอตอนตรวจไฟล์ (แสดงให้ผู้ใช้อ่าน)
type RenewalUploadProblem struct {
	Row     int    `json:"row"`
	Sheet   string `json:"sheet"`
	Message string `json:"message"`
}

// renewalHistoryDraft = แถวที่อ่านมาจากไฟล์ ก่อนตรวจความถูกต้อง
type renewalHistoryDraft struct {
	models.LicenseRenewalHistory
	RowNo     int
	SheetName string
	RawType   string
}

// readRenewalHistoryDirect: อ่านไฟล์ Renewal แบบที่ 1 (มีคอลัมน์ Old/New License No.)
func readRenewalHistoryDirect(sheets []namedSheetRows) ([]renewalHistoryDraft, []RenewalUploadProblem, bool) {
	var out []renewalHistoryDraft
	var problems []RenewalUploadProblem
	found := false
	var order int64

	for _, sh := range sheets {
		headerIdx, layout := findRenewalHistoryHeader(sh.rows)
		if headerIdx < 0 {
			continue
		}
		found = true
		hasTypeCol := false
		for _, k := range layout {
			if k == "type" {
				hasTypeCol = true
			}
		}

		for r := headerIdx + 1; r < len(sh.rows); r++ {
			get := func(key string) string {
				for c, k := range layout {
					if k == key {
						return strings.TrimSpace(unwrapExcelText(cellAt(sh.rows, r, c)))
					}
				}
				return ""
			}

			oldNo := strings.ToUpper(get("oldNo"))
			newNo := strings.ToUpper(get("newNo"))
			if oldNo == "" && newNo == "" {
				continue
			}

			rowNo := r + 1
			if !hasTypeCol {
				problems = append(problems, RenewalUploadProblem{
					Row: headerIdx + 1, Sheet: sh.name,
					Message: "ไฟล์นี้ไม่มีคอลัมน์ License Type — ระบบไม่ทราบว่าเป็น Import หรือ Export จึงข้ามทั้งชีต",
				})
				break
			}

			rawType := get("type")
			order++
			out = append(out, renewalHistoryDraft{
				LicenseRenewalHistory: models.LicenseRenewalHistory{
					LicenseType:   models.NormalizeLicenseType(rawType),
					OldLicenseNo:  oldNo,
					NewLicenseNo:  newNo,
					RenewalNo:     renewalInt(get("renewalNo")),
					RenewalDate:   parseLicenseDate(get("renewalDate")),
					OldExpireDate: parseLicenseDate(get("oldExpire")),
					NewExpireDate: parseLicenseDate(get("newExpire")),
					Remark:        get("remark"),
					SortOrder:     order,
				},
				RowNo:     rowNo,
				SheetName: sh.name,
				RawType:   rawType,
			})
		}
	}
	return out, problems, found
}

// readRenewalHistoryFromLedger: อ่านชีต "ต่ออายุ" เดิม แล้วแปลงเป็นคู่ "ใบเดิม → ใบใหม่"
//
// ใบนำออกที่อยู่ใต้ใบนำเข้าเดียวกัน = ชุดเดียวกัน เรียงตามวันที่ออกใบ
// ใบที่ i จึงเป็นใบเดิมของใบที่ i+1 — ไม่ได้เดา เพราะไฟล์จัดกลุ่มไว้ให้แล้ว
// ledgerGroupKey: คีย์ที่ใช้จัดกลุ่มแถวในชีต "ต่ออายุ"
//
// คอลัมน์ NO. (เช่น "Completed 01", "117") คือรหัสงาน 1 ชุด
// แต่ภายในงานชุดเดียวกันอาจมีใบนำเข้าได้มากกว่า 1 ใบ และใบนำเข้าแต่ละใบ
// มีโควต้าของตัวเองแยกกัน จึงเป็นคนละใบกัน ไม่ใช่การต่ออายุของกันและกัน
//
//	Completed 01 — ใบนำเข้าใบเดียว ใบนำออกเปลี่ยนทุกเดือน = ต่ออายุ 1 โซ่
//	Completed 02 — ใบนำเข้า 3 ใบ โควต้า 55 / 10 / 55 = 3 ใบแยกกัน จบพร้อมกัน
//
// คีย์จึงต้องเป็น NO. + เลขใบนำเข้า เพื่อไม่ให้ใบคนละใบถูกยุบเป็นโซ่เดียวกัน
func ledgerGroupKey(r models.LicenseRenewal) string {
	imp := strings.ToUpper(strings.TrimSpace(r.ImportLicenseNo))
	if g := strings.TrimSpace(r.GroupNo); g != "" {
		return "G|" + strings.ToUpper(g) + "|" + imp
	}
	return "I|" + imp
}

// ---------------------------------------------------------------------------
// สายประเทศ (branch) ภายในโซ่เดียวกัน
//
// ใบนำเข้าใบเดียวแบ่งโควต้าออกหลายประเทศได้ แล้วแต่ละประเทศเดินต่ออายุของตัวเอง
// คนละรอบ คนละวันที่ เช่นกลุ่ม 109 ใบนำเข้า E05036902379
//
//	INDONESIA  E05046900435 (05/08) → E05046900456 (11/08)        REMAIN 0
//	MALAYSIA   E05046900432 (05/08) → ... → E05046900754 (10/09)  REMAIN 2
//
// ในไฟล์จริงผู้ใช้เรียงเป็นบล็อกตามประเทศ ไม่ได้เรียงสลับตามวันที่
// ledgerSteps จึงแยกสายให้ไม่ได้ เพราะมันดูแค่ "แถวติดกัน + วันที่ตรงกัน"
// ถ้าไล่โซ่รวดเดียวทั้งก้อน ระบบจะเข้าใจว่าใบของ MALAYSIA ต่ออายุมาจากใบของ
// INDONESIA (456 → 432) แล้วเหลือขั้นสุดท้ายแค่สายเดียว อีกสายหายไปทั้งสาย
// ---------------------------------------------------------------------------

// ledgerCountrySep: ตัวคั่นชื่อประเทศในช่อง COUNTRY ที่พบในไฟล์จริง
var ledgerCountrySep = regexp.MustCompile(`[,/]|\s+และ\s+|\s+and\s+`)

// ledgerCountryNames: แยกช่อง COUNTRY เป็นรายชื่อประเทศ เรียงให้คงที่เพื่อใช้เทียบกันได้
//
//	"MALAYSIA "            → ["MALAYSIA"]              (ไฟล์จริงมีเว้นวรรคท้ายติดมา)
//	"INDONESIA , MALAYSIA" → ["INDONESIA","MALAYSIA"]
func ledgerCountryNames(country string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range ledgerCountrySep.Split(strings.ToUpper(country), -1) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// ledgerCountryCovers: ชุด a ครอบคลุมทุกชื่อในชุด b หรือไม่
func ledgerCountryCovers(a, b []string) bool {
	if len(b) == 0 {
		return false
	}
	in := make(map[string]bool, len(a))
	for _, s := range a {
		in[s] = true
	}
	for _, s := range b {
		if !in[s] {
			return false
		}
	}
	return true
}

// ledgerBranchIndexes: แยกโซ่ 1 โซ่ออกเป็นสายตามประเทศ
// คืนเป็น "ตำแหน่งของแถวใน list" เพื่อให้ผู้เรียกแมปกลับไปยัง index ของตัวเองได้
//
// กติกา:
//   - สายจริง = ชุดประเทศที่ไม่ได้ครอบชุดอื่นไว้
//     "INDONESIA , MALAYSIA" ครอบ "INDONESIA" และ "MALAYSIA" จึงไม่ใช่สาย
//     แต่เป็นช่วงที่ยังไม่แตกสาย = ต้นทางร่วมของทุกสายที่แตกออกมาทีหลัง
//   - แถวต้นทางร่วม และแถวที่ไม่ได้กรอกประเทศ ใส่เข้าไปในทุกสายที่เกี่ยวข้อง
//   - โซ่ที่ทุกแถวเป็นชุดประเทศเดียวกัน คืนสายเดียว = พฤติกรรมเดิมทุกอย่าง
func ledgerBranchIndexes(list []models.LicenseRenewal) [][]int {
	all := make([]int, len(list))
	for i := range list {
		all[i] = i
	}

	var keys []string
	sets := map[string][]string{}
	for _, r := range list {
		names := ledgerCountryNames(r.Country)
		if len(names) == 0 {
			continue
		}
		k := strings.Join(names, "|")
		if _, ok := sets[k]; !ok {
			sets[k] = names
			keys = append(keys, k)
		}
	}
	if len(keys) <= 1 {
		return [][]int{all}
	}

	var leaves []string
	for _, k := range keys {
		covers := false
		for _, o := range keys {
			if o != k && ledgerCountryCovers(sets[k], sets[o]) {
				covers = true
				break
			}
		}
		if !covers {
			leaves = append(leaves, k)
		}
	}
	if len(leaves) <= 1 {
		return [][]int{all}
	}

	out := make([][]int, 0, len(leaves))
	for _, lf := range leaves {
		var idx []int
		for i, r := range list {
			names := ledgerCountryNames(r.Country)
			// ไม่กรอกประเทศ = แถวร่วม เข้าได้ทุกสาย (ไม่ทิ้งแถวไหนหาย)
			if len(names) == 0 || ledgerCountryCovers(names, sets[lf]) {
				idx = append(idx, i)
			}
		}
		if len(idx) > 0 {
			out = append(out, idx)
		}
	}
	if len(out) == 0 {
		return [][]int{all}
	}
	return out
}

// ledgerPick: หยิบแถวตามตำแหน่งที่ ledgerBranchIndexes คืนมา
func ledgerPick(list []models.LicenseRenewal, idx []int) []models.LicenseRenewal {
	out := make([]models.LicenseRenewal, len(idx))
	for n, i := range idx {
		out[n] = list[i]
	}
	return out
}

// ledgerSteps: แบ่งโซ่ออกเป็น "ขั้นการต่ออายุ" โดยถือวันที่ออกใบเป็นตัวแบ่ง
//
// ใบนำเข้าใบเดียวแตกเป็นใบนำออกหลายใบพร้อมกันได้ เพราะแบ่งโควต้าตามประเทศ
// เช่น E05036903112 (TOTAL 60) ออกวันเดียวกันสองใบ
//
//	INDONESIA  E05046900717  STOCK 50  REMAIN 50
//	MALAYSIA   E05046900718  STOCK 10  REMAIN 10
//
// สองแถวนี้เป็นพี่น้องกัน ไม่ใช่ใบเก่ากับใบใหม่
// ถ้าไล่โซ่ทีละแถวแบบเดิม ระบบจะเข้าใจว่า 718 ต่ออายุมาจาก 717
// แล้วนับเป็นต่ออายุ 1 ครั้งทั้งที่ยังไม่เคยต่อเลย
//
// แถวที่ออกวันเดียวกันจึงถูกยุบเป็นขั้นเดียวกัน การต่ออายุนับเป็นขั้น ไม่ใช่นับเป็นแถว
// แถวที่ไม่มีวันที่ (เช่นแถวจดบันทึก) แยกเป็นขั้นของตัวเอง ไม่ไปรวมกับใคร
//
// แต่ "ออกวันเดียวกัน" อย่างเดียวไม่พอ ต้องเป็นคนละประเทศด้วย
//
// ในไฟล์จริงมีใบที่ออกวันเดียวกัน ประเทศเดียวกัน แต่เป็นคนละรอบจริง ๆ เช่น
//
//	Completed 19  050167005216 (29/08)  →  050167005273 (29/08)
//	80            050169000801 (30/01)  →  050169001192 (30/01)
//
// ถ้ายุบสองแถวนี้เป็นขั้นเดียวกัน ledgerStepMatch จะเลือกใบเดียวไปต่อโซ่
// อีกใบจะไม่ได้เป็น "ใบเดิม" ของใครเลย แล้วหลุดออกไปเป็นแถวของตัวเองในหน้า Overview
// ค้างเป็น "หมดอายุไปแล้ว" ตลอดไป ทั้งที่งานชุดนั้นปิดไปนานแล้ว
//
// การแบ่งโควต้าตามประเทศถูกแยกไปเป็นสาย (ledgerBranchIndexes) ตั้งแต่ต้นแล้ว
// ภายในสายเดียวกันประเทศย่อมเหมือนกัน เงื่อนไขนี้จึงแทบไม่ทำงาน
// และคงไว้เผื่อกรณีที่เรียก ledgerSteps กับรายการที่ยังไม่ได้แยกสาย
func ledgerSteps(list []models.LicenseRenewal) [][]models.LicenseRenewal {
	var steps [][]models.LicenseRenewal
	for _, r := range list {
		n := len(steps)
		if n > 0 && r.IssueDate != nil {
			last := steps[n-1]
			sibling := last[len(last)-1]
			if prev := sibling.IssueDate; prev != nil && prev.Equal(*r.IssueDate) &&
				ledgerCountryKey(sibling.Country) != ledgerCountryKey(r.Country) {
				steps[n-1] = append(last, r)
				continue
			}
		}
		steps = append(steps, []models.LicenseRenewal{r})
	}
	return steps
}

// ledgerCountryKey: ชุดประเทศของแถวในรูปแบบที่เทียบกันได้
func ledgerCountryKey(country string) string {
	return strings.Join(ledgerCountryNames(country), "|")
}

// ledgerStepMatch: หาใบก่อนหน้าของแถวนี้ จากขั้นก่อนหน้า
//
// ขั้นก่อนหน้ามีใบเดียว → ทุกใบในขั้นนี้ต่อยอดมาจากใบนั้น (กรณีใบรวมแล้วแตกตามประเทศ)
// ขั้นก่อนหน้ามีหลายใบ → จับคู่ด้วยประเทศ ถ้าไม่ตรงค่อยใช้ลำดับในขั้น
func ledgerStepMatch(prevStep []models.LicenseRenewal, cur models.LicenseRenewal, idx int) models.LicenseRenewal {
	if len(prevStep) == 1 {
		return prevStep[0]
	}
	want := NormalizeCodeValue(cur.Country)
	for _, p := range prevStep {
		if want != "" && NormalizeCodeValue(p.Country) == want {
			return p
		}
	}
	if idx < len(prevStep) {
		return prevStep[idx]
	}
	return prevStep[len(prevStep)-1]
}

// groupLedgerRows: จัดกลุ่มแถวตามคอลัมน์ NO. โดยคงลำดับเดิมในไฟล์ไว้ทั้งลำดับกลุ่มและลำดับแถว
func groupLedgerRows(rows []models.LicenseRenewal) ([]string, map[string][]models.LicenseRenewal) {
	byGroup := map[string][]models.LicenseRenewal{}
	order := []string{}
	for _, r := range rows {
		key := ledgerGroupKey(r)
		if key == "G|" || key == "I|" {
			continue
		}
		if _, ok := byGroup[key]; !ok {
			order = append(order, key)
		}
		byGroup[key] = append(byGroup[key], r)
	}
	return order, byGroup
}

// readRenewalHistoryFromLedger: แปลงชีต "ต่ออายุ" เป็นคู่ "ใบเดิม → ใบใหม่"
//
// จับคู่จากแถวที่อยู่ติดกันในกลุ่มเดียวกันเท่านั้น ไม่เดาเพิ่มเอง
// (เลขใบนำเข้าก็เปลี่ยนได้ระหว่างทาง จึงต้องไล่โซ่ทั้งสองฝั่ง)
func readRenewalHistoryFromLedger(rows []models.LicenseRenewal) []renewalHistoryDraft {
	order, byGroup := groupLedgerRows(rows)

	var out []renewalHistoryDraft
	var seq int64

	// ช่วงที่ยังไม่แตกสายประเทศเป็นต้นทางร่วมของทุกสาย จึงถูกไล่ซ้ำสายละรอบ
	// คู่ "ใบเดิม → ใบใหม่" เดียวกันต้องบันทึกครั้งเดียว
	seenPair := map[string]bool{}

	add := func(licenseType, oldNo, newNo string, round int, cur, prev models.LicenseRenewal, rowNo int) {
		oldNo = strings.ToUpper(strings.TrimSpace(oldNo))
		newNo = strings.ToUpper(strings.TrimSpace(newNo))
		if oldNo == "" || newNo == "" || oldNo == newNo {
			return
		}
		pair := licenseType + "|" + oldNo + "|" + newNo
		if seenPair[pair] {
			return
		}
		seenPair[pair] = true
		seq++
		out = append(out, renewalHistoryDraft{
			LicenseRenewalHistory: models.LicenseRenewalHistory{
				LicenseType:   licenseType,
				OldLicenseNo:  oldNo,
				NewLicenseNo:  newNo,
				RenewalNo:     round,
				RenewalDate:   cur.IssueDate,
				OldExpireDate: prev.ExpireDate,
				NewExpireDate: cur.ExpireDate,
				GroupNo:       strings.TrimSpace(cur.GroupNo),
				Remark:        "จากชีตต่ออายุ — กลุ่ม " + strings.TrimSpace(cur.GroupNo),
				SortOrder:     seq,
			},
			RowNo:     rowNo,
			SheetName: "ต่ออายุ",
			RawType:   licenseType,
		})
	}

	// ต่อโซ่จากเฉพาะแถวที่มีเลขใบจริงเท่านั้น
	//
	// บางแถวในไฟล์ไม่ได้กรอกเลขใบ แต่ใช้จดบันทึก เช่น "วันที่ส่ง E-mail : Thu 19/12/2024 ..."
	// ถ้าจับคู่แถวติดกันแบบตรง ๆ แถวจดบันทึกจะทำให้โซ่ขาดกลางคัน
	// กลุ่มเดียวจึงถูกแยกเป็นสองโซ่ และจำนวนครั้งที่ต่ออายุน้อยกว่าความจริง
	// (เช่น Completed 28 มี 16 แถว แต่นับได้แค่ 3 ครั้ง)
	//
	// จึงคัดเฉพาะแถวที่มีเลขใบออกมาก่อน แล้วค่อยจับคู่แถวที่อยู่ติดกันในชุดที่คัดแล้ว
	withNo := func(list []models.LicenseRenewal, pick func(models.LicenseRenewal) string) []models.LicenseRenewal {
		out := make([]models.LicenseRenewal, 0, len(list))
		for _, r := range list {
			if strings.TrimSpace(pick(r)) != "" {
				out = append(out, r)
			}
		}
		return out
	}

	// จับคู่ทีละขั้น ไม่ใช่ทีละแถว — ใบที่ออกวันเดียวกันเป็นพี่น้องกัน ไม่ใช่ใบเก่ากับใบใหม่
	// และจับคู่ภายในสายประเทศเดียวกันเท่านั้น ใบของคนละประเทศไม่ใช่ใบเก่า/ใบใหม่ของกันและกัน
	link := func(licenseType string, list []models.LicenseRenewal, pick func(models.LicenseRenewal) string) {
		for _, bidx := range ledgerBranchIndexes(list) {
			steps := ledgerSteps(withNo(ledgerPick(list, bidx), pick))
			for si := 1; si < len(steps); si++ {
				for idx, cur := range steps[si] {
					prev := ledgerStepMatch(steps[si-1], cur, idx)
					add(licenseType, pick(prev), pick(cur), si, cur, prev, si+1)
				}
			}
		}
	}

	for _, key := range order {
		list := byGroup[key]
		link(models.LicenseTypeExport, list, func(r models.LicenseRenewal) string { return r.ExportLicenseNo })
		link(models.LicenseTypeImport, list, func(r models.LicenseRenewal) string { return r.ImportLicenseNo })
	}
	return out
}

// ---------------------------------------------------------------------------
// ตรวจความถูกต้องก่อนบันทึก
// ---------------------------------------------------------------------------

// knownLicenseNos: เลขใบอนุญาตที่มีอยู่ในระบบแล้ว แยกตามประเภท
//
//	IMPORT — จากบัญชีใบอนุญาตนำเข้า และคอลัมน์ IMPORT LICENSE ในชีตต่ออายุ
//	EXPORT — จากบัญชีใบอนุญาตนำออก และคอลัมน์ EXPORT LICENSE ในชีตต่ออายุ
//	         บวกเลขใบใหม่ที่เคยบันทึกเป็นประวัติการต่ออายุไว้แล้ว
func knownLicenseNos() map[string]map[string]bool {
	out := map[string]map[string]bool{
		models.LicenseTypeImport: {},
		models.LicenseTypeExport: {},
	}
	add := func(t, no string) {
		no = strings.ToUpper(strings.TrimSpace(no))
		if no == "" {
			return
		}
		out[t][NormalizeCodeValue(no)] = true
	}

	var importNos []string
	config.DB.Model(&models.LicenseItem{}).Pluck("license_no", &importNos)
	for _, v := range importNos {
		add(models.LicenseTypeImport, v)
	}

	var exportNos []string
	config.DB.Model(&models.LicenseItem{}).Pluck("export_license_no", &exportNos)
	for _, v := range exportNos {
		add(models.LicenseTypeExport, v)
	}

	var ledger []models.LicenseRenewal
	config.DB.Select("import_license_no", "export_license_no").Find(&ledger)
	for _, r := range ledger {
		add(models.LicenseTypeImport, r.ImportLicenseNo)
		add(models.LicenseTypeExport, r.ExportLicenseNo)
	}

	var history []models.LicenseRenewalHistory
	config.DB.Select("license_type", "old_license_no", "new_license_no").Find(&history)
	for _, h := range history {
		add(h.LicenseType, h.OldLicenseNo)
		add(h.LicenseType, h.NewLicenseNo)
	}

	return out
}

// validateRenewalDrafts: ตรวจทุกแถวก่อนบันทึก — คืนเฉพาะแถวที่ผ่าน พร้อมรายการปัญหา
func validateRenewalDrafts(drafts []renewalHistoryDraft, strictOldExists bool) ([]renewalHistoryDraft, []RenewalUploadProblem) {
	known := knownLicenseNos()

	// เลขใบใหม่ที่เคยถูกใช้เป็น "ใบใหม่" ไปแล้ว — ห้ามซ้ำ เพราะ 1 ใบเกิดจากการต่ออายุได้ครั้งเดียว
	usedNew := map[string]string{} // type|newNo → oldNo
	var existing []models.LicenseRenewalHistory
	config.DB.Select("license_type", "old_license_no", "new_license_no").Find(&existing)
	for _, h := range existing {
		usedNew[h.LicenseType+"|"+NormalizeCodeValue(h.NewLicenseNo)] = h.OldLicenseNo
	}
	// คู่ที่มีอยู่แล้ว (type|old|new) = อัปโหลดซ้ำ ไม่ถือเป็น error แต่ไม่สร้างซ้ำ
	existingPair := map[string]bool{}
	for _, h := range existing {
		existingPair[h.LicenseType+"|"+NormalizeCodeValue(h.OldLicenseNo)+"|"+NormalizeCodeValue(h.NewLicenseNo)] = true
	}

	// เลขใบที่กำลังจะถูกเพิ่มในรอบนี้ — ใช้ต่อโซ่ภายในไฟล์เดียวกันได้
	pending := map[string]bool{}
	seenInFile := map[string]bool{}

	out := make([]renewalHistoryDraft, 0, len(drafts))
	var problems []RenewalUploadProblem

	bad := func(d renewalHistoryDraft, msg string) {
		problems = append(problems, RenewalUploadProblem{Row: d.RowNo, Sheet: d.SheetName, Message: msg})
	}

	for _, d := range drafts {
		where := "แถวที่ " + strconv.Itoa(d.RowNo)

		if d.LicenseType == "" {
			bad(d, where+": License Type \""+d.RawType+"\" ไม่ถูกต้อง — ต้องเป็น Import หรือ Export")
			continue
		}
		if d.OldLicenseNo == "" {
			bad(d, where+": ไม่มี Old License No.")
			continue
		}
		if d.NewLicenseNo == "" {
			bad(d, where+": ไม่มี New License No.")
			continue
		}
		if SameCode(d.OldLicenseNo, d.NewLicenseNo) {
			bad(d, where+": เลขใบเดิมกับเลขใบใหม่เป็นเลขเดียวกัน ("+d.OldLicenseNo+")")
			continue
		}

		oldKey := d.LicenseType + "|" + NormalizeCodeValue(d.OldLicenseNo)
		newKey := d.LicenseType + "|" + NormalizeCodeValue(d.NewLicenseNo)
		pairKey := oldKey + "|" + NormalizeCodeValue(d.NewLicenseNo)

		if existingPair[pairKey] {
			bad(d, where+": การต่ออายุ "+d.OldLicenseNo+" → "+d.NewLicenseNo+" เคยอัปโหลดแล้ว (ข้ามแถวนี้)")
			continue
		}
		if seenInFile[pairKey] {
			bad(d, where+": มีคู่ "+d.OldLicenseNo+" → "+d.NewLicenseNo+" ซ้ำกันในไฟล์เดียวกัน (ข้ามแถวนี้)")
			continue
		}

		if prev, dup := usedNew[newKey]; dup {
			bad(d, where+": เลขใบใหม่ "+d.NewLicenseNo+" ถูกใช้เป็นใบต่ออายุของ "+prev+" ไปแล้ว")
			continue
		}

		// เลขใบเดิมต้องมีอยู่ในระบบ หรือเป็นใบที่เพิ่งถูกสร้างจากแถวก่อนหน้าในไฟล์เดียวกัน
		if strictOldExists && !known[d.LicenseType][NormalizeCodeValue(d.OldLicenseNo)] && !pending[oldKey] {
			bad(d, where+": ไม่พบเลขใบเดิม "+d.OldLicenseNo+" ใน "+
				models.LicenseTypeLabel(d.LicenseType)+" — กรุณาอัปโหลดไฟล์ License ก่อน หรือตรวจเลขใบอีกครั้ง")
			continue
		}

		seenInFile[pairKey] = true
		usedNew[newKey] = d.OldLicenseNo
		pending[newKey] = true
		pending[oldKey] = true
		out = append(out, d)
	}

	return out, problems
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

type renewalUploadResult struct {
	Total     int                    `json:"total"`
	Imported  int                    `json:"imported"`
	Skipped   int                    `json:"skipped"`
	Source    string                 `json:"source"`
	FileName  string                 `json:"fileName"`
	Problems  []RenewalUploadProblem `json:"problems"`
	Message   string                 `json:"message"`
	Preview   bool                   `json:"preview"`
	SavedRows int                    `json:"savedRows"`

	// ตัวเลขจากชีต "ต่ออายุ" — ใช้ยืนยันว่าระบบอ่านตารางทะเบียนและคอลัมน์ NO. ได้จริง
	LedgerRows      int `json:"ledgerRows"`
	CompletedGroups int `json:"completedGroups"`
}

// countCompletedGroups: นับงานที่ปิดแล้วจากคอลัมน์ NO.
//
// named = จำนวนกลุ่ม Completed NN (นับตามชื่อกลุ่ม)
// done  = จำนวนใบอนุญาตที่จะขึ้นสถานะเสร็จสิ้น
//
// สองตัวนี้ไม่เท่ากันได้ เพราะกลุ่มเดียวมีใบนำเข้าได้หลายใบ
// เช่น Completed 02 เป็น 1 กลุ่ม แต่มีใบนำเข้า 3 ใบที่จบพร้อมกัน
func countCompletedGroups(rows []models.LicenseRenewal) (named int, done int) {
	groups := map[string]bool{}
	chains := map[string]bool{}
	for i := range rows {
		g := strings.TrimSpace(rows[i].GroupNo)
		if g == "" || !ledgerGroupCompleted(g) {
			continue
		}
		groups[strings.ToUpper(g)] = true
		chains[ledgerGroupKey(rows[i])] = true
	}
	return len(groups), len(chains)
}

// readRenewalHistoryFile: อ่านไฟล์ Renewal (ลองแบบที่ 1 ก่อน ถ้าไม่ใช่ค่อยลองชีตต่ออายุเดิม)
// คืนตารางทะเบียน (ledger) ออกมาด้วย เพราะคอลัมน์ NO. / TOTAL / STOCK / REMAIN
// อยู่ในตารางนั้น ไม่ได้อยู่ในประวัติการต่ออายุที่แปลงออกมา
func readRenewalHistoryFile(fileHeader *multipart.FileHeader) ([]renewalHistoryDraft, []RenewalUploadProblem, string, []models.LicenseRenewal, error) {
	sheets, err := readAllUploadedSheets(fileHeader)
	if err != nil {
		return nil, nil, "", nil, err
	}

	drafts, problems, found := readRenewalHistoryDirect(sheets)
	if found {
		return drafts, problems, "renewal_table", nil, nil
	}

	// ไฟล์ชีต "ต่ออายุ" เดิม
	ledger, ok, err := readLicenseRenewalSheets(fileHeader)
	if err != nil {
		return nil, nil, "", nil, err
	}
	if ok {
		return readRenewalHistoryFromLedger(ledger), nil, "ledger_sheet", ledger, nil
	}

	return nil, nil, "", nil, errNoRenewalHistorySheet
}

// AbsorbLedgerSheet: ถ้าไฟล์ที่อัปโหลดมีชีต "ต่ออายุ" อยู่ด้วย ให้เก็บตารางทะเบียนไว้เลย
//
// ผู้ใช้ทำงานบนไฟล์ Excel ไฟล์เดียวที่มีครบทุกชีต (Import / Export / ต่ออายุ)
// แล้วอัปไฟล์เดิมนี้เข้าทุกตัวเลือกในหน้าเว็บ
// ถ้าอ่านชีตต่ออายุเฉพาะตอนเลือก Renewal เท่านั้น คนที่อัปแค่ Import กับ Export
// จะไม่ได้สถานะ "เสร็จสิ้น" (คอลัมน์ NO. = Completed NN) เลย ทั้งที่ข้อมูลอยู่ในไฟล์แล้ว
//
// จึงอ่านชีตนี้ทุกครั้งที่เจอ ไม่ว่าจะอัปผ่านตัวเลือกไหน
// ไฟล์ที่ไม่มีชีตต่ออายุจะไม่ถูกแตะต้องอะไรเลย
func AbsorbLedgerSheet(fileHeader *multipart.FileHeader, userID uint, now time.Time) (int, int) {
	if fileHeader == nil || config.DB == nil {
		return 0, 0
	}
	ledger, ok, err := readLicenseRenewalSheets(fileHeader)
	if err != nil || !ok || len(ledger) == 0 {
		return 0, 0
	}
	if err := saveLedgerRows(ledger, fileHeader.Filename, userID, now); err != nil {
		return 0, 0
	}
	_, done := countCompletedGroups(ledger)
	return len(ledger), done
}

// respondLedgerOnlyUpload: ไฟล์ที่อัปเข้าการ์ด Import / Export อาจเป็น "ชีตทะเบียน (ต่ออายุ)" ล้วน ๆ
//
// ชีตทะเบียนมักตั้งชื่อชีตว่า "IMPORT LICENSE" ทั้งที่ข้างในไม่มีคอลัมน์หมายเลขเครื่อง/Serial เลย
// ผู้ใช้จึงอัปไฟล์นี้เข้าการ์ด Import หรือ Export ตามชื่อชีต แล้วตัวอ่านแถวเครื่องก็ตอบ
// "หาหัวตารางไม่เจอ" และออกไปก่อนถึงขั้นเก็บตารางทะเบียน ตารางทะเบียนในระบบจึงว่างตลอด
// STOCK / คงเหลือ เลยขึ้นเป็นขีดทุกใบ ไม่ว่าจะอัปกี่ครั้ง
//
// ถ้าไฟล์มีชีตทะเบียนอยู่ ให้เก็บทะเบียนแล้วตอบสำเร็จแทนการปฏิเสธ
// คืน true เมื่อตอบไปแล้ว (handler ต้อง return ทันที)
func respondLedgerOnlyUpload(c *gin.Context, fileHeader *multipart.FileHeader, from string) bool {
	if fileHeader == nil {
		return false
	}
	userID, userName := lookupUserName(c)
	n, done := AbsorbLedgerSheet(fileHeader, userID, time.Now())
	if n == 0 {
		return false
	}
	CreateAuditLog("LICENSE_RENEWAL", 0, "upload",
		fileHeader.Filename+": ทะเบียน "+strconv.Itoa(n)+" แถว · ปิดงานแล้ว "+
			strconv.Itoa(done)+" ใบ (ไฟล์ทะเบียนล้วน จากการ์ด "+from+")", userID, userName)
	c.JSON(200, gin.H{
		"imported":   0,
		"updated":    0,
		"deleted":    0,
		"skipped":    0,
		"ledgerRows": n,
		"file":       fileHeader.Filename,
		"message": "ไฟล์นี้เป็นชีตทะเบียนใบอนุญาต (ต่ออายุ) ไม่มีรายการเครื่อง — บันทึกทะเบียนแล้ว " +
			strconv.Itoa(n) + " แถว ทำให้ TOTAL / STOCK / คงเหลือ ขึ้นตัวเลขแล้ว",
	})
	return true
}

// saveLedgerRows: เก็บตารางทะเบียนใบอนุญาต (ชีต "ต่ออายุ") ลงฐานข้อมูล
//
// เดิมการอัปโหลดไฟล์ Renewal อ่านชีตนี้เพื่อสร้าง "ประวัติการต่ออายุ" อย่างเดียว
// แล้วทิ้งตัวตารางไป ทำให้คอลัมน์ NO. / TOTAL / STOCK / REMAIN หายไปทั้งหมด
//
// ใช้กติกาการซิงก์เดียวกับทะเบียนอื่น (ดู license_register_merge.go)
// คือเทียบรายแถวตามคีย์ ไฟล์ชื่อเดิมซิงก์กับไฟล์ ไฟล์ชื่อใหม่เพิ่มเข้าไป
func saveLedgerRows(rows []models.LicenseRenewal, fileName string, userID uint, now time.Time) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := mergeLicenseRegister(rows, fileName, userID, now)
	return err
}

// PreviewLicenseRenewalHistory: POST /license-renewal-history/preview
// ตรวจไฟล์อย่างเดียว ยังไม่บันทึก — ให้ผู้ใช้เห็น error ก่อน
func PreviewLicenseRenewalHistory(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": errUploadNoFile.Error()})
		return
	}
	drafts, readProblems, source, ledger, err := readRenewalHistoryFile(fileHeader)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	ok, problems := validateRenewalDrafts(drafts, true)
	problems = append(readProblems, problems...)

	_, completedGroups := countCompletedGroups(ledger)

	// แยกข้อความเป็นบรรทัดสั้น ๆ ตามหัวข้อ — ไม่รวมเป็นประโยคยาวเส้นเดียวที่อ่านแล้วงง
	// ทะเบียน (ledger) กับประวัติการต่ออายุ (renewal pair) เป็นคนละข้อมูลกัน
	// ต้องแยกให้ชัดว่า "จะบันทึกได้ 0" ไม่ได้แปลว่าไฟล์มีปัญหา — ไฟล์ทะเบียนล้วน ๆ
	// ไม่มีคู่เลขใบเดิม/ใบใหม่ให้บันทึกเป็นปกติอยู่แล้ว
	lines := []string{"ตรวจไฟล์แล้ว"}
	if len(ledger) > 0 {
		lines = append(lines,
			"ทะเบียนใบอนุญาต: "+strconv.Itoa(len(ledger))+" ใบ",
			"ปิดแล้ว: "+strconv.Itoa(completedGroups)+" ใบ")
	}
	switch {
	case len(ok) > 0:
		lines = append(lines, "ประวัติการต่ออายุใหม่: "+strconv.Itoa(len(ok))+" ใบ")
	case len(drafts) > 0:
		lines = append(lines, "ประวัติการต่ออายุใหม่: ไม่มีในไฟล์นี้")
	}
	msg := strings.Join(lines, "\n")

	c.JSON(200, renewalUploadResult{
		Total:    len(drafts),
		Imported: len(ok),
		Skipped:  len(drafts) - len(ok),
		Source:   source,
		FileName: fileHeader.Filename,
		Problems: problems,
		Preview:  true,
		Message:  msg,

		LedgerRows:      len(ledger),
		CompletedGroups: completedGroups,
	})
}

// UploadLicenseRenewalHistory: POST /license-renewal-history/upload
// เพิ่ม/อัปเดตประวัติการต่ออายุ — ไม่ล้างของเดิม ไม่แตะข้อมูล License เดิม
func UploadLicenseRenewalHistory(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": errUploadNoFile.Error()})
		return
	}

	drafts, readProblems, source, ledger, err := readRenewalHistoryFile(fileHeader)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	userID, name := lookupUserName(c)
	now := time.Now()

	// เก็บตารางทะเบียนก่อนเป็นอันดับแรก — เป็นที่มาของคอลัมน์ NO. / TOTAL / STOCK / REMAIN
	// และของสถานะ "เสร็จสิ้น" (NO. = Completed NN)
	//
	// ต้องทำก่อนตรวจความถูกต้องของคู่การต่ออายุ เพราะไฟล์ที่ไม่มีคู่ต่ออายุเลย
	// (เช่น ใบนำเข้าทุกใบมีใบนำออกใบเดียว) ก็ยังมีข้อมูลทะเบียนที่ใช้ได้
	var problems []RenewalUploadProblem
	ledgerSaved := 0
	completedGroups := 0
	if len(ledger) > 0 {
		if err := saveLedgerRows(ledger, fileHeader.Filename, userID, now); err != nil {
			problems = append(problems, RenewalUploadProblem{
				Message: "บันทึกตารางทะเบียนใบอนุญาตไม่สำเร็จ: " + err.Error(),
			})
		} else {
			ledgerSaved = len(ledger)
			_, completedGroups = countCompletedGroups(ledger)
			CreateAuditLog("LICENSE_RENEWAL", 0, "upload",
				fileHeader.Filename+": ทะเบียน "+strconv.Itoa(ledgerSaved)+" แถว · เสร็จสิ้น "+
					strconv.Itoa(completedGroups)+" กลุ่ม", userID, name)
		}
	}

	if len(drafts) == 0 && ledgerSaved == 0 {
		c.JSON(400, gin.H{"message": "พบตารางต่ออายุแต่ไม่มีข้อมูลในตาราง"})
		return
	}

	okRows, validateProblems := validateRenewalDrafts(drafts, true)
	problems = append(problems, readProblems...)
	problems = append(problems, validateProblems...)

	saved := 0
	for i := range okRows {
		row := okRows[i].LicenseRenewalHistory
		row.FileName = fileHeader.Filename
		row.UploadedBy = name
		row.UploadedAt = now
		row.UserID = userID
		if err := config.DB.Create(&row).Error; err != nil {
			problems = append(problems, RenewalUploadProblem{
				Row:     okRows[i].RowNo,
				Sheet:   okRows[i].SheetName,
				Message: "บันทึกไม่สำเร็จ: " + err.Error(),
			})
			continue
		}
		saved++
	}

	// Audit log: ใคร / เมื่อไร / ไฟล์อะไร / กี่รายการ / สำเร็จกี่ / error กี่
	CreateAuditLog("LICENSE_RENEWAL_HISTORY", 0, "upload",
		licenseUploadLogLine(fileHeader.Filename, len(drafts), saved, 0, 0, len(drafts)-saved),
		userID, name)

	// บรรทัดสั้น ๆ ตามหัวข้อ เหมือน preview — จะได้อ่านแบบเดียวกันทั้งสองจุด
	lines := []string{"บันทึกสำเร็จ"}
	if ledgerSaved > 0 {
		// บอกจำนวนที่ปิดงานแล้วตรง ๆ จะได้รู้ทันทีว่าระบบอ่านคอลัมน์ NO. ได้จริง
		lines = append(lines,
			"ทะเบียนใบอนุญาต: "+strconv.Itoa(ledgerSaved)+" ใบ",
			"ปิดแล้ว: "+strconv.Itoa(completedGroups)+" ใบ")
	} else if len(ledger) > 0 {
		// เคยเงียบมาก่อน ทำให้ STOCK / คงเหลือ ขึ้นเป็นขีดโดยไม่รู้สาเหตุ
		lines = append(lines, "บันทึกตารางทะเบียนไม่สำเร็จ (STOCK / คงเหลือ จะยังไม่ขึ้นตัวเลข)")
	}
	switch {
	case saved > 0:
		lines = append(lines, "ประวัติการต่ออายุใหม่: "+strconv.Itoa(saved)+" ใบ")
	case len(drafts) > 0:
		lines = append(lines, "ประวัติการต่ออายุใหม่: ไม่มีในไฟล์นี้")
	}
	if len(problems) > 0 {
		lines = append(lines, "ข้าม "+strconv.Itoa(len(drafts)-saved)+" รายการ (ดูรายละเอียดด้านล่าง)")
	}
	msg := strings.Join(lines, "\n")

	c.JSON(200, renewalUploadResult{
		Total:     len(drafts),
		Imported:  saved,
		Skipped:   len(drafts) - saved,
		Source:    source,
		FileName:  fileHeader.Filename,
		Problems:  problems,
		Message:   msg,
		SavedRows: saved,

		LedgerRows:      ledgerSaved,
		CompletedGroups: completedGroups,
	})
}

// GetLicenseRenewalHistory: GET /license-renewal-history
// ดูประวัติทั้งหมด หรือกรองด้วย type / licenseNo (ไล่ทั้งโซ่ของใบนั้น)
func GetLicenseRenewalHistory(c *gin.Context) {
	licenseType := models.NormalizeLicenseType(c.Query("type"))
	licenseNo := strings.TrimSpace(c.Query("licenseNo"))

	all := loadRenewalHistory()

	if licenseNo != "" && licenseType != "" {
		chain := buildLicenseChain(all, licenseType, licenseNo)
		c.JSON(200, gin.H{
			"rows":         chain.Steps,
			"total":        len(chain.Steps),
			"chain":        chain,
			"renewalCount": chain.RenewalCount,
		})
		return
	}

	rows := all
	if licenseType != "" {
		filtered := rows[:0:0]
		for _, r := range rows {
			if r.LicenseType == licenseType {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	c.JSON(200, gin.H{"rows": rows, "total": len(rows)})
}

// ClearLicenseRenewalHistory: DELETE /license-renewal-history
// ลบเฉพาะประวัติการต่ออายุ — ไม่แตะข้อมูล License เดิม
func ClearLicenseRenewalHistory(c *gin.Context) {
	res := config.DB.Where("1 = 1").Delete(&models.LicenseRenewalHistory{})
	if res.Error != nil {
		c.JSON(500, gin.H{"message": res.Error.Error()})
		return
	}
	userID, name := lookupUserName(c)
	CreateAuditLog("LICENSE_RENEWAL_HISTORY", 0, "clear",
		"deleted="+strconv.FormatInt(res.RowsAffected, 10), userID, name)
	c.JSON(200, gin.H{"deleted": res.RowsAffected})
}

func loadRenewalHistory() []models.LicenseRenewalHistory {
	var rows []models.LicenseRenewalHistory
	config.DB.Order("license_type asc, renewal_date asc, sort_order asc, id asc").Find(&rows)
	return rows
}