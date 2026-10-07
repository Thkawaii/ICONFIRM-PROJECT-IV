package controllers

import (
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// importExportLicenseNoKey / importExportIssueDateKey = คีย์ของสองคอลัมน์ท้ายชีต
// ที่บอกว่าเครื่องนี้ถูกส่งออกด้วยใบอนุญาตนำออกใบไหน ออกเมื่อไร
const (
	importExportLicenseNoKey = "เลขใบอนุญาตนำออก"
	importExportIssueDateKey = "วันที่ออกใบอนุญาตนำออก"
)

var importLicenseColumns = map[string]func(*models.LicenseItem, string){

	"ลำดับ":  func(*models.LicenseItem, string) {},
	"no":     func(*models.LicenseItem, string) {},
	"itemno": func(*models.LicenseItem, string) {},

	"ตราอักษร": func(m *models.LicenseItem, v string) { m.Brand = v },
	"brand":    func(m *models.LicenseItem, v string) { m.Brand = v },

	"แบบรุ่น": func(m *models.LicenseItem, v string) { m.Model = v },
	"รุ่น":    func(m *models.LicenseItem, v string) { m.Model = v },
	"model":   func(m *models.LicenseItem, v string) { m.Model = v },

	"เลขใบอนุญาตนำเข้า": func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"ใบอนุญาตนำเข้า":    func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	// ชื่อหัวคอลัมน์ที่พบในไฟล์จริง เขียนได้หลายแบบ — รับให้ครบ
	// ถ้าอ่านเลขใบอนุญาตไม่ได้ แถวนั้นจะไม่ขึ้นในหน้า License เลย
	"เลขที่ใบอนุญาตนำเข้า": func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"เลขที่ใบอนุญาต":       func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"เลขใบอนุญาต":          func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"ใบอนุญาต":             func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"licenseno":            func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"licensenumber":        func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"importlicenseno":      func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"importlicense":        func(m *models.LicenseItem, v string) { m.LicenseNo = v },
	"importlicensenumber":  func(m *models.LicenseItem, v string) { m.LicenseNo = v },

	"เลขอินวอยซ์นำเข้า": func(m *models.LicenseItem, v string) { m.InvoiceNo = v },
	"อินวอยซ์":          func(m *models.LicenseItem, v string) { m.InvoiceNo = v },
	"invoiceno":         func(m *models.LicenseItem, v string) { m.InvoiceNo = v },
	"invoice":           func(m *models.LicenseItem, v string) { m.InvoiceNo = v },

	"เลขใบขนสินค้าขาเข้า": func(m *models.LicenseItem, v string) { m.DeclarationNo = v },
	"declarationno": func(m *models.LicenseItem, v string) { m.DeclarationNo = v },

	"จำนวนเครื่อง": func(m *models.LicenseItem, v string) { m.Qty = atoiSafe(v) },
	"จำนวน":        func(m *models.LicenseItem, v string) { m.Qty = atoiSafe(v) },
	"qty":          func(m *models.LicenseItem, v string) { m.Qty = atoiSafe(v) },
	"quantity":     func(m *models.LicenseItem, v string) { m.Qty = atoiSafe(v) },

	"หมายเลขเครื่อง": func(m *models.LicenseItem, v string) { m.MachineNo = normalizeDigitCell(v) },
	"machineno":      func(m *models.LicenseItem, v string) { m.MachineNo = normalizeDigitCell(v) },
	"itcontrollerno": func(m *models.LicenseItem, v string) { m.MachineNo = normalizeDigitCell(v) },
	"itcno":          func(m *models.LicenseItem, v string) { m.MachineNo = normalizeDigitCell(v) },

	"หมายเลขการผลิต": func(m *models.LicenseItem, v string) { m.ProductionNo = normalizeDigitCell(v) },
	"productionno": func(m *models.LicenseItem, v string) { m.ProductionNo = normalizeDigitCell(v) },
	"imei":         func(m *models.LicenseItem, v string) { m.ProductionNo = normalizeDigitCell(v) },

	"หมายเหตุ": func(m *models.LicenseItem, v string) { m.Remark = v },
	"remark":   func(m *models.LicenseItem, v string) { m.Remark = v },

	// ---- ใบอนุญาตนำออกที่ใช้ส่งเครื่องนี้ออก (คอลัมน์ท้ายของชีตบัญชีใบอนุญาตนำเข้า) ----
	// ไฟล์มีสองคอลัมน์นี้มาตลอด แต่ระบบไม่เคยอ่าน เลขใบนำออกจึงไปจมอยู่ใน ExtraJSON
	// ส่วนวันที่ออกใบนำออกถูกตัดทิ้งเพราะหัวคอลัมน์ชื่อซ้ำกับของใบนำเข้า
	importExportLicenseNoKey: func(m *models.LicenseItem, v string) {
		m.ExportLicenseNo = strings.ToUpper(strings.TrimSpace(v))
	},
	"ใบอนุญาตนำออก":   func(m *models.LicenseItem, v string) { m.ExportLicenseNo = strings.ToUpper(strings.TrimSpace(v)) },
	"exportlicenseno": func(m *models.LicenseItem, v string) { m.ExportLicenseNo = strings.ToUpper(strings.TrimSpace(v)) },
	"exportlicense":   func(m *models.LicenseItem, v string) { m.ExportLicenseNo = strings.ToUpper(strings.TrimSpace(v)) },

	importExportIssueDateKey: func(m *models.LicenseItem, v string) { m.ExportIssueDate = parseLicenseDate(v) },
	"exportlicensedate":      func(m *models.LicenseItem, v string) { m.ExportIssueDate = parseLicenseDate(v) },

	"ส่งออกไปประเทศ": func(m *models.LicenseItem, v string) { m.ExportCountry = v },
	"ประเทศ":         func(m *models.LicenseItem, v string) { m.ExportCountry = v },
	"country":        func(m *models.LicenseItem, v string) { m.ExportCountry = v },
	"exportcountry":  func(m *models.LicenseItem, v string) { m.ExportCountry = v },

	"วันที่ออกใบอนุญาต": func(m *models.LicenseItem, v string) { m.IssueDate = parseLicenseDate(v) },
	"วันออกใบอนุญาต":    func(m *models.LicenseItem, v string) { m.IssueDate = parseLicenseDate(v) },
	"วันนำเข้า":         func(m *models.LicenseItem, v string) { m.IssueDate = parseLicenseDate(v) },
	"issuedate":         func(m *models.LicenseItem, v string) { m.IssueDate = parseLicenseDate(v) },

	"expiredate":        func(m *models.LicenseItem, v string) { m.ExpireDate = parseLicenseDate(v) },
	"expirydate":        func(m *models.LicenseItem, v string) { m.ExpireDate = parseLicenseDate(v) },
	"expire":            func(m *models.LicenseItem, v string) { m.ExpireDate = parseLicenseDate(v) },
	"วันหมดอายุ":        func(m *models.LicenseItem, v string) { m.ExpireDate = parseLicenseDate(v) },
	"หมดอายุ":           func(m *models.LicenseItem, v string) { m.ExpireDate = parseLicenseDate(v) },
	"หมดอายุ6เดือน":     func(m *models.LicenseItem, v string) { m.ExpireDate = parseLicenseDate(v) },
	"importlicensedate": func(m *models.LicenseItem, v string) { m.IssueDate = parseLicenseDate(v) },
	"licensedate":       func(m *models.LicenseItem, v string) { m.IssueDate = parseLicenseDate(v) },
	"importdate":        func(m *models.LicenseItem, v string) { m.IssueDate = parseLicenseDate(v) },
}

// importIgnoredHeaders: คอลัมน์ NOTE เดิม (เคยใช้สั่งลบข้อมูลตอนอัปโหลด) — เลิกใช้แล้ว ข้ามเงียบ ๆ
var importIgnoredHeaders = map[string]bool{"note": true, "notes": true}

func titleCaseWords(s string) string {
	var b strings.Builder
	prevLetter := false
	for _, r := range s {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		switch {
		case isLetter && !prevLetter:
			if r >= 'a' && r <= 'z' {
				r -= 32
			}
		case isLetter && prevLetter:
			if r >= 'A' && r <= 'Z' {
				r += 32
			}
		}
		b.WriteRune(r)
		prevLetter = isLetter
	}
	return b.String()
}

func parseLicenseDate(v string) *time.Time {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil
	}

	if i := strings.IndexByte(s, ' '); i > 0 && strings.Contains(s, ":") {
		s = strings.TrimSpace(s[:i])
	}

	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"02/01/2006",
		"02-01-2006",
		"01/02/2006",
		"2/1/2006",
		"1/2/2006",
		"1/2/06",
		"01/02/06",
		"2/1/06",
		"01-02-06",
		"1-2-06",
		"02-01-06",
		"2-1-06",
		"01-02-2006",
		"1-2-2006",
		"2-Jan-06",
		"02-Jan-06",
		"2-Jan-2006",
		"02-Jan-2006",
		"2 Jan 2006",
		"2 Jan 06",
		"2-January-2006",
		"January 2, 2006",
		"Jan 2, 2006",
	}

	candidates := []string{s}
	if titled := titleCaseWords(s); titled != s {
		candidates = append(candidates, titled)
	}

	for _, layout := range layouts {
		for _, cand := range candidates {
			if t, err := time.Parse(layout, cand); err == nil {
				if t.Year() > 2400 {
					t = t.AddDate(-543, 0, 0)
				}
				return &t
			}
		}
	}

	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 20000 && f < 90000 {
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		t := base.AddDate(0, 0, int(f))
		return &t
	}

	return nil
}

func scanIssueDateFromHeaderBlock(rows [][]string, headerIdx int) *time.Time {
	for i := 0; i < headerIdx && i < len(rows); i++ {
		for j, cell := range rows[i] {
			key := normalizeHeader(cell)
			if key != "issuedate" && key != "วันที่ออกใบอนุญาต" && key != "วันนำเข้า" {
				continue
			}
			for k := j + 1; k < len(rows[i]); k++ {
				if d := parseLicenseDate(rows[i][k]); d != nil {
					return d
				}
			}
		}
	}
	return nil
}

func normalizeDigitCell(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return ""
	}

	if strings.ContainsAny(s, "eE") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return strconv.FormatFloat(f, 'f', 0, 64)
		}
	}

	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		frac := s[dot+1:]
		allZero := frac != ""
		for _, r := range frac {
			if r != '0' {
				allZero = false
				break
			}
		}
		if allZero {
			return s[:dot]
		}
	}

	return s
}

// importLicenseNoKeys = หัวคอลัมน์ที่หมายถึง "เลขใบอนุญาตนำเข้า"
var importLicenseNoKeys = map[string]bool{
	"เลขใบอนุญาตนำเข้า": true, "ใบอนุญาตนำเข้า": true, "เลขที่ใบอนุญาตนำเข้า": true,
	"เลขที่ใบอนุญาต": true, "เลขใบอนุญาต": true, "ใบอนุญาต": true,
	"licenseno": true, "licensenumber": true,
	"importlicenseno": true, "importlicense": true, "importlicensenumber": true,
}

// importMachineNoKeys = หัวคอลัมน์ที่หมายถึงหมายเลขเครื่อง IT Controller
var importMachineNoKeys = map[string]bool{
	"หมายเลขเครื่อง": true, "machineno": true, "itcontrollerno": true, "itcno": true,
}

// importIssueDateKeys = หัวคอลัมน์ที่หมายถึงวันที่ออกใบอนุญาต "นำเข้า"
var importIssueDateKeys = map[string]bool{
	"วันที่ออกใบอนุญาต": true, "วันออกใบอนุญาต": true, "วันนำเข้า": true,
	"issuedate": true, "importlicensedate": true, "licensedate": true, "importdate": true,
}

func findImportLicenseHeader(rows [][]string) (int, []string) {

	reverse := loadColumnAliasReverse("import_license")

	limit := 30
	if len(rows) < limit {
		limit = len(rows)
	}

	for i := 0; i < limit; i++ {

		headers := make([]string, len(rows[i]))
		hits := 0
		hasMachineNo := false
		hasLicenseNo := false

		for j, cell := range rows[i] {
			key := aliasHeaderKey(reverse, normalizeHeader(cell))
			headers[j] = key

			if _, ok := importLicenseColumns[key]; ok {
				hits++
				if importMachineNoKeys[key] {
					hasMachineNo = true
				}
				if importLicenseNoKeys[key] {
					hasLicenseNo = true
				}
			}
		}

		// ต้องมีคอลัมน์เลขใบอนุญาตนำเข้าด้วย ไม่ใช่แค่หมายเลขเครื่อง
		//
		// ชีตฝั่ง Export (Machine No · Country · Invoice no. · ...) เข้าเงื่อนไขเดิมได้
		// เพราะมีคอลัมน์ที่รู้จักครบ 3 และมีหมายเลขเครื่อง ไฟล์ Serial Allocation
		// จึงถูกดูดเข้ามาเป็นรายการใบอนุญาตนำเข้าหลายพันแถวโดยไม่มีเลขใบอนุญาตสักใบ
		// บัญชีใบอนุญาตนำเข้าของจริงมีคอลัมน์เลขใบอนุญาตเสมอ ใช้เป็นตัวแยกได้
		if hits >= 3 && hasMachineNo && hasLicenseNo {
			retagImportExportColumns(headers)
			return i, headers
		}
	}

	return -1, nil
}

// retagImportExportColumns: แยกคอลัมน์ "วันที่ออกใบอนุญาต" ตัวที่สองออกจากตัวแรก
//
// ชีตบัญชีใบอนุญาตนำเข้าของจริงมีหัวคอลัมน์ชื่อนี้สองครั้ง
//
//	... เลขใบอนุญาตนำเข้า · วันที่ออกใบอนุญาต · ... · เลขใบอนุญาตนำออก · วันที่ออกใบอนุญาต · ส่งออกไปประเทศ
//
// ชื่อเหมือนกันเป๊ะ ตัวตัดคอลัมน์ซ้ำจึงทิ้งคอลัมน์หลังไปทั้งคอลัมน์
// วันที่ออกใบ "นำออก" เลยไม่เคยถูกอ่านเข้าระบบ ทั้งที่ไฟล์มีให้อยู่แล้ว
//
// แยกด้วยตำแหน่ง: ตัวที่อยู่หลังคอลัมน์เลขใบอนุญาตนำออก คือวันที่ของใบนำออก
func retagImportExportColumns(headers []string) {
	seenExportNo := false
	for i, key := range headers {
		if key == importExportLicenseNoKey {
			seenExportNo = true
			continue
		}
		if seenExportNo && importIssueDateKeys[key] {
			headers[i] = importExportIssueDateKey
			return
		}
	}
}

// importLicenseHasRemarkColumn: ไฟล์นี้มีคอลัมน์หมายเหตุไหม
//
// สถานะ "เสร็จสิ้น" อ่านจากช่องหมายเหตุ ถ้าไฟล์ไม่มีคอลัมน์นี้เลย
// ต้องไม่ไปล้างสถานะเดิมที่อยู่ในระบบทิ้ง (ใช้เกณฑ์เดียวกับฝั่ง Export)
func importLicenseHasRemarkColumn(headers []string) bool {
	for _, h := range headers {
		if h == "หมายเหตุ" || h == "remark" {
			return true
		}
	}
	return false
}

// importLicenseSheetScore: ไฟล์หลายชีต — เลือกชีต Import License ให้อัตโนมัติ
func importLicenseSheetScore(name string, rows [][]string) int {
	idx, headers := findImportLicenseHeader(rows)
	if idx < 0 {
		return -1
	}
	score := countKnownHeaders(headers, func(h string) bool { _, ok := importLicenseColumns[h]; return ok })
	if sheetNameHas(name, "import", "นำเข้า") {
		score += sheetNameMatchBonus
	}
	return score
}

func GetImportLicenseItems(c *gin.Context) {

	var items []models.LicenseItem

	query := config.DB.Order("sort_order asc, id asc")

	if v := strings.TrimSpace(c.Query("license_no")); v != "" {
		query = query.Where("license_no = ?", v)
	}
	if v := strings.TrimSpace(c.Query("invoice_no")); v != "" {
		query = query.Where("invoice_no = ?", v)
	}
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		query = query.Where("confirm_status = ?", strings.ToUpper(v))
	}
	if code := strings.TrimSpace(c.Query("code")); code != "" {
		query = query.Where("machine_no = ? OR production_no = ?", code, code)
	}

	query.Find(&items)

	buildScanLockIndex().annotateImportLicense(items)

	c.JSON(200, items)
}

func GetImportLicenseSummary(c *gin.Context) {

	type summaryRow struct {
		LicenseNo      string `json:"LicenseNo"`
		InvoiceNo      string `json:"InvoiceNo"`
		DeclarationNo  string `json:"DeclarationNo"`
		Model          string `json:"Model"`
		Total          int    `json:"Total"`
		Confirmed      int    `json:"Confirmed"`
		CompletedCount int    `json:"CompletedCount"`
	}

	var rows []summaryRow

	config.DB.Model(&models.LicenseItem{}).
		Select(`license_no,
			invoice_no,
			max(declaration_no) as declaration_no,
			max(model) as model,
			count(*) as total,
			count(*) filter (where confirm_status = 'CONFIRMED') as confirmed,
			count(*) filter (where completed) as completed_count`).
		Group("license_no, invoice_no").
		Order("license_no asc").
		Scan(&rows)

	c.JSON(200, rows)
}

const LicenseValidityMonths = models.ImportLicenseValidityMonths

const (
	LicenseExpiryExpired = "EXPIRED"
	LicenseExpirySoon    = "EXPIRING"
	LicenseExpiryValid   = "VALID"
	LicenseExpiryNoDate  = "NO_DATE"
)

func GetImportLicenseAlerts(c *gin.Context) {

	withinDays := 30
	if v := strings.TrimSpace(c.Query("within_days")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			withinDays = n
		}
	}
	onlyAlert := strings.EqualFold(strings.TrimSpace(c.Query("only")), "alert")

	type groupRow struct {
		LicenseNo     string
		InvoiceNo     string
		DeclarationNo string
		Model         string
		Brand         string
		Total         int
		Confirmed     int
		IssueDate     *time.Time
	}

	var groups []groupRow
	config.DB.Model(&models.LicenseItem{}).
		Where("completed IS NOT TRUE").
		Select(`license_no,
			invoice_no,
			max(declaration_no) as declaration_no,
			max(model) as model,
			max(brand) as brand,
			count(*) as total,
			count(*) filter (where confirm_status = 'CONFIRMED') as confirmed,
			min(issue_date) as issue_date`).
		Group("license_no, invoice_no").
		Scan(&groups)

	type alertRow struct {
		LicenseNo     string     `json:"LicenseNo"`
		InvoiceNo     string     `json:"InvoiceNo"`
		DeclarationNo string     `json:"DeclarationNo"`
		Model         string     `json:"Model"`
		Brand         string     `json:"Brand"`
		Total         int        `json:"Total"`
		Confirmed     int        `json:"Confirmed"`
		IssueDate     *time.Time `json:"IssueDate"`
		ExpiryDate    *time.Time `json:"ExpiryDate"`
		DaysLeft      int        `json:"DaysLeft"`
		Status        string     `json:"Status"`
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var (
		out                                   = []alertRow{}
		expiredCnt, soonCnt, validCnt, noDate int
	)

	for _, g := range groups {
		row := alertRow{
			LicenseNo:     g.LicenseNo,
			InvoiceNo:     g.InvoiceNo,
			DeclarationNo: g.DeclarationNo,
			Model:         g.Model,
			Brand:         g.Brand,
			Total:         g.Total,
			Confirmed:     g.Confirmed,
			IssueDate:     g.IssueDate,
		}

		if g.IssueDate == nil {
			row.Status = LicenseExpiryNoDate
			noDate++
			if !onlyAlert {
				out = append(out, row)
			}
			continue
		}

		expiry := g.IssueDate.AddDate(0, LicenseValidityMonths, 0)
		expDay := time.Date(expiry.Year(), expiry.Month(), expiry.Day(), 0, 0, 0, 0, now.Location())
		row.ExpiryDate = &expDay
		row.DaysLeft = int(expDay.Sub(today).Hours() / 24)

		switch {
		case row.DaysLeft < 0:
			row.Status = LicenseExpiryExpired
			expiredCnt++
		case row.DaysLeft <= withinDays:
			row.Status = LicenseExpirySoon
			soonCnt++
		default:
			row.Status = LicenseExpiryValid
			validCnt++
		}

		if onlyAlert && row.Status == LicenseExpiryValid {
			continue
		}
		out = append(out, row)
	}

	rank := func(r alertRow) int {
		switch r.Status {
		case LicenseExpiryExpired:
			return 0
		case LicenseExpirySoon:
			return 1
		case LicenseExpiryValid:
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if rank(out[i]) != rank(out[j]) {
			return rank(out[i]) < rank(out[j])
		}
		return out[i].DaysLeft < out[j].DaysLeft
	})

	c.JSON(200, gin.H{
		"generatedAt": now,
		"withinDays":  withinDays,
		"counts": gin.H{
			"expired":  expiredCnt,
			"expiring": soonCnt,
			"valid":    validCnt,
			"noDate":   noDate,
			"alert":    expiredCnt + soonCnt,
		},
		"items": out,
	})
}

func UploadImportLicenseItems(c *gin.Context) {

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": "กรุณาแนบไฟล์ Excel หรือ CSV (field name: file)"})
		return
	}

	rows, _, err := readBestSheet(fileHeader, importLicenseSheetScore)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if len(rows) < 2 {
		c.JSON(400, gin.H{"message": "ไฟล์ไม่มีข้อมูล หรืออ่านไม่ได้"})
		return
	}

	headerIdx, headers := findImportLicenseHeader(rows)
	if headerIdx < 0 {
		if respondLedgerOnlyUpload(c, fileHeader, "Import") {
			return
		}
		c.JSON(400, gin.H{
			"message": "หาหัวตารางไม่เจอ — ไฟล์ต้องมีคอลัมน์ 'หมายเลขเครื่อง' และคอลัมน์อื่นอย่างน้อย 2 คอลัมน์",
		})
		return
	}

	userID, userName := lookupUserName(c)
	now := time.Now()

	hasRemarkCol := importLicenseHasRemarkColumn(headers)
	fallbackIssueDate := scanIssueDateFromHeaderBlock(rows, headerIdx)

	var (
		parsed   []models.LicenseItem
		seen     = map[string]bool{}
		skipped  int
		problems []string
	)

	dupSkip, dupProblems := findDuplicateKnownColumns(
		headers,
		func(k string) bool { _, ok := importLicenseColumns[k]; return ok },
		rows[headerIdx],
	)
	problems = append(problems, dupProblems...)

	// คอลัมน์ที่ระบบไม่รู้จัก — เก็บลง extra_json ให้ แล้วบอกผู้ใช้ตอนจบว่าเจออะไรบ้าง
	var extraCols extraColumnTracker

	for i := headerIdx + 1; i < len(rows); i++ {

		row := models.LicenseItem{
			Qty:           1,
			ConfirmStatus: models.LicenseItemPending,
			FileName:      clampRunes(fileHeader.Filename, 255),
			UploadDate:    now,
			UserID:        userID,
		}

		extra := map[string]string{}
		for col, header := range headers {
			if col >= len(rows[i]) {
				break
			}
			if dupSkip[col] {
				continue
			}
			val := strings.TrimSpace(rows[i][col])
			if setter, ok := importLicenseColumns[header]; ok {
				setter(&row, val)
				continue
			}
			if importIgnoredHeaders[header] {
				continue
			}
			label := ""
			if headerIdx >= 0 && headerIdx < len(rows) && col < len(rows[headerIdx]) {
				label = strings.TrimSpace(rows[headerIdx][col])
			}
			if label != "" {
				// จดชื่อคอลัมน์ไว้แม้แถวนี้จะเว้นว่าง เพราะคอลัมน์มีอยู่จริงในไฟล์
				extraCols.add(label)
				if val != "" {
					extra["[+] "+label] = val
				}
			}
		}
		if len(extra) > 0 {
			if b, err := json.Marshal(extra); err == nil {
				row.ExtraJSON = string(b)
			}
		}

		if row.IssueDate == nil {
			row.IssueDate = fallbackIssueDate
		}

		row.FillExpireDate()

		// สถานะ "เสร็จสิ้น" มาจากช่องหมายเหตุในไฟล์ — อัปโหลดแล้วขึ้นสถานะให้เลย
		// ไฟล์ที่ไม่มีคอลัมน์หมายเหตุจะไม่แตะสถานะเดิม (เติมค่าเก่ากลับตอนจับคู่ด้านล่าง)
		if hasRemarkCol {
			row.ApplyRemarkStatus(userName, now)
		}

		if row.MachineNo == "" {
			skipped++
			continue
		}

		if seen[row.MachineNo] {
			problems = append(problems, "แถว "+strconv.Itoa(i+1)+": หมายเลขเครื่อง "+row.MachineNo+" ซ้ำกันเองในไฟล์")
			continue
		}
		seen[row.MachineNo] = true

		parsed = append(parsed, row)
	}

	if len(parsed) == 0 {
		if respondLedgerOnlyUpload(c, fileHeader, "Import") {
			return
		}
		c.JSON(400, gin.H{"message": "ไม่พบแถวข้อมูลที่นำเข้าได้ในไฟล์นี้"})
		return
	}

	// แถวที่ไม่มีเลขใบอนุญาตจะนำเข้าได้ แต่จะไม่ขึ้นในหน้า License เพราะระบบจับกลุ่มด้วยเลขใบ
	// เคยมีเคสที่หัวคอลัมน์ในไฟล์เขียนไม่ตรงกับที่ระบบรู้จัก แล้วเลขใบหายไปเงียบ ๆ ทั้งไฟล์
	// จึงต้องแจ้งให้เห็นตั้งแต่ตอนอัปโหลด ไม่ใช่ปล่อยให้ไปงงที่หน้า License
	noLicenseNo := 0
	for i := range parsed {
		if strings.TrimSpace(parsed[i].LicenseNo) == "" {
			noLicenseNo++
		}
	}
	if noLicenseNo > 0 {
		problems = append(problems, "มี "+strconv.Itoa(noLicenseNo)+" แถวที่อ่านเลขใบอนุญาตนำเข้าไม่ได้ "+
			"— แถวเหล่านี้จะไม่ขึ้นในหน้าใบอนุญาต ตรวจหัวคอลัมน์ในไฟล์ว่าเขียนว่า 'เลขใบอนุญาตนำเข้า' หรือ 'Import License No.'")
	}

	matches, err := matchImportLicenseExisting(parsed)
	if err != nil {
		c.JSON(500, gin.H{"message": "อ่านข้อมูลเดิมไม่สำเร็จ: " + err.Error()})
		return
	}
	fileName := clampRunes(fileHeader.Filename, 255)
	deletes, err := importLicenseSyncDeletes(matches, fileName)
	if err != nil {
		c.JSON(500, gin.H{"message": "อ่านข้อมูลเดิมไม่สำเร็จ: " + err.Error()})
		return
	}
	sortBase := uploadSortBase(func() *gorm.DB { return config.DB.Model(&models.LicenseItem{}) }, fileName)
	var moves []rowReposition

	type pendingUpdate struct {
		id  uint
		row models.LicenseItem
	}
	var (
		toCreate      []models.LicenseItem
		toUpdate      []pendingUpdate
		toRekey       []pendingUpdate // แก้หมายเลขเครื่องใน Excel
		lockedSkipped int
		unchanged     int
	)
	locks := buildScanLockIndex()
	for i, row := range parsed {
		row.SortOrder = sortBase + int64(i)
		old := matches[i]
		if old == nil {
			toCreate = append(toCreate, row)
			continue
		}
		// ไฟล์ไม่มีคอลัมน์หมายเหตุ = ไม่ได้บอกอะไรเรื่องสถานะ → เก็บของเดิมไว้
		if !hasRemarkCol {
			row.Completed = old.Completed
			row.CompletedBy = old.CompletedBy
			row.CompletedAt = old.CompletedAt
		}
		if len(importLicenseDiffs(*old, row)) == 0 && extraJSONEqual(old.ExtraJSON, row.ExtraJSON) {
			unchanged++
			moves = append(moves, rowReposition{old.ID, row.SortOrder})
			continue
		}
		if locked, reason := locks.importLicense(old); locked {
			lockedSkipped++
			problems = append(problems, "หมายเลขเครื่อง "+old.MachineNo+": "+scanLockedMessage+" ("+reason+") — ไม่อัปเดตแถวนี้")
			moves = append(moves, rowReposition{old.ID, row.SortOrder})
			continue
		}
		if old.MachineNo != row.MachineNo {
			toRekey = append(toRekey, pendingUpdate{id: old.ID, row: row})
			continue
		}
		toUpdate = append(toUpdate, pendingUpdate{id: old.ID, row: row})
	}

	applyUpdate := func(db *gorm.DB, u pendingUpdate) error {
		row := u.row
		return db.Model(&models.LicenseItem{}).
			Where("id = ?", u.id).
			Updates(map[string]interface{}{
				"brand":          row.Brand,
				"model":          row.Model,
				"license_no":     row.LicenseNo,
				"invoice_no":     row.InvoiceNo,
				"declaration_no": row.DeclarationNo,
				"qty":            row.Qty,
				"production_no":  row.ProductionNo,
				"remark":         row.Remark,
				"completed":      row.Completed,
				"completed_by":   row.CompletedBy,
				"completed_at":   row.CompletedAt,
				"export_country": row.ExportCountry,
				// ใบอนุญาตนำออกที่ไฟล์ระบุไว้ท้ายแถว
				"export_license_no": row.ExportLicenseNo,
				"export_issue_date": row.ExportIssueDate,
				"issue_date":        row.IssueDate,
				"expire_date":       row.ExpireDate,
				"extra_json":        row.ExtraJSON,
				"file_name":         row.FileName,
				"upload_date":       now,
				"user_id":           userID,
				"machine_no":        row.MachineNo,
				"sort_order":        row.SortOrder,
			}).Error
	}

	var imported, updated int

	upsert := clause.OnConflict{
		Columns: []clause.Column{{Name: "machine_no"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"brand", "model", "license_no", "invoice_no", "declaration_no",
			"qty", "production_no", "remark", "export_country",
			"export_license_no", "export_issue_date", "issue_date", "expire_date",
			"completed", "completed_by", "completed_at",
			"extra_json", "file_name", "upload_date", "user_id", "sort_order",
		}),
	}
	for _, part := range chunkSlice(toUpdate, dbInsertBatch) {
		batch := make([]models.LicenseItem, len(part))
		for i, u := range part {
			batch[i] = u.row

			batch[i].ID = u.id
			batch[i].UploadDate = now
			batch[i].UserID = userID
		}
		if err := config.DB.Clauses(upsert).Create(&batch).Error; err == nil {
			updated += len(part)
			continue
		}
		for _, u := range part {
			if err := applyUpdate(config.DB, u); err != nil {
				problems = append(problems, "หมายเลขเครื่อง "+u.row.MachineNo+": อัปเดตไม่สำเร็จ ("+err.Error()+")")
				continue
			}
			updated++
		}
	}

	for _, u := range toRekey {
		if err := applyUpdate(config.DB, u); err != nil {
			problems = append(problems, "หมายเลขเครื่อง "+u.row.MachineNo+": อัปเดตไม่สำเร็จ (หมายเลขเครื่องอาจซ้ำกับรายการอื่น)")
			continue
		}
		updated++
	}

	if len(toUpdate) > 0 {
		SyncIdentityToMax(config.DB, &models.LicenseItem{})
	}

	for _, part := range chunkSlice(toCreate, dbInsertBatch) {
		if err := config.DB.Create(&part).Error; err == nil {
			imported += len(part)
			continue
		}
		for i := range part {
			row := part[i]
			row.ID = 0
			if err := config.DB.Create(&row).Error; err != nil {
				problems = append(problems, "หมายเลขเครื่อง "+row.MachineNo+": เพิ่มไม่สำเร็จ ("+err.Error()+")")
				continue
			}
			imported++
		}
	}

	if err := applyRepositions(config.DB, &models.LicenseItem{}, moves, fileName); err != nil {
		problems = append(problems, "อัปเดตลำดับแถวไม่สำเร็จ ("+err.Error()+")")
	}

	deleteIDs := syncDeleteIDs(deletes)
	_, lockedKept := countSyncDeletes(deletes)
	for _, d := range deletes {
		if d.locked {
			problems = append(problems, "หมายเลขเครื่อง "+d.label+": ไม่มีในไฟล์แล้ว แต่"+scanLockedMessage+" ("+d.reason+") — ไม่ลบแถวนี้")
		}
	}
	deleted := 0
	for _, part := range chunkSlice(deleteIDs, dbInsertBatch) {
		res := config.DB.Where("id IN ?", part).Delete(&models.LicenseItem{})
		if res.Error != nil {
			problems = append(problems, "ลบรายการที่ไม่มีในไฟล์ไม่สำเร็จ")
			continue
		}
		deleted += int(res.RowsAffected)
	}
	if deleted > 0 {
		InvalidateMachineIndex()
	}

	// ไฟล์เดียวกันมักมีชีต "ต่ออายุ" ติดมาด้วย — เก็บตารางทะเบียนไว้เลย
	// เพื่อให้สถานะ "เสร็จสิ้น" (NO. = Completed NN) ใช้งานได้โดยไม่ต้องอัปซ้ำอีกตัวเลือก
	if n, done := AbsorbLedgerSheet(fileHeader, userID, now); n > 0 {
		CreateAuditLog("LICENSE_RENEWAL", 0, "upload",
			fileHeader.Filename+": ทะเบียน "+strconv.Itoa(n)+" แถว · ปิดงานแล้ว "+
				strconv.Itoa(done)+" ใบ (จากไฟล์ Import)", userID, userName)
	}

	// Audit log: ใคร / เมื่อไร / ไฟล์อะไร / กี่รายการ / สำเร็จกี่ / error กี่
	CreateAuditLog("IMPORT_LICENSE", 0, "upload_excel",
		licenseUploadLogLine(fileHeader.Filename, imported+updated+unchanged+skipped+lockedSkipped,
			imported, updated, deleted, len(problems)),
		userID, userName)
	if deleted > 0 {
		CreateAuditLog("IMPORT_LICENSE", 0, "sync_delete", fileName+": "+strconv.Itoa(deleted), userID, userName)
	}

	c.JSON(201, gin.H{
		"imported":     imported,
		"updated":      updated,
		"deleted":      deleted,
		"lockedKept":   lockedKept,
		"unchanged":    unchanged,
		"locked":       lockedSkipped,
		"skipped":      skipped,
		"problems":     capProblems(problems),
		"file":         fileHeader.Filename,
		"extraColumns": extraCols.labels(),
		"extraNotice":  extraCols.notice(),
	})
}

func matchImportLicense(code, invoiceNo, productionNo string) (string, string, *models.LicenseItem) {

	code = strings.TrimSpace(code)
	if code == "" {
		return models.MatchStatusNotFound, "ไม่มีค่าที่สแกน", nil
	}

	var item models.LicenseItem
	err := config.DB.
		Where("machine_no = ? OR production_no = ?", code, code).
		First(&item).Error

	if err != nil {
		candidates := []string{ResolveMachineNo(code)}
		if alias := lookupCodeAlias("import_license", code); alias != nil {
			candidates = append(candidates, alias.ToOld)
		}

		found := false
		for _, alt := range dedupeCodes(candidates...) {
			if strings.EqualFold(alt, code) {
				continue
			}
			if e2 := config.DB.
				Where("machine_no = ? OR production_no = ?", alt, alt).
				First(&item).Error; e2 == nil {
				code = alt
				found = true
				break
			}
		}

		if !found {
			return models.MatchStatusNotFound,
				"ไม่พบ " + code + " ในบัญชีใบอนุญาตนำเข้า", nil
		}
	}

	if invoiceNo != "" && !strings.EqualFold(strings.TrimSpace(invoiceNo), item.InvoiceNo) {
		return models.MatchStatusWrongInv,
			"เลขเครื่องนี้อยู่ในอินวอยซ์ " + item.InvoiceNo + " ไม่ใช่ " + invoiceNo, &item
	}

	if productionNo != "" && item.ProductionNo != "" &&
		strings.TrimSpace(productionNo) != item.ProductionNo {
		return models.MatchStatusWrongProd,
			"หมายเลขการผลิตไม่ตรง — ในบัญชีคือ " + item.ProductionNo, &item
	}

	if item.ConfirmStatus == models.LicenseItemConfirmed {
		return models.MatchStatusDuplicate,
			"IT Controller นี้ถูกยืนยันไปแล้ว", &item
	}

	return models.MatchStatusMatch, "ตรงกับบัญชีใบอนุญาตนำเข้า", &item
}

type verifyImportLicenseRequest struct {
	Code         string `json:"code" binding:"required"`
	InvoiceNo    string `json:"invoiceNo"`
	ProductionNo string `json:"productionNo"`
}

func VerifyImportLicenseCode(c *gin.Context) {

	var req verifyImportLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	status, message, item := matchImportLicense(req.Code, req.InvoiceNo, req.ProductionNo)

	c.JSON(200, gin.H{
		"status":  status,
		"matched": status == models.MatchStatusMatch,
		"message": message,
		"item":    item,
	})
}

func PreviewImportLicenseMapping(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": "กรุณาแนบไฟล์ (field name: file)"})
		return
	}
	rows, _, err := readBestSheet(fileHeader, importLicenseSheetScore)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if len(rows) < 1 {
		c.JSON(400, gin.H{"message": "ไฟล์ไม่มีข้อมูล หรืออ่านไม่ได้"})
		return
	}

	headerIdx, headers := findImportLicenseHeader(rows)
	if headerIdx < 0 {
		c.JSON(200, gin.H{
			"file":        fileHeader.Filename,
			"headerFound": false,
			"message":     "รูปแบบข้อมูลที่อัพโหลดไม่ถูกต้อง",
		})
		return
	}

	var matched, extra []string
	seenTarget := map[string]bool{}
	for col, key := range headers {
		label := ""
		if col < len(rows[headerIdx]) {
			label = strings.TrimSpace(rows[headerIdx][col])
		}
		if _, ok := importLicenseColumns[key]; ok {
			if !seenTarget[key] {
				matched = append(matched, label)
				seenTarget[key] = true
			}
			continue
		}
		if label != "" && !importIgnoredHeaders[key] {
			extra = append(extra, label)
		}
	}

	fallbackIssueDate := scanIssueDateFromHeaderBlock(rows, headerIdx)
	hasRemarkCol := importLicenseHasRemarkColumn(headers)
	var newItems []models.LicenseItem
	seenMachine := map[string]bool{}
	dupSkip, _ := findDuplicateKnownColumns(
		headers,
		func(k string) bool { _, ok := importLicenseColumns[k]; return ok },
		rows[headerIdx],
	)
	for i := headerIdx + 1; i < len(rows); i++ {
		it := models.LicenseItem{Qty: 1}
		for col, header := range headers {
			if col >= len(rows[i]) {
				break
			}
			if dupSkip[col] {
				continue
			}
			val := strings.TrimSpace(rows[i][col])
			if setter, ok := importLicenseColumns[header]; ok {
				setter(&it, val)
			}
		}
		if it.IssueDate == nil {
			it.IssueDate = fallbackIssueDate
		}
		it.FillExpireDate()
		// ให้ preview ตีความสถานะเสร็จสิ้นแบบเดียวกับตอนอัปโหลดจริง
		if hasRemarkCol {
			it.ApplyRemarkStatus("", time.Now())
		}
		if it.MachineNo == "" || seenMachine[it.MachineNo] {
			continue
		}
		seenMachine[it.MachineNo] = true
		newItems = append(newItems, it)
	}

	matches, err := matchImportLicenseExisting(newItems)
	if err != nil {
		c.JSON(500, gin.H{"message": "อ่านข้อมูลเดิมไม่สำเร็จ: " + err.Error()})
		return
	}
	deletes, err := importLicenseSyncDeletes(matches, clampRunes(fileHeader.Filename, 255))
	if err != nil {
		c.JSON(500, gin.H{"message": "อ่านข้อมูลเดิมไม่สำเร็จ: " + err.Error()})
		return
	}
	deleteCount, deleteLocked := countSyncDeletes(deletes)

	type fieldDiff struct {
		Field string `json:"field"`
		Old   string `json:"old"`
		New   string `json:"new"`
	}
	type rowResult struct {
		Key    string      `json:"key"`
		Status string      `json:"status"`
		Diffs  []fieldDiff `json:"diffs,omitempty"`
	}
	counts := map[string]int{"NEW": 0, "UPDATED": 0, "CHANGED": 0, "UNCHANGED": 0, "LOCKED": 0}
	preview := make([]rowResult, 0, 300)
	locks := buildScanLockIndex()
	for _, d := range deletes {
		if len(preview) >= 300 {
			break
		}
		status := "DELETE"
		if d.locked {
			status = "DELETE_LOCKED"
		}
		preview = append(preview, rowResult{Key: d.label, Status: status})
	}

	for i, it := range newItems {
		oldPtr := matches[i]
		if oldPtr == nil {
			counts["NEW"]++
			if len(preview) < 300 {
				preview = append(preview, rowResult{Key: it.MachineNo, Status: "NEW"})
			}
			continue
		}
		old := *oldPtr
		if !hasRemarkCol {
			it.Completed = old.Completed
		}
		var diffs []fieldDiff
		coreChanged := false
		for _, d := range importLicenseDiffs(old, it) {
			diffs = append(diffs, fieldDiff{Field: d[0], Old: d[1], New: d[2]})
			if importLicenseCoreFields[d[0]] {
				coreChanged = true
			}
		}
		locked, _ := locks.importLicense(&old)

		var status string
		switch {
		case len(diffs) == 0 && extraJSONEqual(old.ExtraJSON, it.ExtraJSON):
			status = "UNCHANGED"
		case locked:
			status = "LOCKED"
		case coreChanged:
			status = "CHANGED"
		default:
			status = "UPDATED"
		}
		counts[status]++
		if status != "UNCHANGED" && len(preview) < 300 {
			preview = append(preview, rowResult{Key: it.MachineNo, Status: status, Diffs: diffs})
		}
	}

	total := counts["NEW"] + counts["UPDATED"] + counts["CHANGED"] + counts["UNCHANGED"] + counts["LOCKED"]

	c.JSON(200, gin.H{
		"file":        fileHeader.Filename,
		"headerFound": true,
		"headerRow":   headerIdx + 1,
		"matched":     matched,
		"extra":       extra,
		"keyLabel":    "หมายเลขเครื่อง",
		"coreFields":  []string{"เลขใบอนุญาต", "อินวอยซ์", "หมายเลขการผลิต", "แบบ/รุ่น"},
		"summary": gin.H{
			"total":        total,
			"new":          counts["NEW"],
			"updated":      counts["UPDATED"],
			"changed":      counts["CHANGED"],
			"unchanged":    counts["UNCHANGED"],
			"locked":       counts["LOCKED"],
			"deleted":      deleteCount,
			"deleteLocked": deleteLocked,
		},
		"rows": preview,
	})
}

var importLicenseCoreFields = map[string]bool{"หมายเลขเครื่อง": true, "เลขใบอนุญาต": true, "อินวอยซ์": true, "หมายเลขการผลิต": true, "แบบ/รุ่น": true}

func fmtDatePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// importLicenseDiffs เทียบค่าในไฟล์กับของเดิม
func importLicenseDiffs(old, cur models.LicenseItem) [][3]string {
	var out [][3]string
	add := func(field, o, n string) {
		if strings.TrimSpace(o) != strings.TrimSpace(n) {
			out = append(out, [3]string{field, o, n})
		}
	}
	add("หมายเลขเครื่อง", old.MachineNo, cur.MachineNo)
	add("เลขใบอนุญาต", old.LicenseNo, cur.LicenseNo)
	add("อินวอยซ์", old.InvoiceNo, cur.InvoiceNo)
	add("หมายเลขการผลิต", old.ProductionNo, cur.ProductionNo)
	add("แบบ/รุ่น", old.Model, cur.Model)
	add("ตราอักษร", old.Brand, cur.Brand)
	add("ใบขนสินค้า", old.DeclarationNo, cur.DeclarationNo)
	add("จำนวน", strconv.Itoa(old.Qty), strconv.Itoa(cur.Qty))
	add("วันที่ออกใบอนุญาต", fmtDatePtr(old.IssueDate), fmtDatePtr(cur.IssueDate))
	add("ส่งออกไปประเทศ", old.ExportCountry, cur.ExportCountry)
	add("เลขใบอนุญาตนำออก", old.ExportLicenseNo, cur.ExportLicenseNo)
	add("วันที่ออกใบอนุญาตนำออก", fmtDatePtr(old.ExportIssueDate), fmtDatePtr(cur.ExportIssueDate))
	add("หมายเหตุ", old.Remark, cur.Remark)
	// สถานะเสร็จสิ้นคำนวณจากหมายเหตุ — ถ้าของเดิมในฐานข้อมูลไม่ตรงกับที่ไฟล์บอก
	// ต้องถือว่าเป็นการเปลี่ยนแปลง ไม่งั้นแถวนั้นจะถูกข้ามเป็น "ไม่มีอะไรเปลี่ยน"
	// (สำคัญกับข้อมูลเก่าที่อัปโหลดไว้ก่อนระบบจะอ่านสถานะจากหมายเหตุ)
	add("สถานะเสร็จสิ้น", completedLabel(old.Completed), completedLabel(cur.Completed))
	return out
}

func completedLabel(v bool) string {
	if v {
		return "เสร็จสิ้น"
	}
	return "ยังไม่เสร็จ"
}

// UpdateImportLicenseItem แก้ไขรายการ Import License จากตาราง (ส่งมาเฉพาะช่องที่แก้)
// แถวที่สแกนยืนยันแล้วจะได้ 409 (locked)
func UpdateImportLicenseItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.LicenseItem
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}
	if locked, reason := buildScanLockIndex().importLicense(&row); locked {
		respondLocked(c, reason)
		return
	}

	var body map[string]*string
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	for key, ptr := range body {
		if ptr == nil {
			continue
		}
		v := strings.TrimSpace(*ptr)
		switch key {
		case "Brand":
			updates["brand"] = v
		case "Model":
			updates["model"] = v
		case "LicenseNo":
			updates["license_no"] = v
		case "InvoiceNo":
			updates["invoice_no"] = v
		case "DeclarationNo":
			updates["declaration_no"] = v
		case "Qty":
			updates["qty"] = atoiSafe(v)
		case "MachineNo":
			v = normalizeDigitCell(v)
			if v == "" {
				c.JSON(400, gin.H{"message": "หมายเลขเครื่องต้องไม่ว่าง"})
				return
			}
			updates["machine_no"] = v
		case "ProductionNo":
			updates["production_no"] = normalizeDigitCell(v)
		case "Remark":
			updates["remark"] = v
			// แก้หมายเหตุในตาราง = แก้สถานะเสร็จสิ้นไปด้วย ให้ผลเหมือนอัปโหลดไฟล์
			probe := models.LicenseItem{
				Remark:      v,
				CompletedBy: row.CompletedBy,
				CompletedAt: row.CompletedAt,
			}
			_, editorName := lookupUserName(c)
			probe.ApplyRemarkStatus(editorName, time.Now())
			updates["completed"] = probe.Completed
			updates["completed_by"] = probe.CompletedBy
			updates["completed_at"] = probe.CompletedAt
		case "ExportCountry":
			updates["export_country"] = v
		case "IssueDate":
			d := parseLicenseDate(v)
			if v != "" && d == nil {
				c.JSON(400, gin.H{"message": "รูปแบบวันที่ไม่ถูกต้อง: " + v})
				return
			}
			updates["issue_date"] = d
			if d != nil {
				exp := d.AddDate(0, models.ImportLicenseValidityMonths, 0)
				updates["expire_date"] = &exp
			} else {
				updates["expire_date"] = nil
			}
		default:
			c.JSON(400, gin.H{"message": "แก้ไขช่อง " + key + " ไม่ได้"})
			return
		}
	}
	if len(updates) == 0 {
		c.JSON(400, gin.H{"message": "ไม่มีช่องที่ต้องแก้ไข"})
		return
	}

	if err := config.DB.Model(&models.LicenseItem{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		log.Printf("update license_items id=%d: %v", id, err)
		c.JSON(400, gin.H{"message": "อัปเดตไม่สำเร็จ"})
		return
	}

	userID, userName := lookupUserName(c)
	InvalidateMachineIndex()
	CreateAuditLog("IMPORT_LICENSE", uint(id), "edit", row.MachineNo, userID, userName)

	var out models.LicenseItem
	config.DB.First(&out, id)
	out.Locked, out.LockReason = buildScanLockIndex().importLicense(&out)
	c.JSON(200, out)
}

func DeleteImportLicenseItem(c *gin.Context) {

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.LicenseItem
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	if locked, reason := buildScanLockIndex().importLicense(&row); locked {
		respondLocked(c, reason)
		return
	}

	if err := config.DB.Delete(&models.LicenseItem{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, userName := lookupUserName(c)
	CreateAuditLog("IMPORT_LICENSE", row.ID, "delete", row.MachineNo, userID, userName)

	c.JSON(200, gin.H{"deleted": true})
}

func ClearImportLicenseItems(c *gin.Context) {

	licenseNo := strings.TrimSpace(c.Query("license_no"))
	invoiceNo := strings.TrimSpace(c.Query("invoice_no"))
	_, hasLicense := c.GetQuery("license_no")
	_, hasInvoice := c.GetQuery("invoice_no")
	deleteAll := strings.EqualFold(strings.TrimSpace(c.Query("all")), "true")

	userID, userName := lookupUserName(c)

	if deleteAll {
		res := config.DB.Where("1 = 1").Delete(&models.LicenseItem{})
		if res.Error != nil {
			c.JSON(500, gin.H{"message": res.Error.Error()})
			return
		}
		ResetIdentityIfEmpty(config.DB, &models.LicenseItem{})
		CreateAuditLog("IMPORT_LICENSE", 0, "clear_all", "ALL", userID, userName)
		c.JSON(200, gin.H{"deleted": res.RowsAffected})
		return
	}

	if !hasLicense && !hasInvoice {
		c.JSON(400, gin.H{"message": "ต้องระบุล็อตที่จะลบ (license_no และ/หรือ invoice_no) หรือส่ง all=true เพื่อลบทั้งหมด"})
		return
	}

	tx := config.DB
	if hasLicense {
		tx = tx.Where("license_no = ?", licenseNo)
	}
	if hasInvoice {
		tx = tx.Where("invoice_no = ?", invoiceNo)
	}

	res := tx.Delete(&models.LicenseItem{})
	if res.Error != nil {
		c.JSON(500, gin.H{"message": res.Error.Error()})
		return
	}

	ResetIdentityIfEmpty(config.DB, &models.LicenseItem{})
	CreateAuditLog("IMPORT_LICENSE", 0, "clear_license",
		"license_no="+licenseNo+" invoice_no="+invoiceNo, userID, userName)

	c.JSON(200, gin.H{"deleted": res.RowsAffected})
}

func RenewImportLicense(c *gin.Context) {
	var req struct {
		LicenseNo string `json:"licenseNo"`
		InvoiceNo string `json:"invoiceNo"`
		Days      int    `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}

	licenseNo := strings.TrimSpace(req.LicenseNo)
	invoiceNo := strings.TrimSpace(req.InvoiceNo)
	if req.Days <= 0 {
		c.JSON(400, gin.H{"message": "จำนวนวันที่ต่อต้องมากกว่า 0"})
		return
	}
	if req.Days > 3650 {
		c.JSON(400, gin.H{"message": "จำนวนวันที่ต่อมากเกินไป (สูงสุด 3650 วัน)"})
		return
	}

	var rows []models.LicenseItem
	if err := config.DB.
		Where("license_no = ? AND invoice_no = ?", licenseNo, invoiceNo).
		Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	if len(rows) == 0 {
		c.JSON(404, gin.H{"message": "ไม่พบล็อตใบอนุญาตนี้"})
		return
	}

	now := time.Now()
	noDateBase := now.AddDate(0, -LicenseValidityMonths, 0)

	updated := 0
	for i := range rows {
		base := noDateBase
		if rows[i].IssueDate != nil {
			base = *rows[i].IssueDate
		}
		newIssue := base.AddDate(0, 0, req.Days)
		newExpire := newIssue.AddDate(0, LicenseValidityMonths, 0)

		if err := config.DB.Model(&models.LicenseItem{}).
			Where("id = ?", rows[i].ID).
			Updates(map[string]interface{}{
				"issue_date":  newIssue,
				"expire_date": newExpire,
			}).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		rows[i].IssueDate = &newIssue
		rows[i].ExpireDate = &newExpire
		updated++
	}

	newExpiry := rows[0].IssueDate.AddDate(0, LicenseValidityMonths, 0)

	userID, userName := lookupUserName(c)
	CreateAuditLog("IMPORT_LICENSE", 0, "renew",
		"license_no="+licenseNo+" invoice_no="+invoiceNo+" days="+strconv.Itoa(req.Days),
		userID, userName)

	c.JSON(200, gin.H{
		"renewed":   updated,
		"days":      req.Days,
		"newExpiry": newExpiry,
	})
}