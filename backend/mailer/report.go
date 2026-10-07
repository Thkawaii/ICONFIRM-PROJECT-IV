package mailer

import (
	"fmt"
	"strings"
	"time"
)

const (
	StatusExpired  = "EXPIRED"
	StatusExpiring = "EXPIRING"
	StatusValid    = "VALID"
	StatusNoDate   = "NO_DATE"
)

const (
	LeadOverdue = "LEAD_OVERDUE"
	LeadDue     = "LEAD_DUE"
	LeadNoDate  = "LEAD_NO_DATE"
)

type ImportRow struct {
	LicenseNo     string     `json:"licenseNo"`
	InvoiceNo     string     `json:"invoiceNo"`
	DeclarationNo string     `json:"declarationNo"`
	Brand         string     `json:"brand"`
	Model         string     `json:"model"`
	Machines      int        `json:"machines"`
	Confirmed     int        `json:"confirmed"`
	IssueDate     *time.Time `json:"issueDate"`
	ExpiryDate    *time.Time `json:"expiryDate"`
	DaysLeft      int        `json:"daysLeft"`
	Status        string     `json:"status"`
	// ExportLicenseNo: ใบนำเข้านี้ตอนนำออกจะใช้ใบอนุญาตนำออกเลขนี้ — คอลัมน์ท้ายไฟล์
	// อัปโหลดใบนำเข้าเอง (เลขใบอนุญาตนำออก/วันที่ออก/ส่งออกไปประเทศ) ว่างได้ถ้ายังไม่ทราบ
	ExportLicenseNo string `json:"exportLicenseNo"`
}

type ExportRow struct {
	ExportLicenseNo string     `json:"exportLicenseNo"`
	// ImportLicenseNo/ImportExpiryDate: ใบนำออกใบนี้อ้างอิงมาจากใบนำเข้าใบใด
	// (ว่างได้ถ้าไม่พบใบนำเข้าที่อ้างอิง เช่น ใบเก่าที่ไม่มีเลขอ้างอิงในระบบ)
	ImportLicenseNo  string     `json:"importLicenseNo"`
	ImportExpiryDate *time.Time `json:"importExpiryDate"`
	Machines         int        `json:"machines"`
	IssueDate       *time.Time `json:"issueDate"`
	ExpiryDate      *time.Time `json:"expiryDate"`
	DaysLeft        int        `json:"daysLeft"`
	Status          string     `json:"status"`
	LeadDate        *time.Time `json:"leadDate"`
	LeadDaysLeft    int        `json:"leadDaysLeft"`
	LeadStatus      string     `json:"leadStatus"`
	LeadUrgent      bool       `json:"leadUrgent"`
}

type Counts struct {
	Expired     int `json:"expired"`
	Expiring    int `json:"expiring"`
	LeadOverdue int `json:"leadOverdue"`
	LeadDueSoon int `json:"leadDueSoon"`
}

func (c Counts) Total() int {
	return c.Expired + c.Expiring + c.LeadOverdue + c.LeadDueSoon
}

type WeeklyReport struct {
	GeneratedAt time.Time `json:"generatedAt"`

	WeekKey     string    `json:"weekKey"`
	WeekNo      int       `json:"weekNo"`
	WeekYear    int       `json:"weekYear"`
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`

	ImportWithinDays int `json:"importWithinDays"`
	ExportWithinDays int `json:"exportWithinDays"`
	LeadDays         int `json:"leadDays"`
	LeadWarnDays     int `json:"leadWarnDays"`

	Import []ImportRow `json:"import"`
	Export []ExportRow `json:"export"`

	ImportCounts Counts `json:"importCounts"`
	ExportCounts Counts `json:"exportCounts"`

	ImportTracked int `json:"importTracked"`
	ExportTracked int `json:"exportTracked"`

	AppURL      string   `json:"appUrl"`
	Recipients  []string `json:"recipients"`
	MaxRows     int      `json:"maxRows"`
	BuddhistEra bool     `json:"buddhistEra"`

	Greeting string `json:"greeting"`

	Org  string `json:"org"`
	Dept string `json:"dept"`

	LogoSrc   string `json:"logoSrc"`
	LogoWidth int    `json:"logoWidth"`
}

const LogoContentID = "iconfirm-logo"

func (r WeeklyReport) anchorDay() time.Time {
	if r.GeneratedAt.IsZero() {
		return r.PeriodStart
	}
	if r.PeriodStart.IsZero() {
		return r.GeneratedAt
	}

	sent := r.GeneratedAt.In(r.PeriodStart.Location())
	day := time.Date(sent.Year(), sent.Month(), sent.Day(), 0, 0, 0, 0, sent.Location())
	if day.Before(r.PeriodStart) || day.After(r.PeriodStart.AddDate(0, 0, 6)) {
		return r.PeriodStart
	}
	return day
}

func (r WeeklyReport) WeekOfMonth() int {
	d := r.anchorDay()
	first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, d.Location())
	offset := (int(first.Weekday()) + 6) % 7
	week := (d.Day()-1+offset)/7 + 1
	if week > maxWeeksInMonth {
		week = maxWeeksInMonth
	}
	return week
}

const maxWeeksInMonth = 5

func (r WeeklyReport) MonthName() string {
	return thaiMonthsFull[int(r.anchorDay().Month())-1]
}

func (r WeeklyReport) MonthYear() int {
	return displayYear(r.anchorDay().Year(), r.BuddhistEra)
}

func (r WeeklyReport) TotalActions() int {
	return r.ImportCounts.Total() + r.ExportCounts.Total()
}

func (r WeeklyReport) IsEmpty() bool { return r.TotalActions() == 0 }

func (r WeeklyReport) Urgent() int {
	return r.ImportCounts.Expired + r.ExportCounts.Expired + r.ExportCounts.LeadOverdue
}

func ISOWeekKey(t time.Time) string {
	monday, _ := WeekBounds(t)
	return monday.Format("2006-01-02")
}

func (r WeeklyReport) FileDateKey() string {
	if !r.GeneratedAt.IsZero() {
		sent := r.GeneratedAt
		if !r.PeriodStart.IsZero() {
			sent = sent.In(r.PeriodStart.Location())
		}
		return sent.Format("2006-01-02")
	}
	if r.WeekKey != "" {
		return r.WeekKey
	}
	return time.Now().Format("2006-01-02")
}

func WeekBounds(t time.Time) (time.Time, time.Time) {
	loc := t.Location()
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	offset := (int(day.Weekday()) + 6) % 7
	monday := day.AddDate(0, 0, -offset)
	sunday := monday.AddDate(0, 0, 6)
	return monday, sunday
}

var thaiMonthsShort = []string{"ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.", "ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."}
var thaiMonthsFull = []string{"มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน", "กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม"}

func displayYear(year int, buddhist bool) int {
	if buddhist {
		return year + 543
	}
	return year
}

func ThaiDate(t *time.Time, buddhist bool) string {
	if t == nil || t.IsZero() {
		return "—"
	}
	return fmt.Sprintf("%d %s %d", t.Day(), thaiMonthsShort[int(t.Month())-1], displayYear(t.Year(), buddhist))
}

func ThaiDateFull(t time.Time, buddhist bool) string {
	return fmt.Sprintf("%d %s %d", t.Day(), thaiMonthsFull[int(t.Month())-1], displayYear(t.Year(), buddhist))
}

func ThaiDateTime(t time.Time, buddhist bool) string {
	return fmt.Sprintf("%s เวลา %02d:%02d น.", ThaiDate(&t, buddhist), t.Hour(), t.Minute())
}

func (r WeeklyReport) PeriodLabel() string {
	start := r.PeriodStart
	end := r.PeriodEnd
	if start.Year() == end.Year() {
		return fmt.Sprintf("%d %s – %s",
			start.Day(), thaiMonthsShort[int(start.Month())-1],
			ThaiDate(&end, r.BuddhistEra))
	}
	return fmt.Sprintf("%s – %s", ThaiDate(&start, r.BuddhistEra), ThaiDate(&end, r.BuddhistEra))
}

func (r WeeklyReport) WeekLabel() string {
	return fmt.Sprintf("สัปดาห์ที่ %d ของเดือน%s %d", r.WeekOfMonth(), r.MonthName(), r.MonthYear())
}

func StatusLabel(status string) string {
	switch status {
	case StatusExpired:
		return "หมดอายุแล้ว"
	case StatusExpiring:
		return "ใกล้หมดอายุ"
	case StatusValid:
		return "ปกติ"
	default:
		return "ยังไม่ระบุวันที่"
	}
}

func DaysLeftLabel(status string, daysLeft int) string {
	if status == StatusNoDate {
		return "ยังไม่ระบุวันที่"
	}
	switch {
	case daysLeft < 0:
		return fmt.Sprintf("เลยมา %d วัน", -daysLeft)
	case daysLeft == 0:
		return "หมดอายุวันนี้"
	default:
		return fmt.Sprintf("เหลือ %d วัน", daysLeft)
	}
}

func DaysCountLabel(days int) string {
	switch {
	case days < 0:
		return fmt.Sprintf("เลยมา %d วัน", -days)
	case days == 0:
		return "วันนี้"
	default:
		return fmt.Sprintf("%d วัน", days)
	}
}

func LeadLabel(leadStatus string, leadDaysLeft int) string {
	if leadStatus == LeadNoDate {
		return "ยังไม่ระบุวันที่"
	}
	switch {
	case leadDaysLeft < 0:
		return fmt.Sprintf("เลยกำหนดยื่น %d วัน", -leadDaysLeft)
	case leadDaysLeft == 0:
		return "ต้องยื่นภายในวันนี้"
	default:
		return fmt.Sprintf("ยื่นภายใน %d วัน", leadDaysLeft)
	}
}

var thaiDigitReplacer = strings.NewReplacer(
	"\u0E50", "0", "\u0E51", "1", "\u0E52", "2", "\u0E53", "3", "\u0E54", "4",
	"\u0E55", "5", "\u0E56", "6", "\u0E57", "7", "\u0E58", "8", "\u0E59", "9",
)

func ArabicDigits(s string) string {
	return thaiDigitReplacer.Replace(s)
}

func (r WeeklyReport) Title() string {
	return fmt.Sprintf("รายงานสถานะใบอนุญาตนำเข้าและนำออก ประจำ%s", r.WeekLabel())
}

func (r WeeklyReport) Subject() string {
	return ArabicDigits(r.subjectText())
}

func (r WeeklyReport) subjectText() string {
	if r.IsEmpty() {
		return fmt.Sprintf("%s (ไม่มีรายการต้องดำเนินการ)", r.Title())
	}
	if urgent := r.Urgent(); urgent > 0 {
		return fmt.Sprintf("%s (ต้องดำเนินการ %d รายการ เร่งด่วน %d รายการ)",
			r.Title(), r.TotalActions(), urgent)
	}
	return fmt.Sprintf("%s (ต้องดำเนินการ %d รายการ)", r.Title(), r.TotalActions())
}
