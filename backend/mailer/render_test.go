package mailer

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

var testLoc = time.FixedZone("ICT", 7*60*60)

func day(base time.Time, n int) *time.Time {
	d := base.AddDate(0, 0, n)
	return &d
}

func sampleReport() WeeklyReport {
	now := time.Date(2026, 9, 7, 8, 30, 0, 0, testLoc)
	start, end := WeekBounds(now)
	year, week := now.ISOWeek()

	return WeeklyReport{
		GeneratedAt:      now,
		WeekKey:          ISOWeekKey(now),
		WeekNo:           week,
		WeekYear:         year,
		PeriodStart:      start,
		PeriodEnd:        end,
		ImportWithinDays: 30,
		ExportWithinDays: 7,
		LeadDays:         15,
		LeadWarnDays:     7,
		ImportTracked:    12,
		ExportTracked:    9,
		MaxRows:          25,
		Recipients:       []string{"theeparat.metheepooriwat@kobelco.com"},
		Greeting:         "คุณธีปรัชญ์ เมธีภูริวัจน์",
		Org:              "Kobelco Construction Machinery Southeast Asia Co., Ltd.",
		Dept:             "แผนกโลจิสติกส์",
		AppURL:           "http://iconfirm.local",
		Import: []ImportRow{
			{
				LicenseNo: "IL-2026-0148", InvoiceNo: "KCMT-25-0912", Model: "SK75-8",
				Machines: 6, Confirmed: 6,
				IssueDate: day(now, -190), ExpiryDate: day(now, -9),
				DaysLeft: -9, Status: StatusExpired,
			},
			{
				LicenseNo: "IL-2026-0163", InvoiceNo: "KCMT-26-0021", Model: "SK200-10",
				Machines: 12, Confirmed: 9,
				IssueDate: day(now, -176), ExpiryDate: day(now, 4),
				DaysLeft: 4, Status: StatusExpiring,
			},
		},
		Export: []ExportRow{
			{
				ExportLicenseNo: "EX-2026-0091", Machines: 5,
				IssueDate: day(now, -38), ExpiryDate: day(now, -8), DaysLeft: -8,
				Status:   StatusExpired,
				LeadDate: day(now, -23), LeadDaysLeft: -23, LeadStatus: LeadOverdue,
			},
			{
				ExportLicenseNo: "EX-2026-0133", Machines: 7,
				ImportLicenseNo: "IL-2026-0163", ImportExpiryDate: day(now, 4),
				IssueDate: day(now, -25), ExpiryDate: day(now, 5), DaysLeft: 5,
				Status:   StatusExpiring,
				LeadDate: day(now, -10), LeadDaysLeft: -10, LeadStatus: LeadOverdue,
			},
		},
		ImportCounts: Counts{Expired: 1, Expiring: 1},
		ExportCounts: Counts{Expired: 1, Expiring: 1, LeadOverdue: 2},
	}
}

func TestWeekBoundsStartsOnMonday(t *testing.T) {
	now := time.Date(2026, 9, 9, 15, 0, 0, 0, testLoc)
	start, end := WeekBounds(now)

	if start.Weekday() != time.Monday {
		t.Fatalf("สัปดาห์ต้องเริ่มวันจันทร์ ได้ %v", start.Weekday())
	}
	if start.Day() != 7 || end.Day() != 13 {
		t.Fatalf("ช่วงสัปดาห์ผิด: %v – %v", start, end)
	}
}

func TestISOWeekKeyFormat(t *testing.T) {
	if got := ISOWeekKey(time.Date(2026, 1, 5, 0, 0, 0, 0, testLoc)); got != "2026-01-05" {
		t.Fatalf("คีย์สัปดาห์ผิด: %s", got)
	}

	monday := ISOWeekKey(time.Date(2026, 9, 7, 9, 0, 0, 0, testLoc))
	for _, day := range []int{8, 9, 10, 11, 12, 13} {
		if got := ISOWeekKey(time.Date(2026, 9, day, 23, 30, 0, 0, testLoc)); got != monday {
			t.Fatalf("วันที่ %d ก.ย. ควรได้คีย์ %s แต่ได้ %s", day, monday, got)
		}
	}

	if got := ISOWeekKey(time.Date(2026, 9, 14, 0, 0, 0, 0, testLoc)); got == monday {
		t.Fatal("สัปดาห์ถัดไปต้องได้คีย์คนละตัว")
	}
}

func TestNextRunAfterSkipsToNextWeekWhenPassed(t *testing.T) {
	cfg := WeeklyConfig{Weekday: time.Monday, Hour: 8, Minute: 30, Location: testLoc}

	now := time.Date(2026, 9, 7, 9, 0, 0, 0, testLoc)
	next := cfg.NextRunAfter(now)

	if next.Weekday() != time.Monday {
		t.Fatalf("รอบถัดไปต้องเป็นวันจันทร์ ได้ %v", next.Weekday())
	}
	if next.Day() != 14 || next.Hour() != 8 || next.Minute() != 30 {
		t.Fatalf("รอบถัดไปผิด: %v", next)
	}
}

func TestCountsAndSubject(t *testing.T) {
	r := sampleReport()

	if got := r.TotalActions(); got != 6 {
		t.Fatalf("จำนวนรายการต้องดำเนินการผิด: %d", got)
	}
	if got := r.Urgent(); got != 4 {
		t.Fatalf("จำนวนรายการเร่งด่วนผิด: %d", got)
	}

	subject := r.Subject()
	if !strings.Contains(subject, "เร่งด่วน 4 รายการ") {
		t.Fatalf("หัวข้ออีเมลไม่บอกจำนวนเร่งด่วน: %s", subject)
	}
	if !strings.Contains(subject, "ต้องดำเนินการ 6 รายการ") {
		t.Fatalf("หัวข้ออีเมลไม่บอกจำนวนรวม: %s", subject)
	}
	if strings.HasPrefix(subject, "[") {
		t.Fatalf("หัวข้ออีเมลควรขึ้นต้นด้วยข้อความปกติ: %s", subject)
	}
}

func TestSubjectWhenNothingToDo(t *testing.T) {
	r := sampleReport()
	r.Import = nil
	r.Export = nil
	r.ImportCounts = Counts{}
	r.ExportCounts = Counts{}

	if !r.IsEmpty() {
		t.Fatal("รายงานที่ไม่มีรายการต้องนับว่าว่าง")
	}
	if !strings.Contains(r.Subject(), "ไม่มีรายการต้องดำเนินการ") {
		t.Fatalf("หัวข้ออีเมลกรณีว่างผิด: %s", r.Subject())
	}
}

func TestRenderHTMLContainsKeyContent(t *testing.T) {
	html := RenderHTML(sampleReport())

	must := []string{
		"คุณธีปรัชญ์ เมธีภูริวัจน์",
		"Kobelco Construction Machinery Southeast Asia Co., Ltd.",
		"เรื่อง",
		"เรียน",
		"ด้วยระบบ I-CONFIRMATION ได้ตรวจสอบ",
		"ปรากฏว่ามีรายการที่ต้องดำเนินการรวมทั้งสิ้น 4 รายการ จำแนกเป็น",
		"ใบอนุญาตนำเข้า",
		"ใบอนุญาตนำออก",
		"จำนวน",
		"IL-2026-0148",
		"EX-2026-0091",
		"หมดอายุแล้ว",
		"เลยกำหนดยื่น",
		"1. ใบอนุญาตนำเข้า (Import License)",
		"2. ใบอนุญาตนำออก (Export License)",
		"จึงเรียนมาเพื่อโปรดทราบ",
	}
	for _, want := range must {
		if !strings.Contains(html, want) {
			t.Fatalf("อีเมล HTML ไม่มีข้อความ %q", want)
		}
	}

	for _, banned := range []string{"ขอแสดงความนับถือ", "ระบบ I-CONFIRMATION<br>", "ที่ IC-"} {
		if strings.Contains(html, banned) {
			t.Fatalf("อีเมล HTML ไม่ควรมีคำลงท้าย %q", banned)
		}
	}

	if strings.Contains(html, "ZgotmplZ") {
		t.Fatal("พบค่าที่ถูก escape ผิดพลาดในอีเมล")
	}
}

func TestRenderHTMLStaysPlain(t *testing.T) {
	html := RenderHTML(sampleReport())

	for _, banned := range []string{"border-radius", "linear-gradient", "box-shadow"} {
		if strings.Contains(html, banned) {
			t.Fatalf("อีเมลไม่ควรมีสไตล์ %q", banned)
		}
	}
}

func TestRenderHTMLEmptyStateShown(t *testing.T) {
	r := sampleReport()
	r.Import = nil
	r.Export = nil
	r.ImportCounts = Counts{}
	r.ExportCounts = Counts{}

	html := RenderHTML(r)
	if !strings.Contains(html, "ไม่มีใบอนุญาตนำเข้าที่หมดอายุหรือใกล้หมดอายุในสัปดาห์นี้") ||
		!strings.Contains(html, "ไม่มีใบอนุญาตนำออกที่หมดอายุหรือใกล้หมดอายุในสัปดาห์นี้") {
		t.Fatal("ไม่ขึ้นข้อความเมื่อไม่มีใบอนุญาตต้องดำเนินการ")
	}
}

func TestRenderHTMLEscapesUserData(t *testing.T) {
	r := sampleReport()
	r.Import[0].LicenseNo = `<script>alert(1)</script>`

	html := RenderHTML(r)
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatal("ข้อมูลจากผู้ใช้ต้องถูก escape ก่อนใส่ลงอีเมล")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("ไม่พบข้อความที่ escape แล้ว")
	}
}

func TestRenderTextIncludesBothSections(t *testing.T) {
	text := RenderText(sampleReport())

	if !strings.Contains(text, "1. ใบอนุญาตนำเข้า (Import License)") ||
		!strings.Contains(text, "2. ใบอนุญาตนำออก (Export License)") {
		t.Fatal("ฉบับข้อความล้วนต้องมีทั้งหัวข้อนำเข้าและนำออก")
	}
	if !strings.Contains(text, "IL-2026-0148") || !strings.Contains(text, "EX-2026-0091") {
		t.Fatal("ฉบับข้อความล้วนต้องมีรายการของทั้งสองประเภท")
	}
	if !strings.Contains(text, "ต้องดำเนินการรวมทั้งสิ้น 4 รายการ จำแนกเป็น") {
		t.Fatalf("ฉบับข้อความล้วนสรุปตัวเลขผิด:\n%s", text)
	}
	if !strings.Contains(text, "เรื่อง") || !strings.Contains(text, "เรียน") {
		t.Fatal("ฉบับข้อความล้วนต้องมีหัวเรื่องและคำขึ้นต้นตามแบบหนังสือ")
	}
	if strings.Contains(text, "ขอแสดงความนับถือ") {
		t.Fatal("ฉบับข้อความล้วนไม่ควรมีคำลงท้าย")
	}
}

func TestBuildCSVHasBOMAndAllRows(t *testing.T) {
	att := BuildCSV(sampleReport())

	if len(att.Data) < 3 || att.Data[0] != 0xEF || att.Data[1] != 0xBB || att.Data[2] != 0xBF {
		t.Fatal("ไฟล์ CSV ต้องขึ้นต้นด้วย BOM เพื่อให้ Excel อ่านภาษาไทยได้")
	}

	body := string(att.Data)
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 5 {
		t.Fatalf("จำนวนบรรทัดใน CSV ผิด: %d", len(lines))
	}
	if !regexp.MustCompile(`^license-weekly-alert-\d{4}-\d{2}-\d{2}\.csv$`).MatchString(att.FileName) {
		t.Fatalf("รูปแบบชื่อไฟล์แนบผิด: %s", att.FileName)
	}
}

func TestMessageBuildHasBothPartsAndAttachment(t *testing.T) {
	r := sampleReport()
	msg := Message{
		FromEmail: "iconfirm@kobelco.com",
		FromName:  "I-CONFIRMATION System",
		To:        r.Recipients,
		Subject:   r.Subject(),
		HTML:      RenderHTML(r),
		Text:      RenderText(r),
		Attachments: []Attachment{
			BuildCSV(r),
		},
	}

	raw := string(msg.Build())

	for _, want := range []string{
		"multipart/mixed",
		"multipart/alternative",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Disposition: attachment;",
		"Auto-Submitted: auto-generated",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("จดหมาย MIME ไม่มีส่วน %q", want)
		}
	}

	if strings.Contains(raw, "รายงานสถานะใบอนุญาตนำเข้าและนำออก") {
		t.Fatal("หัวข้ออีเมลต้องเข้ารหัสแบบ MIME ก่อนใส่ในส่วนหัวจดหมาย")
	}
}

func TestRecipientNameStripsRedundantPrefix(t *testing.T) {
	r := sampleReport()
	r.Greeting = "เรียน คุณธีปรัชญ์ เมธีภูริวัจน์"

	if got := recipientName(r); got != "คุณธีปรัชญ์ เมธีภูริวัจน์" {
		t.Fatalf("ตัดคำว่า เรียน ออกไม่ถูก: %q", got)
	}

	r.Greeting = "   "
	if got := recipientName(r); got != "ผู้เกี่ยวข้อง" {
		t.Fatalf("ไม่ได้ใช้ค่าเริ่มต้นเมื่อไม่ระบุชื่อผู้รับ: %q", got)
	}
}

func TestWeekOfMonthAndLabel(t *testing.T) {
	r := sampleReport()
	r.BuddhistEra = true

	if got := r.WeekOfMonth(); got != 2 {
		t.Fatalf("ลำดับสัปดาห์ในเดือนผิด: %d", got)
	}
	if got := r.WeekLabel(); got != "สัปดาห์ที่ 2 ของเดือนกันยายน 2569" {
		t.Fatalf("ข้อความสัปดาห์ผิด: %s", got)
	}
	if !strings.Contains(r.Title(), "ประจำสัปดาห์ที่ 2 ของเดือนกันยายน 2569") {
		t.Fatalf("ชื่อเรื่องผิด: %s", r.Title())
	}
}

func TestWeekLabelFollowsSendDate(t *testing.T) {
	cases := []struct {
		sent time.Time
		want string
	}{
		{time.Date(2026, 9, 7, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 2 ของเดือนกันยายน 2569"},
		{time.Date(2026, 9, 11, 10, 0, 0, 0, testLoc), "สัปดาห์ที่ 2 ของเดือนกันยายน 2569"},
		{time.Date(2026, 9, 1, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 1 ของเดือนกันยายน 2569"},
		{time.Date(2026, 9, 28, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 5 ของเดือนกันยายน 2569"},
		{time.Date(2026, 10, 1, 9, 0, 0, 0, testLoc), "สัปดาห์ที่ 1 ของเดือนตุลาคม 2569"},
		{time.Date(2026, 8, 31, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 5 ของเดือนสิงหาคม 2569"},
		{time.Date(2026, 11, 30, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 5 ของเดือนพฤศจิกายน 2569"},
		{time.Date(2026, 3, 30, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 5 ของเดือนมีนาคม 2569"},
		{time.Date(2026, 11, 23, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 5 ของเดือนพฤศจิกายน 2569"},
		{time.Date(2025, 12, 29, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 5 ของเดือนธันวาคม 2568"},
		{time.Date(2026, 12, 28, 8, 30, 0, 0, testLoc), "สัปดาห์ที่ 5 ของเดือนธันวาคม 2569"},
	}

	for _, c := range cases {
		r := sampleReport()
		r.BuddhistEra = true
		r.GeneratedAt = c.sent
		r.PeriodStart, r.PeriodEnd = WeekBounds(c.sent)

		if got := r.WeekLabel(); got != c.want {
			t.Errorf("ส่ง %s: ได้ %q ต้องเป็น %q", c.sent.Format("2006-01-02"), got, c.want)
		}
	}
}

func TestWeekLabelUsesReportTimezone(t *testing.T) {
	sentTH := time.Date(2026, 9, 7, 0, 30, 0, 0, testLoc)

	r := sampleReport()
	r.BuddhistEra = true
	r.PeriodStart, r.PeriodEnd = WeekBounds(sentTH)
	r.GeneratedAt = sentTH.UTC()

	if got := r.WeekLabel(); got != "สัปดาห์ที่ 2 ของเดือนกันยายน 2569" {
		t.Fatalf("ต้องนับตามเวลาไทย: %s", got)
	}
}

func TestAttachmentFileNameFollowsSendDate(t *testing.T) {
	cases := []struct {
		sent time.Time
		want string
	}{
		{time.Date(2026, 9, 11, 10, 0, 0, 0, testLoc), "2026-09-11"},
		{time.Date(2026, 9, 30, 23, 59, 0, 0, testLoc), "2026-09-30"},
		{time.Date(2026, 10, 1, 0, 5, 0, 0, testLoc), "2026-10-01"},
		{time.Date(2025, 12, 31, 23, 50, 0, 0, testLoc), "2025-12-31"},
		{time.Date(2028, 2, 29, 8, 30, 0, 0, testLoc), "2028-02-29"},
	}

	for _, c := range cases {
		for _, stored := range []time.Time{c.sent, c.sent.UTC()} {
			r := sampleReport()
			r.PeriodStart, r.PeriodEnd = WeekBounds(c.sent)
			r.GeneratedAt = stored

			if got := BuildXLSX(r).FileName; got != "license-weekly-alert-"+c.want+".xlsx" {
				t.Errorf("ส่ง %s (เก็บเป็น %s): ชื่อไฟล์ %s", c.sent.Format("2006-01-02 15:04"), stored.Location(), got)
			}
			if got := BuildCSV(r).FileName; got != "license-weekly-alert-"+c.want+".csv" {
				t.Errorf("ส่ง %s (เก็บเป็น %s): ชื่อไฟล์ %s", c.sent.Format("2006-01-02 15:04"), stored.Location(), got)
			}
		}
	}
}

func TestWeekLabelWithoutSendDate(t *testing.T) {
	r := sampleReport()
	r.BuddhistEra = true
	r.GeneratedAt = time.Time{}

	if got := r.WeekLabel(); got != "สัปดาห์ที่ 2 ของเดือนกันยายน 2569" {
		t.Fatalf("ข้อความสัปดาห์ผิด: %s", got)
	}
}

func TestTableCellsDoNotWrap(t *testing.T) {
	html := RenderHTML(sampleReport())

	if strings.Count(html, "white-space:nowrap;") < 10 {
		t.Fatal("ช่องตารางที่เป็นเลขที่เอกสารและวันที่ต้องตั้ง white-space:nowrap")
	}
}

func TestNoEnclosureLine(t *testing.T) {
	if strings.Contains(RenderHTML(sampleReport()), "สิ่งที่ส่งมาด้วย") {
		t.Fatal("หนังสือไม่ควรมีบรรทัดสิ่งที่ส่งมาด้วย")
	}
	if strings.Contains(RenderText(sampleReport()), "สิ่งที่ส่งมาด้วย") {
		t.Fatal("ฉบับข้อความล้วนไม่ควรมีบรรทัดสิ่งที่ส่งมาด้วย")
	}
}

func TestBreakdownGroupsSumToTotal(t *testing.T) {
	r := sampleReport()
	groups := breakdownGroups(r)

	if len(groups) != 2 {
		t.Fatalf("ต้องแยกเป็นกลุ่มใบนำเข้าและใบนำออก ได้ %d", len(groups))
	}
	if groups[0].Name != "ใบอนุญาตนำเข้า" || groups[1].Name != "ใบอนุญาตนำออก" {
		t.Fatalf("ชื่อกลุ่มผิด: %s / %s", groups[0].Name, groups[1].Name)
	}

	sum := 0
	for _, g := range groups {
		for _, it := range g.Items {
			sum += it.Count
		}
	}

	// ยอดจำแนกนับเฉพาะอายุใบอนุญาต ไม่รวมกำหนดยื่น กสทช.
	// ใบหนึ่งใบจึงถูกนับบรรทัดเดียวเสมอ ไม่ซ้ำ
	want := r.ImportCounts.Expired + r.ImportCounts.Expiring +
		r.ExportCounts.Expired + r.ExportCounts.Expiring
	if sum != want {
		t.Fatalf("ผลรวมยอดจำแนก %d ไม่ตรงกับจำนวนใบที่หมดอายุ/ใกล้หมดอายุ %d", sum, want)
	}
	if sum != breakdownTotal(groups) {
		t.Fatalf("breakdownTotal %d ไม่ตรงกับผลรวมยอดจำแนก %d", breakdownTotal(groups), sum)
	}
}

// ยอดจำแนกต้องมีแต่บรรทัดอายุใบอนุญาต — กำหนดยื่น กสทช. อยู่ในตารางด้านล่างแทน
//
// ของเดิมใบนำออกใบเดียวโผล่ได้ทั้งบรรทัด "ใกล้หมดอายุ" และ "เลยกำหนดยื่นต่อ กสทช."
// ยอดรวมหัวจดหมายจึงมากกว่าจำนวนใบจริง
func TestBreakdownHasNoNbtcLines(t *testing.T) {
	r := sampleReport()
	groups := breakdownGroups(r)

	for _, g := range groups {
		if len(g.Items) != 2 {
			t.Fatalf("กลุ่ม %s ต้องมี 2 บรรทัด ได้ %+v", g.Name, g.Items)
		}
		if g.Items[0].Label != "ใกล้หมดอายุ" || g.Items[1].Label != "หมดอายุ" {
			t.Fatalf("กลุ่ม %s ลำดับหรือข้อความผิด: %+v", g.Name, g.Items)
		}
		for _, it := range g.Items {
			if strings.Contains(it.Label, "กสทช.") {
				t.Fatalf("ยอดจำแนกต้องไม่มีบรรทัดกำหนดยื่น กสทช.: %s", it.Label)
			}
		}
	}

	// แต่ข้อมูลกำหนดยื่นต้องยังอยู่ครบในตารางใบนำออก
	if !strings.Contains(renderExportTable(r, 25), "เลยกำหนดยื่น") {
		t.Fatal("ตารางใบนำออกต้องยังแสดงสถานะการยื่น กสทช.")
	}
}

// กลุ่มที่มีรายการ ต้องขึ้นครบทุกบรรทัดแม้บรรทัดนั้นจะเป็น 0
//
// ของเดิมซ่อนบรรทัดศูนย์ทิ้ง ผู้อ่านจึงแยกไม่ออกว่า "ไม่มีใบหมดอายุ"
// หรือ "ระบบไม่ได้ตรวจเรื่องหมดอายุให้"
func TestBreakdownShowsZeroLineInsideActiveGroup(t *testing.T) {
	r := sampleReport()
	r.ExportCounts.Expired = 0
	r.ExportCounts.Expiring = 14

	groups := breakdownGroups(r)
	if len(groups) != 2 {
		t.Fatalf("ต้องมีสองกลุ่ม ได้ %d", len(groups))
	}

	exp := groups[1]
	if len(exp.Items) != 2 {
		t.Fatalf("กลุ่มใบนำออกต้องมี 2 บรรทัด ได้ %+v", exp.Items)
	}
	if exp.Items[0].Label != "ใกล้หมดอายุ" || exp.Items[0].Count != 14 {
		t.Fatalf("บรรทัดใกล้หมดอายุผิด: %+v", exp.Items[0])
	}
	if exp.Items[1].Label != "หมดอายุ" || exp.Items[1].Count != 0 {
		t.Fatalf("ต้องขึ้นบรรทัดหมดอายุ 0 รายการ ได้ %+v", exp.Items[1])
	}

	if !strings.Contains(RenderText(r), "หมดอายุ") {
		t.Fatal("ฉบับข้อความล้วนต้องมีบรรทัดหมดอายุ")
	}
}

func TestBreakdownSkipsEmptyItem(t *testing.T) {
	r := sampleReport()
	r.ImportCounts.Expired = 0
	r.ImportCounts.Expiring = 0

	groups := breakdownGroups(r)
	if len(groups) != 1 || groups[0].Name != "ใบอนุญาตนำออก" {
		t.Fatalf("กลุ่มที่ไม่มีรายการเลยต้องหายไปทั้งกลุ่ม ได้ %+v", groups)
	}
}

func TestNoCriteriaNotes(t *testing.T) {
	for _, body := range []string{RenderHTML(sampleReport()), RenderText(sampleReport())} {
		if strings.Contains(body, "นับแต่วันที่ออกใบอนุญาต") || strings.Contains(body, "ยังมิได้ปิดงานรวม") {
			t.Fatal("หนังสือไม่ควรมีคำอธิบายเกณฑ์ใต้หัวข้อ")
		}
	}
}

func TestImportTableHasRequiredColumns(t *testing.T) {
	html := renderImportTable(sampleReport(), 25)

	for _, want := range []string{
		">เลขที่ใบอนุญาต<", ">เลขอินวอยซ์นำเข้า<", ">เลขใบขนสินค้าขาเข้า<",
		">ตราอักษร<", ">แบบ/รุ่น<", ">จำนวน<", ">วันหมดอายุ<", ">คงเหลือ<", ">สถานะ<",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("ตารางใบนำเข้าต้องมีคอลัมน์ %s", want)
		}
	}
	if strings.Contains(html, ">กำหนดยื่น กสทช.<") {
		t.Fatal("ตารางใบนำเข้าไม่ควรมีคอลัมน์กำหนดยื่น กสทช.")
	}
	if !strings.Contains(html, "KCMT-25-0912") {
		t.Fatal("ตารางใบนำเข้าต้องแสดงเลขอินวอยซ์นำเข้า")
	}
}

func TestExportTableHasRequiredColumns(t *testing.T) {
	html := renderExportTable(sampleReport(), 25)

	for _, want := range []string{
		">เลขที่ใบอนุญาต<", ">จำนวน<", ">วันหมดอายุ<", ">คงเหลือ<",
		">สถานะ<", ">กำหนดยื่น กสทช.<", ">สถานะการยื่น<",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("ตารางใบนำออกต้องมีคอลัมน์ %s", want)
		}
	}
	if strings.Contains(html, ">ตราอักษร<") {
		t.Fatal("ตารางใบนำออกไม่ควรมีคอลัมน์ตราอักษร")
	}
	if !strings.Contains(html, "หมดอายุแล้ว") {
		t.Fatal("คอลัมน์สถานะต้องแสดงข้อความสถานะอายุใบอนุญาต")
	}
}

// ใบนำเข้าและใบนำออกต้องอยู่คนละตาราง ใต้หัวข้อที่มีเลขกำกับของตัวเอง
// คอลัมน์เลขที่ใบอนุญาตต้องถูกกำหนดความกว้างไว้ ไม่งั้นมันจะกินพื้นที่ที่เหลือทั้งหมด
// แล้วดันคอลัมน์จำนวนไปอยู่คนละฟากตาราง (พื้นที่ส่วนเกินต้องไปลงคอลัมน์ท้ายสุด)
func TestLicenseNoColumnIsPinnedNotStretched(t *testing.T) {
	for _, c := range []struct {
		name  string
		html  string
		last  string
	}{
		{"ใบนำเข้า", renderImportTable(sampleReport(), 25), "สถานะ"},
		{"ใบนำออก", renderExportTable(sampleReport(), 25), "สถานะการยื่น"},
	} {
		idx := strings.Index(c.html, ">เลขที่ใบอนุญาต<")
		if idx < 0 {
			t.Fatalf("%s: ไม่พบหัวคอลัมน์เลขที่ใบอนุญาต", c.name)
		}
		cellStart := strings.LastIndex(c.html[:idx], "<th")
		if !strings.Contains(c.html[cellStart:idx], "width=") {
			t.Fatalf("%s: คอลัมน์เลขที่ใบอนุญาตต้องกำหนดความกว้างไว้: %s", c.name, c.html[cellStart:idx])
		}

		// คอลัมน์ท้ายสุดเท่านั้นที่ปล่อยให้ยืดได้
		lastIdx := strings.Index(c.html, ">"+c.last+"<")
		lastStart := strings.LastIndex(c.html[:lastIdx], "<th")
		if strings.Contains(c.html[lastStart:lastIdx], "width=") {
			t.Fatalf("%s: คอลัมน์ท้ายสุดต้องไม่กำหนดความกว้าง เพื่อรับพื้นที่ส่วนเกิน", c.name)
		}
	}
}

func TestMailHasTwoSeparateLicenseTables(t *testing.T) {
	html := RenderHTML(sampleReport())

	importHeading := strings.Index(html, "1. ใบอนุญาตนำเข้า (Import License)")
	exportHeading := strings.Index(html, "2. ใบอนุญาตนำออก (Export License)")
	if importHeading < 0 || exportHeading < 0 {
		t.Fatal("หนังสือต้องมีหัวข้อใบอนุญาตนำเข้าและนำออกแยกกัน")
	}
	if importHeading > exportHeading {
		t.Fatal("หัวข้อใบอนุญาตนำเข้าต้องมาก่อนใบอนุญาตนำออก")
	}

	// ทุกใบต้องปรากฏในตารางของประเภทตัวเอง และนับลำดับแยกกันตารางละ 1..N
	importPart, exportPart := html[importHeading:exportHeading], html[exportHeading:]
	if !strings.Contains(importPart, "IL-2026-0148") || !strings.Contains(importPart, "IL-2026-0163") {
		t.Fatal("ใบนำเข้าทุกใบต้องอยู่ในตารางใบนำเข้า")
	}
	if strings.Contains(importPart, "EX-2026-") {
		t.Fatal("ตารางใบนำเข้าต้องไม่มีเลขใบนำออกปน")
	}
	if !strings.Contains(exportPart, "EX-2026-0091") || !strings.Contains(exportPart, "EX-2026-0133") {
		t.Fatal("ใบนำออกทุกใบต้องอยู่ในตารางใบนำออก")
	}
	if strings.Contains(exportPart, "IL-2026-") {
		t.Fatal("ตารางใบนำออกต้องไม่มีเลขใบนำเข้าปน")
	}

	text := RenderText(sampleReport())
	if !strings.Contains(text, "1. ใบอนุญาตนำเข้า (Import License)") ||
		!strings.Contains(text, "2. ใบอนุญาตนำออก (Export License)") {
		t.Fatal("ฉบับข้อความล้วนต้องแยกสองหัวข้อเช่นกัน")
	}
}

func TestCSVIncludesImportReferenceOnExportRows(t *testing.T) {
	att := BuildCSV(sampleReport())
	body := string(att.Data)

	if !strings.Contains(body, "เลขที่ใบอนุญาตนำเข้า (อ้างอิง)") {
		t.Fatal("หัวตาราง CSV ต้องมีคอลัมน์เลขที่ใบอนุญาตนำเข้า (อ้างอิง)")
	}
	if !strings.Contains(body, "IL-2026-0163") {
		t.Fatal("แถวใบนำออกใน CSV ต้องมีเลขที่ใบนำเข้าที่อ้างอิง")
	}
}

func TestTablesNeverShowNormalStatus(t *testing.T) {
	for _, body := range []string{RenderHTML(sampleReport()), RenderText(sampleReport())} {
		if strings.Contains(body, "ปกติ") {
			t.Fatal("ตารางไม่ควรมีแถวที่สถานะเป็นปกติ")
		}
	}
}

func TestQuantityColumnLabel(t *testing.T) {
	html := RenderHTML(sampleReport())

	if strings.Contains(html, ">เครื่อง<") {
		t.Fatal("หัวคอลัมน์ต้องเป็น จำนวน ไม่ใช่ เครื่อง")
	}
	if strings.Count(html, ">จำนวน<") != 2 {
		t.Fatal("ต้องมีหัวคอลัมน์ จำนวน ตารางละหนึ่งคอลัมน์")
	}
}

func TestRenderHTMLShowsLogo(t *testing.T) {
	r := sampleReport()
	r.LogoSrc = "cid:" + LogoContentID
	r.LogoWidth = 190

	html := RenderHTML(r)
	if !strings.Contains(html, `src="cid:`+LogoContentID+`"`) {
		t.Fatal("หัวจดหมายต้องอ้างรูปโลโก้ด้วย cid:")
	}
	if !strings.Contains(html, `alt="Kobelco Construction Machinery Southeast Asia Co., Ltd."`) {
		t.Fatal("รูปโลโก้ต้องมี alt เป็นชื่อบริษัท")
	}
	if !strings.Contains(html, `width="190"`) {
		t.Fatal("รูปโลโก้ต้องกำหนดความกว้างเป็นพิกเซล")
	}

	if strings.Contains(html, "margin:0 auto 10px") {
		t.Fatal("โลโก้ต้องชิดซ้าย ไม่ใช่จัดกึ่งกลาง")
	}

	logoAt := strings.Index(html, `src="cid:`)
	orgAt := strings.Index(html, "Kobelco Construction Machinery Southeast Asia Co., Ltd.</div>")
	if logoAt < 0 || orgAt < 0 || orgAt < logoAt {
		t.Fatal("ชื่อบริษัทต้องอยู่ถัดจากโลโก้ในหัวจดหมาย")
	}
}

func TestRenderHTMLWithoutLogo(t *testing.T) {
	html := RenderHTML(sampleReport())

	if strings.Contains(html, "<img") {
		t.Fatal("ไม่ควรมีแท็กรูปเมื่อไม่ได้ตั้งค่าโลโก้")
	}
	if !strings.Contains(html, "Kobelco Construction Machinery Southeast Asia Co., Ltd.") {
		t.Fatal("ยังต้องมีชื่อบริษัทเป็นหัวจดหมาย")
	}
}

func TestMessageEmbedsInlineImage(t *testing.T) {
	r := sampleReport()
	msg := Message{
		FromEmail: "iconfirm@kobelco.com",
		To:        r.Recipients,
		Subject:   r.Subject(),
		HTML:      RenderHTML(r),
		Text:      RenderText(r),
		Attachments: []Attachment{
			{FileName: "logo.png", ContentType: "image/png", Data: []byte("fake-png"), Inline: true, ContentID: LogoContentID},
			BuildCSV(r),
		},
	}

	raw := string(msg.Build())

	for _, want := range []string{
		"multipart/mixed",
		`multipart/related; type="multipart/alternative"`,
		"multipart/alternative",
		"Content-ID: <" + LogoContentID + ">",
		"Content-Disposition: inline; filename=\"logo.png\"",
		"Content-Disposition: attachment;",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("จดหมาย MIME ไม่มีส่วน %q", want)
		}
	}
}

func TestMessageWithoutInlineImageHasNoRelatedPart(t *testing.T) {
	r := sampleReport()
	msg := Message{
		FromEmail: "iconfirm@kobelco.com",
		To:        r.Recipients,
		Subject:   r.Subject(),
		HTML:      RenderHTML(r),
		Text:      RenderText(r),
	}

	raw := string(msg.Build())
	if strings.Contains(raw, "multipart/related") {
		t.Fatal("ไม่ควรมี multipart/related เมื่อไม่มีรูปฝังในเนื้อจดหมาย")
	}
	if !strings.Contains(raw, "multipart/alternative") {
		t.Fatal("ต้องมี multipart/alternative เสมอ")
	}
}

func TestSplitListAcceptsCommonSeparators(t *testing.T) {
	got := SplitList("a@x.com, b@x.com; c@x.com  a@x.com")
	if len(got) != 3 {
		t.Fatalf("แยกรายชื่อผิด: %v", got)
	}
}

func TestThaiDateHandlesNil(t *testing.T) {
	if ThaiDate(nil, false) != "—" {
		t.Fatal("วันที่ว่างต้องแสดงเป็นขีด")
	}
	d := time.Date(2026, 9, 4, 0, 0, 0, 0, testLoc)
	if got := ThaiDate(&d, false); got != "4 ก.ย. 2026" {
		t.Fatalf("รูปแบบวันที่ผิด: %s", got)
	}
	if got := ThaiDate(&d, true); got != "4 ก.ย. 2569" {
		t.Fatalf("รูปแบบวันที่ พ.ศ. ผิด: %s", got)
	}
}

func TestEmailUsesArabicDigitsOnly(t *testing.T) {
	thaiDigits := regexp.MustCompile("[\u0E50-\u0E59]")

	r := sampleReport()
	r.BuddhistEra = true
	r.Greeting = "คุณสมชาย แผนก ๒"
	r.Dept = "แผนกโลจิสติกส์ ๑"
	r.Import[0].LicenseNo = "IL-๒๕๖๙-๐๑๔๘"

	parts := map[string]string{
		"หัวข้ออีเมล":     r.Subject(),
		"เนื้อหา HTML":    RenderHTML(r),
		"ฉบับข้อความล้วน": RenderText(r),
	}
	for name, content := range parts {
		if loc := thaiDigits.FindStringIndex(content); loc != nil {
			from := loc[0] - 60
			if from < 0 {
				from = 0
			}
			t.Errorf("%s ยังมีเลขไทย: ...%s...", name, content[from:loc[1]])
		}
	}

	html := RenderHTML(r)
	for _, want := range []string{
		"1. ใบอนุญาตนำเข้า (Import License)",
		"IL-2569-0148",
		"คุณสมชาย แผนก 2",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("อีเมล HTML ไม่มีข้อความ %q", want)
		}
	}
}

func TestEmptyEmailUsesArabicDigits(t *testing.T) {
	r := sampleReport()
	r.Import = nil
	r.Export = nil
	r.ImportCounts = Counts{}
	r.ExportCounts = Counts{}

	html := RenderHTML(r)
	if !strings.Contains(html, "ไม่มีใบอนุญาตนำเข้าที่หมดอายุหรือใกล้หมดอายุในสัปดาห์นี้") {
		t.Fatal("ฉบับไม่มีรายการต้องแสดงข้อความไม่มีรายการ")
	}
	if regexp.MustCompile("[\u0E50-\u0E59]").MatchString(html + RenderText(r) + r.Subject()) {
		t.Fatal("ฉบับไม่มีรายการยังมีเลขไทย")
	}
}

func TestArabicDigits(t *testing.T) {
	if got := ArabicDigits("ข้อ ๑ ๒ ๓ ๔ ๕ ๖ ๗ ๘ ๙ ๐ / 2569"); got != "ข้อ 1 2 3 4 5 6 7 8 9 0 / 2569" {
		t.Fatalf("แปลงเลขไทยผิด: %q", got)
	}
	if got := ArabicDigits("ไม่มีเลขไทย 123"); got != "ไม่มีเลขไทย 123" {
		t.Fatalf("ข้อความที่ไม่มีเลขไทยต้องไม่เปลี่ยน: %q", got)
	}
}
