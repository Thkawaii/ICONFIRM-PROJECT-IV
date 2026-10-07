package mailer

import (
	"fmt"
	"html"
	"strings"
)

const (
	colBody = "#1a1a1a"
	colNote = "#666666"

	colBorder     = "#d7e1e8"
	colHead       = "#00cec8"
	colHeadText   = "#ffffff"
	colHeadBorder = "#00b8b2"

	colAlert = "#a52019"
	colLink  = "#0f6cbd"
)

const fontStack = "Tahoma,'Leelawadee UI','IBM Plex Sans Thai','Segoe UI',Arial,sans-serif"

var cellFont = fmt.Sprintf("font-family:%s;font-size:13px;line-height:19px;", fontStack)

var layoutFont = fmt.Sprintf("font-family:%s;", fontStack)

func esc(s string) string { return html.EscapeString(s) }

func dash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return esc(s)
}

func alert(s string) string {
	return fmt.Sprintf(`<span style="color:%s;">%s</span>`, colAlert, s)
}

func th(text, align, width string) string {
	w := ""
	if width != "" {
		w = ` width="` + width + `"`
	}
	return fmt.Sprintf(
		`<th%s align="%s" style="%sborder:1px solid %s;background:%s;color:%s;padding:7px 10px;`+
			`text-align:%s;font-weight:700;white-space:nowrap;">%s</th>`,
		w, align, cellFont, colHeadBorder, colHead, colHeadText, align, esc(text))
}

func td(main, align string) string {
	return tdStyled(main, align, "")
}

func tdNoWrap(main, align string) string {
	return tdStyled(main, align, "white-space:nowrap;")
}

func tdStyled(main, align, extra string) string {
	return fmt.Sprintf(
		`<td align="%s" style="%sborder:1px solid %s;padding:7px 10px;text-align:%s;vertical-align:top;%s">%s</td>`,
		align, cellFont, colBorder, align, extra, main)
}

func statusText(status string) string {
	label := esc(StatusLabel(status))
	if status == StatusExpired {
		return alert(label)
	}
	return label
}

func tableOpen() string {
	return `<table cellpadding="0" cellspacing="0" border="0" ` +
		`style="border-collapse:collapse;width:100%;` + cellFont + `margin:0 0 6px;">`
}

// DefaultMaxRows: จำนวนรายการสูงสุดที่แสดงในตารางของอีเมล (นับแยกนำเข้า/นำออก)
//
// ที่เหลือไม่ได้หายไปไหน อยู่ในไฟล์แนบครบทุกรายการ
// ในอีเมลจะมีบรรทัดบอกไว้ว่า "และรายการอื่นอีก N รายการ ปรากฏตามไฟล์แนบ"
// ตั้งทับได้ด้วย WEEKLY_ALERT_MAX_ROWS ในไฟล์ .env
const DefaultMaxRows = 10

func maxRowsOf(r WeeklyReport) int {
	if r.MaxRows > 0 {
		return r.MaxRows
	}
	return DefaultMaxRows
}

func RenderHTML(r WeeklyReport) string {
	var b strings.Builder

	maxRows := maxRowsOf(r)

	preheader := fmt.Sprintf("%s — ต้องดำเนินการ %d รายการ", r.Title(), r.TotalActions())

	b.WriteString(`<!DOCTYPE html>
<html lang="th">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="x-apple-disable-message-reformatting">
<title>` + esc(r.Subject()) + `</title>
<style>
  body,table,td,div,p{ -webkit-text-size-adjust:100%; -ms-text-size-adjust:100%; }
  table,td{ mso-table-lspace:0pt; mso-table-rspace:0pt; }
  @media only screen and (max-width:600px){
    .kc-page{ padding:20px 16px !important; }
    .kc-indent{ text-indent:0 !important; }
    .kc-data{ font-size:11.5px !important; }
    .kc-head-cell{ display:block !important; width:100% !important; padding-right:0 !important; padding-bottom:10px !important; }
  }
</style>
</head>
<body style="margin:0;padding:0;background:#ffffff;">
<div style="display:none;font-size:1px;color:#ffffff;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;">` + esc(preheader) + `</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
  <tr>
    <td class="kc-page" align="left" style="padding:30px 28px;font-family:` + fontStack + `;font-size:14px;line-height:24px;color:` + colBody + `;">
      <div style="max-width:740px;">
`)

	logoCell := ""
	if r.LogoSrc != "" {
		width := r.LogoWidth
		if width <= 0 {
			width = 190
		}
		logoCell = fmt.Sprintf(`
              <td class="kc-head-cell" width="%d" valign="middle" style="padding-right:16px;">
                <img src="%s" alt="%s" width="%d" style="display:block;border:0;outline:none;text-decoration:none;height:auto;">
              </td>`, width, esc(r.LogoSrc), esc(r.Org), width)
	}

	b.WriteString(fmt.Sprintf(`
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
          <tr>%s
            <td class="kc-head-cell" valign="middle" align="left">
              <div style="`+layoutFont+`font-size:16px;font-weight:600;letter-spacing:.02em;">%s</div>
              <div style="`+layoutFont+`font-size:12.5px;color:%s;margin-top:2px;">%s</div>
            </td>
          </tr>
        </table>
        <div style="border-bottom:2px solid %s;margin:12px 0 0;"></div>
        <div style="border-bottom:1px solid %s;margin:2px 0 20px;"></div>
`, logoCell, esc(r.Org), colNote, esc(r.Dept), colBody, colBody))

	b.WriteString(fmt.Sprintf(`
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 16px;">
          <tr>
            <td align="right" style="`+layoutFont+`font-size:13px;color:%s;">%s</td>
          </tr>
        </table>
`, colBody, esc(ThaiDateFull(r.GeneratedAt, r.BuddhistEra))))

	b.WriteString(headerField("เรื่อง", esc(r.Title())))
	b.WriteString(headerField("เรียน", esc(recipientName(r))))
	b.WriteString(`<div style="height:14px;"></div>`)

	b.WriteString(indentPara(fmt.Sprintf(
		"ด้วยระบบ I-CONFIRMATION ได้ตรวจสอบสถานะใบอนุญาตนำเข้าและนำออกที่อยู่ระหว่างดำเนินการ ณ วันที่ %s แล้ว %s",
		esc(ThaiDateFull(r.GeneratedAt, r.BuddhistEra)), esc(introSentence(r)))))

	b.WriteString(renderBreakdown(r))

	b.WriteString(indentPara("จึงขอเรียนรายละเอียดของแต่ละประเภทใบอนุญาต เพื่อโปรดพิจารณาดำเนินการ ดังนี้"))

	b.WriteString(sectionHeading("1. ใบอนุญาตนำเข้า (Import License)"))
	if len(r.Import) == 0 {
		b.WriteString(clausePara("ไม่มีใบอนุญาตนำเข้าที่หมดอายุหรือใกล้หมดอายุในสัปดาห์นี้"))
	} else {
		b.WriteString(clauseTable(renderImportTable(r, maxRows)))
		if rem := len(r.Import) - maxRows; rem > 0 {
			b.WriteString(clauseNote(fmt.Sprintf("และรายการใบอนุญาตนำเข้าอื่นอีก %d รายการ ปรากฏตามไฟล์แนบ", rem)))
		}
	}

	b.WriteString(`<div style="height:10px;"></div>`)

	b.WriteString(sectionHeading("2. ใบอนุญาตนำออก (Export License)"))
	if len(r.Export) == 0 {
		b.WriteString(clausePara("ไม่มีใบอนุญาตนำออกที่หมดอายุหรือใกล้หมดอายุในสัปดาห์นี้"))
	} else {
		b.WriteString(clauseTable(renderExportTable(r, maxRows)))
		if rem := len(r.Export) - maxRows; rem > 0 {
			b.WriteString(clauseNote(fmt.Sprintf("และรายการใบอนุญาตนำออกอื่นอีก %d รายการ ปรากฏตามไฟล์แนบ", rem)))
		}
	}

	b.WriteString(`<div style="height:6px;"></div>`)

	b.WriteString(indentPara("จึงเรียนมาเพื่อโปรดทราบและดำเนินการในส่วนที่เกี่ยวข้องต่อไป"))

	if r.AppURL != "" {
		b.WriteString(indentPara(fmt.Sprintf(
			`ทั้งนี้ สามารถตรวจสอบรายละเอียดทั้งหมดได้ที่ <a href="%s" style="color:%s;">%s</a>`,
			esc(r.AppURL), colLink, esc(r.AppURL))))
	}

	b.WriteString(fmt.Sprintf(`
        <div style="border-top:1px solid %s;margin:26px 0 0;padding-top:12px;font-size:11.5px;line-height:18px;color:%s;">
          <div style="font-weight:600;color:%s;margin-bottom:2px;">หมายเหตุ</div>
          1. ใบอนุญาตที่ทำเครื่องหมาย “เสร็จสิ้น” แล้ว จะหยุดนับอายุและไม่ปรากฏในรายงานฉบับนี้<br>
          2. ใบอนุญาตที่หมดอายุแล้วและมิได้ต่ออายุภายในกำหนด ระบบจะถือว่าปิดงานแล้ว และยุติการแจ้งเตือน<br>
          &nbsp;&nbsp;&nbsp;&nbsp;หากประสงค์จะต่ออายุในภายหลัง โปรดนำเข้าข้อมูลใบอนุญาตฉบับใหม่เข้าระบบ ระบบจะกลับมาแจ้งเตือนให้อีกครั้ง<br>
          3. วันหมดอายุคำนวณจากวันที่ออกใบอนุญาตเป็นหลัก และรายการจัดกลุ่มตามเลขที่ใบอนุญาต<br>
          4. หนังสือฉบับนี้จัดทำและจัดส่งโดยระบบอัตโนมัติ จึงมิได้ลงลายมือชื่อ และขอความกรุณามิให้ตอบกลับ<br>
          &nbsp;&nbsp;&nbsp;&nbsp;หากประสงค์จะแก้ไขรอบการแจ้งเตือนหรือรายชื่อผู้รับ โปรดติดต่อผู้ดูแลระบบ
        </div>
      </div>
    </td>
  </tr>
</table>
</body>
</html>`, colBorder, colNote, colBody))

	return ArabicDigits(b.String())
}

func recipientName(r WeeklyReport) string {
	name := strings.TrimSpace(r.Greeting)
	name = strings.TrimSpace(strings.TrimPrefix(name, "เรียน"))
	if name == "" {
		return "ผู้เกี่ยวข้อง"
	}
	return name
}

func headerField(label, value string) string {
	return fmt.Sprintf(`
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 4px;">
          <tr>
            <td width="108" valign="top" style="`+layoutFont+`font-size:14px;line-height:24px;font-weight:600;">%s</td>
            <td valign="top" style="`+layoutFont+`font-size:14px;line-height:24px;">%s</td>
          </tr>
        </table>`, esc(label), value)
}

func indentPara(inner string) string {
	return fmt.Sprintf(`<p class="kc-indent" style="margin:0 0 14px;text-indent:3.2em;text-align:justify;">%s</p>`, inner)
}

func sectionHeading(title string) string {
	return fmt.Sprintf(`<p style="margin:0 0 4px;padding-left:3.2em;font-weight:600;">%s</p>`, esc(title))
}

func clauseNote(text string) string {
	return fmt.Sprintf(
		`<p style="margin:0 0 8px;padding-left:4.6em;font-size:12.5px;line-height:20px;color:%s;text-align:justify;">%s</p>`,
		colNote, esc(text))
}

func clausePara(text string) string {
	return fmt.Sprintf(`<p style="margin:0 0 16px;padding-left:4.6em;">%s</p>`, esc(text))
}

func clauseTable(inner string) string {
	return fmt.Sprintf(`<div style="padding-left:4.6em;margin:0 0 16px;">%s</div>`, inner)
}

type breakdownItem struct {
	Label string
	Count int
}

type breakdownGroup struct {
	Name  string
	Items []breakdownItem
}

func breakdownGroups(r WeeklyReport) []breakdownGroup {
	// กลุ่มที่มีรายการต้องแสดงครบทุกบรรทัด รวมบรรทัดที่เป็น 0 ด้วย
	//
	// ของเดิมซ่อนบรรทัดที่เป็นศูนย์ ผู้อ่านจึงแยกไม่ออกระหว่าง "ไม่มีใบหมดอายุ"
	// กับ "ระบบไม่ได้ตรวจเรื่องหมดอายุ" — การเห็นเลข 0 ชัด ๆ คือคำตอบว่าตรวจแล้วและไม่มี
	//
	// กลุ่มที่ไม่มีรายการเลยยังถูกตัดทั้งกลุ่มเหมือนเดิม จะได้ไม่มีหัวข้อว่างลอยอยู่
	build := func(name string, items []breakdownItem) (breakdownGroup, bool) {
		total := 0
		for _, it := range items {
			total += it.Count
		}
		if total == 0 {
			return breakdownGroup{}, false
		}
		return breakdownGroup{Name: name, Items: items}, true
	}

	groups := []breakdownGroup{}

	// ทั้งสองฝั่งจำแนกด้วยเกณฑ์เดียวกัน คือ "อายุใบอนุญาต" อย่างเดียว
	//
	// ของเดิมฝั่งนำออกมีบรรทัดกำหนดยื่นต่อ กสทช. เพิ่มมาอีกสองบรรทัด ทำให้ใบหนึ่งใบ
	// ถูกนับซ้ำได้สองบรรทัด (ทั้งใกล้หมดอายุและเลยกำหนดยื่น) ยอดรวมหัวจดหมายจึงบวม
	// เกินจำนวนใบจริง กำหนดยื่น กสทช. ยังดูได้ครบในตารางใบนำออกด้านล่างเหมือนเดิม
	if g, ok := build("ใบอนุญาตนำเข้า", []breakdownItem{
		{"ใกล้หมดอายุ", r.ImportCounts.Expiring},
		{"หมดอายุ", r.ImportCounts.Expired},
	}); ok {
		groups = append(groups, g)
	}

	if g, ok := build("ใบอนุญาตนำออก", []breakdownItem{
		{"ใกล้หมดอายุ", r.ExportCounts.Expiring},
		{"หมดอายุ", r.ExportCounts.Expired},
	}); ok {
		groups = append(groups, g)
	}

	return groups
}

func breakdownTotal(groups []breakdownGroup) int {
	total := 0
	for _, g := range groups {
		for _, it := range g.Items {
			total += it.Count
		}
	}
	return total
}

func introSentence(r WeeklyReport) string {
	if r.IsEmpty() {
		return "ปรากฏว่าไม่มีใบอนุญาตที่หมดอายุ ใกล้หมดอายุ หรือถึงกำหนดยื่นเรื่องต่อสำนักงาน กสทช. แต่อย่างใด"
	}
	return fmt.Sprintf("ปรากฏว่ามีรายการที่ต้องดำเนินการรวมทั้งสิ้น %d รายการ จำแนกเป็น", breakdownTotal(breakdownGroups(r)))
}

func renderBreakdown(r WeeklyReport) string {
	groups := breakdownGroups(r)
	if len(groups) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(`<div style="padding-left:3.2em;margin:-4px 0 16px;">
          <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="` + layoutFont + `font-size:14px;line-height:22px;">`)

	for i, g := range groups {
		top := "6px"
		if i == 0 {
			top = "0"
		}
		b.WriteString(fmt.Sprintf(`
            <tr><td colspan="3" style="padding-top:%s;font-weight:600;">%s</td></tr>`,
			top, esc(g.Name)))

		for _, it := range g.Items {
			b.WriteString(fmt.Sprintf(`
            <tr>
              <td width="18" style="padding-left:2.2em;">-</td>
              <td width="190" style="white-space:nowrap;">%s</td>
              <td width="110" align="right" style="white-space:nowrap;">%d รายการ</td>
            </tr>`, esc(it.Label), it.Count))
		}
	}

	b.WriteString(`
          </table>
        </div>`)

	return b.String()
}

func renderBreakdownText(r WeeklyReport) string {
	groups := breakdownGroups(r)
	if len(groups) == 0 {
		return ""
	}

	var b strings.Builder
	for _, g := range groups {
		b.WriteString("           " + g.Name + "\n")
		for _, it := range g.Items {
			label := []rune(it.Label)
			padding := 26 - len(label)
			if padding < 1 {
				padding = 1
			}
			b.WriteString(fmt.Sprintf("             - %s%s%d รายการ\n",
				it.Label, strings.Repeat(" ", padding), it.Count))
		}
	}
	return b.String()
}

// daysCellHTML: ช่อง "คงเหลือ" — ถ้าเลยกำหนดแล้วให้เป็นสีแจ้งเตือน
func daysCellHTML(status string, daysLeft int) string {
	s := esc(DaysLeftLabel(status, daysLeft))
	if daysLeft < 0 && status != StatusNoDate {
		return alert(s)
	}
	return s
}

// renderImportTable: ตารางใบอนุญาตนำเข้า
//
//	ลำดับ · เลขที่ใบอนุญาต · เลขอินวอยซ์นำเข้า · เลขใบขนสินค้าขาเข้า ·
//	ตราอักษร · แบบ/รุ่น · จำนวน · วันหมดอายุ · คงเหลือ · สถานะ
func renderImportTable(r WeeklyReport, maxRows int) string {
	var b strings.Builder

	b.WriteString(tableOpen())
	b.WriteString(`<tr>`)
	// คอลัมน์ทุกตัว "ยกเว้นตัวสุดท้าย" ต้องกำหนดความกว้างไว้
	//
	// ตารางกว้าง 100% และเบราว์เซอร์จะโยนพื้นที่ที่เหลือทั้งหมดไปให้คอลัมน์ที่ไม่ได้
	// กำหนดความกว้าง ถ้าปล่อยคอลัมน์เลขที่ใบอนุญาตว่างไว้ มันจะอ้วนจนเลขที่ใบอนุญาต
	// กับจำนวนห่างกันคนละฟากตาราง — พื้นที่ส่วนเกินต้องไปลงที่คอลัมน์ท้ายสุดแทน
	b.WriteString(th("ลำดับ", "center", "40"))
	b.WriteString(th("เลขที่ใบอนุญาต", "left", "130"))
	b.WriteString(th("เลขอินวอยซ์นำเข้า", "left", "120"))
	b.WriteString(th("เลขใบขนสินค้าขาเข้า", "left", "120"))
	b.WriteString(th("ตราอักษร", "left", "84"))
	b.WriteString(th("แบบ/รุ่น", "left", "84"))
	b.WriteString(th("จำนวน", "center", "56"))
	b.WriteString(th("วันหมดอายุ", "left", "90"))
	b.WriteString(th("คงเหลือ", "left", "100"))
	b.WriteString(th("สถานะ", "left", ""))
	b.WriteString(`</tr>`)

	for i, row := range r.Import {
		if i >= maxRows {
			break
		}

		b.WriteString(`<tr>`)
		b.WriteString(td(fmt.Sprintf("%d", i+1), "center"))
		b.WriteString(tdNoWrap(dash(row.LicenseNo), "left"))
		b.WriteString(tdNoWrap(dash(row.InvoiceNo), "left"))
		b.WriteString(tdNoWrap(dash(row.DeclarationNo), "left"))
		b.WriteString(tdNoWrap(dash(row.Brand), "left"))
		b.WriteString(tdNoWrap(dash(row.Model), "left"))
		b.WriteString(td(fmt.Sprintf("%d", row.Machines), "center"))
		b.WriteString(tdNoWrap(esc(ThaiDate(row.ExpiryDate, r.BuddhistEra)), "left"))
		b.WriteString(tdNoWrap(daysCellHTML(row.Status, row.DaysLeft), "left"))
		b.WriteString(tdNoWrap(statusText(row.Status), "left"))
		b.WriteString(`</tr>`)
	}

	b.WriteString(`</table>`)
	return b.String()
}

// renderExportTable: ตารางใบอนุญาตนำออก
//
//	ลำดับ · เลขที่ใบอนุญาต · จำนวน · วันหมดอายุ · คงเหลือ · สถานะ ·
//	กำหนดยื่น กสทช. · สถานะการยื่น
func renderExportTable(r WeeklyReport, maxRows int) string {
	var b strings.Builder

	b.WriteString(tableOpen())
	b.WriteString(`<tr>`)
	// เหตุผลเดียวกับตารางใบนำเข้า — พื้นที่ส่วนเกินไปลงคอลัมน์ท้ายสุด
	// ไม่ใช่คอลัมน์เลขที่ใบอนุญาต
	b.WriteString(th("ลำดับ", "center", "40"))
	b.WriteString(th("เลขที่ใบอนุญาต", "left", "150"))
	b.WriteString(th("จำนวน", "center", "56"))
	b.WriteString(th("วันหมดอายุ", "left", "90"))
	b.WriteString(th("คงเหลือ", "left", "100"))
	b.WriteString(th("สถานะ", "left", "84"))
	b.WriteString(th("กำหนดยื่น กสทช.", "left", "100"))
	b.WriteString(th("สถานะการยื่น", "left", ""))
	b.WriteString(`</tr>`)

	for i, row := range r.Export {
		if i >= maxRows {
			break
		}

		lead := esc(LeadLabel(row.LeadStatus, row.LeadDaysLeft))
		if row.LeadStatus == LeadOverdue {
			lead = alert(lead)
		}

		b.WriteString(`<tr>`)
		b.WriteString(td(fmt.Sprintf("%d", i+1), "center"))
		b.WriteString(tdNoWrap(dash(row.ExportLicenseNo), "left"))
		b.WriteString(td(fmt.Sprintf("%d", row.Machines), "center"))
		b.WriteString(tdNoWrap(esc(ThaiDate(row.ExpiryDate, r.BuddhistEra)), "left"))
		b.WriteString(tdNoWrap(daysCellHTML(row.Status, row.DaysLeft), "left"))
		b.WriteString(tdNoWrap(statusText(row.Status), "left"))
		b.WriteString(tdNoWrap(esc(ThaiDate(row.LeadDate, r.BuddhistEra)), "left"))
		b.WriteString(tdNoWrap(lead, "left"))
		b.WriteString(`</tr>`)
	}

	b.WriteString(`</table>`)
	return b.String()
}

func RenderText(r WeeklyReport) string {
	var b strings.Builder

	maxRows := maxRowsOf(r)
	pad := "        "

	b.WriteString(center(r.Org) + "\n")
	b.WriteString(center(r.Dept) + "\n")
	b.WriteString(strings.Repeat("=", 68) + "\n\n")

	b.WriteString(right(ThaiDateFull(r.GeneratedAt, r.BuddhistEra)) + "\n\n")

	b.WriteString("เรื่อง   " + r.Title() + "\n")
	b.WriteString("เรียน   " + recipientName(r) + "\n")
	b.WriteString("\n")

	b.WriteString(pad + fmt.Sprintf("ด้วยระบบ I-CONFIRMATION ได้ตรวจสอบสถานะใบอนุญาตนำเข้าและนำออกที่อยู่ระหว่างดำเนินการ\nณ วันที่ %s แล้ว %s\n",
		ThaiDateFull(r.GeneratedAt, r.BuddhistEra), introSentence(r)))
	b.WriteString(renderBreakdownText(r))
	b.WriteString("\n")

	b.WriteString(pad + "จึงขอเรียนรายละเอียดของแต่ละประเภทใบอนุญาต เพื่อโปรดพิจารณาดำเนินการ ดังนี้\n\n")

	b.WriteString(pad + "1. ใบอนุญาตนำเข้า (Import License)\n")
	if len(r.Import) == 0 {
		b.WriteString("           ไม่มีใบอนุญาตนำเข้าที่หมดอายุหรือใกล้หมดอายุในสัปดาห์นี้\n")
	} else {
		for i, row := range r.Import {
			if i >= maxRows {
				break
			}
			b.WriteString(fmt.Sprintf("           (%d) %s  จำนวน %d\n", i+1, fallback(row.LicenseNo), row.Machines))
			b.WriteString(fmt.Sprintf("               อินวอยซ์ %s · ใบขนสินค้า %s\n",
				fallback(row.InvoiceNo), fallback(row.DeclarationNo)))
			b.WriteString(fmt.Sprintf("               ตราอักษร %s · แบบ/รุ่น %s\n",
				fallback(row.Brand), fallback(row.Model)))
			b.WriteString(fmt.Sprintf("               หมดอายุ %s (%s) - %s\n",
				ThaiDate(row.ExpiryDate, r.BuddhistEra),
				DaysLeftLabel(row.Status, row.DaysLeft),
				StatusLabel(row.Status)))
		}
		if rem := len(r.Import) - maxRows; rem > 0 {
			b.WriteString(fmt.Sprintf("           และรายการใบอนุญาตนำเข้าอื่นอีก %d รายการ ปรากฏตามไฟล์แนบ\n", rem))
		}
	}
	b.WriteString("\n")

	b.WriteString(pad + "2. ใบอนุญาตนำออก (Export License)\n")
	if len(r.Export) == 0 {
		b.WriteString("           ไม่มีใบอนุญาตนำออกที่หมดอายุหรือใกล้หมดอายุในสัปดาห์นี้\n")
	} else {
		for i, row := range r.Export {
			if i >= maxRows {
				break
			}
			b.WriteString(fmt.Sprintf("           (%d) %s  จำนวน %d\n", i+1, fallback(row.ExportLicenseNo), row.Machines))
			b.WriteString(fmt.Sprintf("               หมดอายุ %s (%s) - %s\n",
				ThaiDate(row.ExpiryDate, r.BuddhistEra),
				DaysLeftLabel(row.Status, row.DaysLeft),
				StatusLabel(row.Status)))
			b.WriteString(fmt.Sprintf("               กำหนดยื่น กสทช. %s - %s\n",
				ThaiDate(row.LeadDate, r.BuddhistEra),
				LeadLabel(row.LeadStatus, row.LeadDaysLeft)))
		}
		if rem := len(r.Export) - maxRows; rem > 0 {
			b.WriteString(fmt.Sprintf("           และรายการใบอนุญาตนำออกอื่นอีก %d รายการ ปรากฏตามไฟล์แนบ\n", rem))
		}
	}
	b.WriteString("\n")

	b.WriteString(pad + "จึงเรียนมาเพื่อโปรดทราบและดำเนินการในส่วนที่เกี่ยวข้องต่อไป\n\n")

	if r.AppURL != "" {
		b.WriteString(pad + "ทั้งนี้ สามารถตรวจสอบรายละเอียดทั้งหมดได้ที่\n" + pad + r.AppURL + "\n\n")
	}

	b.WriteString(strings.Repeat("-", 68) + "\n")
	b.WriteString("หมายเหตุ\n")
	b.WriteString("1. ใบอนุญาตที่ทำเครื่องหมาย \"เสร็จสิ้น\" แล้ว จะหยุดนับอายุและไม่ปรากฏในรายงานฉบับนี้\n")
	b.WriteString("2. ใบอนุญาตที่หมดอายุแล้วและมิได้ต่ออายุภายในกำหนด ระบบจะถือว่าปิดงานแล้ว\n")
	b.WriteString("   และยุติการแจ้งเตือน หากประสงค์จะต่ออายุในภายหลัง โปรดนำเข้าข้อมูลใบอนุญาตฉบับใหม่เข้าระบบ\n")
	b.WriteString("3. วันหมดอายุคำนวณจากวันที่ออกใบอนุญาตเป็นหลัก และรายการจัดกลุ่มตามเลขที่ใบอนุญาต\n")
	b.WriteString("4. หนังสือฉบับนี้จัดทำและจัดส่งโดยระบบอัตโนมัติ จึงมิได้ลงลายมือชื่อ\n")
	b.WriteString("   และขอความกรุณามิให้ตอบกลับ\n")

	return ArabicDigits(b.String())
}

func right(text string) string {
	width := 68
	n := len([]rune(text))
	if n >= width {
		return text
	}
	return strings.Repeat(" ", width-n) + text
}

func center(text string) string {
	width := 68
	n := len([]rune(text))
	if n >= width {
		return text
	}
	return strings.Repeat(" ", (width-n)/2) + text
}

func fallback(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
