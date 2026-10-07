package mailer

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ProviderSMTP    = "smtp"
	ProviderGraph   = "graph"
	ProviderOutlook = "outlook"
	ProviderFlow    = "flow"
	ProviderFile    = "file"
	ProviderLog     = "log"
)

// DefaultSMTPHost / DefaultSMTPPort — SMTP relay ภายในบริษัท ชุดเดียวกับที่
// ระบบแจ้งซ่อม (Repair System) ใช้ส่งอีเมลเข้า Outlook อยู่ทุกวันนี้
// จึงรู้แน่ว่าใช้ได้จริงบนวง LAN relay ตัวนี้ไม่ต้องล็อกอินและไม่เปิด STARTTLS
//
// DefaultFromEmail — ที่อยู่ผู้ส่งตั้งต้น ใช้เมื่อไม่ได้ตั้ง MAIL_FROM ไว้
// relay ไม่ตรวจว่าใครเป็นผู้ส่ง จึงใส่เป็นอีเมลผู้ดูแลระบบได้เลย
// วันไหนเปลี่ยนไปใช้กล่องจดหมายกลาง ก็ตั้ง MAIL_FROM ทับได้โดยไม่ต้องแก้โค้ด
//
// ทุกค่าตั้งทับได้เสมอด้วย SMTP_HOST / SMTP_PORT / MAIL_FROM ในไฟล์ .env
const (
	DefaultSMTPHost  = "10.1.7.204"
	DefaultSMTPPort  = 25
	DefaultFromEmail = "theeparat.metheepooriwat@kobelco.com"
)

type Config struct {
	Provider string

	FromEmail string
	FromName  string

	To  []string
	CC  []string
	BCC []string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPStartTLS bool

	GraphTenantID     string
	GraphClientID     string
	GraphClientSecret string
	GraphSender       string

	FlowURL    string
	FlowSecret string
	FlowInline string

	OutDir string

	OutlookDisplayOnly bool

	Timeout time.Duration
}

// defaultStartTLS: เดาค่าเริ่มต้นของ STARTTLS ให้ตรงกับชนิดของเซิร์ฟเวอร์เมล
//
// relay ภายในองค์กร (เช่น Exchange relay ที่พอร์ต 25 แบบไม่ต้องล็อกอิน)
// ส่วนใหญ่ไม่เปิด STARTTLS เลย ถ้าบังคับเปิดไว้จะส่งไม่ออกพร้อมข้อความว่า
// "เซิร์ฟเวอร์เมลไม่รองรับ STARTTLS" ทั้งที่ตั้งค่าอื่นถูกหมด
//
// เกณฑ์: พอร์ต 25 และไม่มีชื่อผู้ใช้ = relay ภายใน → ปิด STARTTLS
// กรณีอื่น (เช่น Office 365 พอร์ต 587 ที่ต้องล็อกอิน) → เปิดไว้ตามเดิม
// ตั้ง SMTP_STARTTLS เองได้เสมอ ค่าที่ตั้งเองชนะค่าเดานี้
func defaultStartTLS() bool {
	if envInt("SMTP_PORT", DefaultSMTPPort) == 25 && strings.TrimSpace(os.Getenv("SMTP_USERNAME")) == "" {
		return false
	}
	return true
}

type WeeklyConfig struct {
	Enabled bool

	Weekday time.Weekday

	Hour   int
	Minute int

	Location *time.Location

	ImportWithinDays int

	ExportWithinDays int

	SendWhenEmpty bool

	CatchUp bool

	SendOnStart bool

	ForceSendOnStart bool

	SendOnAdd bool

	MaxRows int

	BuddhistEra bool

	AppURL string

	Greeting string

	Org string

	Dept string

	LogoPath string

	LogoWidth int

	AttachFormat string

	AttachCSV bool
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "":
		return fallback
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func SplitList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, f := range fields {
		addr := strings.TrimSpace(f)
		if addr == "" || !strings.Contains(addr, "@") {
			continue
		}
		key := strings.ToLower(addr)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, addr)
	}
	return out
}

const DefaultRecipient = "theeparat.metheepooriwat@kobelco.com"

func LoadConfig() Config {
	c := Config{
		Provider: strings.ToLower(env("MAIL_PROVIDER", ProviderSMTP)),

		FromEmail: env("MAIL_FROM", ""),
		FromName:  env("MAIL_FROM_NAME", "I-CONFIRMATION System"),

		To:  SplitList(env("WEEKLY_ALERT_TO", DefaultRecipient)),
		CC:  SplitList(env("WEEKLY_ALERT_CC", "")),
		BCC: SplitList(env("WEEKLY_ALERT_BCC", "")),

		SMTPHost:     env("SMTP_HOST", DefaultSMTPHost),
		SMTPPort:     envInt("SMTP_PORT", DefaultSMTPPort),
		SMTPUsername: env("SMTP_USERNAME", ""),
		SMTPPassword: env("SMTP_PASSWORD", ""),
		SMTPStartTLS: envBool("SMTP_STARTTLS", defaultStartTLS()),

		GraphTenantID:     env("GRAPH_TENANT_ID", ""),
		GraphClientID:     env("GRAPH_CLIENT_ID", ""),
		GraphClientSecret: env("GRAPH_CLIENT_SECRET", ""),
		GraphSender:       env("GRAPH_SENDER", ""),

		FlowURL:    env("MAIL_FLOW_URL", ""),
		FlowSecret: env("MAIL_FLOW_SECRET", ""),
		FlowInline: strings.ToLower(env("MAIL_FLOW_INLINE", "strip")),

		OutDir:             env("MAIL_OUT_DIR", "outbox"),
		OutlookDisplayOnly: envBool("MAIL_OUTLOOK_DISPLAY_ONLY", false),

		Timeout: time.Duration(envInt("MAIL_TIMEOUT_SECONDS", 30)) * time.Second,
	}

	if c.FromEmail == "" {
		switch {
		case c.Provider == ProviderGraph:
			c.FromEmail = c.GraphSender
		case c.SMTPUsername != "":
			c.FromEmail = c.SMTPUsername
		default:
			// relay ภายในไม่ต้องล็อกอิน จึงไม่มีชื่อผู้ใช้ให้เดาที่อยู่ผู้ส่ง
			// ใช้ที่อยู่ผู้ส่งตั้งต้นแทน (ตั้งทับได้ด้วย MAIL_FROM)
			c.FromEmail = DefaultFromEmail
		}
	}
	if c.Provider == ProviderGraph && c.GraphSender == "" {
		c.GraphSender = c.FromEmail
	}

	return c
}

func (c Config) Validate() error {
	if len(c.To) == 0 {
		return fmt.Errorf("ยังไม่ได้ตั้งผู้รับอีเมล — ตั้ง WEEKLY_ALERT_TO ในไฟล์ .env หรือเพิ่มผู้รับจากหน้า Admin")
	}

	switch c.Provider {
	case ProviderLog:
		return nil

	case ProviderFile, ProviderOutlook:
		return nil

	case ProviderFlow:
		if strings.TrimSpace(c.FlowURL) == "" {
			return fmt.Errorf("ยังไม่ได้ตั้ง MAIL_FLOW_URL — คัดลอก URL จาก trigger ของ Power Automate มาใส่ในไฟล์ .env")
		}
		if !validFlowURL(c.FlowURL) {
			return fmt.Errorf("MAIL_FLOW_URL ต้องขึ้นต้นด้วย https:// (ใช้ http:// ได้เฉพาะ localhost ตอนทดสอบ)")
		}
		return nil

	case ProviderSMTP:
		if c.SMTPHost == "" {
			return fmt.Errorf("ยังไม่ได้ตั้ง SMTP_HOST")
		}
		if c.SMTPPort <= 0 {
			return fmt.Errorf("SMTP_PORT ไม่ถูกต้อง")
		}
		if (c.SMTPUsername == "") != (c.SMTPPassword == "") {
			return fmt.Errorf("ต้องตั้ง SMTP_USERNAME และ SMTP_PASSWORD คู่กัน (หรือเว้นว่างทั้งคู่ถ้าเป็น relay ที่ไม่ต้องล็อกอิน)")
		}
		if c.FromEmail == "" {
			return fmt.Errorf("ยังไม่ได้ตั้ง MAIL_FROM (อีเมลที่จะใช้เป็นผู้ส่ง)")
		}
		return nil

	case ProviderGraph:
		if c.GraphTenantID == "" || c.GraphClientID == "" || c.GraphClientSecret == "" {
			return fmt.Errorf("ยังไม่ได้ตั้ง GRAPH_TENANT_ID / GRAPH_CLIENT_ID / GRAPH_CLIENT_SECRET")
		}
		if c.GraphSender == "" {
			return fmt.Errorf("ยังไม่ได้ตั้ง GRAPH_SENDER (อีเมลกล่องที่ใช้ส่ง)")
		}
		return nil

	default:
		return fmt.Errorf("MAIL_PROVIDER ไม่รองรับ: %q (ใช้ได้: smtp, graph, flow, outlook, file, log)", c.Provider)
	}
}

func (c Config) Ready() bool { return c.Validate() == nil }

var weekdayNames = map[string]time.Weekday{
	"SUN": time.Sunday, "SUNDAY": time.Sunday, "อาทิตย์": time.Sunday,
	"MON": time.Monday, "MONDAY": time.Monday, "จันทร์": time.Monday,
	"TUE": time.Tuesday, "TUESDAY": time.Tuesday, "อังคาร": time.Tuesday,
	"WED": time.Wednesday, "WEDNESDAY": time.Wednesday, "พุธ": time.Wednesday,
	"THU": time.Thursday, "THURSDAY": time.Thursday, "พฤหัสบดี": time.Thursday,
	"FRI": time.Friday, "FRIDAY": time.Friday, "ศุกร์": time.Friday,
	"SAT": time.Saturday, "SATURDAY": time.Saturday, "เสาร์": time.Saturday,
}

func ThaiWeekdayName(d time.Weekday) string {
	names := []string{"วันอาทิตย์", "วันจันทร์", "วันอังคาร", "วันพุธ", "วันพฤหัสบดี", "วันศุกร์", "วันเสาร์"}
	i := int(d)
	if i < 0 || i > 6 {
		return ""
	}
	return names[i]
}

func parseWeekday(raw string, fallback time.Weekday) time.Weekday {
	key := strings.ToUpper(strings.TrimSpace(raw))
	if key == "" {
		return fallback
	}
	if d, ok := weekdayNames[key]; ok {
		return d
	}
	if n, err := strconv.Atoi(key); err == nil && n >= 0 && n <= 6 {
		return time.Weekday(n)
	}
	return fallback
}

func parseClock(raw string, fallbackHour, fallbackMinute int) (int, int) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fallbackHour, fallbackMinute
	}
	parts := strings.SplitN(s, ":", 2)
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || h < 0 || h > 23 {
		return fallbackHour, fallbackMinute
	}
	m := 0
	if len(parts) == 2 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && v >= 0 && v <= 59 {
			m = v
		}
	}
	return h, m
}

func LoadWeeklyConfig() WeeklyConfig {
	loc, err := time.LoadLocation(env("WEEKLY_ALERT_TZ", "Asia/Bangkok"))
	if err != nil || loc == nil {
		loc = time.FixedZone("ICT", 7*60*60)
	}

	hour, minute := parseClock(env("WEEKLY_ALERT_TIME", "08:30"), 8, 30)

	return WeeklyConfig{
		Enabled:          envBool("WEEKLY_ALERT_ENABLED", true),
		Weekday:          parseWeekday(env("WEEKLY_ALERT_DAY", "MON"), time.Monday),
		Hour:             hour,
		Minute:           minute,
		Location:         loc,
		ImportWithinDays: envInt("WEEKLY_ALERT_IMPORT_WITHIN_DAYS", 30),
		ExportWithinDays: envInt("WEEKLY_ALERT_EXPORT_WITHIN_DAYS", 7),
		SendWhenEmpty:    envBool("WEEKLY_ALERT_SEND_WHEN_EMPTY", true),
		CatchUp:          envBool("WEEKLY_ALERT_CATCH_UP", true),
		SendOnStart:      envBool("WEEKLY_ALERT_SEND_ON_START", true),
		ForceSendOnStart: envBool("WEEKLY_ALERT_FORCE_SEND_ON_START", false),
		SendOnAdd:        envBool("WEEKLY_ALERT_SEND_ON_ADD", true),
		MaxRows:          envInt("WEEKLY_ALERT_MAX_ROWS", DefaultMaxRows),
		BuddhistEra:      envBool("WEEKLY_ALERT_BUDDHIST_YEAR", true),
		AppURL:           strings.TrimRight(env("APP_BASE_URL", ""), "/"),
		Greeting:         env("WEEKLY_ALERT_GREETING", "ผู้เกี่ยวข้อง"),
		Org:              env("WEEKLY_ALERT_ORG", "Kobelco Construction Machinery Southeast Asia Co., Ltd."),
		Dept:             env("WEEKLY_ALERT_DEPT", "แผนกโลจิสติกส์"),
		LogoPath:         env("WEEKLY_ALERT_LOGO", "assets/kobelco-logo.png"),
		LogoWidth:        envInt("WEEKLY_ALERT_LOGO_WIDTH", 190),
		AttachCSV:        envBool("WEEKLY_ALERT_ATTACH_CSV", true),
		AttachFormat:     strings.ToLower(env("WEEKLY_ALERT_ATTACH_FORMAT", "xlsx")),
	}
}

func (w WeeklyConfig) ScheduleLabel() string {
	tz := "Asia/Bangkok"
	if w.Location != nil {
		tz = w.Location.String()
	}
	return fmt.Sprintf("ทุก%s เวลา %02d:%02d น. (%s)", ThaiWeekdayName(w.Weekday), w.Hour, w.Minute, tz)
}

func (w WeeklyConfig) NextRunAfter(from time.Time) time.Time {
	loc := w.Location
	if loc == nil {
		loc = time.UTC
	}
	t := from.In(loc)
	candidate := time.Date(t.Year(), t.Month(), t.Day(), w.Hour, w.Minute, 0, 0, loc)

	diff := (int(w.Weekday) - int(t.Weekday()) + 7) % 7
	candidate = candidate.AddDate(0, 0, diff)

	if !candidate.After(t) {
		candidate = candidate.AddDate(0, 0, 7)
	}
	return candidate
}

// MAIL_FLOW_URL ต้องเป็น https เสมอ เพราะใน URL มีโทเคนของ Flow ติดไปด้วย
// ยกเว้น localhost ที่อนุญาตให้เป็น http ได้ เผื่อเอาไว้ทดสอบบนเครื่องตัวเอง
func validFlowURL(raw string) bool {
	u := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(u, "https://") {
		return true
	}
	if strings.HasPrefix(u, "http://") {
		host := strings.SplitN(strings.TrimPrefix(u, "http://"), "/", 2)[0]
		host = strings.SplitN(host, ":", 2)[0]
		return host == "localhost" || host == "127.0.0.1" || host == "[::1]"
	}
	return false
}
