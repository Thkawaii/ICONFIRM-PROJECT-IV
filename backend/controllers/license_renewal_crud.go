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
// แก้ไข / ลบแถวทะเบียนใบอนุญาต (ชีตต่ออายุ) จากในตารางโดยตรง
//
// ตารางนี้ซิงก์กับไฟล์ Excel — การอัปโหลดไฟล์ชื่อเดิมจะเขียนทับแถวนั้นตามไฟล์
// การแก้ในระบบจึงเป็นการแก้ชั่วคราว มีผลจนกว่าจะอัปไฟล์รอบถัดไป
//
// ไม่มีการ "เพิ่มใบอนุญาต" จากในระบบแล้ว เพราะแถวที่ไฟล์ไม่รู้จักจะรอดจากการล้าง
// แล้วชนกับแถวของไฟล์เองในรอบถัดไป กลายเป็นสองแถวเลขใบเดียวกัน
// ---------------------------------------------------------------------------

type licenseRenewalInput struct {
	GroupNo           *string `json:"groupNo"`
	ITControllerModel *string `json:"itControllerModel"`
	ImportLicenseNo   *string `json:"importLicenseNo"`
	ExportLicenseNo   *string `json:"exportLicenseNo"`
	Country           *string `json:"country"`
	Note              *string `json:"note"`

	Total  *string `json:"total"`
	Stock  *string `json:"stock"`
	Remain *string `json:"remain"`

	IssueDate    *string `json:"issueDate"`
	ExpireDate   *string `json:"expireDate"`
	EmailDate    *string `json:"emailDate"`
	PaymentDate  *string `json:"paymentDate"`
	ReceivedDate *string `json:"receivedDate"`
}

// applyRenewalInput: เขียนค่าที่ส่งมาลงแถว — ฟิลด์ที่ไม่ได้ส่งมาจะไม่ถูกแตะ
func applyRenewalInput(row *models.LicenseRenewal, in licenseRenewalInput) error {
	text := func(dst *string, src *string, max int) {
		if src != nil {
			*dst = clampRunes(strings.TrimSpace(*src), max)
		}
	}
	text(&row.GroupNo, in.GroupNo, 50)
	text(&row.Country, in.Country, 120)
	text(&row.Note, in.Note, 4000)

	if in.ITControllerModel != nil {
		row.ITControllerModel = clampRunes(strings.ToUpper(strings.TrimSpace(*in.ITControllerModel)), 60)
	}
	if in.ImportLicenseNo != nil {
		row.ImportLicenseNo = clampRunes(strings.ToUpper(strings.TrimSpace(*in.ImportLicenseNo)), renewalLicenseNoMaxLen)
	}
	if in.ExportLicenseNo != nil {
		row.ExportLicenseNo = clampRunes(strings.ToUpper(strings.TrimSpace(*in.ExportLicenseNo)), renewalLicenseNoMaxLen)
	}

	if in.Total != nil {
		row.Total = renewalInt(*in.Total)
	}
	if in.Stock != nil {
		row.Stock = renewalInt(*in.Stock)
	}
	// REMAIN ต้องแยกว่า "กรอก 0" กับ "ไม่ได้กรอก" เพราะ 0 แปลว่านำออกหมดแล้ว
	if in.Remain != nil {
		row.Remain, row.HasRemain = renewalIntValue(*in.Remain)
	}

	dates := []struct {
		dst **time.Time
		src *string
		lbl string
	}{
		{&row.IssueDate, in.IssueDate, "วันที่ออกใบอนุญาต"},
		{&row.ExpireDate, in.ExpireDate, "วันหมดอายุ"},
		{&row.EmailDate, in.EmailDate, "วันที่ส่งอีเมล"},
		{&row.PaymentDate, in.PaymentDate, "วันที่จ่ายเงิน"},
		{&row.ReceivedDate, in.ReceivedDate, "วันที่รับเอกสาร"},
	}
	for _, d := range dates {
		if d.src == nil {
			continue
		}
		v := strings.TrimSpace(*d.src)
		if v == "" {
			*d.dst = nil
			continue
		}
		parsed := parseLicenseDate(v)
		if parsed == nil {
			return &renewalInputError{"รูปแบบ" + d.lbl + "ไม่ถูกต้อง: " + v}
		}
		*d.dst = parsed
	}
	return nil
}

type renewalInputError struct{ msg string }

func (e *renewalInputError) Error() string { return e.msg }

// UpdateLicenseRenewal: PATCH /license-renewal/:id
func UpdateLicenseRenewal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.LicenseRenewal
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบแถวนี้"})
		return
	}

	var in licenseRenewalInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลที่ส่งมาไม่ถูกต้อง"})
		return
	}
	if err := applyRenewalInput(&row, in); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	// ไม่ติดธง ManualEntry อีกแล้ว
	//
	// ของเดิมติดธงไว้เพื่อกันแถวที่แก้ไม่ให้หายตอนอัปไฟล์ทับ แต่ผลข้างเคียงคือ
	// แถวนั้นรอดจากการล้าง แล้วไฟล์ก็ใส่แถวของตัวเองกลับมาอีกใบ กลายเป็นสองแถวเลขใบเดียวกัน
	// ซ้ำร้ายแถวที่ติดธงถูกดันไปท้ายสุด ระบบจึงถือว่ามันคือใบปัจจุบันของโซ่
	// ทั้งที่เป็นค่าเก่ากว่าไฟล์ ยอดคงเหลือและสถานะเลยเพี้ยนและทับถมทุกครั้งที่แก้
	//
	// การแก้ในระบบจะถูกเขียนทับเมื่ออัปไฟล์ชื่อเดิมที่มีแถวนี้อยู่ (ไฟล์ชนะ)

	if err := config.DB.Save(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	CreateAuditLog("LICENSE_RENEWAL", row.ID, "update", row.ImportLicenseNo, userID, name)

	rows := []models.LicenseRenewal{row}
	enrichLicenseRenewals(rows)
	c.JSON(200, gin.H{"row": rows[0], "message": "บันทึกแล้ว"})
}

// DeleteLicenseRenewal: DELETE /license-renewal/:id
func DeleteLicenseRenewal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.LicenseRenewal
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบแถวนี้"})
		return
	}
	if err := config.DB.Delete(&models.LicenseRenewal{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	CreateAuditLog("LICENSE_RENEWAL", uint(id), "delete", row.ImportLicenseNo, userID, name)

	c.JSON(200, gin.H{"message": "ลบแถวแล้ว"})
}
