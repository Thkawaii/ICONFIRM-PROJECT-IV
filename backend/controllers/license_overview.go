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
// License Overview — ดู Import + Export ในหน้าเดียว พร้อมประวัติการต่ออายุ
//
// ใบอนุญาต 1 ใบอาจถูกต่ออายุหลายครั้ง เลขใบเปลี่ยนทุกครั้ง:
//
//	EXP-001 → EXP-002 → EXP-003 → EXP-004
//	  ต้นฉบับ   ครั้ง1    ครั้ง2    ครั้ง3   → Current = EXP-004, Renewal Count = 3
//
// ระบบจึงไม่นับจากเลขใบ แต่ไล่ "โซ่" จากตาราง license_renewal_history
// โดยใช้คีย์ LicenseType + LicenseNo เสมอ — Import กับ Export แยกกันเด็ดขาด
//
// สถานะเดิมของระบบยังทำงานเหมือนเดิมทุกอย่าง:
//   - Current License ใช้วันหมดอายุของใบปัจจุบันคำนวณ VALID / EXPIRING / EXPIRED
//   - ใบเก่าที่ถูกต่ออายุไปแล้ว = RENEWED ไม่ถูกนำไปนับ countdown
// ---------------------------------------------------------------------------

// ช่วงที่ถือว่า "ใกล้หมดอายุ" — ใช้เกณฑ์เดิมของแต่ละประเภท
const (
	ImportLicenseExpiringWithinDays = 30
	ExportLicenseExpiringWithinDays = models.LicenseExpiringWithinDays
)

// LicenseChainStep = การต่ออายุ 1 ครั้งในโซ่ (Round 0 = ใบต้นฉบับ)
type LicenseChainStep struct {
	Round        int    `json:"round"`
	Label        string `json:"label"`
	OldLicenseNo string `json:"oldLicenseNo"`
	NewLicenseNo string `json:"newLicenseNo"`
	// GroupNo = คอลัมน์ NO. ของชีตต่ออายุ เช่น "Completed 01"
	GroupNo     string     `json:"groupNo"`
	RenewalDate *time.Time `json:"renewalDate"`
	ExpireDate  *time.Time `json:"expireDate"`
	Remark      string     `json:"remark"`
	FileName    string     `json:"fileName"`
	UploadedBy  string     `json:"uploadedBy"`
	UploadedAt  *time.Time `json:"uploadedAt"`

	// ตัวเลขจากชีตต่ออายุของใบนั้น ๆ (เติมตอนอ่านรายละเอียด)
	//
	// Country จำเป็นเพราะใบนำเข้าใบเดียวแบ่งโควต้าได้หลายประเทศ
	// แต่ละประเทศเดินต่ออายุของตัวเอง ประวัติจึงมีใบของสองประเทศปนกันอยู่
	// ถ้าไม่บอกว่าแถวไหนของประเทศไหน ตัวเลข STOCK / คงเหลือ จะอ่านไม่ออก
	HasLedger bool   `json:"hasLedger"`
	Country   string `json:"country"`
	Quota     int    `json:"quota"`
	Stock     int    `json:"stock"`
	Remain    int    `json:"remain"`

	// IsNote = แถวนี้ไม่ใช่การต่ออายุ แต่เป็นข้อความที่ผู้ใช้จดแทรกไว้ในไฟล์
	IsNote bool   `json:"isNote"`
	Note   string `json:"note"`
}

// expandStepsByCountry: เติมตัวเลขจากทะเบียนให้แต่ละขั้น และแตกขั้นออกเป็นรายประเทศ
//
// ใบนำเข้าใบเดียวแบ่งโควต้าได้หลายประเทศ แต่ละประเทศมีใบนำออก STOCK และ REMAIN
// ของตัวเอง ของเดิมหยิบมาแค่แถวหลังสุดแถวเดียว ประวัติจึงเห็นประเทศเดียว
// อีกประเทศหายไปทั้งสาย ทั้งที่มีโควต้าอยู่จริง
//
//	ต้นฉบับ  Indonesia  E05036903112  —  50  50
//	ต้นฉบับ  Malaysia   E05036903112  —  10  10
//
// คืน steps ที่แตกแล้ว · SortOrder ของแต่ละ step (ใช้วางหมายเหตุให้ตรงตำแหน่งเดิมในไฟล์)
// · และชุดโซ่ที่เกี่ยวข้อง
func expandStepsByCountry(licenseType string, steps []LicenseChainStep, ledger []models.LicenseRenewal) ([]LicenseChainStep, []int64, map[string]bool) {
	// เก็บทุกแถวของเลขใบนั้น ไม่ใช่แถวเดียว
	byNo := map[string][]models.LicenseRenewal{}
	for i := range ledger {
		no := ledger[i].ExportLicenseNo
		if licenseType == models.LicenseTypeImport {
			no = ledger[i].ImportLicenseNo
		}
		key := NormalizeCodeValue(no)
		if key == "" {
			continue
		}
		byNo[key] = append(byNo[key], ledger[i])
	}

	out := make([]LicenseChainStep, 0, len(steps))
	orderOf := make([]int64, 0, len(steps))
	chainGroups := map[string]bool{}

	for _, step := range steps {
		// ขั้นต้นฉบับยังไม่มีเลขใบใหม่ จึงใช้เลขใบเดิมแทน
		no := step.NewLicenseNo
		if strings.TrimSpace(no) == "" {
			no = step.OldLicenseNo
		}
		matched := byNo[NormalizeCodeValue(no)]
		if len(matched) == 0 {
			out = append(out, step)
			orderOf = append(orderOf, 0)
			continue
		}

		// แต่ละสายประเทศใช้แถวหลังสุดของสายตัวเอง = สถานะล่าสุดของประเทศนั้น
		// ใบที่ไม่ได้แบ่งประเทศจะได้สายเดียว ผลเหมือนเดิมทุกอย่าง
		for _, bidx := range ledgerBranchIndexes(matched) {
			row := matched[bidx[len(bidx)-1]]

			cur := step
			cur.HasLedger = true
			cur.Country = strings.TrimSpace(row.Country)
			cur.Quota = row.Total
			cur.Stock = row.Stock
			cur.Remain = row.Remain

			out = append(out, cur)
			orderOf = append(orderOf, row.SortOrder)
			chainGroups[ledgerGroupKey(row)] = true
		}
	}

	return out, orderOf, chainGroups
}

// itoa64: แปลงลำดับแถวเป็นข้อความ ใช้ทำคีย์กันซ้ำ
func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}

// ledgerStepsForImport: ประวัติการต่ออายุของ "ใบนำเข้า" สร้างจากชีตทะเบียนโดยตรง
//
// ใบนำเข้าแทบไม่เคยเปลี่ยนเลขใบ โซ่ใน license_renewal_history ของมันจึงมีขั้นเดียว
// ทั้งที่การ์ดด้านบนขึ้นว่าต่ออายุ 5 ครั้ง (นับจากจำนวนใบนำออกใต้โควต้าเดิม)
// ตารางกับการ์ดจึงเล่าคนละเรื่อง
//
// ในทางปฏิบัติ "การต่ออายุ" ของใบนำเข้าคือการออกใบนำออกใบใหม่ใต้โควต้าเดิม
// ประวัติของมันจึงคือลำดับใบนำออกในชีตทะเบียน:
//
//	ใบแรก      050167001924                  03/04/2024  R 124
//	ครั้งที่ 1  050167001924 → 050167003380   10/06/2024  R  48
//	ครั้งที่ 2  050167003380 → 050167003725   25/06/2024  R  11
//
// แยกสายตามประเทศเหมือนทุกที่ ช่วงที่ยังไม่แตกสายอยู่ในทุกสาย จึงกันไม่ให้ซ้ำ
// แล้วเรียงกลับตามลำดับเดิมในไฟล์ เพื่อให้อ่านคู่กับ Excel ได้
func ledgerStepsForImport(importNo string, ledger []models.LicenseRenewal) ([]LicenseChainStep, []int64, map[string]bool) {
	key := NormalizeCodeValue(importNo)
	if key == "" {
		return nil, nil, nil
	}

	var rows []models.LicenseRenewal
	for i := range ledger {
		if NormalizeCodeValue(ledger[i].ImportLicenseNo) != key {
			continue
		}
		if strings.TrimSpace(ledger[i].ExportLicenseNo) == "" {
			continue
		}
		rows = append(rows, ledger[i])
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}

	type built struct {
		step  LicenseChainStep
		order int64
	}
	var list []built
	seen := map[string]bool{}
	groups := map[string]bool{}

	for _, bidx := range ledgerBranchIndexes(rows) {
		steps := ledgerSteps(ledgerPick(rows, bidx))
		for si, step := range steps {
			for n, cur := range step {
				// กันซ้ำด้วย "แถวไหนในไฟล์" ไม่ใช่ด้วยเลขใบ
				//
				// แถวช่วงที่ยังไม่แตกสายประเทศ อยู่ในทุกสาย จึงถูกไล่ซ้ำ — แถวเดียวกัน
				// ลำดับในไฟล์เท่ากัน จึงตัดทิ้งได้
				// แต่ไฟล์จริงมีกรณีเขียนเลขใบเดียวกันสองแถวเพื่ออัปเดต REMAIN
				// (เช่น 050168000826 แถว R 1 แล้วแถว R 0) สองแถวนั้นคนละลำดับ ต้องเก็บไว้ทั้งคู่
				dedupe := itoa64(cur.SortOrder) + "|" + NormalizeCodeValue(cur.ExportLicenseNo)
				if seen[dedupe] {
					continue
				}
				seen[dedupe] = true

				b := built{order: cur.SortOrder}
				b.step = LicenseChainStep{
					Round:       si,
					GroupNo:     cur.GroupNo,
					RenewalDate: cur.IssueDate,
					ExpireDate:  cur.ExpireDate,
					HasLedger:   true,
					Country:     strings.TrimSpace(cur.Country),
					Quota:       cur.Total,
					Stock:       cur.Stock,
					Remain:      cur.Remain,
				}
				if si == 0 {
					b.step.Label = "ใบแรก"
					b.step.OldLicenseNo = strings.TrimSpace(cur.ExportLicenseNo)
				} else {
					prev := ledgerStepMatch(steps[si-1], cur, n)
					b.step.Label = "ครั้งที่ " + itoa(si)
					b.step.OldLicenseNo = strings.TrimSpace(prev.ExportLicenseNo)
					b.step.NewLicenseNo = strings.TrimSpace(cur.ExportLicenseNo)
				}

				list = append(list, b)
				groups[ledgerGroupKey(cur)] = true
			}
		}
	}

	sort.SliceStable(list, func(a, b int) bool { return list[a].order < list[b].order })

	out := make([]LicenseChainStep, len(list))
	orderOf := make([]int64, len(list))
	for i, b := range list {
		out[i], orderOf[i] = b.step, b.order
	}
	return out, orderOf, groups
}

// licenseDetailSteps: ประวัติที่จะโชว์ในหน้ารายละเอียด
//
// ใบนำออกใช้โซ่ปกติ ส่วนใบนำเข้าสร้างจากชีตทะเบียน เพราะโซ่ของมันมีขั้นเดียวเสมอ
func licenseDetailSteps(licenseType, licenseNo string, steps []LicenseChainStep) []LicenseChainStep {
	if config.DB == nil {
		return steps
	}
	var ledger []models.LicenseRenewal
	config.DB.Order("sort_order asc, id asc").Find(&ledger)
	if len(ledger) == 0 {
		return steps
	}

	if licenseType == models.LicenseTypeImport {
		if built, orderOf, groups := ledgerStepsForImport(licenseNo, ledger); len(built) > 0 {
			return insertLedgerNotes(built, orderOf, groups, ledger)
		}
	}

	out, orderOf, groups := expandStepsByCountry(licenseType, steps, ledger)
	return insertLedgerNotes(out, orderOf, groups, ledger)
}

// insertLedgerNotes: แทรกข้อความที่ผู้ใช้จดไว้ในไฟล์ กลับเข้าไปในประวัติตามตำแหน่งเดิม
//
// บางแถวในชีตต่ออายุไม่ได้กรอกเลขใบ แต่ใช้จดบันทึก เช่นเหตุผลที่การต่ออายุล่าช้า
// แถวพวกนี้ไม่ใช่การต่ออายุ จึงไม่นับเป็นครั้ง แต่ก็เป็นข้อมูลที่คนทำงานต้องการเห็น
// จึงนำมาแสดงคั่นระหว่างขั้น ตามลำดับเดิมในไฟล์
func insertLedgerNotes(steps []LicenseChainStep, orderOf []int64, chainGroups map[string]bool, ledger []models.LicenseRenewal) []LicenseChainStep {
	if len(chainGroups) == 0 {
		return steps
	}
	type noteRow struct {
		order int64
		text  string
	}
	var notes []noteRow
	for i := range ledger {
		if strings.TrimSpace(ledger[i].Note) == "" {
			continue
		}
		if !chainGroups[ledgerGroupKey(ledger[i])] {
			continue
		}
		notes = append(notes, noteRow{order: ledger[i].SortOrder, text: strings.TrimSpace(ledger[i].Note)})
	}
	if len(notes) == 0 {
		return steps
	}
	sort.Slice(notes, func(a, b int) bool { return notes[a].order < notes[b].order })

	out := make([]LicenseChainStep, 0, len(steps)+len(notes))
	n := 0
	for i := range steps {
		// หมายเหตุที่อยู่ก่อนขั้นนี้ในไฟล์ ให้แสดงก่อน
		for n < len(notes) && orderOf[i] > 0 && notes[n].order < orderOf[i] {
			out = append(out, LicenseChainStep{
				Label:  "หมายเหตุ",
				IsNote: true,
				Note:   notes[n].text,
			})
			n++
		}
		out = append(out, steps[i])
	}
	for ; n < len(notes); n++ {
		out = append(out, LicenseChainStep{
			Label:  "หมายเหตุ",
			IsNote: true,
			Note:   notes[n].text,
		})
	}
	return out
}

// LicenseChain = โซ่ของใบอนุญาต 1 ชุด
type LicenseChain struct {
	LicenseType string `json:"licenseType"`
	// OriginalNo = ใบต้นฉบับ · CurrentNo = ใบที่ใช้อยู่ตอนนี้
	OriginalNo string `json:"originalLicenseNo"`
	CurrentNo  string `json:"currentLicenseNo"`
	// RenewalCount = จำนวนครั้งที่ต่ออายุ (ใบต้นฉบับไม่นับ)
	RenewalCount int `json:"renewalCount"`

	LastRenewalDate *time.Time `json:"lastRenewalDate"`
	CurrentExpire   *time.Time `json:"currentExpireDate"`

	// PreviousNos = เลขใบเก่าทั้งหมดในโซ่ (เรียงจากเก่าไปใหม่)
	PreviousNos []string `json:"previousLicenseNos"`

	Steps []LicenseChainStep `json:"steps"`
}

// renewalLinkIndex: ดัชนีของโซ่ — แยกตามประเภทใบอนุญาต
type renewalLinkIndex struct {
	// next[type|oldNorm] = ประวัติที่บอกว่าใบนี้ต่อเป็นใบไหน
	next map[string]models.LicenseRenewalHistory
	// prev[type|newNorm] = ใบนี้ต่อมาจากใบไหน
	prev map[string]models.LicenseRenewalHistory
	// display[type|norm] = เลขใบตามที่เขียนจริง (ไว้แสดงผล)
	display map[string]string
}

func buildRenewalLinkIndex(rows []models.LicenseRenewalHistory) renewalLinkIndex {
	idx := renewalLinkIndex{
		next:    map[string]models.LicenseRenewalHistory{},
		prev:    map[string]models.LicenseRenewalHistory{},
		display: map[string]string{},
	}
	for _, r := range rows {
		t := strings.ToUpper(strings.TrimSpace(r.LicenseType))
		if t == "" {
			continue
		}
		oldKey := t + "|" + NormalizeCodeValue(r.OldLicenseNo)
		newKey := t + "|" + NormalizeCodeValue(r.NewLicenseNo)
		// แถวแรกที่เจอชนะ (ข้อมูลเรียงตามวันที่ต่ออายุมาแล้ว)
		if _, ok := idx.next[oldKey]; !ok {
			idx.next[oldKey] = r
		}
		if _, ok := idx.prev[newKey]; !ok {
			idx.prev[newKey] = r
		}
		idx.display[oldKey] = strings.TrimSpace(r.OldLicenseNo)
		idx.display[newKey] = strings.TrimSpace(r.NewLicenseNo)
	}
	return idx
}

// chainRootOf: ไล่ย้อนขึ้นไปหาใบต้นฉบับของเลขใบที่ให้มา
func (idx renewalLinkIndex) chainRootOf(licenseType, licenseNo string) string {
	cur := strings.TrimSpace(licenseNo)
	seen := map[string]bool{}
	for {
		key := licenseType + "|" + NormalizeCodeValue(cur)
		if seen[key] {
			return cur // กันข้อมูลวนลูป
		}
		seen[key] = true
		h, ok := idx.prev[key]
		if !ok {
			return cur
		}
		cur = strings.TrimSpace(h.OldLicenseNo)
	}
}

// buildLicenseChain: สร้างโซ่ทั้งชุดจากเลขใบใดก็ได้ในโซ่นั้น
func buildLicenseChain(rows []models.LicenseRenewalHistory, licenseType, licenseNo string) LicenseChain {
	licenseType = strings.ToUpper(strings.TrimSpace(licenseType))
	idx := buildRenewalLinkIndex(rows)
	return buildLicenseChainWith(idx, licenseType, licenseNo)
}

func buildLicenseChainWith(idx renewalLinkIndex, licenseType, licenseNo string) LicenseChain {
	root := idx.chainRootOf(licenseType, licenseNo)

	out := LicenseChain{
		LicenseType: licenseType,
		OriginalNo:  root,
		CurrentNo:   root,
	}

	// ใบต้นฉบับ — ไม่ถือเป็นการต่ออายุ
	out.Steps = append(out.Steps, LicenseChainStep{
		Round:        0,
		Label:        "Original",
		OldLicenseNo: root,
	})

	cur := root
	seen := map[string]bool{}
	for {
		key := licenseType + "|" + NormalizeCodeValue(cur)
		if seen[key] {
			break // กันข้อมูลวนลูป
		}
		seen[key] = true

		h, ok := idx.next[key]
		if !ok {
			break
		}

		out.RenewalCount++
		out.PreviousNos = append(out.PreviousNos, cur)

		step := LicenseChainStep{
			Round:        out.RenewalCount,
			Label:        "ครั้งที่ " + itoa(out.RenewalCount),
			OldLicenseNo: strings.TrimSpace(h.OldLicenseNo),
			NewLicenseNo: strings.TrimSpace(h.NewLicenseNo),
			GroupNo:      strings.TrimSpace(h.GroupNo),
			RenewalDate:  h.RenewalDate,
			ExpireDate:   h.NewExpireDate,
			Remark:       h.Remark,
			FileName:     h.FileName,
			UploadedBy:   h.UploadedBy,
		}
		if !h.UploadedAt.IsZero() {
			t := h.UploadedAt
			step.UploadedAt = &t
		}
		// วันหมดอายุของใบต้นฉบับ (แถว Original) มาจากช่อง Old Expiry Date ของการต่อครั้งแรก
		if out.RenewalCount == 1 && h.OldExpireDate != nil {
			out.Steps[0].ExpireDate = h.OldExpireDate
		}
		out.Steps = append(out.Steps, step)

		out.LastRenewalDate = h.RenewalDate
		out.CurrentExpire = h.NewExpireDate
		cur = strings.TrimSpace(h.NewLicenseNo)
	}

	out.CurrentNo = cur
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// ---------------------------------------------------------------------------
// ใบอนุญาตตั้งต้นจากไฟล์ Import / Export ที่อัปโหลดไว้แล้ว
// ---------------------------------------------------------------------------

// baseLicense = ใบอนุญาต 1 ใบที่รู้จากไฟล์ Import / Export (ยังไม่รวมโซ่ต่ออายุ)
type baseLicense struct {
	Type       string
	No         string
	IssueDate  *time.Time
	ExpireDate *time.Time
	Items      int
	Completed  int
	Country    string
	Model      string

	// countrySeen = ประเทศทั้งหมดที่ใบนี้เกี่ยวข้อง (ใช้กับใบนำเข้าที่แบ่งโควต้าหลายประเทศ)
	countrySeen map[string]bool
	FileName    string
	UploadDate  *time.Time

	// ---- ข้อมูลจากตารางทะเบียนใบอนุญาต (ชีต "ต่ออายุ") ----
	// GroupNo = คอลัมน์ NO. เช่น "Completed 01" หรือ "117"
	// คำว่า Completed ในช่องนี้คือสัญญาณว่ากลุ่มใบอนุญาตนั้นปิดงานแล้ว
	GroupNo         string
	LedgerCompleted bool
	HasLedger       bool
	Quota           int // TOTAL — โควต้าของใบนำเข้า
	Stock           int // STOCK EXPORT LICENSE — จำนวนที่อยู่บนใบนำออก
	Remain          int // REMAIN — ยอดคงเหลือ

	// ใบนำเข้า 1 ใบมีใบนำออกออกมาได้หลายใบ = ต่ออายุหลายครั้ง
	// นับจากจำนวนใบนำออกในชีตต่ออายุที่อยู่ใต้ใบนำเข้าใบนี้
	LedgerExports   int
	LedgerLastIssue *time.Time

	// ขั้นสุดท้ายของโซ่มีได้หลายสาย (แบ่งโควต้าตามประเทศ)
	// ใบนำเข้าจะปิดงานก็ต่อเมื่อปิดครบทุกสาย
	LastStepRows int
	LastStepDone int
	Branches     []LicenseBranch
}

// LicenseBranch = ใบนำออก 1 ใบในขั้นสุดท้ายของใบนำเข้าใบหนึ่ง
//
// ใบนำเข้าใบเดียวแบ่งโควต้าออกเป็นหลายประเทศได้ เช่น TOTAL 60
// แตกเป็น INDONESIA 50 / MALAYSIA 10 ตัวเลขรวมอย่างเดียวจึงไม่พอให้คนทำงานใช้
type LicenseBranch struct {
	Country         string `json:"country"`
	ExportLicenseNo string `json:"exportLicenseNo"`
	Stock           int    `json:"stock"`
	Remain          int    `json:"remain"`
	Completed       bool   `json:"completed"`

	// แต่ละสายต่ออายุของตัวเอง จำนวนครั้งและวันหมดอายุจึงไม่เท่ากัน
	// เช่น E05036900826 สาย INDONESIA ต่อ 3 ครั้ง หมด 13/08 ส่วน MALAYSIA ต่อ 6 ครั้ง หมด 09/10
	RenewalCount int        `json:"renewalCount"`
	ExpireDate   *time.Time `json:"expireDate"`
}

// ledgerGroupCompleted: คอลัมน์ NO. ในชีตต่ออายุบอกว่ากลุ่มนี้ปิดงานแล้วหรือยัง
// ในไฟล์จริงเขียนเป็น "Completed 01", "Completed 02" ... ส่วนกลุ่มที่ยังทำอยู่เป็นตัวเลขล้วน
func ledgerGroupCompleted(groupNo string) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(groupNo)), "COMPLETED")
}

// addCountry: รวมประเทศของใบนำเข้าจากทุกแถวที่อ้างถึงใบนั้น
//
// ใบนำเข้าใบเดียวแบ่งโควต้าได้หลายประเทศ เช่น E05036902379 มีทั้ง INDONESIA
// และ MALAYSIA ของเดิมเก็บแค่ประเทศของแถวแรกที่เจอ ใบนั้นจึงขึ้นว่า "Indonesia"
// เฉย ๆ แล้วหายไปจากตัวกรอง Malaysia ทั้งที่มีโควต้าของ Malaysia อยู่จริง
//
// เก็บเป็นชุด แล้วต่อกันด้วย ", " ซึ่งเป็นรูปแบบเดียวกับที่ผู้ใช้เขียนในไฟล์
// ฝั่งหน้าเว็บ (countryKeys) แยกด้วยลูกน้ำอยู่แล้ว จึงกรองเจอครบทุกประเทศ
func (b *baseLicense) addCountry(raw string) {
	if b == nil {
		return
	}
	names := ledgerCountryNames(raw)
	if len(names) == 0 {
		return
	}
	if b.countrySeen == nil {
		b.countrySeen = map[string]bool{}
	}
	changed := false
	for _, name := range names {
		if b.countrySeen[name] {
			continue
		}
		b.countrySeen[name] = true
		changed = true
	}
	if !changed {
		return
	}
	all := make([]string, 0, len(b.countrySeen))
	for name := range b.countrySeen {
		all = append(all, name)
	}
	sort.Strings(all)
	b.Country = strings.Join(all, ", ")
}

// ledgerRenewPending: ยื่นเรื่องต่ออายุไว้แล้วแต่ยังไม่ได้เอกสารใบใหม่
// ตราบใดที่เรื่องยังเดินอยู่ ใบนี้ยังไม่ปิด แม้จะเลยวันหมดอายุไปแล้ว
func ledgerRenewPending(r models.LicenseRenewal) bool {
	return r.ReceivedDate == nil && (r.EmailDate != nil || r.PaymentDate != nil)
}

// ledgerChainCompleted: สายนี้ปิดแล้วหรือยัง ตัดสินจากแถวสุดท้ายของสาย
//
// ใบนำออกปิดเมื่อ "หมดอายุ" เท่านั้น ไม่ใช่เมื่อ REMAIN ลงถึง 0
//
// ของบนใบที่ขายไม่ออกจะค้างอยู่จนใบหมดอายุไปเอง แล้วกลายเป็นรายการคงค้าง
// ไม่ใช่เรื่องผิดปกติ และไม่ได้แปลว่างานยังไม่จบ
// ในทางกลับกัน REMAIN = 0 บนใบที่ยังไม่หมดอายุก็ไม่ได้แปลว่าปิดแล้ว
// เพราะผู้ใช้ยังเพิ่มรายการเข้ามาบนใบนั้นได้อีก
//
// จึงถือว่ายังไม่ปิด เมื่อ
//
//	ยังไม่ถึงวันหมดอายุ              — ใบยังใช้ได้ ยังตัดของออกได้อีก
//	ไม่มีวันหมดอายุในไฟล์            — ไม่รู้ จึงไม่เดาว่าปิด
//	ยื่นต่ออายุค้างอยู่ (ยังไม่ได้ใบใหม่) — งานยังเดินอยู่
//
// ยกเว้นคอลัมน์ NO. เขียนคำว่า Completed ไว้ตรง ๆ อันนั้นผู้ใช้สั่งเองถือว่าปิด
func ledgerChainCompleted(last models.LicenseRenewal, now time.Time) bool {
	if ledgerGroupCompleted(last.GroupNo) {
		return true
	}
	if ledgerRenewPending(last) {
		return false
	}
	if last.ExpireDate == nil {
		return false
	}
	return last.ExpireDate.Truncate(24 * time.Hour).Before(now.Truncate(24 * time.Hour))
}

// collectBaseLicenses: รวบรวมเลขใบอนุญาตทั้งหมดที่มีในระบบ แยกตามประเภท
// key = type|normalized license no
func collectBaseLicenses() map[string]*baseLicense {
	out := map[string]*baseLicense{}
	now := time.Now()

	touch := func(t, no string) *baseLicense {
		no = strings.TrimSpace(no)
		if no == "" {
			return nil
		}
		key := t + "|" + NormalizeCodeValue(no)
		if b, ok := out[key]; ok {
			return b
		}
		b := &baseLicense{Type: t, No: strings.ToUpper(no)}
		out[key] = b
		return b
	}

	// --- Import License ---
	var imports []models.LicenseItem
	config.DB.Order("sort_order asc, id asc").Find(&imports)
	for i := range imports {
		b := touch(models.LicenseTypeImport, imports[i].LicenseNo)
		if b == nil {
			continue
		}
		b.Items++
		// เสร็จสิ้นได้ 2 ทาง: ค่าที่บันทึกไว้ หรือคำว่า "เสร็จสิ้น" ในช่องหมายเหตุของไฟล์
		// (เผื่อข้อมูลเก่าที่อัปโหลดไว้ก่อนระบบจะอ่านหมายเหตุ จะได้ขึ้นสถานะถูกทันทีโดยไม่ต้องอัปโหลดซ้ำ)
		if imports[i].Completed || models.NoteMeansCompleted(imports[i].Remark) {
			b.Completed++
		}
		if b.IssueDate == nil && imports[i].IssueDate != nil {
			b.IssueDate = imports[i].IssueDate
		}
		exp := imports[i].ExpireDate
		if exp == nil && imports[i].IssueDate != nil {
			e := imports[i].IssueDate.AddDate(0, models.ImportLicenseValidityMonths, 0)
			exp = &e
		}
		if exp != nil && (b.ExpireDate == nil || exp.After(*b.ExpireDate)) {
			b.ExpireDate = exp
		}
		b.addCountry(imports[i].ExportCountry)
		if b.Model == "" {
			b.Model = imports[i].Model
		}
		if b.FileName == "" {
			b.FileName = imports[i].FileName
		}
		if !imports[i].UploadDate.IsZero() && (b.UploadDate == nil || imports[i].UploadDate.After(*b.UploadDate)) {
			t := imports[i].UploadDate
			b.UploadDate = &t
		}
	}

	// --- Export License ---
	//
	// ใบนำออกไม่มีตารางของตัวเองแล้ว — ข้อมูลอยู่ในทะเบียนเครื่องชุดเดียวกับใบนำเข้า
	// เพราะไฟล์บัญชีแสดงหมายเลขเครื่องมีคอลัมน์เลขใบอนุญาตนำออกอยู่ในแถวเดียวกัน
	// เครื่องหนึ่งเครื่องจึงถูกนับทั้งในใบนำเข้าของมันและในใบนำออกของมัน
	for i := range imports {
		exportNo := strings.TrimSpace(imports[i].ExportLicenseNo)
		if exportNo == "" {
			continue
		}
		b := touch(models.LicenseTypeExport, exportNo)
		if b == nil {
			continue
		}
		b.Items++
		if importItemDone(imports[i]) {
			b.Completed++
		}
		if b.IssueDate == nil && imports[i].ExportIssueDate != nil {
			b.IssueDate = imports[i].ExportIssueDate
		}
		if exp := imports[i].ExportEffectiveExpireDate(); exp != nil &&
			(b.ExpireDate == nil || exp.After(*b.ExpireDate)) {
			b.ExpireDate = exp
		}
		if b.Country == "" {
			b.Country = imports[i].ExportCountry
		}
		if b.FileName == "" {
			b.FileName = imports[i].FileName
		}
		if !imports[i].UploadDate.IsZero() && (b.UploadDate == nil || imports[i].UploadDate.After(*b.UploadDate)) {
			t := imports[i].UploadDate
			b.UploadDate = &t
		}
	}

	// --- ทะเบียนใบอนุญาต (ชีตต่ออายุ) — เติมใบที่ยังไม่มีในสองไฟล์ข้างบน ---
	//
	// ชีตนี้เป็นที่เดียวที่มีคอลัมน์ NO. / TOTAL / STOCK / REMAIN
	// ใบนำออก 1 ใบ = 1 แถว จึงหยิบค่ามาตรง ๆ ได้
	// ใบนำเข้า 1 ใบมีได้หลายแถว (ต่ออายุหลายครั้ง) จึงรวมยอดและใช้ค่าของแถวล่าสุด
	var ledger []models.LicenseRenewal
	config.DB.Order("sort_order asc, id asc").Find(&ledger)

	// สถานะ "เสร็จสิ้น" อ่านจากชีตต่ออายุตามลำดับนี้
	//
	//   1. จัดกลุ่มตามคอลัมน์ NO. + เลขใบนำเข้า (1 โซ่ = 1 ใบนำเข้า)
	//   2. เรียงแถวตามลำดับในไฟล์
	//   3. แถวสุดท้ายของโซ่ = ใบที่ใช้อยู่จริง (Current License)
	//   4. ชื่อกลุ่มว่า Completed = งานชุดนั้นปิดแล้ว → ใบสุดท้ายของทุกโซ่ในกลุ่มขึ้นเสร็จสิ้น
	//
	//   5. หรือแถวสุดท้ายของโซ่มี REMAIN เหลือ 0 หรือติดลบ = นำออกหมดแล้ว
	//   6. หรือแถวสุดท้ายมี REMAIN เท่ากับแถวก่อนหน้า = ยอดไม่ขยับแล้ว
	//
	// ข้อ 5 และ 6 จำเป็นเพราะผู้ใช้เขียนคำว่า Completed ไว้เฉพาะช่วงแรก ภายหลังไม่ได้เขียนแล้ว
	// ถ้าดูแต่ชื่อกลุ่ม ใบที่ปิดงานไปนานแล้วจะค้างอยู่ในรายการแจ้งเตือนตลอดไป
	//
	// REMAIN ติดลบคือกรอกเกินโควต้า (พิมพ์ผิดหน้างาน) ก็ถือว่าหมดเหมือนกัน
	// ส่วน REMAIN ที่ยังไม่ได้กรอกเลยไม่นับ เพราะช่องว่างกับเลข 0 คนละความหมาย
	//
	// ข้อ 6 คือกรณีของที่เหลือขายไม่ออก เช่นโซ่ที่ลงท้ายด้วย REMAIN 10 → 3 → 3 → 3
	// ยอดหยุดนิ่งตั้งแต่แถวก่อนสุดท้าย แปลว่าไม่มีการตัดออกอีกแล้วทั้งที่ยังเหลือ 3
	// ต้องเทียบกับแถวก่อนหน้าเท่านั้น เทียบแค่แถวสุดท้ายแถวเดียวไม่ได้
	// ไม่งั้นทุกโซ่จะกลายเป็นเสร็จสิ้นหมด แล้วระบบจะไม่เตือนอะไรเลย
	// และโซ่ที่มีแถวเดียวก็ไม่เข้าข้อนี้ เพราะไม่มีแถวก่อนหน้าให้เทียบ
	//
	// ดูเฉพาะแถวสุดท้ายของโซ่ เพราะระหว่างทาง REMAIN แตะ 0 หรือซ้ำกันชั่วคราวได้
	// ก่อนจะมีการต่ออายุใบถัดไปมาเติมโควต้า (ยื่นต่ออายุมักล่าช้ากว่าของจริง)
	//
	// แถวก่อนหน้าในโซ่เป็นใบเก่าที่ถูกต่ออายุไปแล้ว ไม่ใช่ใบที่เสร็จ
	// และระบบจะไม่สร้างการต่ออายุใบถัดไปเองเพียงเพราะโควต้าหมด
	//   7. ใบนำเข้าใบเดียวแตกเป็นใบนำออกหลายใบพร้อมกันได้ (แบ่งโควต้าตามประเทศ)
	//      แถวที่ออกวันเดียวกันถือเป็นขั้นเดียวกัน ไม่ใช่การต่ออายุคนละครั้ง
	//      จึงต้องดู "ขั้นสุดท้าย" ทั้งขั้น ไม่ใช่แถวสุดท้ายแถวเดียว
	chainRows := map[string][]int{}
	chainOrder := []string{}
	for i := range ledger {
		k := ledgerGroupKey(ledger[i])
		if _, seen := chainRows[k]; !seen {
			chainOrder = append(chainOrder, k)
		}
		chainRows[k] = append(chainRows[k], i)
	}

	// lastStepRow = แถวที่อยู่ในขั้นสุดท้ายของโซ่ (มีได้หลายแถวถ้าแบ่งตามประเทศ)
	// completedRow = แถวในขั้นสุดท้ายที่ถือว่าปิดงานแล้ว
	lastStepRow := map[int]bool{}
	completedRow := map[int]bool{}
	// branchRenewals = สายของแถวนี้ต่ออายุไปกี่ครั้ง (ขั้นแรกไม่นับ)
	branchRenewals := map[int]int{}
	for _, k := range chainOrder {
		idxs := chainRows[k]
		group := make([]models.LicenseRenewal, len(idxs))
		for n, i := range idxs {
			group[n] = ledger[i]
		}
		// แต่ละประเทศเป็นคนละสาย มีขั้นสุดท้ายของตัวเอง
		// ใบนำเข้าจะปิดงานก็ต่อเมื่อปิดครบทุกสาย
		for _, bidx := range ledgerBranchIndexes(group) {
			steps := ledgerSteps(ledgerPick(group, bidx))
			if len(steps) == 0 {
				continue
			}

			last := steps[len(steps)-1]
			base := len(bidx) - len(last) // ตำแหน่งเริ่มของขั้นสุดท้ายในสายนี้

			for n, cur := range last {
				i := idxs[bidx[base+n]]
				lastStepRow[i] = true
				branchRenewals[i] = len(steps) - 1
				if ledgerChainCompleted(cur, now) {
					completedRow[i] = true
				}
			}
		}
	}

	for i := range ledger {
		if b := touch(models.LicenseTypeImport, ledger[i].ImportLicenseNo); b != nil {
			// ประเทศของใบนำเข้ามาจากทุกสายที่แบ่งไว้ ไม่ใช่สายแรกที่เจอ
			b.addCountry(ledger[i].Country)
			// ไม่เอา IT CONTROLLER MODEL (เป็น P/N ของ IT controller) มาเป็น "รุ่น" ของใบนำเข้า
			// รุ่นของใบนำเข้าอ่านจากคอลัมน์ "แบบ/รุ่น" ในไฟล์รายการเครื่องเท่านั้น
			// ใบที่รู้จักจากทะเบียนต่ออายุอย่างเดียวจึงแสดงรุ่นเป็นขีด
			b.HasLedger = true
			if b.GroupNo == "" {
				b.GroupNo = ledger[i].GroupNo
			}
			// ใบนำออกแต่ละใบใต้ใบนำเข้านี้ = การต่ออายุ 1 ครั้ง
			if strings.TrimSpace(ledger[i].ExportLicenseNo) != "" {
				b.LedgerExports++
				if ledger[i].IssueDate != nil {
					b.LedgerLastIssue = ledger[i].IssueDate
				}
			}
			// ปิดงานเฉพาะใบนำเข้าที่อยู่ในขั้นสุดท้ายของโซ่เท่านั้น
			// และต้องปิดครบทุกสาย ถ้าแบ่งตามประเทศแล้วสายใดสายหนึ่งยังเดินอยู่ ถือว่ายังไม่จบ
			if lastStepRow[i] {
				b.LastStepRows++
				if completedRow[i] {
					b.LastStepDone++
					b.GroupNo = ledger[i].GroupNo
				}
				// STOCK / REMAIN ของใบนำเข้า = ยอดของทุกสายในขั้นสุดท้ายรวมกัน
				// เช่น INDONESIA 50 + MALAYSIA 10 = 60 ไม่ใช่ 10 ของแถวสุดท้ายแถวเดียว
				b.Stock += ledger[i].Stock
				b.Remain += ledger[i].Remain
				b.Branches = append(b.Branches, LicenseBranch{
					Country:         strings.TrimSpace(ledger[i].Country),
					ExportLicenseNo: strings.TrimSpace(ledger[i].ExportLicenseNo),
					Stock:           ledger[i].Stock,
					Remain:          ledger[i].Remain,
					Completed:       completedRow[i],
					RenewalCount:    branchRenewals[i],
					ExpireDate:      ledger[i].ExpireDate,
				})
			}
			// TOTAL คือโควต้าของใบนำเข้า จึงใช้ค่าที่มากที่สุดที่เจอ
			if ledger[i].Total > b.Quota {
				b.Quota = ledger[i].Total
			}
		}
		if b := touch(models.LicenseTypeExport, ledger[i].ExportLicenseNo); b != nil {
			if b.IssueDate == nil {
				b.IssueDate = ledger[i].IssueDate
			}
			if b.ExpireDate == nil {
				b.ExpireDate = ledger[i].ExpireDate
			}
			if b.Country == "" {
				b.Country = ledger[i].Country
			}
			if b.Model == "" {
				b.Model = ledger[i].ITControllerModel
			}
			b.HasLedger = true
			b.GroupNo = ledger[i].GroupNo
			// เฉพาะใบนำออกบนแถวสุดท้ายของโซ่ในกลุ่ม Completed เท่านั้นที่ถือว่าเสร็จ
			b.LedgerCompleted = completedRow[i]
			b.Quota = ledger[i].Total
			b.Stock = ledger[i].Stock
			b.Remain = ledger[i].Remain
		}
	}

	// ใบนำเข้าปิดงานก็ต่อเมื่อทุกสายในขั้นสุดท้ายปิดครบ
	for _, b := range out {
		if b.Type == models.LicenseTypeImport && b.LastStepRows > 0 {
			b.LedgerCompleted = b.LastStepDone == b.LastStepRows
		}
	}

	return out
}

// ---------------------------------------------------------------------------
// ผลลัพธ์หน้า License Overview
// ---------------------------------------------------------------------------

type LicenseOverviewRow struct {
	LicenseType      string `json:"licenseType"`
	LicenseTypeLabel string `json:"licenseTypeLabel"`

	CurrentLicenseNo  string `json:"currentLicenseNo"`
	OriginalLicenseNo string `json:"originalLicenseNo"`

	IssueDate  *time.Time `json:"issueDate"`
	ExpiryDate *time.Time `json:"expiryDate"`
	DaysLeft   *int       `json:"daysLeft"`

	Status string `json:"status"`

	RenewalCount    int        `json:"renewalCount"`
	LastRenewalDate *time.Time `json:"lastRenewalDate"`

	PreviousLicenseNos []string `json:"previousLicenseNos"`

	Country string `json:"country"`
	Model   string `json:"model"`

	Items          int  `json:"items"`
	CompletedItems int  `json:"completedItems"`
	Completed      bool `json:"completed"`

	// AutoClosed = ใบนี้ปิดจบเพราะหมดอายุแล้วไม่มีใครต่ออายุ ไม่ใช่เพราะคนกดปิด
	// แยกธงไว้เพื่อให้หน้าเว็บบอกผู้ใช้ได้ว่าทำไมใบถึงหายไปจากรายการที่ต้องทำ
	AutoClosed      bool   `json:"autoClosed"`
	AutoCloseReason string `json:"autoCloseReason,omitempty"`

	// AutoCloseInDays = ใบที่หมดอายุแล้วแต่ยังอยู่ในระยะผ่อนผัน เหลืออีกกี่วันจึงจะถูกปิด
	//
	// ตอบคำถาม "ใบหมดอายุไปแล้ว ทำไมยังไม่เสร็จสิ้น" ไว้บนหน้าจอเลย
	// ไม่งั้นระยะผ่อนผันจะเป็นความลับที่มีแต่คนแก้ .env เท่านั้นที่รู้
	AutoCloseInDays *int `json:"autoCloseInDays,omitempty"`

	// LeadDate = วันครบกำหนดยื่นขอต่ออายุ (เฉพาะใบนำออก)
	// ใบนำออกมีอายุ 1 เดือน และต้องยื่นเข้าระบบ กสทช. ล่วงหน้า 15 วันทำการ
	LeadDate     *time.Time `json:"leadDate"`
	LeadDaysLeft *int       `json:"leadDaysLeft"`

	// ---- จากตารางทะเบียนใบอนุญาต (ชีต "ต่ออายุ") ----
	GroupNo   string `json:"groupNo"`
	HasLedger bool   `json:"hasLedger"`
	Quota     int    `json:"quota"`
	Stock     int    `json:"stock"`
	Remain    int    `json:"remain"`
	// LedgerCompleted = คอลัมน์ NO. เขียนว่า Completed
	LedgerCompleted bool `json:"ledgerCompleted"`

	// Branches = ใบนำออกที่ออกพร้อมกันใต้ใบนำเข้าใบนี้ แบ่งตามประเทศ
	// มีค่าเฉพาะใบที่แบ่งจริงมากกว่า 1 สาย
	Branches []LicenseBranch `json:"branches,omitempty"`

	FileName   string     `json:"fileName"`
	UploadDate *time.Time `json:"uploadDate"`
}

// licenseStatusOf: สถานะของใบปัจจุบัน — ใช้ตรรกะเดิมของระบบ
// fillExportLeadDate: เติมวันครบกำหนดยื่นขอต่ออายุให้ใบนำออก
//
// ใบนำออกมีอายุ 1 เดือน และต้องยื่นเข้าระบบ กสทช. ล่วงหน้า 15 วันทำการ
// จึงนับถอยหลังจากวันหมดอายุแบบข้ามเสาร์-อาทิตย์
// ใบที่ปิดงานแล้วไม่ต้องยื่นอีก จึงไม่ต้องคิดกำหนดให้
func fillExportLeadDate(row *LicenseOverviewRow, now time.Time) {
	if row.LicenseType != models.LicenseTypeExport || row.ExpiryDate == nil || row.Completed {
		return
	}
	lead := models.SubtractBusinessDays(*row.ExpiryDate, models.ExportLicenseLeadDays)
	row.LeadDate = &lead
	d := models.DaysBetween(now, lead)
	row.LeadDaysLeft = &d
}

func licenseStatusOf(licenseType string, expire *time.Time, completed bool, now time.Time) (string, *int) {
	if completed {
		return models.LicenseStatusCompleted, nil
	}
	if expire == nil {
		return models.LicenseStatusNoDate, nil
	}
	days := models.DaysBetween(now, *expire)
	within := ExportLicenseExpiringWithinDays
	if licenseType == models.LicenseTypeImport {
		within = ImportLicenseExpiringWithinDays
	}
	switch {
	case days < 0:
		return models.LicenseStatusExpired, &days
	case days <= within:
		return models.LicenseStatusExpiring, &days
	}
	return models.LicenseStatusValid, &days
}

// buildLicenseOverview: รวม Import + Export เป็นรายการเดียว (1 แถว = 1 โซ่ใบอนุญาต)
// loadRenewalHistoryWithLedger: ประวัติการต่ออายุที่ใช้สร้างภาพรวม
//
// ชีต "ต่ออายุ" บอกโซ่ไว้ในตัวเองอยู่แล้ว — ใบนำออกหลายใบใต้ใบนำเข้าเดียวกัน
// เรียงตามวันที่ออกใบ = การต่ออายุต่อเนื่องกัน ใบสุดท้ายคือใบที่ใช้อยู่จริง
//
// เดิมระบบใช้เฉพาะตาราง license_renewal_history ซึ่งต้องผ่านการตรวจตอนอัปโหลด
// ถ้าแถวไหนถูกตีตก (เช่นยังไม่ได้อัปไฟล์ License ของใบเดิม) โซ่จะขาด
// ทำให้ใบเก่าโผล่เป็นแถวของตัวเอง และสถานะ "ปิดงานแล้ว" ตามคอลัมน์ NO. ก็หายไปด้วย
//
// จึงอ่านโซ่จากตารางทะเบียนมาเติมให้ครบเสมอ คู่ที่มีอยู่แล้วไม่เติมซ้ำ
func loadRenewalHistoryWithLedger() []models.LicenseRenewalHistory {
	history := loadRenewalHistory()

	var ledger []models.LicenseRenewal
	config.DB.Order("sort_order asc, id asc").Find(&ledger)
	if len(ledger) == 0 {
		return history
	}

	have := map[string]bool{}
	for _, h := range history {
		key := strings.ToUpper(strings.TrimSpace(h.LicenseType)) + "|" +
			NormalizeCodeValue(h.OldLicenseNo) + "|" + NormalizeCodeValue(h.NewLicenseNo)
		have[key] = true
	}

	for _, d := range readRenewalHistoryFromLedger(ledger) {
		h := d.LicenseRenewalHistory
		key := strings.ToUpper(strings.TrimSpace(h.LicenseType)) + "|" +
			NormalizeCodeValue(h.OldLicenseNo) + "|" + NormalizeCodeValue(h.NewLicenseNo)
		if have[key] {
			continue
		}
		have[key] = true
		history = append(history, h)
	}
	return history
}

func buildLicenseOverview() []LicenseOverviewRow {
	history := loadRenewalHistoryWithLedger()
	idx := buildRenewalLinkIndex(history)
	base := collectBaseLicenses()
	now := time.Now()
	autoCloseDays := LicenseAutoCloseDays()

	// ใบที่เป็น "ใบเก่า" ของโซ่ ไม่ต้องขึ้นเป็นแถวของตัวเอง
	renewedAway := map[string]bool{}
	for k := range idx.next {
		renewedAway[k] = true
	}

	// รวมเลขใบจากทั้งสองแหล่ง (ไฟล์ License + ตารางต่ออายุ)
	keys := make([]string, 0, len(base)+len(idx.display))
	for k := range base {
		keys = append(keys, k)
	}
	for k := range idx.display {
		if _, ok := base[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	out := make([]LicenseOverviewRow, 0, len(keys))
	for _, key := range keys {
		if renewedAway[key] {
			continue // ใบนี้ถูกต่ออายุไปแล้ว → จะไปโผล่เป็นประวัติของใบปัจจุบัน
		}

		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		licenseType := parts[0]

		no := ""
		if b, ok := base[key]; ok {
			no = b.No
		} else {
			no = idx.display[key]
		}
		if strings.TrimSpace(no) == "" {
			continue
		}

		chain := buildLicenseChainWith(idx, licenseType, no)

		row := LicenseOverviewRow{
			LicenseType:        licenseType,
			LicenseTypeLabel:   models.LicenseTypeLabel(licenseType),
			CurrentLicenseNo:   no,
			OriginalLicenseNo:  chain.OriginalNo,
			RenewalCount:       chain.RenewalCount,
			LastRenewalDate:    chain.LastRenewalDate,
			PreviousLicenseNos: chain.PreviousNos,
		}

		// ข้อมูลของใบปัจจุบัน — เอาจากไฟล์ License ก่อน ถ้าไม่มีค่อยใช้วันหมดอายุจากไฟล์ต่ออายุ
		if b, ok := base[key]; ok {
			row.IssueDate = b.IssueDate
			row.ExpiryDate = b.ExpireDate
			row.Country = b.Country
			row.Model = b.Model
			row.Items = b.Items
			row.CompletedItems = b.Completed
			row.FileName = b.FileName
			row.UploadDate = b.UploadDate
			row.GroupNo = b.GroupNo
			row.HasLedger = b.HasLedger
			row.Quota = b.Quota
			row.Stock = b.Stock
			row.Remain = b.Remain
			row.LedgerCompleted = b.LedgerCompleted
			// แสดงเฉพาะตอนที่แบ่งจริงมากกว่า 1 สาย สายเดียวดูจากยอดรวมก็พอ
			if len(b.Branches) > 1 {
				row.Branches = b.Branches
			}

			// ใบนำเข้าแทบไม่เคยเปลี่ยนเลขใบ โซ่การต่ออายุของมันจึงว่างเสมอ
			// แต่ในทางปฏิบัติ "การต่ออายุ" ของใบนำเข้าคือการออกใบนำออกใบใหม่ใต้โควต้าเดิม
			// จึงนับจากชีตต่ออายุแทน ไม่งั้นใบที่ออกใบนำออกมา 14 ใบ
			// จะขึ้นว่าต่ออายุ 0 ครั้ง ซึ่งขัดกับสิ่งที่เห็นในไฟล์
			if licenseType == models.LicenseTypeImport && row.RenewalCount == 0 && b.LedgerExports > 1 {
				// ใบที่แบ่งโควต้าหลายประเทศ แต่ละสายต่ออายุของตัวเอง
				// "จำนวนใบนำออกทั้งหมด - 1" จึงไม่ตรงกับเลขครั้งที่ในตารางประวัติ
				// เช่น E05036900826 ออกใบนำออก 8 ใบ แต่สายยาวสุด (MALAYSIA) ต่อแค่ 6 ครั้ง
				// เพราะใบของ INDONESIA ไม่ได้ต่อจากสาย MALAYSIA
				// ใช้สายที่ยาวที่สุดเป็นตัวเลขของใบนำเข้า จะได้ตรงกับแถวสุดท้ายของตาราง
				row.RenewalCount = b.LedgerExports - 1
				if len(b.Branches) > 1 {
					longest := 0
					for _, br := range b.Branches {
						if br.RenewalCount > longest {
							longest = br.RenewalCount
						}
					}
					row.RenewalCount = longest
				}
				if row.LastRenewalDate == nil {
					row.LastRenewalDate = b.LedgerLastIssue
				}
			}

			// ใบนำเข้าที่ไม่มีวันหมดอายุของตัวเอง (ไม่อยู่ในบัญชีหมายเลขเครื่อง)
			// ใช้วันหมดอายุของสายที่ใกล้หมดที่สุด เพราะเป็นสายที่ต้องรีบจัดการก่อน
			// ไม่งั้นใบจะขึ้นว่า "ยังไม่ระบุวันหมดอายุ" ทั้งที่ใบนำออกใต้มันมีวันครบทุกใบ
			if licenseType == models.LicenseTypeImport && row.ExpiryDate == nil {
				for _, br := range b.Branches {
					if br.ExpireDate == nil {
						continue
					}
					if row.ExpiryDate == nil || br.ExpireDate.Before(*row.ExpiryDate) {
						row.ExpiryDate = br.ExpireDate
					}
				}
			}
		}
		if row.ExpiryDate == nil && chain.CurrentExpire != nil {
			row.ExpiryDate = chain.CurrentExpire
		}
		// ใบต้นฉบับที่ยังไม่เคยต่ออายุ และไม่มีข้อมูลจากไฟล์ License
		//
		// ใช้ได้เฉพาะตอนที่ใบนี้คือใบต้นฉบับของโซ่จริง ๆ เท่านั้น
		// ถ้าเป็นใบปลายโซ่ที่ยังไม่มีวันหมดอายุ (เช่นยื่นแล้วแต่ยังไม่ได้เลขใบจริง
		// ผู้ใช้จดเลขรับเรื่องไว้ในช่องเลขใบไปก่อน) การหยิบวันของขั้นแรกมาใช้
		// จะกลายเป็นเอาวันหมดอายุของ "ใบเก่า" มาแปะให้ใบใหม่
		// แล้วใบที่ยังไม่ออกจะขึ้นว่าหมดอายุไปแล้วทันที
		if row.ExpiryDate == nil && len(chain.Steps) > 0 &&
			NormalizeCodeValue(chain.OriginalNo) == NormalizeCodeValue(no) {
			row.ExpiryDate = chain.Steps[0].ExpireDate
		}

		// เผื่อกรณีตารางทะเบียนถูกล้างไปแล้วแต่ประวัติการต่ออายุยังอยู่
		// ใช้ได้เฉพาะใบที่ไม่มีข้อมูลทะเบียนเหลือแล้วเท่านั้น
		// ถ้ายังมีทะเบียนอยู่ ต้องเชื่อผลการตรวจ REMAIN ข้างบนเป็นหลัก
		if !row.LedgerCompleted && !row.HasLedger {
			for _, st := range chain.Steps {
				if ledgerGroupCompleted(st.GroupNo) {
					row.LedgerCompleted = true
					if row.GroupNo == "" {
						row.GroupNo = st.GroupNo
					}
					break
				}
			}
		}

		// เสร็จสิ้นได้ 3 ทาง: เครื่องในใบนั้นปิดงานครบทุกเครื่อง
		// ชีตต่ออายุเขียนไว้แล้วว่ากลุ่มนี้ Completed
		// หรือหมดอายุมานานแล้วโดยไม่มีใครต่ออายุ = ผู้ใช้ไม่เอาใบนี้ต่อแล้ว
		row.Completed = row.LedgerCompleted || (row.Items > 0 && row.CompletedItems >= row.Items)
		if !row.Completed && models.LicenseAutoCloseDue(row.ExpiryDate, autoCloseDays, now) {
			row.Completed = true
			row.AutoClosed = true
			row.AutoCloseReason = models.LicenseAutoCloseNote
		} else if !row.Completed {
			row.AutoCloseInDays = models.LicenseAutoCloseInDays(row.ExpiryDate, autoCloseDays, now)
		}
		row.Status, row.DaysLeft = licenseStatusOf(licenseType, row.ExpiryDate, row.Completed, now)
		fillExportLeadDate(&row, now)

		out = append(out, row)
	}

	// เรียงให้ใบที่ต้องรีบดูขึ้นก่อน
	rank := map[string]int{
		models.LicenseStatusExpiring:  0,
		models.LicenseStatusExpired:   1,
		models.LicenseStatusValid:     2,
		models.LicenseStatusNoDate:    3,
		models.LicenseStatusCompleted: 4,
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := rank[out[i].Status], rank[out[j].Status]
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
		if di != dj {
			return di < dj
		}
		return out[i].CurrentLicenseNo < out[j].CurrentLicenseNo
	})
	return out
}

// LicenseOverviewSummary = ตัวเลขบนหัวหน้า License Overview
type LicenseOverviewSummary struct {
	Total      int `json:"total"`
	Import     int `json:"import"`
	Export     int `json:"export"`
	Active     int `json:"active"`
	NearExpiry int `json:"nearExpiry"`
	Expired    int `json:"expired"`
	Completed  int `json:"completed"`
	Renewed    int `json:"renewed"`
}

func licenseOverviewSummary(rows []LicenseOverviewRow) LicenseOverviewSummary {
	s := LicenseOverviewSummary{Total: len(rows)}
	for _, r := range rows {
		switch r.LicenseType {
		case models.LicenseTypeImport:
			s.Import++
		case models.LicenseTypeExport:
			s.Export++
		}
		switch r.Status {
		case models.LicenseStatusValid:
			s.Active++
		case models.LicenseStatusExpiring:
			s.Active++
			s.NearExpiry++
		case models.LicenseStatusExpired:
			s.Expired++
		case models.LicenseStatusCompleted:
			s.Completed++
		}
		if r.RenewalCount > 0 {
			s.Renewed++
		}
	}
	return s
}

// ตัวกรองบนหน้า License Overview
const (
	LicenseFilterAll        = "ALL"
	LicenseFilterImport     = "IMPORT"
	LicenseFilterExport     = "EXPORT"
	LicenseFilterActive     = "ACTIVE"
	LicenseFilterNearExpiry = "NEAR_EXPIRY"
	LicenseFilterExpired    = "EXPIRED"
	LicenseFilterCompleted  = "COMPLETED"
)

func licenseRowMatchesFilter(r LicenseOverviewRow, filter string) bool {
	switch filter {
	case "", LicenseFilterAll:
		return true
	case LicenseFilterImport:
		return r.LicenseType == models.LicenseTypeImport
	case LicenseFilterExport:
		return r.LicenseType == models.LicenseTypeExport
	case LicenseFilterActive:
		return r.Status == models.LicenseStatusValid || r.Status == models.LicenseStatusExpiring
	case LicenseFilterNearExpiry:
		return r.Status == models.LicenseStatusExpiring
	case LicenseFilterExpired:
		return r.Status == models.LicenseStatusExpired
	case LicenseFilterCompleted:
		return r.Status == models.LicenseStatusCompleted
	}
	return true
}

// GetLicenseOverview: GET /license-overview?filter=&keyword=
func GetLicenseOverview(c *gin.Context) {
	all := buildLicenseOverview()

	filter := strings.ToUpper(strings.TrimSpace(c.Query("filter")))
	filter = strings.ReplaceAll(filter, "-", "_")
	filter = strings.ReplaceAll(filter, " ", "_")
	kw := strings.ToUpper(strings.TrimSpace(c.Query("keyword")))

	rows := make([]LicenseOverviewRow, 0, len(all))
	for _, r := range all {
		if !licenseRowMatchesFilter(r, filter) {
			continue
		}
		if kw != "" {
			hit := strings.Contains(strings.ToUpper(r.CurrentLicenseNo), kw) ||
				strings.Contains(strings.ToUpper(r.OriginalLicenseNo), kw) ||
				strings.Contains(strings.ToUpper(r.Country), kw) ||
				strings.Contains(strings.ToUpper(r.Model), kw)
			for _, p := range r.PreviousLicenseNos {
				if strings.Contains(strings.ToUpper(p), kw) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}
		rows = append(rows, r)
	}

	c.JSON(200, gin.H{
		"rows":    rows,
		"total":   len(rows),
		"summary": licenseOverviewSummary(all),
		"sources": licenseDataSourceCounts(),
	})
}

// LicenseDataSources = จำนวนแถวดิบของแต่ละไฟล์ที่ประกอบกันเป็นตารางภาพรวม
//
// ตาราง 1 แถวอาจเกิดจากหลายไฟล์พร้อมกัน เช่น ใบนำออกใบหนึ่งอยู่ทั้งในไฟล์ Export
// และในชีตต่ออายุ การล้างเฉพาะไฟล์เดียวจึงไม่ทำให้แถวนั้นหายไป
// หน้าเว็บต้องเห็นตัวเลขพวกนี้เพื่อบอกผู้ใช้ได้ว่าอะไรยังค้างอยู่
type LicenseDataSources struct {
	ImportItems    int64 `json:"importItems"`
	ExportItems    int64 `json:"exportItems"`
	RenewalHistory int64 `json:"renewalHistory"`
	RenewalLedger  int64 `json:"renewalLedger"`
}

func licenseDataSourceCounts() LicenseDataSources {
	var out LicenseDataSources
	config.DB.Model(&models.LicenseItem{}).Count(&out.ImportItems)
	// ใบนำออกไม่มีตารางของตัวเอง — นับจากเครื่องที่ระบุเลขใบอนุญาตนำออกไว้
	config.DB.Model(&models.LicenseItem{}).
		Where("export_license_no IS NOT NULL AND export_license_no <> ''").
		Count(&out.ExportItems)
	config.DB.Model(&models.LicenseRenewalHistory{}).Count(&out.RenewalHistory)
	config.DB.Model(&models.LicenseRenewal{}).Count(&out.RenewalLedger)
	return out
}

// GetLicenseDetail: GET /license-overview/detail?type=EXPORT&licenseNo=EXP-004
// คืนข้อมูลใบปัจจุบัน + ประวัติการต่ออายุทั้งโซ่
func GetLicenseDetail(c *gin.Context) {
	licenseType := models.NormalizeLicenseType(c.Query("type"))
	licenseNo := strings.TrimSpace(c.Query("licenseNo"))

	if licenseType == "" || licenseNo == "" {
		c.JSON(400, gin.H{"message": "กรุณาระบุ type (Import/Export) และ licenseNo"})
		return
	}

	history := loadRenewalHistoryWithLedger()
	idx := buildRenewalLinkIndex(history)
	chain := buildLicenseChainWith(idx, licenseType, licenseNo)

	// หาแถวสรุปของใบปัจจุบันจากหน้า Overview เพื่อให้สถานะตรงกันเป๊ะ
	var detail *LicenseOverviewRow
	for _, r := range buildLicenseOverview() {
		if r.LicenseType == licenseType && SameCode(r.CurrentLicenseNo, chain.CurrentNo) {
			row := r
			detail = &row
			break
		}
	}
	if detail == nil {
		now := time.Now()
		row := LicenseOverviewRow{
			LicenseType:        licenseType,
			LicenseTypeLabel:   models.LicenseTypeLabel(licenseType),
			CurrentLicenseNo:   chain.CurrentNo,
			OriginalLicenseNo:  chain.OriginalNo,
			RenewalCount:       chain.RenewalCount,
			LastRenewalDate:    chain.LastRenewalDate,
			ExpiryDate:         chain.CurrentExpire,
			PreviousLicenseNos: chain.PreviousNos,
		}
		row.Status, row.DaysLeft = licenseStatusOf(licenseType, row.ExpiryDate, false, now)
		fillExportLeadDate(&row, now)
		detail = &row
	}

	// บอกไปด้วยว่าตารางทะเบียนในระบบมีกี่แถว
	// ถ้าเป็น 0 แปลว่ายังไม่เคยอัปโหลดชีตต่ออายุเข้าระบบหลังอัปเดตเวอร์ชัน
	// ซึ่งเป็นสาเหตุที่ STOCK / คงเหลือ ขึ้นเป็นขีดทั้งตาราง
	var ledgerRows int64
	config.DB.Model(&models.LicenseRenewal{}).Count(&ledgerRows)

	c.JSON(200, gin.H{
		"detail":     detail,
		"chain":      chain,
		"steps":      licenseDetailSteps(licenseType, licenseNo, chain.Steps),
		"ledgerRows": ledgerRows,
	})
}

// LicenseUploadLogRow = ข้อมูลการอัปโหลดล่าสุดของไฟล์ 1 ประเภท
type LicenseUploadLogRow struct {
	Key        string     `json:"key"`
	Label      string     `json:"label"`
	FileName   string     `json:"fileName"`
	UploadedAt *time.Time `json:"uploadedAt"`
	UploadedBy string     `json:"uploadedBy"`
	Rows       int64      `json:"rows"`
	// Detail = บรรทัดสรุปจาก Audit Log เช่น "renewal.xlsx | total=50 new=48 ... error=2"
	Detail string `json:"detail"`
	// ตัวเลขที่แยกออกมาจาก Detail แล้ว เพื่อให้หน้าเว็บไม่ต้องมาแกะเอง
	Total   int `json:"total"`
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
	Errors  int `json:"errors"`
	// HasCounts = บรรทัดนี้มีตัวเลขให้แสดงไหม (log เก่าที่เก็บแค่ชื่อไฟล์จะเป็น false)
	HasCounts bool `json:"hasCounts"`
}

// applyUploadLogDetail: แกะชื่อไฟล์และตัวเลขออกจากบรรทัดสรุปใน Audit Log
// ถ้าเป็น log รูปแบบเก่า (ชื่อไฟล์ล้วน ๆ) จะได้แค่ชื่อไฟล์ ไม่มีตัวเลข
func applyUploadLogDetail(row *LicenseUploadLogRow, detail string) {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return
	}
	row.Detail = detail
	if row.FileName == "" {
		row.FileName = licenseUploadLogFileName(detail)
	}
	counts, ok := licenseUploadLogCounts(detail)
	if !ok {
		return
	}
	row.HasCounts = true
	row.Total = counts["total"]
	row.Added = counts["new"]
	row.Updated = counts["update"]
	row.Deleted = counts["delete"]
	row.Errors = counts["error"]
}

// LicenseUploadHistoryRow = 1 บรรทัดในตาราง Log การอัปโหลด
type LicenseUploadHistoryRow struct {
	ID         uint      `json:"id"`
	Source     string    `json:"source"`
	Label      string    `json:"label"`
	Action     string    `json:"action"`
	FileName   string    `json:"fileName"`
	UploadedBy string    `json:"uploadedBy"`
	UploadedAt time.Time `json:"uploadedAt"`
	Detail     string    `json:"detail"`

	Total     int  `json:"total"`
	Added     int  `json:"added"`
	Updated   int  `json:"updated"`
	Deleted   int  `json:"deleted"`
	Errors    int  `json:"errors"`
	HasCounts bool `json:"hasCounts"`
}

// licenseUploadSourceLabel: ชื่อไฟล์ประเภทนั้นที่แสดงให้ผู้ใช้เห็น
var licenseUploadSourceLabel = map[string]string{
	"IMPORT_LICENSE":          "Import License",
	"EXPORT_LICENSE":          "Export License",
	"LICENSE_RENEWAL":         "ทะเบียนใบอนุญาต (ต่ออายุ)",
	"LICENSE_RENEWAL_HISTORY": "Renewal (ต่ออายุ)",
}

// GetLicenseUploadLog: GET /license-overview/upload-log
// แสดง "ไฟล์ที่อัปโหลดล่าสุด + วันเวลา + ผู้อัปโหลด" ของทั้ง 3 ไฟล์
func GetLicenseUploadLog(c *gin.Context) {
	out := []LicenseUploadLogRow{}

	latestAudit := func(table string) models.AuditLog {
		var l models.AuditLog
		config.DB.Where("source_table = ?", table).
			Where("action IN ?", []string{"upload", "upload_excel"}).
			Order("action_datetime desc").First(&l)
		return l
	}

	// Import
	{
		var item models.LicenseItem
		config.DB.Order("upload_date desc, id desc").First(&item)
		var n int64
		config.DB.Model(&models.LicenseItem{}).Count(&n)
		l := latestAudit("IMPORT_LICENSE")
		row := LicenseUploadLogRow{
			Key: "import", Label: "Import License", FileName: item.FileName,
			Rows: n, UploadedBy: l.Name,
		}
		applyUploadLogDetail(&row, l.ResultStatus)
		if !item.UploadDate.IsZero() {
			t := item.UploadDate
			row.UploadedAt = &t
		}
		out = append(out, row)
	}


	// Renewal (ประวัติการต่ออายุ)
	{
		var item models.LicenseRenewalHistory
		config.DB.Order("uploaded_at desc, id desc").First(&item)
		var n int64
		config.DB.Model(&models.LicenseRenewalHistory{}).Count(&n)
		l := latestAudit("LICENSE_RENEWAL_HISTORY")
		row := LicenseUploadLogRow{
			Key: "renewal", Label: "Renewal (ต่ออายุ)", FileName: item.FileName,
			Rows: n, UploadedBy: item.UploadedBy,
		}
		applyUploadLogDetail(&row, l.ResultStatus)
		if !item.UploadedAt.IsZero() {
			t := item.UploadedAt
			row.UploadedAt = &t
		}
		if row.UploadedBy == "" {
			row.UploadedBy = l.Name
		}
		out = append(out, row)
	}

	// ประวัติการอัปโหลดย้อนหลัง (ใคร / เมื่อไร / ไฟล์อะไร / กี่รายการ / error กี่)
	var logs []models.AuditLog
	config.DB.Where("source_table IN ?", []string{
		"IMPORT_LICENSE", "EXPORT_LICENSE", "LICENSE_RENEWAL", "LICENSE_RENEWAL_HISTORY",
	}).Where("action IN ?", []string{"upload", "upload_excel"}).
		Order("action_datetime desc").Limit(200).Find(&logs)

	history := make([]LicenseUploadHistoryRow, 0, len(logs))
	for _, l := range logs {
		h := LicenseUploadHistoryRow{
			ID:         l.ID,
			Source:     l.SourceTable,
			Label:      licenseUploadSourceLabel[l.SourceTable],
			Action:     l.Action,
			UploadedBy: l.Name,
			UploadedAt: l.ActionDatetime,
			FileName:   licenseUploadLogFileName(l.ResultStatus),
			Detail:     l.ResultStatus,
		}
		if h.Label == "" {
			h.Label = l.SourceTable
		}
		if counts, ok := licenseUploadLogCounts(l.ResultStatus); ok {
			h.HasCounts = true
			h.Total = counts["total"]
			h.Added = counts["new"]
			h.Updated = counts["update"]
			h.Deleted = counts["delete"]
			h.Errors = counts["error"]
		}
		history = append(history, h)
	}

	c.JSON(200, gin.H{"latest": out, "history": history})
}
