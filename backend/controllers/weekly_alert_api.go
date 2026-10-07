package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/mailer"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// API ของหน้า Weekly Alert — ดูสถานะ ดูตัวอย่างอีเมล และกดส่งเดี๋ยวนี้
//
// ปุ่ม "ส่งเดี๋ยวนี้" มีไว้ทดสอบหลัง deploy ว่า SMTP relay ส่งออกได้จริงไหม
// โดยไม่ต้องรอถึงเช้าวันจันทร์ ถ้าส่งไม่ออกจะได้ข้อความจากเซิร์ฟเวอร์เมลกลับมาเลย

type weeklyAlertConfigView struct {
	Enabled   bool      `json:"enabled"`
	Schedule  string    `json:"schedule"`
	NextRunAt time.Time `json:"nextRunAt"`

	To  []string `json:"to"`
	CC  []string `json:"cc"`
	BCC []string `json:"bcc"`

	From     string `json:"from"`
	FromName string `json:"fromName"`
	Provider string `json:"provider"`

	Ready   bool   `json:"ready"`
	Problem string `json:"problem"`

	SendWhenEmpty bool `json:"sendWhenEmpty"`
}

func buildWeeklyAlertConfigView(w mailer.WeeklyConfig, cfg mailer.Config) weeklyAlertConfigView {
	view := weeklyAlertConfigView{
		Enabled:   w.Enabled,
		Schedule:  w.ScheduleLabel(),
		NextRunAt: w.NextRunAfter(time.Now()),

		To:  emptyIfNil(cfg.To),
		CC:  emptyIfNil(cfg.CC),
		BCC: emptyIfNil(cfg.BCC),

		From:     cfg.FromEmail,
		FromName: cfg.FromName,
		Provider: cfg.Provider,

		SendWhenEmpty: w.SendWhenEmpty,
	}

	if err := cfg.Validate(); err != nil {
		view.Problem = err.Error()
	} else {
		view.Ready = true
	}

	return view
}

func emptyIfNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

type weeklyAlertLogView struct {
	ID          uint      `json:"id"`
	WeekKey     string    `json:"weekKey"`
	Mode        string    `json:"mode"`
	Status      string    `json:"status"`
	Subject     string    `json:"subject"`
	Recipients  string    `json:"recipients"`
	ItemCount   int       `json:"itemCount"`
	Provider    string    `json:"provider"`
	Error       string    `json:"error"`
	TriggeredBy string    `json:"triggeredBy"`
	SentAt      time.Time `json:"sentAt"`
}

func toWeeklyAlertLogView(r models.WeeklyAlertLog) weeklyAlertLogView {
	return weeklyAlertLogView{
		ID:          r.ID,
		WeekKey:     r.WeekKey,
		Mode:        r.Mode,
		Status:      r.Status,
		Subject:     r.Subject,
		Recipients:  r.Recipients,
		ItemCount:   r.ItemCount,
		Provider:    r.Provider,
		Error:       r.Error,
		TriggeredBy: r.TriggeredBy,
		SentAt:      r.SentAt,
	}
}

func GetWeeklyAlertStatus(c *gin.Context) {
	w := mailer.LoadWeeklyConfig()
	cfg := LoadEffectiveMailConfig()

	loc := w.Location
	if loc == nil {
		loc = time.Local
	}
	weekKey := mailer.ISOWeekKey(time.Now().In(loc))

	out := gin.H{
		"config":         buildWeeklyAlertConfigView(w, cfg),
		"currentWeekKey": weekKey,
		"sentThisWeek":   WeeklyAlertSentThisWeek(weekKey),
	}

	history := []weeklyAlertLogView{}
	if config.DB != nil {
		var rows []models.WeeklyAlertLog
		config.DB.Order("sent_at desc").Limit(10).Find(&rows)
		for _, r := range rows {
			history = append(history, toWeeklyAlertLogView(r))
		}
	}
	out["history"] = history
	if len(history) > 0 {
		out["lastSent"] = history[0]
	}

	c.JSON(http.StatusOK, out)
}

func GetWeeklyAlertPreview(c *gin.Context) {
	w := mailer.LoadWeeklyConfig()
	cfg := LoadEffectiveMailConfig()

	report := BuildWeeklyReport(w, time.Now())
	report.Recipients = cfg.To

	// ตัวอย่างบนหน้าเว็บอยู่ใน iframe จะอ้างรูปแบบ cid: เหมือนในอีเมลไม่ได้
	// จึงฝังโลโก้เป็น data URI เฉพาะตอนพรีวิว (ตัวอีเมลจริงยังแนบรูปแบบ cid: เหมือนเดิม)
	if logo, ok := loadWeeklyLogo(w); ok {
		report.LogoSrc = logo.DataURI()
	}

	// ขึ้นต้นด้วยชื่อผู้รับคนแรก ให้เห็นหน้าตาตรงกับฉบับที่คนนั้นจะได้รับจริง
	if len(cfg.To) > 0 {
		report.Greeting = personalGreeting(cfg.To[0], ActiveMailRecipientNames(models.MailRecipientTo), w.Greeting)
	}

	c.JSON(http.StatusOK, gin.H{
		"config":  buildWeeklyAlertConfigView(w, cfg),
		"subject": report.Subject(),
		"html":    mailer.RenderHTML(report),
		"summary": gin.H{
			"importCounts":  report.ImportCounts,
			"exportCounts":  report.ExportCounts,
			"total":         report.TotalActions(),
			"urgent":        report.Urgent(),
			"importTracked": report.ImportTracked,
			"exportTracked": report.ExportTracked,
		},
	})
}

type sendWeeklyAlertRequest struct {
	To string `json:"to"`
}

func SendWeeklyAlertNow(c *gin.Context) {
	var req sendWeeklyAlertRequest
	// เนื้อคำขอว่างได้ = ส่งตามรายชื่อผู้รับจริงที่ตั้งไว้
	_ = c.ShouldBindJSON(&req)

	raw := strings.TrimSpace(req.To)
	overrideTo := mailer.SplitList(raw)
	if raw != "" && len(overrideTo) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "อีเมลไม่ถูกต้อง — ตรวจว่าพิมพ์ครบและมีเครื่องหมาย @",
		})
		return
	}

	_, triggeredBy := lookupUserName(c)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	entry, err := SendWeeklyAlert(ctx, models.WeeklyAlertManual, triggeredBy, overrideTo)
	if err != nil {
		detail := err.Error()
		if entry != nil && strings.TrimSpace(entry.Error) != "" {
			detail = entry.Error
		}
		c.JSON(http.StatusBadGateway, gin.H{
			"message": "ส่งอีเมลไม่สำเร็จ — " + firstLine(detail),
			"detail":  detail,
		})
		return
	}

	if entry != nil && entry.Status == models.WeeklyAlertSkipped {
		c.JSON(http.StatusOK, gin.H{
			"status":  entry.Status,
			"message": "สัปดาห์นี้ไม่มีใบอนุญาตที่ต้องดำเนินการ — ข้ามการส่งตามค่าที่ตั้งไว้",
		})
		return
	}

	to := overrideTo
	if len(to) == 0 && entry != nil {
		to = mailer.SplitList(entry.Recipients)
	}

	count := 0
	if entry != nil {
		count = entry.ItemCount
	}

	out := gin.H{
		"status":  models.WeeklyAlertSent,
		"message": fmt.Sprintf("ส่งอีเมลเรียบร้อยแล้ว (%d รายการ) ถึง %s", count, strings.Join(to, ", ")),
	}
	// ส่งออกได้บางส่วน เช่นมีผู้รับบางคนที่เซิร์ฟเวอร์เมลตีกลับ
	if entry != nil && strings.TrimSpace(entry.Error) != "" {
		out["detail"] = entry.Error
	}

	c.JSON(http.StatusOK, out)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	const max = 240
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}
