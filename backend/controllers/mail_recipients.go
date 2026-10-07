package controllers

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"iconfirm/config"
	"iconfirm/mailer"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

func validMailKind(kind string) bool {
	return kind == models.MailRecipientTo || kind == models.MailRecipientCC || kind == models.MailRecipientBCC
}

func ActiveMailRecipients(kind string) []string {
	rows := ActiveMailRecipientRows(kind)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Email)
	}
	return out
}

func ActiveMailRecipientRows(kind string) []models.MailRecipient {
	if config.DB == nil {
		return nil
	}
	var rows []models.MailRecipient
	config.DB.Where("kind = ? AND active = ?", kind, true).Order("id asc").Find(&rows)
	return rows
}

func ActiveMailRecipientNames(kind string) map[string]string {
	out := map[string]string{}
	for _, r := range ActiveMailRecipientRows(kind) {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(r.Email))] = name
	}
	return out
}

func LoadEffectiveMailConfig() mailer.Config {
	cfg := mailer.LoadConfig()
	if config.DB == nil {
		return cfg
	}

	if to := ActiveMailRecipients(models.MailRecipientTo); len(to) > 0 {
		cfg.To = to
	}
	if cc := ActiveMailRecipients(models.MailRecipientCC); len(cc) > 0 {
		cfg.CC = cc
	}
	if bcc := ActiveMailRecipients(models.MailRecipientBCC); len(bcc) > 0 {
		cfg.BCC = bcc
	}
	return cfg
}

type MailRecipientView struct {
	ID     uint   `json:"id"`
	Email  string `json:"email"`
	Kind   string `json:"kind"`
	Active bool   `json:"active"`
	Note   string `json:"note"`
	Name   string `json:"name"`
}

func toMailRecipientView(r models.MailRecipient) MailRecipientView {
	return MailRecipientView{ID: r.ID, Email: r.Email, Kind: r.Kind, Active: r.Active, Note: r.Note, Name: r.Name}
}

func GetMailRecipients(c *gin.Context) {
	var rows []models.MailRecipient
	q := config.DB.Model(&models.MailRecipient{})
	if kind := strings.ToUpper(strings.TrimSpace(c.Query("kind"))); kind != "" {
		q = q.Where("kind = ?", kind)
	}
	q.Order("kind asc, id asc").Find(&rows)

	out := make([]MailRecipientView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toMailRecipientView(r))
	}
	c.JSON(200, out)
}

type createMailRecipientRequest struct {
	Email  string `json:"email"`
	Kind   string `json:"kind"`
	Note   string `json:"note"`
	Name   string `json:"name"`
	Active *bool  `json:"active"`
}

func CreateMailRecipient(c *gin.Context) {
	var req createMailRecipientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	kind := strings.ToUpper(strings.TrimSpace(req.Kind))
	if kind == "" {
		kind = models.MailRecipientTo
	}
	if email == "" || !strings.Contains(email, "@") {
		c.JSON(400, gin.H{"message": "กรุณากรอกอีเมลให้ถูกต้อง"})
		return
	}
	if !validMailKind(kind) {
		c.JSON(400, gin.H{"message": "kind ต้องเป็น TO, CC หรือ BCC"})
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	row := models.MailRecipient{
		Email:  email,
		Kind:   kind,
		Active: active,
		Note:   strings.TrimSpace(req.Note),
		Name:   strings.TrimSpace(req.Name),
	}
	if err := config.DB.Create(&row).Error; err != nil {
		c.JSON(409, gin.H{"message": "อีเมลนี้อยู่ในรายชื่อ " + kind + " อยู่แล้ว"})
		return
	}

	adminID, adminName := lookupUserName(c)
	CreateAuditLog("MAIL_RECIPIENT", row.ID, "create", row.Email, adminID, adminName)

	welcome := welcomeMailInactive
	if row.Active {
		w := mailer.LoadWeeklyConfig()
		if w.Enabled && w.SendOnAdd {
			go sendWeeklyAlertOnAdd(row.Email, row.Name, adminName)
			welcome = welcomeMailQueued
		} else {
			welcome = welcomeMailDisabled
		}
	}

	c.JSON(201, createMailRecipientResponse{
		MailRecipientView: toMailRecipientView(row),
		WelcomeMail:       welcome,
	})
}

const (
	welcomeMailQueued   = "queued"
	welcomeMailInactive = "inactive"
	welcomeMailDisabled = "disabled"
)

type createMailRecipientResponse struct {
	MailRecipientView
	WelcomeMail string `json:"welcome_mail"`
}

var welcomeMailMu sync.Mutex

func sendWeeklyAlertOnAdd(email, name, triggeredBy string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[weekly-alert] ❌ ส่งอีเมลให้ผู้รับใหม่ %s ล้มเหลว (panic): %v", email, r)
		}
	}()

	welcomeMailMu.Lock()
	defer welcomeMailMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	log.Printf("[weekly-alert] เพิ่มผู้รับใหม่ %s — กำลังส่งรายงานฉบับล่าสุดให้ทันที", email)

	entry, err := SendWeeklyAlertToNewRecipient(ctx, email, name, triggeredBy)
	if err != nil {
		log.Printf("[weekly-alert] ❌ ส่งอีเมลให้ผู้รับใหม่ %s ไม่สำเร็จ: %v", email, err)
		return
	}
	if entry != nil && entry.Status == models.WeeklyAlertSkipped {
		log.Printf("[weekly-alert] ไม่มีรายการต้องแจ้งเตือน — ข้ามการส่งให้ผู้รับใหม่ %s ตามค่าที่ตั้งไว้", email)
		return
	}
	log.Printf("[weekly-alert] ✅ ส่งอีเมลให้ผู้รับใหม่ %s เรียบร้อย — วันจันทร์ถัดไปจะได้รับพร้อมทุกคนตามปกติ", email)
}

type updateMailRecipientRequest struct {
	Active *bool   `json:"active"`
	Note   *string `json:"note"`
	Name   *string `json:"name"`
	Kind   string  `json:"kind"`
}

func UpdateMailRecipient(c *gin.Context) {
	id := c.Param("id")
	var row models.MailRecipient
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายชื่อนี้"})
		return
	}

	var req updateMailRecipientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}

	updates := map[string]interface{}{}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if req.Note != nil {
		updates["note"] = strings.TrimSpace(*req.Note)
	}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if kind := strings.ToUpper(strings.TrimSpace(req.Kind)); kind != "" {
		if !validMailKind(kind) {
			c.JSON(400, gin.H{"message": "kind ต้องเป็น TO, CC หรือ BCC"})
			return
		}
		updates["kind"] = kind
	}
	if len(updates) == 0 {
		c.JSON(400, gin.H{"message": "ไม่มีข้อมูลที่จะแก้ไข"})
		return
	}

	if err := config.DB.Model(&row).Updates(updates).Error; err != nil {
		c.JSON(409, gin.H{"message": "อีเมลนี้ซ้ำกับรายชื่อที่มีอยู่แล้วใน kind เดียวกัน"})
		return
	}

	adminID, adminName := lookupUserName(c)
	CreateAuditLog("MAIL_RECIPIENT", row.ID, "update", row.Email, adminID, adminName)

	config.DB.First(&row, id)
	c.JSON(200, toMailRecipientView(row))
}

func DeleteMailRecipient(c *gin.Context) {
	id := c.Param("id")

	var row models.MailRecipient
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายชื่อนี้"})
		return
	}

	if err := config.DB.Delete(&models.MailRecipient{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	adminID, adminName := lookupUserName(c)
	CreateAuditLog("MAIL_RECIPIENT", row.ID, "delete", row.Email, adminID, adminName)

	c.JSON(200, gin.H{"deleted": 1})
}
