package jobs

import (
	"context"
	"log"
	"strings"
	"time"

	"iconfirm/controllers"
	"iconfirm/mailer"
	"iconfirm/models"
)

const checkInterval = time.Minute

func StartWeeklyAlertScheduler() {
	w := mailer.LoadWeeklyConfig()
	cfg := controllers.LoadEffectiveMailConfig()

	if !w.Enabled {
		log.Println("[weekly-alert] ปิดการแจ้งเตือนรายสัปดาห์อยู่ (ตั้ง WEEKLY_ALERT_ENABLED=true เพื่อเปิด)")
		return
	}

	if err := cfg.Validate(); err != nil {
		log.Printf("[weekly-alert] ⚠️  ยังส่งอีเมลไม่ได้: %v", err)
		log.Printf("[weekly-alert]     ตัวจับเวลายังทำงานอยู่ — แก้ค่าใน .env แล้วรีสตาร์ท backend ได้เลย")
		log.Printf("[weekly-alert]     อยากเห็นเมลที่ระบบเขียนก่อนโดยไม่ต้องมีบัญชีเมล ให้ตั้ง MAIL_PROVIDER=file")
	}

	if cfg.Provider == mailer.ProviderSMTP && cfg.SMTPUsername == "" {
		log.Printf("[weekly-alert] หมายเหตุ: ไม่ได้ตั้ง SMTP_USERNAME — จะส่งแบบไม่ล็อกอิน")
		log.Printf("[weekly-alert]           ใช้ได้เฉพาะ SMTP relay ภายในองค์กรที่อนุญาตให้เครื่องนี้ส่งได้")
		log.Printf("[weekly-alert]           ถ้า SMTP_HOST เป็น smtp.office365.com ต้องใส่รหัสผ่าน หรือเปลี่ยนไปใช้ MAIL_PROVIDER=graph")
		log.Printf("[weekly-alert]           ถ้าองค์กรไม่เปิด relay ให้ ใช้ MAIL_PROVIDER=flow (Power Automate) ได้โดยไม่ต้องขอสิทธิ์เพิ่ม")
	}

	if cfg.Provider == mailer.ProviderFlow {
		log.Printf("[weekly-alert] ส่งผ่าน Power Automate — เซิร์ฟเวอร์ต้องออกเน็ตไปหา *.environment.api.powerplatform.com หรือ *.logic.azure.com ได้ (พอร์ต 443)")
	}

	log.Printf("[weekly-alert] เปิดการแจ้งเตือนรายสัปดาห์ — %s", w.ScheduleLabel())
	log.Printf("[weekly-alert] ผู้รับ: %v (ส่งผ่าน %s)", cfg.To, cfg.Provider)
	log.Printf("[weekly-alert] รอบถัดไป: %s", w.NextRunAfter(time.Now()).Format("2006-01-02 15:04 -0700"))

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		if w.SendOnStart {
			sendNowIfNotSentThisWeek(w)
		}

		runIfDue(w)

		for range ticker.C {
			runIfDue(mailer.LoadWeeklyConfig())
		}
	}()
}

func sendNowIfNotSentThisWeek(w mailer.WeeklyConfig) {
	loc := w.Location
	if loc == nil {
		loc = time.Local
	}
	weekKey := mailer.ISOWeekKey(time.Now().In(loc))

	if controllers.WeeklyAlertSentThisWeek(weekKey) {
		if !w.ForceSendOnStart {
			log.Printf("[weekly-alert] รอบ %s ส่งไปแล้ว — ข้ามการส่งตอนเปิดเซิร์ฟเวอร์", weekKey)
			log.Printf("[weekly-alert]     อยากให้ส่งซ้ำ ตั้ง WEEKLY_ALERT_FORCE_SEND_ON_START=true ใน .env")
			return
		}
		log.Printf("[weekly-alert] รอบ %s ส่งไปแล้ว แต่ WEEKLY_ALERT_FORCE_SEND_ON_START=true — จะส่งซ้ำอีกฉบับ", weekKey)
	}

	cfg := controllers.LoadEffectiveMailConfig()
	log.Printf("[weekly-alert] รอบ %s ยังไม่ได้ส่ง — กำลังเขียนและส่งอีเมลถึง %s",
		weekKey, strings.Join(cfg.To, ", "))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	entry, err := controllers.SendWeeklyAlert(ctx, models.WeeklyAlertAuto, "", nil)
	if err != nil {
		log.Printf("[weekly-alert] ❌ ส่งอีเมลรอบ %s ไม่สำเร็จ: %v", weekKey, err)
		return
	}

	if entry != nil && entry.Status == models.WeeklyAlertSkipped {
		log.Printf("[weekly-alert] รอบ %s ไม่มีรายการต้องแจ้งเตือน — ข้ามการส่งตามค่าที่ตั้งไว้", weekKey)
		return
	}

	count := 0
	if entry != nil {
		count = entry.ItemCount
	}
	log.Printf("[weekly-alert] ✅ ส่งอีเมลรอบ %s เรียบร้อย (%d รายการ)", weekKey, count)
}

func dueTimeOfWeek(w mailer.WeeklyConfig, now time.Time) time.Time {
	loc := w.Location
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)

	monday, _ := mailer.WeekBounds(now)
	offset := (int(w.Weekday) + 6) % 7
	day := monday.AddDate(0, 0, offset)

	return time.Date(day.Year(), day.Month(), day.Day(), w.Hour, w.Minute, 0, 0, loc)
}

func runIfDue(w mailer.WeeklyConfig) {
	if !w.Enabled {
		return
	}

	loc := w.Location
	if loc == nil {
		loc = time.Local
	}
	now := time.Now().In(loc)

	due := dueTimeOfWeek(w, now)
	if now.Before(due) {
		return
	}

	if !w.CatchUp && now.Sub(due) > 2*time.Hour {
		return
	}

	weekKey := mailer.ISOWeekKey(now)
	if controllers.WeeklyAlertSentThisWeek(weekKey) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	entry, err := controllers.SendWeeklyAlert(ctx, models.WeeklyAlertAuto, "", nil)
	if err != nil {
		log.Printf("[weekly-alert] ❌ ส่งอีเมลรอบ %s ไม่สำเร็จ: %v", weekKey, err)
		return
	}

	if entry != nil && entry.Status == models.WeeklyAlertSkipped {
		log.Printf("[weekly-alert] รอบ %s ไม่มีรายการต้องแจ้งเตือน — ข้ามการส่งตามค่าที่ตั้งไว้", weekKey)
		return
	}

	count := 0
	if entry != nil {
		count = entry.ItemCount
	}
	log.Printf("[weekly-alert] ✅ ส่งอีเมลรอบ %s เรียบร้อย (%d รายการ) — รอบถัดไป %s",
		weekKey, count, w.NextRunAfter(now).Format("2006-01-02 15:04 -0700"))
}
