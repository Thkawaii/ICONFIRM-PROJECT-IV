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
// สืบจากเครื่องหนึ่งเครื่อง ไปถึงใบอนุญาตที่ใช้อยู่จริงตอนนี้
//
//	S/N 878270000101
//	  └─ ทะเบียนใบนำเข้า (บัญชีแสดงหมายเลขเครื่อง)
//	       → Import  E05036903281
//	            └─ ชีตต่ออายุ (IMPORT LICENSE) จับด้วยเลขใบนำเข้า
//	                 → แถวต่ออายุทั้งหมด เรียงตามวันออกใบ
//	                      → แถวล่าสุด = ใบนำออกที่ใช้อยู่ตอนนี้
//
// ตอบคำถามว่า "เครื่องนี้ใช้ใบนำออกเลขอะไร ออกวันไหน เหลืออีกกี่วัน"
//
// ---------------------------------------------------------------------------
// ทำไมต้องเอาใบล่าสุดจากชีตต่ออายุ ไม่ใช่จากคอลัมน์ในบัญชี
//
// ไฟล์บัญชีแสดงหมายเลขเครื่องมีคอลัมน์ "เลขใบอนุญาตนำออก" อยู่แล้ว แต่เป็น
// ภาพ ณ วันที่ยื่นเอกสาร ไม่ได้ขยับตามการต่ออายุ ส่วนชีตต่ออายุคือทะเบียน
// ที่ตามรอบอยู่จริง — ใบนำออกอายุเดือนเดียว แต่ใบนำเข้าอยู่เป็นปี
// หนึ่งใบนำเข้าจึงมีใบนำออกต่อกันหลายรอบ (A001-1 → A001-2 → A001-3)
//
// ชีตต่ออายุจึงชนะ และถ้าสองฝั่งไม่ตรงกันต้องบอกให้เห็น ไม่ใช่เลือกข้างเงียบ ๆ
// เพราะแปลว่าบัญชียังไม่ได้อัปหลังต่ออายุ
// ---------------------------------------------------------------------------

// SerialTrace = ผลการสืบของเครื่องหนึ่งเครื่อง
type SerialTrace struct {
	SerialNo string `json:"serialNo"`
	PartNo   string `json:"partNo"`

	ImportLicenseNo string     `json:"importLicenseNo"`
	ImportIssueDate *time.Time `json:"importIssueDate,omitempty"`
	ImportExpireAt  *time.Time `json:"importExpireAt,omitempty"`

	// ExportLicenseNo = ใบนำออกรอบล่าสุดจากชีตต่ออายุ
	ExportLicenseNo string     `json:"exportLicenseNo"`
	ExportIssueDate *time.Time `json:"exportIssueDate,omitempty"`
	ExportExpireAt  *time.Time `json:"exportExpireAt,omitempty"`

	// DaysLeft = จำนวนวันก่อนใบนำออกหมดอายุ ติดลบ = หมดไปแล้วกี่วัน
	DaysLeft *int `json:"daysLeft,omitempty"`

	// RenewalCount = ต่ออายุมาแล้วกี่รอบ (นับแถวในชีตต่ออายุ ลบรอบแรก)
	RenewalCount int `json:"renewalCount"`

	ExportCountry string `json:"exportCountry"`

	// IssuedAt = WH จ่ายเครื่องนี้ออกไปเมื่อไหร่ (nil = ยังไม่จ่าย)
	IssuedAt  *time.Time `json:"issuedAt,omitempty"`
	IssuedTo  string     `json:"issuedTo,omitempty"`
	MachineNo string     `json:"machineNo,omitempty"`

	// Status = นำเข้าแล้ว / ส่งออกแล้ว / ยังค้าง
	Status string `json:"status"`

	// Conflict = บัญชีกับชีตต่ออายุบอกใบนำออกไม่ตรงกัน
	Conflict string `json:"conflict,omitempty"`
}

const (
	SerialStatusPending  = "ยังค้าง"       // นำเข้าแล้ว แต่ WH ยังไม่จ่ายออก
	SerialStatusIssued   = "จ่ายแล้ว"      // WH จ่ายออกจากคลังแล้ว
	SerialStatusNoImport = "ไม่พบใบนำเข้า" // สแกนเจอแต่ไม่มีในทะเบียน
)

// latestRenewalOf: แถวต่ออายุรอบล่าสุดของใบนำเข้าใบหนึ่ง
//
// เรียงตามวันออกใบ ไม่ใช่ลำดับแถวในไฟล์ เพราะคนกรอกแทรกแถวย้อนหลังได้
// แถวที่ไม่มีวันออกใบจะถูกข้าม — เดาว่าใหม่กว่าหรือเก่ากว่าไม่ได้
func latestRenewalOf(rows []models.LicenseRenewal) *models.LicenseRenewal {
	var best *models.LicenseRenewal
	for i := range rows {
		r := rows[i]
		if r.IssueDate == nil {
			continue
		}
		if best == nil || r.IssueDate.After(*best.IssueDate) {
			best = &rows[i]
		}
	}
	return best
}

// buildSerialTrace: ประกอบผลการสืบของเครื่องหนึ่งเครื่องจากแถวในทะเบียน
//
// แยกออกมาจาก handler เพราะมีผู้เรียกสองทาง — endpoint เดิม และตัวค้นหารวม
// ถ้าปล่อยให้โค้ดซ้ำสองที่ วันหนึ่งจะแก้ที่เดียวแล้วอีกที่ตอบคนละอย่าง
func buildSerialTrace(item *models.LicenseItem) SerialTrace {
	out := SerialTrace{
		SerialNo:        item.MachineNo,
		PartNo:          item.PartNo,
		ImportLicenseNo: item.LicenseNo,
		ImportIssueDate: item.IssueDate,
		ImportExpireAt:  item.ExpireDate,
		ExportCountry:   item.ExportCountry,
		Status:          SerialStatusPending,
	}

	var renewals []models.LicenseRenewal
	config.DB.Where("import_license_no = ?", item.LicenseNo).Find(&renewals)
	if len(renewals) > 0 {
		out.RenewalCount = len(renewals) - 1
		if last := latestRenewalOf(renewals); last != nil {
			out.ExportLicenseNo = last.ExportLicenseNo
			out.ExportIssueDate = last.IssueDate
			out.ExportExpireAt = last.ExpireDate
			if last.ExpireDate != nil {
				d := int(last.ExpireDate.Truncate(24*time.Hour).
					Sub(time.Now().Truncate(24*time.Hour)).Hours() / 24)
				out.DaysLeft = &d
			}
			if item.ExportLicenseNo != "" && last.ExportLicenseNo != "" &&
				!SameCode(item.ExportLicenseNo, last.ExportLicenseNo) {
				out.Conflict = "บัญชีแสดงหมายเลขเครื่องระบุใบนำออก " + item.ExportLicenseNo +
					" แต่ชีตต่ออายุระบุใบล่าสุดเป็น " + last.ExportLicenseNo
			}
		}
	}
	if out.ExportLicenseNo == "" {
		out.ExportLicenseNo = item.ExportLicenseNo
		out.ExportIssueDate = item.ExportIssueDate
	}

	var issue models.PartCheck
	if err := config.DB.Where("sn = ? AND part_type = ?", item.MachineNo, ComponentITC).
		Order("checked_datetime desc").First(&issue).Error; err == nil {
		t := issue.CheckedDatetime
		out.IssuedAt = &t
		out.IssuedTo = issue.MachineNo
		out.MachineNo = issue.MachineNo
		out.Status = SerialStatusIssued
	}
	return out
}

// TraceSerial: GET /license-overview/serial?sn=878270000101
func TraceSerial(c *gin.Context) {
	sn := strings.TrimSpace(c.Query("sn"))
	if sn == "" {
		c.JSON(400, gin.H{"message": "กรุณาระบุ S/N"})
		return
	}

	item := licenseItemBySerial(sn)
	if item == nil {
		c.JSON(404, gin.H{
			"message": "ไม่พบ S/N " + sn + " ในทะเบียนใบอนุญาตนำเข้า",
			"trace":   SerialTrace{SerialNo: sn, Status: SerialStatusNoImport},
		})
		return
	}

	c.JSON(200, gin.H{"trace": buildSerialTrace(item)})
}

// ---------------------------------------------------------------------------
// เทียบยอดที่คนกรอก กับยอดที่ระบบนับได้จริง
//
// ช่อง REMAIN ในชีตต่ออายุเป็นตัวเลขที่คนพิมพ์เอง ส่วนระบบรู้ว่า WH จ่ายเครื่อง
// ออกไปแล้วกี่เครื่องเพราะมีบันทึกการสแกนทุกครั้ง สองค่านี้ควรตรงกัน
//
// ไม่เอาค่าที่ระบบนับไปทับค่าที่คนกรอก เพราะทั้งสองฝั่งผิดได้ —
// คนกรอกพลาดก็มี WH ลืมสแกนก็มี การทับค่าจะกลบปัญหาแทนที่จะเปิดเผย
// จึงเก็บทั้งคู่แล้วบอกว่าต่างกันเท่าไร ให้คนไปหาว่าฝั่งไหนผิด
// ---------------------------------------------------------------------------

// issuedCountByImport: จำนวนเครื่องที่ WH จ่ายออกแล้ว แยกตามเลขใบนำเข้า
//
// นับจากบันทึกการสแกนของ IT Controller ซึ่งผูกกับแถวในทะเบียนใบนำเข้าไว้แล้ว
// นับเครื่องละครั้งแม้จะสแกนซ้ำ (จ่ายของชิ้นเดิมใหม่ไม่ใช่การจ่ายเพิ่ม)
func issuedCountByImport() map[string]int {
	type row struct {
		LicenseNo string
		SN        string
	}
	var rows []row
	config.DB.Model(&models.PartCheck{}).
		Select("license_no, sn").
		Where("part_type = ? AND license_no <> ''", ComponentITC).
		Find(&rows)

	seen := map[string]map[string]bool{}
	for _, r := range rows {
		key := NormalizeCodeValue(r.LicenseNo)
		sn := NormalizeCodeValue(r.SN)
		if key == "" || sn == "" {
			continue
		}
		if seen[key] == nil {
			seen[key] = map[string]bool{}
		}
		seen[key][sn] = true
	}

	out := make(map[string]int, len(seen))
	for k, v := range seen {
		out[k] = len(v)
	}
	return out
}

// GetSerialStatusSummary: GET /license-overview/serial-summary
//
// ภาพรวมว่าเครื่องในทะเบียนนำเข้าแล้วกี่เครื่อง จ่ายออกแล้วกี่เครื่อง ค้างกี่เครื่อง
// แยกตามใบนำเข้า — ใช้ตอบคำถาม "ยอดตอนเข้ากับตอนออกตรงกันไหม"
func GetSerialStatusSummary(c *gin.Context) {
	var items []models.LicenseItem
	if err := config.DB.Find(&items).Error; err != nil {
		c.JSON(500, gin.H{"message": "อ่านทะเบียนไม่สำเร็จ: " + err.Error()})
		return
	}
	issued := issuedCountByImport()

	type bucket struct {
		ImportLicenseNo string `json:"importLicenseNo"`
		Country         string `json:"country"`
		Imported        int    `json:"imported"`
		Issued          int    `json:"issued"`
		Pending         int    `json:"pending"`
	}
	byImport := map[string]*bucket{}
	for _, it := range items {
		key := NormalizeCodeValue(it.LicenseNo)
		if key == "" {
			continue
		}
		b := byImport[key]
		if b == nil {
			b = &bucket{ImportLicenseNo: it.LicenseNo, Country: it.ExportCountry}
			byImport[key] = b
		}
		b.Imported++
	}
	for key, b := range byImport {
		b.Issued = issued[key]
		b.Pending = b.Imported - b.Issued
		if b.Pending < 0 {
			b.Pending = 0
		}
	}

	out := make([]bucket, 0, len(byImport))
	for _, b := range byImport {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ImportLicenseNo < out[j].ImportLicenseNo })

	c.JSON(200, gin.H{"rows": out, "total": len(out)})
}

// ---------------------------------------------------------------------------
// รายชื่อเครื่องใต้ใบอนุญาตใบหนึ่ง
//
// ตอบคำถาม "ใบนี้มีเครื่อง S/N อะไรบ้าง และตอนนี้เหลือเครื่องไหนที่ยังไม่ได้จ่าย"
//
// ใบนำเข้า — จับจากคอลัมน์ เลขใบอนุญาตนำเข้า ในบัญชีแสดงหมายเลขเครื่องตรง ๆ
// ใบนำออก  — ทะเบียนเครื่องผูกกับใบนำเข้า ไม่ได้ผูกกับใบนำออกรายใบ
//
//	จึงต้องย้อนผ่านชีตต่ออายุก่อน: ใบนำออกใบนี้อยู่ใต้ใบนำเข้าไหน
//	แล้วค่อยดึงเครื่องของใบนำเข้านั้นมา
//
// ยอดคงเหลือที่คนกรอกไว้ในชีตต่ออายุเทียบกับจำนวนเครื่องที่ยังไม่ถูกจ่ายตรงนี้ได้เลย
// ---------------------------------------------------------------------------

// LicenseSerial = หนึ่งเครื่องในรายชื่อ
type LicenseSerial struct {
	SerialNo  string     `json:"serialNo"`
	PartNo    string     `json:"partNo"`
	Country   string     `json:"country"`
	Issued    bool       `json:"issued"`
	IssuedAt  *time.Time `json:"issuedAt,omitempty"`
	MachineNo string     `json:"machineNo,omitempty"`
}

// GetLicenseSerials: GET /license-overview/serials?type=IMPORT&licenseNo=E05036903281
func GetLicenseSerials(c *gin.Context) {
	licenseType := models.NormalizeLicenseType(c.Query("type"))
	licenseNo := strings.TrimSpace(c.Query("licenseNo"))
	if licenseNo == "" {
		c.JSON(400, gin.H{"message": "กรุณาระบุเลขใบอนุญาต"})
		return
	}

	importNo := licenseNo
	if licenseType == models.LicenseTypeExport {
		// ใบนำออก → ย้อนหาใบนำเข้าที่มันสังกัดจากชีตต่ออายุ
		var r models.LicenseRenewal
		if err := config.DB.Where("export_license_no = ?", licenseNo).First(&r).Error; err == nil {
			importNo = r.ImportLicenseNo
		}
	}

	var items []models.LicenseItem
	config.DB.Where("license_no = ?", importNo).Order("machine_no asc").Find(&items)

	// เครื่องไหนถูกจ่ายไปแล้วบ้าง — อ่านทีเดียว ไม่ยิงรายเครื่อง
	issuedAt := map[string]models.PartCheck{}
	if len(items) > 0 {
		sns := make([]string, 0, len(items))
		for _, it := range items {
			sns = append(sns, it.MachineNo)
		}
		var checks []models.PartCheck
		config.DB.Where("part_type = ? AND sn IN ?", ComponentITC, sns).
			Order("checked_datetime asc").Find(&checks)
		for _, ck := range checks {
			issuedAt[NormalizeCodeValue(ck.SN)] = ck
		}
	}

	out := make([]LicenseSerial, 0, len(items))
	issued := 0
	for _, it := range items {
		row := LicenseSerial{
			SerialNo: it.MachineNo,
			PartNo:   it.PartNo,
			Country:  it.ExportCountry,
		}
		if ck, ok := issuedAt[NormalizeCodeValue(it.MachineNo)]; ok {
			t := ck.CheckedDatetime
			row.Issued = true
			row.IssuedAt = &t
			row.MachineNo = ck.MachineNo
			issued++
		}
		out = append(out, row)
	}

	c.JSON(200, gin.H{
		"rows":            out,
		"total":           len(out),
		"issued":          issued,
		"pending":         len(out) - issued,
		"importLicenseNo": importNo,
	})
}

// ---------------------------------------------------------------------------
// ค้นหาแบบไม่ต้องบอกว่าพิมพ์อะไรลงไป
//
// S/N กับเลขใบอนุญาตนำเข้าหน้าตาเหมือนกันจนแยกไม่ออก
//
//	S/N        878270000101   ← ตัวเลข 12 หลัก
//	Import     050167001815   ← ตัวเลข 12 หลักเหมือนกัน
//	Import     E05036903281   ← บางล็อตมีตัวอักษรนำ
//
// ให้หน้าเว็บเดาเองไม่ได้ จึงลองทีละชั้นที่ฝั่งนี้:
// ทะเบียนเครื่องก่อน (เจาะจงกว่า — หนึ่งแถวต่อหนึ่งเครื่อง)
// ไม่เจอค่อยลองใบอนุญาต
// ---------------------------------------------------------------------------

// LicenseTrace = ภาพรวมของใบอนุญาตใบหนึ่ง พร้อมใบนำออกรอบล่าสุด
type LicenseTrace struct {
	ImportLicenseNo string `json:"importLicenseNo"`
	Country         string `json:"country"`

	// InvoiceNo / DeclarationNo = เลขอินวอยซ์นำเข้า และเลขใบขนสินค้าขาเข้า
	//
	// อยู่ในทะเบียนเครื่อง (บัญชีแสดงหมายเลขเครื่อง) ไม่ได้อยู่ในชีตต่ออายุ
	// จึงต้องอ่านจากแถวของเครื่องที่สังกัดใบนำเข้าใบนี้
	// ทุกเครื่องใต้ใบเดียวกันใช้เลขชุดเดียวกัน จึงหยิบแถวแรกที่มีค่ามาใช้ได้
	InvoiceNo     string `json:"invoiceNo,omitempty"`
	DeclarationNo string `json:"declarationNo,omitempty"`

	// QueryKind = "import" หรือ "export" — บอกว่าเลขที่พิมพ์ค้นหาตรงกับเลขใบนำเข้า
	// หรือไปเจอในฐานะเลขใบนำออกแล้วย้อนหาใบนำเข้าที่สังกัด
	// หน้าเว็บใช้ค่านี้ตัดสินว่าจะขึ้นหัวการ์ดเป็น "ใบอนุญาตนำเข้า" หรือ "ใบนำออก"
	QueryKind string `json:"queryKind"`

	ExportLicenseNo string     `json:"exportLicenseNo"`
	ExportIssueDate *time.Time `json:"exportIssueDate,omitempty"`
	ExportExpireAt  *time.Time `json:"exportExpireAt,omitempty"`
	DaysLeft        *int       `json:"daysLeft,omitempty"`
	RenewalCount    int        `json:"renewalCount"`

	// Stock / Remain = ตัวเลขที่คนกรอกไว้ในชีตต่ออายุ แถวล่าสุด
	Stock     int  `json:"stock"`
	Remain    int  `json:"remain"`
	HasRemain bool `json:"hasRemain"`

	// Machines / Issued / Pending = จำนวนจริงจากทะเบียนเครื่องและบันทึกการจ่ายของ
	Machines int `json:"machines"`
	Issued   int `json:"issued"`
	Pending  int `json:"pending"`

	// Mismatch = ตัวเลขที่คนกรอก ไม่ตรงกับที่ระบบนับได้
	Mismatch string `json:"mismatch,omitempty"`

	// Branches = สายประเทศภายในใบนำเข้าใบเดียวกัน
	//
	// ใบนำเข้าใบเดียวแบ่งโควต้าออกหลายประเทศได้ แล้วแต่ละประเทศเดินต่ออายุของตัวเอง
	// คนละรอบ คนละเลขใบนำออก เช่น E05036903112 → INDONESIA 50 + MALAYSIA 10
	// ถ้าแสดงแค่สายเดียว ตัวเลขจะหายไปครึ่งหนึ่งโดยไม่มีใครรู้
	Branches []LicenseTraceBranch `json:"branches,omitempty"`
}

// LicenseTraceBranch = หนึ่งสายประเทศของใบนำเข้า
type LicenseTraceBranch struct {
	Country         string     `json:"country"`
	ExportLicenseNo string     `json:"exportLicenseNo"`
	IssueDate       *time.Time `json:"issueDate,omitempty"`
	ExpireAt        *time.Time `json:"expireAt,omitempty"`
	DaysLeft        *int       `json:"daysLeft,omitempty"`
	RenewalCount    int        `json:"renewalCount"`
	Stock           int        `json:"stock"`
	Remain          int        `json:"remain"`
	HasRemain       bool       `json:"hasRemain"`
}

// traceLicense: ประกอบภาพรวมของใบอนุญาตใบหนึ่ง
//
// รับได้ทั้งเลขใบนำเข้าและเลขใบนำออก — ถ้าเป็นใบนำออกจะย้อนไปหาใบนำเข้าที่สังกัดก่อน
// คืน nil เมื่อไม่พบเลขนี้ในชีตต่ออายุเลย
func traceLicense(licenseNo string) *LicenseTrace {
	licenseNo = strings.TrimSpace(licenseNo)
	if licenseNo == "" {
		return nil
	}

	var rows []models.LicenseRenewal
	config.DB.Where("import_license_no = ?", licenseNo).Find(&rows)
	importNo := licenseNo
	queryKind := "import"

	if len(rows) == 0 {
		// ลองในฐานะเลขใบนำออก แล้วย้อนไปหาใบนำเข้าที่สังกัด
		var one models.LicenseRenewal
		if err := config.DB.Where("export_license_no = ?", licenseNo).First(&one).Error; err != nil {
			return nil
		}
		importNo = one.ImportLicenseNo
		queryKind = "export"
		config.DB.Where("import_license_no = ?", importNo).Find(&rows)
	}
	if len(rows) == 0 {
		return nil
	}

	// แยกเป็นสายประเทศก่อนเสมอ แล้วค่อยรวมยอด
	//
	// ของเดิมหยิบ "แถวที่ออกใบล่าสุด" มาแถวเดียว ซึ่งผิดกับใบที่แตกหลายประเทศ:
	// E05036903112 มี INDONESIA 50 กับ MALAYSIA 10 ออกใบวันเดียวกัน
	// หยิบแถวเดียวได้สต็อก 50 แทนที่จะเป็น 60 แล้วไปฟ้องว่ายอดไม่ตรงทั้งที่ตรง
	// และนับจำนวนแถวเป็นจำนวนครั้งที่ต่ออายุ ได้ 1 ครั้งทั้งที่ยังไม่เคยต่อเลย
	byBranch := map[string][]models.LicenseRenewal{}
	branchOrder := []string{}
	for _, r := range rows {
		key := ledgerCountryKey(r.Country)
		if _, ok := byBranch[key]; !ok {
			branchOrder = append(branchOrder, key)
		}
		byBranch[key] = append(byBranch[key], r)
	}

	out := &LicenseTrace{ImportLicenseNo: importNo, QueryKind: queryKind}
	countries := []string{}
	exports := []string{}

	for _, key := range branchOrder {
		list := byBranch[key]
		b := LicenseTraceBranch{RenewalCount: len(list) - 1}
		last := latestRenewalOf(list)
		if last == nil {
			last = &list[len(list)-1]
		}
		b.Country = strings.TrimSpace(last.Country)
		b.ExportLicenseNo = last.ExportLicenseNo
		b.IssueDate = last.IssueDate
		b.ExpireAt = last.ExpireDate
		b.Stock = last.Stock
		b.Remain = last.Remain
		b.HasRemain = last.HasRemain
		if last.ExpireDate != nil {
			d := int(last.ExpireDate.Truncate(24*time.Hour).
				Sub(time.Now().Truncate(24*time.Hour)).Hours() / 24)
			b.DaysLeft = &d
		}

		out.Branches = append(out.Branches, b)
		out.Stock += b.Stock
		out.Remain += b.Remain
		if b.HasRemain {
			out.HasRemain = true
		}
		// จำนวนครั้งที่ต่ออายุ = สายที่ต่อมากที่สุด ไม่ใช่ผลรวม
		// เพราะสองสายเดินคู่ขนานกัน ไม่ได้ต่อกันเป็นทอด
		if b.RenewalCount > out.RenewalCount {
			out.RenewalCount = b.RenewalCount
		}
		if b.Country != "" {
			countries = append(countries, b.Country)
		}
		if b.ExportLicenseNo != "" {
			exports = append(exports, b.ExportLicenseNo)
		}
		// วันหมดอายุของใบ = สายที่หมดก่อน เพราะนั่นคือสายที่ต้องรีบต่อ
		if b.ExpireAt != nil && (out.ExportExpireAt == nil || b.ExpireAt.Before(*out.ExportExpireAt)) {
			out.ExportExpireAt = b.ExpireAt
			out.ExportIssueDate = b.IssueDate
			out.DaysLeft = b.DaysLeft
		}
	}

	out.Country = strings.Join(countries, ", ")
	out.ExportLicenseNo = strings.Join(exports, ", ")
	if out.ExportExpireAt == nil && len(out.Branches) > 0 {
		out.ExportIssueDate = out.Branches[0].IssueDate
	}

	// จำนวนเครื่องจริงจากทะเบียน — เทียบกับเลขที่คนกรอก
	var items []models.LicenseItem
	config.DB.Where("license_no = ?", importNo).Find(&items)
	out.Machines = len(items)

	// หยิบเลขอินวอยซ์กับเลขใบขนจากแถวแรกที่กรอกไว้
	// ข้ามแถวที่เว้นว่าง เพราะบางไฟล์กรอกเฉพาะแถวแรกของล็อต
	for i := range items {
		if out.InvoiceNo == "" {
			out.InvoiceNo = strings.TrimSpace(items[i].InvoiceNo)
		}
		if out.DeclarationNo == "" {
			out.DeclarationNo = strings.TrimSpace(items[i].DeclarationNo)
		}
		if out.InvoiceNo != "" && out.DeclarationNo != "" {
			break
		}
	}
	if len(items) > 0 {
		sns := make([]string, 0, len(items))
		for _, it := range items {
			sns = append(sns, it.MachineNo)
		}
		var checks []models.PartCheck
		config.DB.Where("part_type = ? AND sn IN ?", ComponentITC, sns).Find(&checks)
		seen := map[string]bool{}
		for _, ck := range checks {
			seen[NormalizeCodeValue(ck.SN)] = true
		}
		out.Issued = len(seen)
		out.Pending = out.Machines - out.Issued
	}

	if out.HasRemain && out.Machines > 0 && out.Remain != out.Pending {
		out.Mismatch = "ผู้ใช้กรอกคงเหลือ " + strconv.Itoa(out.Remain) +
			" เครื่อง แต่ในระบบยังไม่ได้จ่าย " + strconv.Itoa(out.Pending) + " เครื่อง"
	}
	return out
}

// LookupTrace: GET /license-overview/lookup?q=...
//
// คืน kind บอกว่าเจออะไร — serial หรือ license — ให้หน้าเว็บเลือกการ์ดที่จะแสดง
func LookupTrace(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(400, gin.H{"message": "กรุณาระบุคำค้น"})
		return
	}

	// ทะเบียนเครื่องก่อน เพราะเจาะจงกว่า
	if item := licenseItemBySerial(q); item != nil {
		c.JSON(200, gin.H{"kind": "serial", "trace": buildSerialTrace(item)})
		return
	}

	if lt := traceLicense(q); lt != nil {
		c.JSON(200, gin.H{"kind": "license", "license": lt})
		return
	}

	// สุดท้ายลองในฐานะ "เลขเครื่อง" (MC#)
	//
	// หน้างานจำเลขเครื่องได้ แต่จำ S/N ของกล่อง IT ไม่ได้ เพราะไม่ได้ถือกล่องอยู่
	// พอ WH จ่ายของไปแล้ว ระบบรู้อยู่แล้วว่าเครื่องคันนี้ใช้กล่องไหน
	// จึงย้อนจากเลขเครื่องไปหา S/N แล้วสืบต่อด้วยเส้นทางเดิมได้เลย
	//
	// อยู่ท้ายสุดเพราะเจาะจงน้อยที่สุด และเลขเครื่องกับ S/N หน้าตาต่างกันพอ
	// ที่จะไม่ชนกันอยู่แล้ว
	if sn := itcSerialOfMachine(q); sn != "" {
		if item := licenseItemBySerial(sn); item != nil {
			t := buildSerialTrace(item)
			c.JSON(200, gin.H{"kind": "serial", "trace": t, "viaMachine": strings.TrimSpace(q)})
			return
		}
	}

	c.JSON(404, gin.H{"kind": "none", "message": "ไม่พบ " + q + " ในระบบ"})
}

// itcSerialOfMachine: หา S/N ของกล่อง IT Controller ที่ WH จ่ายให้เครื่องคันนี้
//
// เอาตัวล่าสุดถ้ามีหลายรายการ เพราะกล่องอาจถูกเปลี่ยนระหว่างทาง
// คืนค่าว่างถ้ายังไม่เคยจ่าย ซึ่งไม่ใช่ความผิดพลาด แค่ยังไม่ถึงขั้นตอนนั้น
func itcSerialOfMachine(machineNo string) string {
	machineNo = strings.TrimSpace(machineNo)
	if machineNo == "" {
		return ""
	}
	keys := dedupeCodes(machineNo, ResolveMachineNo(machineNo), CurrentCodeOf(machineNo))
	if len(keys) == 0 {
		keys = []string{machineNo}
	}
	var row models.PartCheck
	if err := config.DB.
		Where("machine_no IN ?", keys).
		Where("part_type = ?", ComponentITC).
		Where("match_status = ?", models.MatchStatusMatch).
		Order("checked_datetime desc").First(&row).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(row.SN)
}
