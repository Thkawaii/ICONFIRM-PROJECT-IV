package controllers

import (
	"testing"
	"time"

	"iconfirm/models"
)

// ใบนำออกปิดเมื่อหมดอายุ ไม่ใช่เมื่อ REMAIN ลงถึง 0
// ของที่ขายไม่ออกค้างอยู่บนใบได้ แล้วกลายเป็นรายการคงค้างตอนใบหมดอายุ
func TestLedgerChainCompleted(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	day := func(d int) *time.Time {
		t := now.AddDate(0, 0, d)
		return &t
	}
	row := func(expire *time.Time, remain int) models.LicenseRenewal {
		return models.LicenseRenewal{ExpireDate: expire, Remain: remain, HasRemain: true}
	}
	withSteps := func(r models.LicenseRenewal, email, payment, received *time.Time) models.LicenseRenewal {
		r.EmailDate, r.PaymentDate, r.ReceivedDate = email, payment, received
		return r
	}

	cases := []struct {
		name string
		last models.LicenseRenewal
		want bool
	}{
		// หมดอายุแล้ว = ปิด ไม่ว่าจะเหลือของค้างอยู่หรือไม่
		{"หมดอายุแล้ว REMAIN 0", row(day(-1), 0), true},
		{"หมดอายุแล้ว ยังมีของค้าง", row(day(-30), 2), true},

		// ยังไม่หมดอายุ = ยังไม่ปิด แม้ REMAIN จะเป็น 0 แล้ว
		// เพราะผู้ใช้ยังเพิ่มรายการบนใบนั้นได้อีก
		{"ยังไม่หมดอายุ REMAIN 0", row(day(7), 0), false},
		{"ยังไม่หมดอายุ ยังมีของเหลือ", row(day(7), 2), false},
		{"หมดอายุวันนี้", row(day(0), 0), false},

		// ไม่มีวันหมดอายุในไฟล์ = ไม่รู้ ไม่เดาว่าปิด
		{"ไม่มีวันหมดอายุ", row(nil, 0), false},

		// ยื่นต่ออายุค้างอยู่ = งานยังเดิน แม้ใบจะเลยวันหมดอายุไปแล้ว
		{"ส่งเมลแล้ว รอจ่ายเงิน", withSteps(row(day(-5), 0), day(-10), nil, nil), false},
		{"จ่ายเงินแล้ว รอเอกสาร", withSteps(row(day(-5), 0), day(-10), day(-8), nil), false},
		// ได้เอกสารแล้ว = รอบนั้นจบ กลับไปตัดสินด้วยวันหมดอายุตามปกติ
		{"ได้เอกสารแล้ว และหมดอายุแล้ว", withSteps(row(day(-5), 0), day(-10), day(-8), day(-6)), true},

		// คำว่า Completed ในคอลัมน์ NO. ผู้ใช้สั่งเอง ถือว่าปิดเสมอ
		{"ชื่อกลุ่มเขียน Completed", models.LicenseRenewal{GroupNo: "Completed 01"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ledgerChainCompleted(tc.last, now); got != tc.want {
				t.Fatalf("ledgerChainCompleted = %v, want %v", got, tc.want)
			}
		})
	}
}

// ใบนำเข้าใบเดียวแตกเป็นใบนำออกหลายใบในวันเดียวกัน = แบ่งโควต้าตามประเทศ
// ไม่ใช่การต่ออายุ จึงต้องถูกยุบเป็นขั้นเดียวกัน
func TestLedgerStepsSplitsByIssueDate(t *testing.T) {
	day := func(d int) *time.Time {
		t := time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	row := func(country, exportNo string, issue *time.Time) models.LicenseRenewal {
		return models.LicenseRenewal{Country: country, ExportLicenseNo: exportNo, IssueDate: issue}
	}

	// E05036903112 — ออกวันเดียวกันสองใบ แบ่ง INDONESIA 50 / MALAYSIA 10
	split := []models.LicenseRenewal{
		row("INDONESIA", "E05046900717", day(9)),
		row("MALAYSIA", "E05046900718", day(9)),
	}
	steps := ledgerSteps(split)
	if len(steps) != 1 {
		t.Fatalf("ได้ %d ขั้น ต้องการ 1 — ใบที่ออกวันเดียวกันต้องอยู่ขั้นเดียวกัน", len(steps))
	}
	if len(steps[0]) != 2 {
		t.Fatalf("ขั้นแรกมี %d สาย ต้องการ 2", len(steps[0]))
	}

	// E05036901603 — ใบรวมสองรอบ แล้วค่อยแตกตามประเทศในรอบที่สาม
	mixed := []models.LicenseRenewal{
		row("INDONESIA , MALAYSIA", "050169002496", day(1)),
		row("INDONESIA , MALAYSIA", "050169002855", day(5)),
		row("INDONESIA", "E05046900299", day(9)),
		row("MALAYSIA", "E05046900301", day(9)),
	}
	steps = ledgerSteps(mixed)
	if len(steps) != 3 {
		t.Fatalf("ได้ %d ขั้น ต้องการ 3 (ต่ออายุ 2 ครั้ง)", len(steps))
	}

	// ขั้นก่อนหน้ามีใบเดียว → ทั้งสองสายต่อยอดมาจากใบนั้น ไม่ใช่จากกันเอง
	for i, cur := range steps[2] {
		prev := ledgerStepMatch(steps[1], cur, i)
		if prev.ExportLicenseNo != "050169002855" {
			t.Fatalf("สาย %s ชี้ไปที่ %s ต้องเป็น 050169002855", cur.Country, prev.ExportLicenseNo)
		}
	}

	// ขั้นก่อนหน้ามีหลายสาย → จับคู่ด้วยประเทศ
	prevStep := []models.LicenseRenewal{
		row("INDONESIA", "E05046900717", day(9)),
		row("MALAYSIA", "E05046900718", day(9)),
	}
	got := ledgerStepMatch(prevStep, row("MALAYSIA", "E05046900900", day(20)), 0)
	if got.ExportLicenseNo != "E05046900718" {
		t.Fatalf("จับคู่ได้ %s ต้องเป็นใบของ MALAYSIA", got.ExportLicenseNo)
	}
}

// ไฟล์จริงเรียงเป็นบล็อกตามประเทศ แต่ละประเทศเดินต่ออายุคนละวัน
// กติกา "แถวติดกัน + วันที่ตรงกัน" ของ ledgerSteps จึงแยกสายให้ไม่ได้
// (กลุ่ม 109 ใบนำเข้า E05036902379 แถว 548–553 ในไฟล์)
func TestLedgerBranchIndexesSplitsByCountry(t *testing.T) {
	day := func(m, d int) *time.Time {
		t := time.Date(2026, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	row := func(country, no string, issue *time.Time) models.LicenseRenewal {
		return models.LicenseRenewal{Country: country, ExportLicenseNo: no, IssueDate: issue}
	}

	chain := []models.LicenseRenewal{
		row("INDONESIA", "E05046900435", day(8, 5)),
		row("INDONESIA", "E05046900456", day(8, 11)),
		row("MALAYSIA", "E05046900432", day(8, 5)),
		row("MALAYSIA", "E05046900457", day(8, 11)),
		row("MALAYSIA", "E05046900619", day(8, 25)),
		row("MALAYSIA ", "E05046900754", day(9, 10)), // เว้นวรรคท้ายแบบในไฟล์จริง
	}

	branches := ledgerBranchIndexes(chain)
	if len(branches) != 2 {
		t.Fatalf("ได้ %d สาย ต้องการ 2 (INDONESIA / MALAYSIA)", len(branches))
	}

	want := map[string]struct {
		steps int
		last  string
	}{
		"INDONESIA": {2, "E05046900456"},
		"MALAYSIA":  {4, "E05046900754"},
	}
	seen := map[string]bool{}
	for _, bidx := range branches {
		branch := ledgerPick(chain, bidx)
		country := ledgerCountryNames(branch[0].Country)[0]
		w, ok := want[country]
		if !ok {
			t.Fatalf("สายที่ไม่รู้จัก %q", country)
		}
		seen[country] = true

		steps := ledgerSteps(branch)
		if len(steps) != w.steps {
			t.Fatalf("สาย %s ได้ %d ขั้น ต้องการ %d", country, len(steps), w.steps)
		}
		last := steps[len(steps)-1]
		if got := last[len(last)-1].ExportLicenseNo; got != w.last {
			t.Fatalf("สาย %s ใบสุดท้าย %s ต้องเป็น %s", country, got, w.last)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("ได้สาย %v ต้องครบทั้ง INDONESIA และ MALAYSIA", seen)
	}
}

// ช่วงที่ยังไม่แตกสาย ("INDONESIA , MALAYSIA") เป็นต้นทางร่วมของทุกสาย
// ไม่ใช่สายของตัวเอง — กลุ่ม 95 ใบนำเข้า E05036900659
func TestLedgerBranchIndexesSharedPrefix(t *testing.T) {
	row := func(country, no string) models.LicenseRenewal {
		return models.LicenseRenewal{Country: country, ExportLicenseNo: no}
	}
	chain := []models.LicenseRenewal{
		row("INDONESIA , MALAYSIA", "050169002513"),
		row("INDONESIA , MALAYSIA", "050169002847"),
		row("INDONESIA", "E05046900240"),
		row("MALAYSIA", "E05046900241"),
		row("MALAYSIA", "E05046900449"),
	}
	branches := ledgerBranchIndexes(chain)
	if len(branches) != 2 {
		t.Fatalf("ได้ %d สาย ต้องการ 2", len(branches))
	}
	for _, bidx := range branches {
		branch := ledgerPick(chain, bidx)
		if len(branch) < 3 {
			t.Fatalf("สายมี %d แถว ต้องมีแถวร่วม 2 แถวนำหน้าด้วย", len(branch))
		}
		if branch[0].ExportLicenseNo != "050169002513" || branch[1].ExportLicenseNo != "050169002847" {
			t.Fatalf("สายไม่ได้เริ่มด้วยแถวร่วมตามลำดับเดิมในไฟล์")
		}
	}
}

// โซ่ที่ทุกแถวเป็นชุดประเทศเดียวกัน ต้องได้สายเดียว = พฤติกรรมเดิมไม่เปลี่ยน
func TestLedgerBranchIndexesSingleCountryUnchanged(t *testing.T) {
	row := func(no string) models.LicenseRenewal {
		return models.LicenseRenewal{Country: "INDONESIA , MALAYSIA", ExportLicenseNo: no}
	}
	chain := []models.LicenseRenewal{row("A1"), row("A2"), row("A3")}
	got := ledgerBranchIndexes(chain)
	if len(got) != 1 || len(got[0]) != 3 {
		t.Fatalf("โซ่ประเทศเดียวต้องได้ 1 สาย ครบ 3 แถว")
	}
}

// ใบของคนละประเทศไม่ใช่ใบเก่า/ใบใหม่ของกันและกัน
// ประวัติต้องไม่มีคู่ E05046900456 → E05046900432
func TestRenewalHistoryFromLedgerDoesNotLinkAcrossCountries(t *testing.T) {
	day := func(m, d int) *time.Time {
		t := time.Date(2026, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	row := func(country, no string, issue *time.Time) models.LicenseRenewal {
		return models.LicenseRenewal{
			GroupNo:         "109",
			ImportLicenseNo: "E05036902379",
			Country:         country,
			ExportLicenseNo: no,
			IssueDate:       issue,
		}
	}
	rows := []models.LicenseRenewal{
		row("INDONESIA", "E05046900435", day(8, 5)),
		row("INDONESIA", "E05046900456", day(8, 11)),
		row("MALAYSIA", "E05046900432", day(8, 5)),
		row("MALAYSIA", "E05046900457", day(8, 11)),
		row("MALAYSIA", "E05046900619", day(8, 25)),
		row("MALAYSIA", "E05046900754", day(9, 10)),
	}

	pairs := map[string]bool{}
	for _, d := range readRenewalHistoryFromLedger(rows) {
		if d.LicenseType != models.LicenseTypeExport {
			continue
		}
		pairs[d.OldLicenseNo+"→"+d.NewLicenseNo] = true
	}

	for _, want := range []string{
		"E05046900435→E05046900456",
		"E05046900432→E05046900457",
		"E05046900457→E05046900619",
		"E05046900619→E05046900754",
	} {
		if !pairs[want] {
			t.Fatalf("ไม่พบคู่ %s — ได้ %v", want, pairs)
		}
	}
	if pairs["E05046900456→E05046900432"] {
		t.Fatalf("เชื่อมข้ามประเทศ INDONESIA → MALAYSIA")
	}
	if len(pairs) != 4 {
		t.Fatalf("ได้ %d คู่ ต้องการ 4 — %v", len(pairs), pairs)
	}
}

// แถวร่วมอยู่ในทุกสาย จึงถูกไล่ซ้ำ คู่เดียวกันต้องบันทึกครั้งเดียว
func TestRenewalHistoryFromLedgerDedupesSharedPrefix(t *testing.T) {
	day := func(d int) *time.Time {
		t := time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	row := func(country, no string, issue *time.Time) models.LicenseRenewal {
		return models.LicenseRenewal{
			GroupNo:         "95",
			ImportLicenseNo: "E05036900659",
			Country:         country,
			ExportLicenseNo: no,
			IssueDate:       issue,
		}
	}
	rows := []models.LicenseRenewal{
		row("INDONESIA , MALAYSIA", "050169002513", day(1)),
		row("INDONESIA , MALAYSIA", "050169002847", day(5)),
		row("INDONESIA", "E05046900240", day(15)),
		row("MALAYSIA", "E05046900241", day(15)),
	}

	count := 0
	for _, d := range readRenewalHistoryFromLedger(rows) {
		if d.LicenseType == models.LicenseTypeExport &&
			d.OldLicenseNo == "050169002513" && d.NewLicenseNo == "050169002847" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("คู่แถวร่วมถูกบันทึก %d ครั้ง ต้องการ 1", count)
	}
}

// ใบที่ออกวันเดียวกัน "ประเทศเดียวกัน" คือคนละรอบจริง ๆ ไม่ใช่การแบ่งโควต้า
//
// เคสจากไฟล์จริง กลุ่ม Completed 19 / Completed 11 / 80 — ใบคู่นี้ออกวันเดียวกัน
// ถ้ายุบเป็นขั้นเดียวกัน ใบที่สองจะไม่ได้ต่อเป็นใบเดิมของใคร แล้วหลุดไปค้าง
// เป็นแถว "หมดอายุไปแล้ว 735 วัน" ในหน้า Overview ตลอดไป
func TestLedgerStepsSameDaySameCountryIsSequential(t *testing.T) {
	day := func(m, d int) *time.Time {
		t := time.Date(2024, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	row := func(no string, issue *time.Time) models.LicenseRenewal {
		return models.LicenseRenewal{
			GroupNo:         "Completed 19",
			ImportLicenseNo: "050167001815",
			Country:         "INDONESIA , MALAYSIA",
			ExportLicenseNo: no,
			IssueDate:       issue,
		}
	}
	chain := []models.LicenseRenewal{
		row("050167004685", day(8, 1)),
		row("050167005216", day(8, 29)),
		row("050167005273", day(8, 29)), // วันเดียวกัน ประเทศเดียวกัน = รอบถัดไป
		row("050167006061", day(9, 30)),
	}

	steps := ledgerSteps(chain)
	if len(steps) != 4 {
		t.Fatalf("ได้ %d ขั้น ต้องการ 4 — ใบประเทศเดียวกันต้องไม่ถูกยุบรวมกัน", len(steps))
	}

	// ทุกใบต้องอยู่ในโซ่ ไม่มีใบไหนหลุดออกไปลอย ๆ
	oldNo := map[string]bool{}
	newNo := map[string]bool{}
	for _, d := range readRenewalHistoryFromLedger(chain) {
		if d.LicenseType != models.LicenseTypeExport {
			continue
		}
		oldNo[d.OldLicenseNo] = true
		newNo[d.NewLicenseNo] = true
	}
	if !oldNo["050167005273"] {
		t.Fatalf("050167005273 ไม่ได้เป็นใบเดิมของใครเลย จะหลุดไปค้างเป็นแถวหมดอายุ — ได้ %v", oldNo)
	}
	for _, no := range []string{"050167005216", "050167005273", "050167006061"} {
		if !newNo[no] {
			t.Fatalf("%s ไม่ได้ถูกต่อมาจากใบก่อนหน้า — ได้ %v", no, newNo)
		}
	}
}

// ใบนำเข้าที่แบ่งโควต้าหลายประเทศต้องเก็บประเทศไว้ครบทุกประเทศ
// ไม่งั้นตัวกรองประเทศจะหาใบนั้นเจอแค่ประเทศเดียว
func TestBaseLicenseAddCountryUnion(t *testing.T) {
	b := &baseLicense{}
	b.addCountry("INDONESIA")
	if b.Country != "INDONESIA" {
		t.Fatalf("ได้ %q", b.Country)
	}
	b.addCountry("MALAYSIA ") // เว้นวรรคท้ายแบบในไฟล์จริง
	if b.Country != "INDONESIA, MALAYSIA" {
		t.Fatalf("ได้ %q ต้องการ INDONESIA, MALAYSIA", b.Country)
	}
	// แถวที่เขียนรวมสองประเทศ และแถวซ้ำ ต้องไม่ทำให้ค่าเพี้ยน
	b.addCountry("INDONESIA , MALAYSIA")
	if b.Country != "INDONESIA, MALAYSIA" {
		t.Fatalf("ได้ %q หลังเติมแถวที่ซ้ำ", b.Country)
	}
	// ช่องว่างไม่ต้องทำอะไร
	b.addCountry("   ")
	if b.Country != "INDONESIA, MALAYSIA" {
		t.Fatalf("ได้ %q หลังเติมช่องว่าง", b.Country)
	}

	// ใบประเทศเดียวต้องได้ค่าเดิม ไม่มีลูกน้ำงอกมา
	single := &baseLicense{}
	single.addCountry("Indonesia")
	if single.Country != "INDONESIA" {
		t.Fatalf("ใบประเทศเดียวได้ %q", single.Country)
	}
}

// ใบนำเข้าที่แบ่งโควต้าหลายประเทศ ประวัติต้องมีแถวของทุกประเทศ
// เคสจริง กลุ่ม 117 ใบนำเข้า E05036903112 แบ่ง INDONESIA 50 / MALAYSIA 10
func TestExpandStepsByCountrySplitsOriginalRow(t *testing.T) {
	ledger := []models.LicenseRenewal{
		{GroupNo: "117", ImportLicenseNo: "E05036903112", Country: "INDONESIA",
			ExportLicenseNo: "E05046900717", Total: 60, Stock: 50, Remain: 50, SortOrder: 1},
		{GroupNo: "117", ImportLicenseNo: "E05036903112", Country: "MALAYSIA",
			ExportLicenseNo: "E05046900718", Total: 60, Stock: 10, Remain: 10, SortOrder: 2},
	}
	steps := []LicenseChainStep{{Round: 0, OldLicenseNo: "E05036903112"}}

	out, orderOf, groups := expandStepsByCountry(models.LicenseTypeImport, steps, ledger)
	if len(out) != 2 {
		t.Fatalf("ได้ %d แถว ต้องการ 2 (ประเทศละแถว)", len(out))
	}
	if len(orderOf) != len(out) {
		t.Fatalf("orderOf ยาว %d ไม่เท่ากับ steps %d", len(orderOf), len(out))
	}
	if len(groups) != 1 {
		t.Fatalf("โซ่ที่เกี่ยวข้อง %d ชุด ต้องการ 1", len(groups))
	}

	want := map[string][2]int{"INDONESIA": {50, 50}, "MALAYSIA": {10, 10}}
	for _, s := range out {
		w, ok := want[s.Country]
		if !ok {
			t.Fatalf("ประเทศที่ไม่คาดคิด %q", s.Country)
		}
		if s.Round != 0 {
			t.Fatalf("สาย %s ต้องยังเป็นแถวต้นฉบับ (Round 0) ได้ %d", s.Country, s.Round)
		}
		if !s.HasLedger || s.Stock != w[0] || s.Remain != w[1] {
			t.Fatalf("สาย %s ได้ STOCK %d REMAIN %d ต้องการ %d / %d",
				s.Country, s.Stock, s.Remain, w[0], w[1])
		}
		if s.Quota != 60 {
			t.Fatalf("สาย %s โควต้า %d ต้องเป็น 60 ของใบนำเข้า", s.Country, s.Quota)
		}
		delete(want, s.Country)
	}
	if len(want) != 0 {
		t.Fatalf("ประเทศที่หายไป %v", want)
	}
}

// ใบที่ไม่ได้แบ่งประเทศต้องได้แถวเดียวเหมือนเดิม และใช้แถวหลังสุดของเลขใบนั้น
func TestExpandStepsByCountrySingleCountryUnchanged(t *testing.T) {
	ledger := []models.LicenseRenewal{
		{GroupNo: "116", ImportLicenseNo: "E05036903281", Country: "INDONESIA",
			ExportLicenseNo: "E05046900716", Total: 75, Stock: 75, Remain: 75, SortOrder: 1},
	}
	steps := []LicenseChainStep{{Round: 0, OldLicenseNo: "E05036903281"}}

	out, _, _ := expandStepsByCountry(models.LicenseTypeImport, steps, ledger)
	if len(out) != 1 {
		t.Fatalf("ได้ %d แถว ต้องการ 1", len(out))
	}
	if out[0].Country != "INDONESIA" || out[0].Remain != 75 {
		t.Fatalf("ได้ %+v", out[0])
	}
}

// ขั้นที่หาแถวในทะเบียนไม่เจอ ต้องยังอยู่ในประวัติ ไม่ใช่หายไปเฉย ๆ
func TestExpandStepsByCountryKeepsUnmatchedStep(t *testing.T) {
	ledger := []models.LicenseRenewal{
		{ImportLicenseNo: "E05036903281", Country: "INDONESIA", ExportLicenseNo: "X1", SortOrder: 1},
	}
	steps := []LicenseChainStep{{Round: 0, OldLicenseNo: "ไม่มีในทะเบียน"}}

	out, orderOf, _ := expandStepsByCountry(models.LicenseTypeImport, steps, ledger)
	if len(out) != 1 || out[0].HasLedger {
		t.Fatalf("ได้ %+v", out)
	}
	if len(orderOf) != 1 || orderOf[0] != 0 {
		t.Fatalf("orderOf = %v", orderOf)
	}
}

// ใบนำเข้าไม่เคยเปลี่ยนเลขใบ ประวัติของมันคือลำดับใบนำออกใต้โควต้าเดิม
// เคสจริง Completed 06 ใบนำเข้า 050167001925 ออกใบนำออกมา 6 ใบ = ต่ออายุ 5 ครั้ง
func TestLedgerStepsForImportListsEveryExportLicense(t *testing.T) {
	day := func(m, d int) *time.Time {
		t := time.Date(2024, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	row := func(order int64, no string, issue *time.Time, remain int) models.LicenseRenewal {
		return models.LicenseRenewal{
			GroupNo: "Completed 06", ImportLicenseNo: "050167001925",
			Country: "INDONESIA , MALAYSIA", ExportLicenseNo: no,
			Total: 220, Stock: 220, Remain: remain,
			IssueDate: issue, SortOrder: order,
		}
	}
	ledger := []models.LicenseRenewal{
		row(1, "050167001924", day(4, 3), 124),
		row(2, "050167003380", day(6, 10), 48),
		row(3, "050167003725", day(6, 25), 11),
		row(4, "050167004686", day(8, 1), 1),
		row(5, "050167005217", day(8, 29), 0),
		row(6, "050167005274", day(8, 29), 0),
		// ใบของใบนำเข้าใบอื่น ต้องไม่ปน
		{ImportLicenseNo: "050167009999", ExportLicenseNo: "050167009998", SortOrder: 7},
	}

	steps, orderOf, groups := ledgerStepsForImport("050167001925", ledger)
	if len(steps) != 6 {
		t.Fatalf("ได้ %d แถว ต้องการ 6 (ใบแรก + ต่ออายุ 5 ครั้ง)", len(steps))
	}
	if len(orderOf) != len(steps) {
		t.Fatalf("orderOf ยาว %d ไม่เท่ากับ steps %d", len(orderOf), len(steps))
	}
	if len(groups) != 1 {
		t.Fatalf("โซ่ที่เกี่ยวข้อง %d ชุด ต้องการ 1", len(groups))
	}

	if steps[0].Round != 0 || steps[0].OldLicenseNo != "050167001924" || steps[0].NewLicenseNo != "" {
		t.Fatalf("แถวแรกต้องเป็นใบแรก ไม่มีเลขใบใหม่ — ได้ %+v", steps[0])
	}
	want := [][3]string{
		{"1", "050167001924", "050167003380"},
		{"2", "050167003380", "050167003725"},
		{"3", "050167003725", "050167004686"},
		{"4", "050167004686", "050167005217"},
		{"5", "050167005217", "050167005274"},
	}
	for i, w := range want {
		s := steps[i+1]
		if itoa(s.Round) != w[0] || s.OldLicenseNo != w[1] || s.NewLicenseNo != w[2] {
			t.Fatalf("ครั้งที่ %s ได้ %d: %s -> %s", w[0], s.Round, s.OldLicenseNo, s.NewLicenseNo)
		}
		if !s.HasLedger || s.Quota != 220 {
			t.Fatalf("ครั้งที่ %s ไม่ได้ตัวเลขจากทะเบียน — %+v", w[0], s)
		}
	}
	if steps[5].Remain != 0 {
		t.Fatalf("ใบล่าสุดคงเหลือ %d ต้องเป็น 0", steps[5].Remain)
	}
}

// ช่วงที่ยังไม่แตกสายประเทศอยู่ในทุกสาย ต้องไม่ถูกนับซ้ำในประวัติ
func TestLedgerStepsForImportDedupesSharedPrefix(t *testing.T) {
	day := func(d int) *time.Time {
		t := time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	row := func(order int64, country, no string, issue *time.Time) models.LicenseRenewal {
		return models.LicenseRenewal{
			GroupNo: "95", ImportLicenseNo: "E05036900659", Country: country,
			ExportLicenseNo: no, IssueDate: issue, SortOrder: order,
		}
	}
	ledger := []models.LicenseRenewal{
		row(1, "INDONESIA , MALAYSIA", "050169002513", day(1)),
		row(2, "INDONESIA , MALAYSIA", "050169002847", day(5)),
		row(3, "INDONESIA", "E05046900240", day(15)),
		row(4, "MALAYSIA", "E05046900241", day(16)),
		row(5, "MALAYSIA", "E05046900449", day(20)),
	}

	steps, orderOf, _ := ledgerStepsForImport("E05036900659", ledger)
	if len(steps) != 5 {
		t.Fatalf("ได้ %d แถว ต้องการ 5 (เท่าจำนวนใบนำออกจริง ไม่นับซ้ำ)", len(steps))
	}
	// เรียงตามลำดับเดิมในไฟล์ เพื่อให้อ่านคู่กับ Excel ได้
	for i := 1; i < len(orderOf); i++ {
		if orderOf[i] < orderOf[i-1] {
			t.Fatalf("ลำดับไม่ตรงกับไฟล์: %v", orderOf)
		}
	}
	seen := map[string]bool{}
	for _, s := range steps {
		no := s.NewLicenseNo
		if no == "" {
			no = s.OldLicenseNo
		}
		if seen[no] {
			t.Fatalf("ใบ %s ถูกบันทึกซ้ำ", no)
		}
		seen[no] = true
	}
}

// ตัวเลขบนการ์ดต้องตรงกับแถวสุดท้ายของตารางประวัติ
//
// ใบที่แบ่งโควต้าหลายประเทศ "จำนวนใบนำออกทั้งหมด - 1" ใช้ไม่ได้
// เคสจริง E05036900826 ออกใบนำออก 8 ใบ แต่สายยาวสุดต่อแค่ 6 ครั้ง
// เพราะใบของ INDONESIA ไม่ได้ต่อจากสาย MALAYSIA
func TestLongestBranchMatchesHistoryLastRound(t *testing.T) {
	day := func(m, d int) *time.Time {
		t := time.Date(2026, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	row := func(order int64, country, no string, issue *time.Time, remain int) models.LicenseRenewal {
		return models.LicenseRenewal{
			GroupNo: "102", ImportLicenseNo: "E05036900826", Country: country,
			ExportLicenseNo: no, Total: 45, Stock: 45, Remain: remain,
			IssueDate: issue, SortOrder: order,
		}
	}
	both := "INDONESIA , MALAYSIA"
	ledger := []models.LicenseRenewal{
		row(1, both, "050169001952", day(4, 3), 45),
		row(2, both, "050169002644", day(6, 2), 37),
		row(3, both, "050169002851", day(6, 25), 7),
		row(4, "INDONESIA", "E05046900247", day(7, 15), 0),
		row(5, "MALAYSIA", "E05046900248", day(7, 15), 7),
		row(6, "MALAYSIA", "E05046900452", day(8, 11), 2),
		row(7, "MALAYSIA", "E05046900615", day(8, 25), 1),
		row(8, "MALAYSIA", "E05046900753", day(9, 10), 1),
	}

	// ตาราง
	steps, _, _ := ledgerStepsForImport("E05036900826", ledger)
	if len(steps) != 8 {
		t.Fatalf("ตารางได้ %d แถว ต้องการ 8 (เท่าจำนวนใบนำออก)", len(steps))
	}
	maxRound := 0
	for _, s := range steps {
		if s.Round > maxRound {
			maxRound = s.Round
		}
	}

	// การ์ด: สายที่ยาวที่สุด
	longest, branches := 0, 0
	perCountry := map[string]int{}
	for _, bidx := range ledgerBranchIndexes(ledger) {
		branches++
		branch := ledgerPick(ledger, bidx)
		n := len(ledgerSteps(branch)) - 1
		perCountry[ledgerCountryKey(branch[len(branch)-1].Country)] = n
		if n > longest {
			longest = n
		}
	}
	if branches != 2 {
		t.Fatalf("ได้ %d สาย ต้องการ 2", branches)
	}
	if perCountry["INDONESIA"] != 3 || perCountry["MALAYSIA"] != 6 {
		t.Fatalf("ต่ออายุรายประเทศ = %v ต้องการ INDONESIA 3 / MALAYSIA 6", perCountry)
	}
	if longest != maxRound {
		t.Fatalf("การ์ดบอก %d ครั้ง แต่ตารางจบที่ครั้งที่ %d", longest, maxRound)
	}
	if longest != 6 {
		t.Fatalf("สายยาวสุดต่อ %d ครั้ง ต้องการ 6", longest)
	}

	// จำนวนใบนำออกทั้งหมด - 1 = 7 ซึ่งเป็นตัวเลขเดิมที่ไม่ตรงกับตาราง
	if len(ledger)-1 == longest {
		t.Fatalf("เคสนี้ต้องเป็นเคสที่สองวิธีให้ผลต่างกัน ไม่งั้นเทสต์ไม่ได้ตรวจอะไร")
	}
}
