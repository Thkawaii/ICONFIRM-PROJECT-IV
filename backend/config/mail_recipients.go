package config

import (
	"log"
	"strings"

	"iconfirm/mailer"
	"iconfirm/models"
)

func SeedMailRecipients() {
	if DB == nil {
		return
	}

	var count int64
	DB.Model(&models.MailRecipient{}).Count(&count)
	if count > 0 {
		return
	}

	cfg := mailer.LoadConfig()

	seedKind := func(kind string, emails []string) int {
		added := 0
		for _, raw := range emails {
			email := strings.ToLower(strings.TrimSpace(raw))
			if email == "" {
				continue
			}
			if err := DB.Create(&models.MailRecipient{
				Email:  email,
				Kind:   kind,
				Active: true,
			}).Error; err != nil {
				log.Printf("[seed-mail-recipients] เพิ่ม %s (%s) ไม่สำเร็จ: %v", email, kind, err)
				continue
			}
			added++
		}
		return added
	}

	total := seedKind(models.MailRecipientTo, cfg.To)
	total += seedKind(models.MailRecipientCC, cfg.CC)
	total += seedKind(models.MailRecipientBCC, cfg.BCC)

	if total > 0 {
		log.Printf("[seed-mail-recipients] ย้ายรายชื่อผู้รับอีเมลจาก .env เข้าตาราง mail_recipients แล้ว %d รายชื่อ (แก้ไขต่อได้จากหน้า Admin)", total)
	}
}
