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
// อนุมัติปิดใบอนุญาตด้วยมือ
//
// ใบนำออกเสร็จสิ้นเองได้ทางเดียวคือ REMAIN = 0 — ของบนใบถูกนำออกหมดแล้วจริง
//
// ที่เหลือระบบไม่ตัดสินใจแทน เพราะ "หมดอายุ" กับ "งานจบ" คนละเรื่องกัน
// ของที่ ship ไปแล้วแต่ไม่ได้ต่ออายุใบ ยอดจะค้างอยู่ในระบบตลอดไป
// คนที่รู้ว่าจบแล้วคือคนหน้างาน ไม่ใช่ปฏิทิน จึงต้องมีคนกดอนุมัติปิดเอง
//
// การกดปิดไม่ไปแตะ REMAIN หรือข้อมูลเครื่องรายตัว — บันทึกแยกไว้ในตาราง
// license_manual_closes เท่านั้น ยกเลิกได้ และตรวจย้อนหลังได้ว่าใครปิดตอนไหน
// ---------------------------------------------------------------------------

type manualCloseRequest struct {
	Type      string `json:"type"`
	LicenseNo string `json:"licenseNo"`
	Note      string `json:"note"`

	// RemainAtClose = ยอดคงเหลือที่หน้าเว็บเห็นตอนกด เก็บไว้เป็นหลักฐานเฉย ๆ
	RemainAtClose int `json:"remainAtClose"`
}

const manualCloseNoteMax = 255

// CloseLicenseManually: POST /license-overview/close
func CloseLicenseManually(c *gin.Context) {
	var req manualCloseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}

	licenseType := models.NormalizeLicenseType(req.Type)
	licenseNo := strings.TrimSpace(req.LicenseNo)
	if licenseType == "" || licenseNo == "" {
		c.JSON(400, gin.H{"message": "กรุณาระบุประเภทและเลขใบอนุญาต"})
		return
	}

	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > manualCloseNoteMax {
		note = string([]rune(note)[:manualCloseNoteMax])
	}

	userID, userName := lookupUserName(c)
	now := time.Now()

	row := models.LicenseManualClose{
		LicenseType:   licenseType,
		LicenseNo:     licenseNo,
		RemainAtClose: req.RemainAtClose,
		Note:          note,
		ClosedBy:      userName,
		ClosedAt:      now,
	}

	// กดซ้ำไม่ควรพัง — ถ้ามีอยู่แล้วให้ทับด้วยคนล่าสุดที่กด
	var existing models.LicenseManualClose
	err := config.DB.Where("license_type = ? AND license_no = ?", licenseType, licenseNo).First(&existing).Error
	if err == nil {
		if err := config.DB.Model(&existing).Updates(map[string]interface{}{
			"remain_at_close": row.RemainAtClose,
			"note":            row.Note,
			"closed_by":       row.ClosedBy,
			"closed_at":       row.ClosedAt,
		}).Error; err != nil {
			c.JSON(500, gin.H{"message": "บันทึกไม่สำเร็จ: " + err.Error()})
			return
		}
	} else {
		if err := config.DB.Create(&row).Error; err != nil {
			c.JSON(500, gin.H{"message": "บันทึกไม่สำเร็จ: " + err.Error()})
			return
		}
	}

	CreateAuditLog("LICENSE_OVERVIEW", row.ID, "manual_close",
		licenseType+" "+licenseNo+" — ปิดเองทั้งที่คงเหลือ "+strconv.Itoa(req.RemainAtClose),
		userID, userName)

	c.JSON(200, gin.H{
		"message":   "ปิดใบอนุญาตเรียบร้อย",
		"closedBy":  userName,
		"closedAt":  now,
		"licenseNo": licenseNo,
	})
}

// ReopenLicenseManually: DELETE /license-overview/close?type=EXPORT&licenseNo=...
func ReopenLicenseManually(c *gin.Context) {
	licenseType := models.NormalizeLicenseType(c.Query("type"))
	licenseNo := strings.TrimSpace(c.Query("licenseNo"))
	if licenseType == "" || licenseNo == "" {
		c.JSON(400, gin.H{"message": "กรุณาระบุประเภทและเลขใบอนุญาต"})
		return
	}

	res := config.DB.Where("license_type = ? AND license_no = ?", licenseType, licenseNo).
		Delete(&models.LicenseManualClose{})
	if res.Error != nil {
		c.JSON(500, gin.H{"message": "ยกเลิกไม่สำเร็จ: " + res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(404, gin.H{"message": "ใบนี้ไม่ได้ถูกปิดด้วยการอนุมัติ"})
		return
	}

	userID, userName := lookupUserName(c)
	CreateAuditLog("LICENSE_OVERVIEW", 0, "manual_reopen",
		licenseType+" "+licenseNo+" — ยกเลิกการปิดใบ", userID, userName)

	c.JSON(200, gin.H{"message": "เปิดใบอนุญาตกลับมาแล้ว"})
}

// loadManualCloses: อ่านรายการที่ถูกกดปิดทั้งหมดมาเป็น map สำหรับหน้า Overview
//
// คืน map ว่างเมื่ออ่านไม่สำเร็จ เพื่อให้หน้า Overview ยังแสดงได้
// (เสียแค่ป้าย "ปิดโดย..." ดีกว่าหน้าพังทั้งหน้า)
func loadManualCloses() map[string]models.LicenseManualClose {
	out := map[string]models.LicenseManualClose{}
	var rows []models.LicenseManualClose
	if err := config.DB.Find(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[models.LicenseManualCloseKey(r.LicenseType, r.LicenseNo)] = r
	}
	return out
}
