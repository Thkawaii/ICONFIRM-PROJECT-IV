package controllers

import (
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Audit log — ประวัติการใช้งาน เก็บทั้งข้อมูลที่ถูกต้องและไม่ถูกต้อง
//
// ของที่ถูกต้อง  → CreateAuditLog(...)        (เหมือนเดิม ไม่ต้องแก้ที่เรียกอยู่แล้ว)
// ของที่ไม่ถูกต้อง → LogInvalidData(c, InvalidData{...})
//
// เวลาอ่าน GET /audit-log จะได้คอลัมน์ภาษาไทยมาด้วย (statusLabel, summary)
// ดูปุ๊บรู้เลยว่า "สแกนได้อะไร" และ "ที่ถูกต้องคืออะไร"
// ---------------------------------------------------------------------------

// AuditLogRow = แถวที่ส่งให้หน้าเว็บ / คนอ่าน (ข้อมูลในตาราง + คำอธิบายไทย)
type AuditLogRow struct {
	models.AuditLog

	// StatusLabel = "ถูกต้อง" / "ไม่ถูกต้อง"
	StatusLabel string `json:"statusLabel"`

	// Summary = สรุปอ่านง่ายบรรทัดเดียว
	// เช่น "P/N สแกนได้ YN22E00851X9 — ที่ถูกต้องคือ YN22E00851F1"
	Summary string `json:"summary"`
}

// GetAuditLog: GET /audit-log
//
// ตัวกรอง (ใส่หรือไม่ใส่ก็ได้)
//
//	?status=invalid       เฉพาะข้อมูลที่ไม่ถูกต้อง
//	?status=valid         เฉพาะข้อมูลที่ถูกต้อง
//	?source_table=WH_FLOW_ISSUE
//	?machine_no=YN30100003
//	?limit=200            (ค่าเริ่มต้น 500 สูงสุด 5000)
func GetAuditLog(c *gin.Context) {

	q := config.DB.Order("action_datetime desc")

	switch strings.ToUpper(strings.TrimSpace(c.Query("status"))) {
	case "INVALID", "0", "FALSE":
		q = q.Where("data_status = ?", models.AuditStatusInvalid)
	case "VALID", "1", "TRUE":
		// แถวเก่าที่ยังไม่มีค่าในคอลัมน์นี้ ถือว่าเป็นรายการที่ทำสำเร็จ = ถูกต้อง
		q = q.Where("data_status IS NULL OR data_status <> ?", models.AuditStatusInvalid)
	}

	if v := strings.TrimSpace(c.Query("source_table")); v != "" {
		q = q.Where("source_table = ?", strings.ToUpper(v))
	}
	if v := strings.TrimSpace(c.Query("machine_no")); v != "" {
		q = q.Where("machine_no = ?", strings.ToUpper(v))
	}

	limit := 500
	if n, err := strconv.Atoi(strings.TrimSpace(c.Query("limit"))); err == nil && n > 0 && n <= 5000 {
		limit = n
	}

	var logs []models.AuditLog
	q.Limit(limit).Find(&logs)

	rows := make([]AuditLogRow, 0, len(logs))
	for _, l := range logs {
		rows = append(rows, AuditLogRow{
			AuditLog:    l,
			StatusLabel: auditStatusLabel(l),
			Summary:     auditSummary(l),
		})
	}

	c.JSON(200, rows)
}

// auditStatusLabel: ป้ายภาษาไทยของคอลัมน์ DataStatus
func auditStatusLabel(l models.AuditLog) string {
	if l.Invalid() {
		return "ไม่ถูกต้อง"
	}
	return "ถูกต้อง"
}

// auditSummary: สรุปแถวนั้นเป็นประโยคเดียว อ่านแล้วเข้าใจทันที
func auditSummary(l models.AuditLog) string {
	if !l.Invalid() {
		if s := strings.TrimSpace(l.ResultStatus); s != "" {
			return s
		}
		return l.Action
	}

	parts := make([]string, 0, 4)

	field := strings.TrimSpace(l.FieldName)
	if field == "" {
		field = "ข้อมูลที่สแกน"
	}

	if v := strings.TrimSpace(l.WrongValue); v != "" {
		parts = append(parts, field+" สแกนได้ "+v)
	} else {
		parts = append(parts, field+" ไม่ถูกต้อง")
	}

	if v := strings.TrimSpace(l.CorrectValue); v != "" {
		parts = append(parts, "ที่ถูกต้องคือ "+v)
	}

	out := strings.Join(parts, " — ")
	if r := strings.TrimSpace(l.Reason); r != "" {
		out += " (" + r + ")"
	}
	return out
}

// CreateAuditLog: บันทึกรายการที่ทำสำเร็จ — ข้อมูลถูกต้อง
//
// ตัวแปรเหมือนเดิมทุกอย่าง ที่เรียกใช้อยู่แล้วทั้งระบบไม่ต้องแก้
func CreateAuditLog(sourceTable string, sourceID uint, action string, resultStatus string, userID uint, name string) {

	writeAuditLog(models.AuditLog{
		SourceTable:  sourceTable,
		SourceID:     sourceID,
		Action:       action,
		ResultStatus: resultStatus,
		DataStatus:   models.AuditStatusValid,
		UserID:       userID,
		Name:         name,
	})
}

// InvalidData = ข้อมูลที่ไม่ถูกต้องหนึ่งรายการ (ที่จะเก็บไว้ตรวจย้อนหลัง)
type InvalidData struct {
	// SourceTable = ขั้นตอนไหน
	//   WH_FLOW_ISSUE    = WH จ่ายของ
	//   MFG_FLOW_ISSUE   = MFG สแกน QR บน Kanban
	//   MFG_FLOW_CONFIRM = MFG ยืนยันการประกอบ
	//   MFG_ASSEMBLY     = สแกนประกอบ (ขั้นตอนเดิม)
	SourceTable string
	SourceID    uint   // id ของแถวที่เกี่ยวข้อง (ไม่มีก็ใส่ 0)
	Action      string // ทำอะไรอยู่ตอนผิด เช่น issue_invalid, confirm_invalid

	MachineNo string // เครื่องที่กำลังทำ
	Component string // ชนิดของ เช่น ITC, EN, CW

	Field        string // ผิดที่ช่องไหน เช่น "P/N", "S/N#", "Product Spec"
	WrongValue   string // ค่าที่สแกน/กรอกเข้ามาจริง
	CorrectValue string // ค่าที่ถูกต้องตามแผน / master_data
	Reason       string // สาเหตุที่ผิด (ข้อความเดียวกับที่แจ้งหน้าจอ)
}

// LogInvalidData: เก็บ "ข้อมูลที่ไม่ถูกต้อง" ลงตาราง audit_logs
//
// เรียกตรงจุดที่ระบบกำลังจะตอบกลับว่าไม่ผ่าน — ระบบยังไม่รับข้อมูลเข้าตารางงานจริง
// แต่ยังเก็บร่องรอยไว้ว่าใครสแกนอะไรผิด เมื่อไหร่ และที่ถูกต้องคืออะไร
func LogInvalidData(c *gin.Context, in InvalidData) {

	userID, name := lookupUserName(c)

	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "invalid"
	}

	entry := models.AuditLog{
		SourceTable:  strings.TrimSpace(in.SourceTable),
		SourceID:     in.SourceID,
		Action:       action,
		DataStatus:   models.AuditStatusInvalid,
		MachineNo:    strings.ToUpper(strings.TrimSpace(in.MachineNo)),
		Component:    strings.ToUpper(strings.TrimSpace(in.Component)),
		FieldName:    strings.TrimSpace(in.Field),
		WrongValue:   strings.TrimSpace(in.WrongValue),
		CorrectValue: strings.TrimSpace(in.CorrectValue),
		Reason:       strings.TrimSpace(in.Reason),
		UserID:       userID,
		Name:         name,
	}

	entry.ResultStatus = invalidResultStatus(entry)

	writeAuditLog(entry)
}

// invalidResultStatus: ข้อความสั้น ๆ ของคอลัมน์ result_status สำหรับแถวที่ไม่ถูกต้อง
// รูปแบบ "<เครื่อง>/<ชนิดของ> ไม่ถูกต้อง"
func invalidResultStatus(l models.AuditLog) string {
	head := l.MachineNo
	if l.Component != "" {
		if head == "" {
			head = l.Component
		} else {
			head += "/" + l.Component
		}
	}
	if head == "" {
		return "ข้อมูลไม่ถูกต้อง"
	}
	return head + " ไม่ถูกต้อง"
}

// writeAuditLog: ตัดข้อความยาวเกินช่อง แล้วบันทึกลงตาราง
func writeAuditLog(entry models.AuditLog) {

	if entry.ActionDatetime.IsZero() {
		entry.ActionDatetime = time.Now()
	}
	if entry.DataStatus == "" {
		entry.DataStatus = models.AuditStatusValid
	}

	entry.FieldName = cutAuditText(entry.FieldName, 64)
	entry.WrongValue = cutAuditText(entry.WrongValue, 255)
	entry.CorrectValue = cutAuditText(entry.CorrectValue, 255)
	entry.Reason = cutAuditText(entry.Reason, 500)

	config.DB.Create(&entry)
}

// cutAuditText: ตัดข้อความให้ไม่เกินความยาวคอลัมน์ (นับเป็นตัวอักษร รองรับภาษาไทย)
func cutAuditText(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func lookupUserName(c *gin.Context) (uint, string) {

	rawID, _ := c.Get("user_id")
	rawUsername, _ := c.Get("username")

	var userID uint
	switch v := rawID.(type) {
	case float64:
		userID = uint(v)
	case uint:
		userID = v
	}

	username, _ := rawUsername.(string)

	var user models.User
	if err := config.DB.First(&user, userID).Error; err == nil {
		return userID, user.Name
	}

	return userID, username
}