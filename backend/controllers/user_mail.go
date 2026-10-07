package controllers

import (
	"strings"

	"iconfirm/config"
	"iconfirm/mailer"
	"iconfirm/models"
)

// ---------------------------------------------------------------------------
// ผูกบัญชีผู้ใช้เข้ากับรายชื่อผู้รับอีเมลรายงานใบอนุญาต
//
// เดิมสองเรื่องนี้อยู่คนละหน้าและไม่รู้จักกันเลย — ตาราง users ไม่มีช่องอีเมลด้วยซ้ำ
// ผู้ดูแลที่เพิ่งสร้างบัญชีให้คนใหม่ จึงต้องไปเพิ่มชื่อซ้ำอีกรอบที่หน้า Mail Recipients
// ถึงจะได้รับรายงาน และถ้าลืม คนนั้นก็เงียบไปเลยโดยไม่มีใครรู้
//
// ตอนนี้กรอกอีเมลตอนสร้างบัญชีแล้วติ๊กรับรายงานได้ในหน้าเดียว ระบบจะเพิ่มชื่อเข้า
// ตาราง mail_recipients ให้เอง แล้วส่งรายงานฉบับล่าสุดให้ทันทีแบบเดียวกับที่
// หน้า Mail Recipients ทำอยู่ — ไม่ต้องรอถึงเช้าวันจันทร์ถัดไป
//
// ตาราง mail_recipients ยังเป็นความจริงเรื่อง "ใครได้รับอีเมลบ้าง" อยู่เหมือนเดิม
// ที่เพิ่มมาคือทางลัดให้เพิ่มชื่อจากหน้าจัดการผู้ใช้ได้ด้วย ไม่ได้ย้ายความจริงไปไว้ที่อื่น
// ---------------------------------------------------------------------------

// welcomeMailNotRequested = ไม่ได้ติ๊กให้รับรายงาน หรือไม่ได้กรอกอีเมล
const welcomeMailNotRequested = "none"

func normalizeUserEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// validateUserEmail: ตรวจอีเมลก่อนบันทึก — คืนข้อความบอกผู้ใช้ ถ้าใช้ไม่ได้
//
// อีเมลว่างถือว่าใช้ได้ เพราะผู้ใช้หน้างานหลายคนไม่มีอีเมลบริษัท
// ยกเว้นตอนที่ติ๊กขอรับรายงานไว้ — จะรับอีเมลโดยไม่มีอีเมลไม่ได้
func validateUserEmail(email string, receiveAlerts bool) string {
	if email == "" {
		if receiveAlerts {
			return "ติ๊กรับรายงานทางอีเมลไว้ แต่ยังไม่ได้กรอกอีเมล — กรุณากรอกอีเมล หรือเอาเครื่องหมายถูกออก"
		}
		return ""
	}
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return "รูปแบบอีเมลไม่ถูกต้อง"
	}
	return ""
}

// mailRecipientActive: อีเมลนี้อยู่ในรายชื่อผู้รับ (TO) และยังเปิดใช้งานอยู่หรือไม่
func mailRecipientActive(email string) bool {
	email = normalizeUserEmail(email)
	if email == "" || config.DB == nil {
		return false
	}
	var n int64
	config.DB.Model(&models.MailRecipient{}).
		Where("email = ? AND kind = ? AND active = ?", email, models.MailRecipientTo, true).
		Count(&n)
	return n > 0
}

// setMailRecipientActive: เปิด/ปิดการรับอีเมลของที่อยู่นี้ในรายชื่อ TO
//
// ยังไม่มีในรายชื่อ + สั่งเปิด → เพิ่มให้ใหม่
// ยังไม่มีในรายชื่อ + สั่งปิด → ไม่ต้องทำอะไร
//
// ตอนปิดจะไม่ลบแถวทิ้ง แค่ปิดใช้งาน เพื่อให้ผู้ดูแลยังเห็นประวัติในหน้า Mail Recipients
// ว่าเคยมีชื่อนี้อยู่ และเปิดกลับได้โดยไม่ต้องพิมพ์ใหม่
func setMailRecipientActive(email, name string, active bool) bool {
	email = normalizeUserEmail(email)
	if email == "" || config.DB == nil {
		return false
	}

	var row models.MailRecipient
	err := config.DB.Where("email = ? AND kind = ?", email, models.MailRecipientTo).First(&row).Error

	if err != nil {
		if !active {
			return false
		}
		row = models.MailRecipient{
			Email:  email,
			Kind:   models.MailRecipientTo,
			Name:   strings.TrimSpace(name),
			Active: true,
			Note:   "เพิ่มจากหน้าจัดการผู้ใช้",
		}
		return config.DB.Create(&row).Error == nil
	}

	updates := map[string]interface{}{"active": active}
	if n := strings.TrimSpace(name); n != "" && strings.TrimSpace(row.Name) == "" {
		updates["name"] = n
	}
	return config.DB.Model(&row).Updates(updates).Error == nil
}

// applyUserAlertSubscription: ตั้งให้ผู้ใช้คนนี้รับ/ไม่รับรายงานรายสัปดาห์
//
// คืนสถานะของ "จดหมายต้อนรับ" ให้หน้าเว็บเอาไปบอกผู้ดูแลว่าเกิดอะไรขึ้น
// ใช้ชุดค่าเดียวกับหน้า Mail Recipients (queued / disabled / none)
//
// การส่งจริงทำในเบื้องหลัง เพราะต้องรอ SMTP ตอบ ถ้ารอในคำขอเดียวกัน
// หน้าเว็บจะค้างหลายวินาทีทุกครั้งที่กดบันทึกผู้ใช้
func applyUserAlertSubscription(user models.User, receiveAlerts bool, triggeredBy string) string {
	email := normalizeUserEmail(user.Email)
	if email == "" {
		return welcomeMailNotRequested
	}

	alreadyReceiving := mailRecipientActive(email)
	setMailRecipientActive(email, user.Name, receiveAlerts)

	if !receiveAlerts {
		return welcomeMailNotRequested
	}

	// เคยอยู่ในรายชื่ออยู่แล้ว = ได้รับรายงานมาตลอด ไม่ต้องส่งฉบับต้อนรับซ้ำ
	if alreadyReceiving {
		return welcomeMailNotRequested
	}

	w := mailer.LoadWeeklyConfig()
	if !w.Enabled || !w.SendOnAdd {
		return welcomeMailDisabled
	}

	go sendWeeklyAlertOnAdd(email, user.Name, triggeredBy)
	return welcomeMailQueued
}

// unsubscribeUserAlerts: ถอนชื่อออกจากรายชื่อผู้รับ ตอนลบผู้ใช้
//
// ถ้าไม่ถอน รายงานจะยังถูกส่งไปหาคนที่ลาออกไปแล้วทุกสัปดาห์
func unsubscribeUserAlerts(user models.User) {
	if strings.TrimSpace(user.Email) == "" {
		return
	}
	setMailRecipientActive(user.Email, "", false)
}
