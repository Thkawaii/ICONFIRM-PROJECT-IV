package controllers

import (
	"encoding/json"
	"errors"
	"math"
	"mime/multipart"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// ชีต "ต่ออายุ" — ทะเบียนใบอนุญาต
//
//	NO. · IT CONTROLLER MODEL · IMPORT LICENSE NO. · COUNTRY · TOTAL ·
//	EXPORT LICENSE NO. · DATE · EXPIRE DATE · STOCK · REMAIN ·
//	Date of E-mail Sending · Payment Date · Received Document Date
//
// ผู้ใช้ยังทำงานใน Excel ตามเดิม ระบบอ่านมาเพื่อเช็คสถานะอย่างเดียว
// อัปโหลดแบบซิงก์ตามชื่อไฟล์ (ดู license_register_merge.go): แถวที่เปลี่ยนจะอัปเดต
// แถวที่หายจากไฟล์ชื่อเดิมจะถูกลบ ไฟล์ชื่อใหม่จะเพิ่มเข้าไป
// ---------------------------------------------------------------------------

var errNoRenewalSheet = errors.New(
	"ไม่พบชีตทะเบียนใบอนุญาต (ต่ออายุ) ในไฟล์นี้ — ต้องมีหัวตารางที่มี IMPORT LICENSE และ EXPORT LICENSE")

// renewalHeaders: หัวคอลัมน์ (normalize แล้ว) → ชื่อช่องที่ใช้ภายใน
var renewalHeaders = map[string]string{
	"no":                                  "no",
	"itcontrollermodel":                   "model",
	"importlicenseitcontrollerno":         "importNo",
	"importlicenseno":                     "importNo",
	"country":                             "country",
	"total":                               "total",
	"exportlicenseitcontrollerno":         "exportNo",
	"exportlicenseno":                     "exportNo",
	"exportlicenseitcontrollerdate":       "issueDate",
	"exportlicensedate":                   "issueDate",
	"exportlicenseitcontrollerexpiredate": "expireDate",
	"exportlicenseexpiredate":             "expireDate",
	"expiredate":                          "expireDate",
	"stockexportlicense":                  "stock",
	"stockexportlicenseitcontroller":      "stock",
	"stockexportlicenseitcontrollerno":    "stock",
	"stock":                               "stock",
	"stockqty":                            "stock",
	"remain":                              "remain",
	"remaining":                           "remain",
	"remainqty":                           "remain",
	"remainquantity":                      "remain",
	"คงเหลือ":                             "remain",
	"จำนวนคงเหลือ":                        "remain",
	"dateofemailsending":                  "emailDate",
	"paymentdate":                         "paymentDate",
	"receiveddocumentdate":                "receivedDate",
}

// findRenewalHeader: หาแถวหัวตารางของชีตทะเบียนใบอนุญาต
func findRenewalHeader(rows [][]string) (int, map[int]string) {
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
		if k, ok := renewalHeaders[n]; ok {
			return k, true
		}
		if t, ok := aliasRev[n]; ok {
			if k, ok := renewalHeaders[t]; ok {
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
		// ต้องมีทั้งเลขใบนำเข้าและเลขใบนำออก จึงจะใช่ชีตนี้
		if taken["importNo"] && taken["exportNo"] && len(layout) >= 5 {
			return i, layout
		}
	}
	return -1, nil
}

// renewalInt: แปลงค่าในช่อง TOTAL / STOCK / REMAIN เป็นจำนวนเต็ม
//
// ช่องพวกนี้ใน Excel มักจัดรูปแบบเป็น Accounting / มีสูตร excelize จึงคืนข้อความตามที่แสดงบนจอ เช่น
//
//	"(10)"   = -10 (ติดลบแบบบัญชี)    " -   " = 0 (ศูนย์แสดงเป็นขีด)
//	"10-"    = -10                     "1,234.00 " = 1234     "−10" (ขีดยูนิโค้ด) = -10
//
// เดิมอ่านได้เฉพาะตัวเลขล้วน ค่าติดลบแบบ (10) จึงกลายเป็น 0 ทั้งที่ REMAIN ของกลุ่ม Completed
// มักติดลบจากการปรับยอดหน้างาน
func renewalInt(v string) int {
	n, _ := renewalIntValue(v)
	return n
}

// renewalIntValue: เหมือน renewalInt แต่บอกด้วยว่าช่องนั้นมีตัวเลขกรอกไว้จริงไหม
// ช่องว่าง / ขีด / ข้อความที่อ่านเป็นเลขไม่ได้ → (0, false)
func renewalIntValue(v string) (int, bool) {
	s := strings.TrimSpace(unwrapExcelText(v))
	s = strings.NewReplacer("\u00a0", " ", "\u2212", "-", "\u2013", "-", "\u2014", "-", ",", "", " ", "", "฿", "", "$", "").Replace(s)
	if s == "" || s == "-" {
		return 0, false
	}
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg = true
		s = s[1 : len(s)-1]
	}
	if strings.HasSuffix(s, "-") {
		neg = !neg
		s = strings.TrimSuffix(s, "-")
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		n := int(math.Round(f))
		if neg {
			n = -n
		}
		return n, true
	}
	return 0, false
}

// renewalLicenseNoMaxLen = ความยาวสูงสุดของคอลัมน์เลขใบอนุญาตในฐานข้อมูล
const renewalLicenseNoMaxLen = 60

// renewalLicenseNo: อ่านค่าในช่องเลขใบอนุญาต และคัดข้อความที่ไม่ใช่เลขใบออก
//
// ในไฟล์จริง ช่อง "EXPORT LICENSE IT CONTROLLER NO." บางแถวถูกใช้จดบันทึกแทน
// เช่น "วันที่ส่ง E-MAIL : THU 19/12/2024 16:50 / เจ้าหน้าที่หลุดเมลล์..." ยาวถึง 371 ตัวอักษร
//
// ของเดิมยัดค่านี้ลงคอลัมน์ varchar(60) ตรง ๆ ฐานข้อมูลจึงปฏิเสธทั้งชุด
// (value too long for type character varying) ทำให้ตารางทะเบียนว่างเปล่าทั้งตาราง
// และ STOCK / คงเหลือ ขึ้นเป็นขีดทุกใบ ทั้งที่ไฟล์มีข้อมูลครบ
//
// เลขใบอนุญาตจริงเป็นรหัสสั้น ๆ ไม่มีช่องว่างและไม่มีภาษาไทย
// ค่าที่ไม่เข้าเกณฑ์จึงถือว่าเป็นหมายเหตุ ไม่ใช่เลขใบ แล้วข้ามไป
func renewalLicenseNo(v string) string {
	s := strings.ToUpper(strings.TrimSpace(v))
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > renewalLicenseNoMaxLen {
		return ""
	}
	for _, r := range s {
		// ตัวอักษรไทยหรืออักขระนอกช่วง ASCII = เป็นข้อความบันทึก ไม่ใช่รหัสใบอนุญาต
		if r > unicode.MaxASCII {
			return ""
		}
	}
	return s
}

// readLicenseRenewalSheets: อ่านชีตทะเบียนใบอนุญาตจากไฟล์ที่อัปโหลด
func readLicenseRenewalSheets(fileHeader *multipart.FileHeader) ([]models.LicenseRenewal, bool, error) {
	rows, ok, _, err := readLicenseRenewalSheetsEx(fileHeader)
	return rows, ok, err
}

// readLicenseRenewalSheetsEx: เหมือน readLicenseRenewalSheets แต่คืนรายชื่อคอลัมน์ที่ไม่รู้จักมาด้วย
//
// แยกเป็นอีกตัวเพราะฟังก์ชันเดิมถูกเรียกจาก AbsorbLedgerSheet ตอนอัป Import/Export ด้วย
// ซึ่งตรงนั้นรายงานคอลัมน์ของชีตต่ออายุไปปนกับของชีตหลักไม่ได้
func readLicenseRenewalSheetsEx(fileHeader *multipart.FileHeader) ([]models.LicenseRenewal, bool, *extraColumnTracker, error) {
	extraCols := &extraColumnTracker{}
	// ฟังก์ชันนี้ถูกเรียกทุกครั้งที่อัป Import และ Export (ผ่าน AbsorbLedgerSheet)
	// ของเดิมอ่านทุกชีตให้ครบก่อนแล้วค่อยหาว่าชีตไหนเป็นชีตทะเบียน
	// ไฟล์เล่มเดียวจึงถูกแกะซ้ำหลายรอบต่อการอัปโหลดหนึ่งครั้ง
	// ตอนนี้ดูหัวตารางจากหัว ๆ ชีตก่อน แล้วอ่านเต็มเฉพาะชีตทะเบียน
	wb, err := openUploadedWorkbook(fileHeader)
	if err != nil {
		return nil, false, extraCols, err
	}
	defer wb.Close()

	var out []models.LicenseRenewal
	found := false
	var order int64

	for _, name := range wb.Names() {
		if idx, _ := findRenewalHeader(wb.Probe(name)); idx < 0 {
			continue
		}

		// ขยายเซลล์ที่ merge (NO. / เลขใบนำเข้า / TOTAL) ลงทุกแถวของโซ่ ไม่งั้นแถวล่างของกลุ่ม Completed
		// จะไม่มีเลขใบนำเข้าและชื่อกลุ่ม แล้ว STOCK / คงเหลือ ขึ้นเป็นขีด
		sheetRows, err := wb.Rows(name, true)
		if err != nil {
			return nil, false, extraCols, err
		}
		headerIdx, layout := findRenewalHeader(sheetRows)
		if headerIdx < 0 {
			continue
		}
		found = true
		for r := headerIdx + 1; r < len(sheetRows); r++ {
			get := func(key string) string {
				for c, k := range layout {
					if k == key {
						return strings.TrimSpace(unwrapExcelText(cellAt(sheetRows, r, c)))
					}
				}
				return ""
			}

			rawImport, rawExport := get("importNo"), get("exportNo")
			importNo := renewalLicenseNo(rawImport)
			exportNo := renewalLicenseNo(rawExport)
			// ค่าที่ไม่ใช่เลขใบคือข้อความที่ผู้ใช้จดไว้ — เก็บไว้เป็นหมายเหตุ ไม่ทิ้ง
			note := ""
			if importNo == "" && strings.TrimSpace(rawImport) != "" {
				note = strings.TrimSpace(rawImport)
			}
			if exportNo == "" && strings.TrimSpace(rawExport) != "" {
				if note != "" {
					note += " / "
				}
				note += strings.TrimSpace(rawExport)
			}
			if importNo == "" && exportNo == "" && note == "" {
				continue
			}

			order++
			remainValue, remainFilled := renewalIntValue(get("remain"))

			// คอลัมน์ที่ไม่อยู่ใน layout คือคอลัมน์ที่ระบบไม่รู้จัก — เก็บไว้ ไม่ทิ้ง
			extra := map[string]string{}
			for c := 0; c < len(sheetRows[headerIdx]); c++ {
				if layout[c] != "" {
					continue
				}
				label := strings.TrimSpace(unwrapExcelText(cellAt(sheetRows, headerIdx, c)))
				if label == "" {
					continue
				}
				extraCols.add(label)
				if v := strings.TrimSpace(unwrapExcelText(cellAt(sheetRows, r, c))); v != "" {
					extra["[+] "+label] = v
				}
			}
			extraJSON := ""
			if len(extra) > 0 {
				if b, err := json.Marshal(extra); err == nil {
					extraJSON = string(b)
				}
			}

			out = append(out, models.LicenseRenewal{
				GroupNo:           clampRunes(get("no"), 50),
				ITControllerModel: clampRunes(strings.ToUpper(get("model")), 60),
				ImportLicenseNo:   importNo,
				ExportLicenseNo:   exportNo,
				Country:           clampRunes(get("country"), 120),
				Note:              note,
				Total:             renewalInt(get("total")),
				IssueDate:         parseLicenseDate(get("issueDate")),
				ExpireDate:        parseLicenseDate(get("expireDate")),
				Stock:             renewalInt(get("stock")),
				Remain:            remainValue,
				HasRemain:         remainFilled,
				EmailDate:         parseLicenseDate(get("emailDate")),
				PaymentDate:       parseLicenseDate(get("paymentDate")),
				ReceivedDate:      parseLicenseDate(get("receivedDate")),
				ExtraJSON:         extraJSON,
				SortOrder:         order,
			})
		}
	}

	if !found {
		return nil, false, extraCols, nil
	}
	return out, true, extraCols, nil
}

// enrichLicenseRenewals: เติมสถานะ / ครั้งที่ต่ออายุ / จำนวนวันที่เหลือ
func enrichLicenseRenewals(rows []models.LicenseRenewal) {
	today := time.Now().Truncate(24 * time.Hour)

	// นับครั้งที่ต่ออายุของแต่ละใบนำเข้า (เรียงตามลำดับแถวในไฟล์)
	//
	// นับเป็น "ขั้น" ไม่ใช่นับเป็นแถว เพราะใบที่ออกวันเดียวกันคือการแบ่งโควต้า
	// ตามประเทศ ไม่ใช่การต่ออายุคนละครั้ง (ดู ledgerSteps)
	roundOfRow := map[uint]int{}
	count := map[string]int{}
	{
		byChain := map[string][]models.LicenseRenewal{}
		chainOrder := []string{}
		for _, r := range rows {
			if strings.TrimSpace(r.ExportLicenseNo) == "" {
				continue
			}
			k := ledgerGroupKey(r)
			if _, seen := byChain[k]; !seen {
				chainOrder = append(chainOrder, k)
			}
			byChain[k] = append(byChain[k], r)
		}
		for _, k := range chainOrder {
			// แต่ละประเทศเดินต่ออายุของตัวเอง ครั้งที่ของสาย INDONESIA
			// ไม่ได้ต่อจากสาย MALAYSIA จึงต้องนับแยกสาย
			for _, bidx := range ledgerBranchIndexes(byChain[k]) {
				steps := ledgerSteps(ledgerPick(byChain[k], bidx))
				for si, step := range steps {
					for _, r := range step {
						roundOfRow[r.ID] = si + 1
						// ต่ออายุไปกี่ครั้ง = สายที่ยาวที่สุดของใบนำเข้านั้น
						if len(steps) > count[r.ImportLicenseNo] {
							count[r.ImportLicenseNo] = len(steps)
						}
					}
				}
			}
		}
	}
	for i := range rows {
		key := rows[i].ImportLicenseNo
		rows[i].RenewalCount = count[key]
		if strings.TrimSpace(rows[i].ExportLicenseNo) != "" {
			rows[i].RenewalRound = roundOfRow[rows[i].ID]
		}

		rows[i].RenewStep = models.RenewStepNone
		switch {
		case rows[i].ReceivedDate != nil:
			rows[i].RenewStep = models.RenewStepReceived
		case rows[i].PaymentDate != nil:
			rows[i].RenewStep = models.RenewStepPayment
		case rows[i].EmailDate != nil:
			rows[i].RenewStep = models.RenewStepEmail
		}

		rows[i].Status = licenseRenewalStatus(&rows[i], today)
	}
}

func licenseRenewalStatus(r *models.LicenseRenewal, today time.Time) string {
	if strings.TrimSpace(r.ExportLicenseNo) == "" {
		return models.LicenseStatusNoLicense
	}
	if r.ExpireDate == nil {
		return models.LicenseStatusNoDate
	}

	days := int(r.ExpireDate.Truncate(24*time.Hour).Sub(today).Hours() / 24)
	r.DaysLeft = &days

	if days < 0 {
		return models.LicenseStatusExpired
	}
	// ยังไม่หมดอายุ แต่โควต้าหมดก่อน — เป็นตัวที่ทำให้ส่งของไม่ได้จริง
	if r.Remain < 0 {
		return models.LicenseStatusOverUsed
	}
	if r.Remain == 0 {
		return models.LicenseStatusUsedUp
	}
	if days <= models.LicenseExpiringWithinDays {
		return models.LicenseStatusExpiring
	}
	return models.LicenseStatusValid
}

// UploadLicenseRenewals: POST /license-renewal/upload
func UploadLicenseRenewals(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": errUploadNoFile.Error()})
		return
	}

	rows, ok, extraCols, err := readLicenseRenewalSheetsEx(fileHeader)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if !ok {
		c.JSON(400, gin.H{"message": errNoRenewalSheet.Error()})
		return
	}
	if len(rows) == 0 {
		c.JSON(400, gin.H{"message": "พบชีตทะเบียนใบอนุญาตแต่ไม่มีข้อมูลในตาราง"})
		return
	}

	userID, name := lookupUserName(c)
	now := time.Now()

	res, err := mergeLicenseRegister(rows, fileHeader.Filename, userID, now)
	if err != nil {
		c.JSON(500, gin.H{"message": "บันทึกทะเบียนไม่สำเร็จ: " + err.Error()})
		return
	}

	var manual int64
	config.DB.Model(&models.LicenseRenewal{}).Where("manual_entry IS TRUE").Count(&manual)

	CreateAuditLog("LICENSE_RENEWAL", 0, "upload",
		fileHeader.Filename+": เพิ่ม "+strconv.Itoa(res.Created)+
			" · อัปเดต "+strconv.Itoa(res.Updated)+" · ลบ "+strconv.Itoa(res.Deleted),
		userID, name)

	all := loadLicenseRenewals()
	c.JSON(200, gin.H{
		"imported":     res.Created,
		"updated":      res.Updated,
		"deleted":      res.Deleted,
		"keptManual":   manual,
		"summary":      licenseRenewalSummary(all),
		"message":      registerSyncMessage(res, manual),
		"extraColumns": extraCols.labels(),
		"extraNotice":  extraCols.notice(),
	})
}

type LicenseRenewalSummary struct {
	Rows int `json:"rows"`
	// ImportLicenses = จำนวนใบอนุญาตนำเข้าทั้งหมดในทะเบียน
	ImportLicenses int `json:"importLicenses"`

	Valid     int `json:"valid"`
	Expiring  int `json:"expiring"`
	Expired   int `json:"expired"`
	UsedUp    int `json:"usedUp"`
	OverUsed  int `json:"overUsed"`
	NoLicense int `json:"noLicense"`

	// Active = ใบที่ยังใช้ได้อยู่ตอนนี้ (ยังไม่หมดอายุ)
	Active int `json:"active"`
	// WaitingRenew = ส่งเรื่องต่ออายุแล้วแต่ยังไม่ได้เอกสาร
	WaitingRenew int `json:"waitingRenew"`
}

func licenseRenewalSummary(rows []models.LicenseRenewal) LicenseRenewalSummary {
	s := LicenseRenewalSummary{Rows: len(rows)}
	imports := map[string]bool{}
	for i := range rows {
		if v := strings.TrimSpace(rows[i].ImportLicenseNo); v != "" {
			imports[v] = true
		}
		switch rows[i].Status {
		case models.LicenseStatusValid:
			s.Valid++
			s.Active++
		case models.LicenseStatusExpiring:
			s.Expiring++
			s.Active++
		case models.LicenseStatusExpired:
			s.Expired++
		case models.LicenseStatusUsedUp:
			s.UsedUp++
			s.Active++
		case models.LicenseStatusOverUsed:
			s.OverUsed++
			s.Active++
		case models.LicenseStatusNoLicense:
			s.NoLicense++
		}
		if rows[i].RenewStep == models.RenewStepEmail || rows[i].RenewStep == models.RenewStepPayment {
			s.WaitingRenew++
		}
	}
	s.ImportLicenses = len(imports)
	return s
}

func loadLicenseRenewals() []models.LicenseRenewal {
	var rows []models.LicenseRenewal
	config.DB.Order("sort_order asc, id asc").Find(&rows)
	enrichLicenseRenewals(rows)
	return rows
}

// GetLicenseRenewals: GET /license-renewal
// คืนทั้งรายแถว และสรุปรายใบอนุญาตนำเข้า (ใบไหนต่ออายุไปกี่ครั้ง ใบล่าสุดหมดเมื่อไหร่)
func GetLicenseRenewals(c *gin.Context) {
	rows := loadLicenseRenewals()

	kw := strings.ToUpper(strings.TrimSpace(c.Query("keyword")))
	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))

	filtered := make([]models.LicenseRenewal, 0, len(rows))
	for i := range rows {
		if status != "" && status != "ALL" && rows[i].Status != status {
			continue
		}
		if kw != "" &&
			!strings.Contains(strings.ToUpper(rows[i].ImportLicenseNo), kw) &&
			!strings.Contains(strings.ToUpper(rows[i].ExportLicenseNo), kw) &&
			!strings.Contains(strings.ToUpper(rows[i].ITControllerModel), kw) &&
			!strings.Contains(strings.ToUpper(rows[i].Country), kw) &&
			!strings.Contains(strings.ToUpper(rows[i].GroupNo), kw) {
			continue
		}
		filtered = append(filtered, rows[i])
	}

	c.JSON(200, gin.H{
		"rows":     filtered,
		"total":    len(filtered),
		"summary":  licenseRenewalSummary(rows),
		"licenses": groupLicenseRenewals(rows),
	})
}

// LicenseGroup = สรุป 1 ใบอนุญาตนำเข้า
type LicenseGroup struct {
	ImportLicenseNo   string `json:"importLicenseNo"`
	ITControllerModel string `json:"itControllerModel"`
	Country           string `json:"country"`
	Total             int    `json:"total"`

	// RenewalCount = ออกใบนำออกไปแล้วกี่ใบใต้ใบนำเข้าใบนี้ (นับทุกประเทศรวมกัน)
	// ถ้าอยากรู้ว่าแต่ละประเทศต่อไปกี่ครั้ง ให้ดู Branches[].RenewalCount
	RenewalCount int `json:"renewalCount"`

	// ใบล่าสุดที่ออก
	LatestExportNo string     `json:"latestExportNo"`
	LatestIssue    *time.Time `json:"latestIssue"`
	LatestExpire   *time.Time `json:"latestExpire"`
	DaysLeft       *int       `json:"daysLeft"`
	Remain         int        `json:"remain"`
	Status         string     `json:"status"`
	RenewStep      string     `json:"renewStep"`

	// Branches = สายประเทศของใบนำเข้าใบนี้
	// มีมากกว่า 1 สายเมื่อแบ่งโควต้าตามประเทศ แต่ละสายต่ออายุของตัวเองแยกกัน
	Branches []LicenseGroupBranch `json:"branches,omitempty"`
}

// LicenseGroupBranch = สายประเทศ 1 สายของใบนำเข้าใบหนึ่ง
type LicenseGroupBranch struct {
	Country        string     `json:"country"`
	LatestExportNo string     `json:"latestExportNo"`
	LatestIssue    *time.Time `json:"latestIssue"`
	LatestExpire   *time.Time `json:"latestExpire"`
	DaysLeft       *int       `json:"daysLeft"`
	Stock          int        `json:"stock"`
	Remain         int        `json:"remain"`
	RenewalCount   int        `json:"renewalCount"`
	Status         string     `json:"status"`
	RenewStep      string     `json:"renewStep"`
}

// licenseStatusRank: สถานะที่ต้องรีบดูก่อน (เลขน้อย = ด่วนกว่า)
// ใช้ทั้งตอนเลือกสถานะรวมของใบนำเข้าจากหลายสาย และตอนเรียงตาราง
var licenseStatusRank = map[string]int{
	models.LicenseStatusOverUsed:  0,
	models.LicenseStatusUsedUp:    1,
	models.LicenseStatusExpiring:  2,
	models.LicenseStatusNoLicense: 3,
	models.LicenseStatusNoDate:    4,
	models.LicenseStatusValid:     5,
	models.LicenseStatusExpired:   6,
}

func groupLicenseRenewals(rows []models.LicenseRenewal) []LicenseGroup {
	order := []string{}
	byImport := map[string][]models.LicenseRenewal{}
	for _, r := range rows {
		key := strings.TrimSpace(r.ImportLicenseNo)
		if key == "" {
			continue
		}
		if _, ok := byImport[key]; !ok {
			order = append(order, key)
		}
		byImport[key] = append(byImport[key], r)
	}

	out := make([]LicenseGroup, 0, len(order))
	for _, key := range order {
		list := byImport[key]
		g := LicenseGroup{
			ImportLicenseNo:   key,
			ITControllerModel: list[0].ITControllerModel,
			Total:             list[0].Total,
			Status:            models.LicenseStatusNoLicense,
			RenewStep:         models.RenewStepNone,
		}

		// ใบนำเข้าใบเดียวแบ่งโควต้าได้หลายประเทศ แต่ละสายมีใบล่าสุดและยอดคงเหลือของตัวเอง
		// ถ้ายุบเป็นแถวเดียวแบบเดิม ประเทศจะขึ้นของสายแรก แต่ยอดคงเหลือมาจากอีกสาย
		var countries []string
		seenCountry := map[string]bool{}
		first := true

		// ต่ออายุแล้วกี่ใบ = นับใบนำออกจริงในไฟล์
		// นับจาก list ตรง ๆ ไม่ใช่บวกรายสาย เพราะช่วงที่ยังไม่แตกสายประเทศ
		// อยู่ในทุกสาย ถ้าบวกรายสายจะนับซ้ำ
		for i := range list {
			if strings.TrimSpace(list[i].ExportLicenseNo) != "" {
				g.RenewalCount++
			}
		}

		for _, bidx := range ledgerBranchIndexes(list) {
			branch := ledgerPick(list, bidx)

			b := LicenseGroupBranch{
				Country:   strings.TrimSpace(branch[len(branch)-1].Country),
				Status:    models.LicenseStatusNoLicense,
				RenewStep: models.RenewStepNone,
			}
			for i := range branch {
				if strings.TrimSpace(branch[i].ExportLicenseNo) == "" {
					continue
				}
				b.RenewalCount++
				// ใบล่าสุดของสายนี้ = วันหมดอายุมากสุด (ถ้าไม่มีวันที่ ใช้แถวหลังสุดในไฟล์)
				newer := b.LatestExpire == nil ||
					(branch[i].ExpireDate != nil && branch[i].ExpireDate.After(*b.LatestExpire))
				if newer {
					b.LatestExportNo = branch[i].ExportLicenseNo
					b.LatestIssue = branch[i].IssueDate
					b.LatestExpire = branch[i].ExpireDate
					b.DaysLeft = branch[i].DaysLeft
					b.Stock = branch[i].Stock
					b.Remain = branch[i].Remain
					b.Status = branch[i].Status
					b.RenewStep = branch[i].RenewStep
				}
			}

			if b.Country != "" && !seenCountry[strings.ToUpper(b.Country)] {
				seenCountry[strings.ToUpper(b.Country)] = true
				countries = append(countries, b.Country)
			}
			g.Branches = append(g.Branches, b)

			// คงเหลือของใบนำเข้า = ทุกสายรวมกัน ไม่ใช่สายใดสายหนึ่ง
			g.Remain += b.Remain

			// สถานะและใบล่าสุดที่โชว์ ใช้สายที่ด่วนที่สุด
			if first || licenseStatusRank[b.Status] < licenseStatusRank[g.Status] {
				g.LatestExportNo = b.LatestExportNo
				g.LatestIssue = b.LatestIssue
				g.LatestExpire = b.LatestExpire
				g.DaysLeft = b.DaysLeft
				g.Status = b.Status
				g.RenewStep = b.RenewStep
				first = false
			}
		}

		g.Country = strings.Join(countries, " / ")
		if g.Country == "" {
			g.Country = strings.TrimSpace(list[0].Country)
		}
		out = append(out, g)
	}

	// เรียงให้ใบที่ต้องรีบดูขึ้นก่อน: ใกล้หมด → โควต้าหมด → ยังไม่ออกใบ → ปกติ → หมดอายุแล้ว
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := licenseStatusRank[out[i].Status], licenseStatusRank[out[j].Status]
		if ri != rj {
			return ri < rj
		}
		di, dj := 1<<30, 1<<30
		if out[i].DaysLeft != nil {
			di = *out[i].DaysLeft
		}
		if out[j].DaysLeft != nil {
			dj = *out[j].DaysLeft
		}
		return di < dj
	})
	return out
}

// GetLicenseRenewalAlerts: GET /license-renewal/alerts
// เตือน 2 แกน — ใกล้หมดอายุ และโควต้าใกล้หมด (อันไหนถึงก่อนเตือนอันนั้น)
func GetLicenseRenewalAlerts(c *gin.Context) {
	rows := loadLicenseRenewals()

	var expiring, usedUp, overUsed, noLicense []models.LicenseRenewal
	for i := range rows {
		switch rows[i].Status {
		case models.LicenseStatusExpiring:
			expiring = append(expiring, rows[i])
		case models.LicenseStatusUsedUp:
			usedUp = append(usedUp, rows[i])
		case models.LicenseStatusOverUsed:
			overUsed = append(overUsed, rows[i])
		case models.LicenseStatusNoLicense:
			noLicense = append(noLicense, rows[i])
		case models.LicenseStatusValid:
			// ยังไม่หมดอายุแต่โควต้าเหลือน้อย
			if rows[i].Remain > 0 && rows[i].Remain <= models.LicenseRemainLowThreshold {
				usedUp = append(usedUp, rows[i])
			}
		}
	}

	c.JSON(200, gin.H{
		"expiring":  expiring,
		"usedUp":    usedUp,
		"overUsed":  overUsed,
		"noLicense": noLicense,
		"summary":   licenseRenewalSummary(rows),
	})
}

// ClearLicenseRenewals: DELETE /license-renewal
func ClearLicenseRenewals(c *gin.Context) {
	res := config.DB.Where("1 = 1").Delete(&models.LicenseRenewal{})
	if res.Error != nil {
		c.JSON(500, gin.H{"message": res.Error.Error()})
		return
	}
	userID, name := lookupUserName(c)
	CreateAuditLog("LICENSE_RENEWAL", 0, "clear", "", userID, name)
	c.JSON(200, gin.H{"deleted": res.RowsAffected})
}
