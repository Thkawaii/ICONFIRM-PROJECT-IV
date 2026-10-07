package controllers

import (
	"strconv"
	"strings"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// ตั้งวันที่ออกใบอนุญาตให้ทั้งใบ
//
// ไฟล์บัญชีหมายเลขเครื่องที่ใช้งานจริงหลายไฟล์ไม่มีคอลัมน์วันที่ออกใบอนุญาตเลย
// (เช่น ไฟล์ "บัญชีแสดงหมายเลขเครื่อง..." มี 11 คอลัมน์ ไม่มีช่องวันที่สักช่อง)
// เมื่อไม่มีวันที่ ระบบก็คำนวณวันหมดอายุไม่ได้ และไม่มีอะไรให้แจ้งเตือน
//
// จึงเปิดให้กรอกวันที่ออกใบได้จากหน้าเว็บ กรอกครั้งเดียวต่อ 1 ใบอนุญาต
// ระบบจะเติมให้ทุกเครื่องในใบนั้น แล้วคิดวันหมดอายุตามกติกาเดิม
// (นำเข้า + 6 เดือน, นำออก + 1 เดือน) การแจ้งเตือนจึงทำงานต่อได้ทันที
// ---------------------------------------------------------------------------

type setIssueDateRequest struct {
	Type      string `json:"type"`
	LicenseNo string `json:"licenseNo"`
	IssueDate string `json:"issueDate"`
}

// SetLicenseIssueDate: PATCH /license-overview/issue-date
func SetLicenseIssueDate(c *gin.Context) {
	var req setIssueDateRequest
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

	issue := parseLicenseDate(strings.TrimSpace(req.IssueDate))
	if issue == nil {
		c.JSON(400, gin.H{"message": "รูปแบบวันที่ไม่ถูกต้อง"})
		return
	}

	// เทียบเลขใบแบบ normalize เพราะไฟล์แต่ละชุดเว้นวรรค/ขีดไม่เหมือนกัน
	want := NormalizeCodeValue(licenseNo)
	if want == "" {
		c.JSON(400, gin.H{"message": "เลขใบอนุญาตไม่ถูกต้อง"})
		return
	}

	updated := 0

	switch licenseType {
	case models.LicenseTypeImport:
		var rows []models.LicenseItem
		if err := config.DB.Find(&rows).Error; err != nil {
			c.JSON(500, gin.H{"message": "อ่านข้อมูลไม่สำเร็จ: " + err.Error()})
			return
		}
		var ids []uint
		for i := range rows {
			if NormalizeCodeValue(rows[i].LicenseNo) == want {
				ids = append(ids, rows[i].ID)
			}
		}
		if len(ids) == 0 {
			c.JSON(404, gin.H{"message": "ไม่พบรายการเครื่องของใบอนุญาตนี้"})
			return
		}

		probe := models.LicenseItem{IssueDate: issue}
		probe.FillExpireDate()
		if err := updateByIDs(&models.LicenseItem{}, ids, map[string]interface{}{
			"issue_date":  issue,
			"expire_date": probe.ExpireDate,
		}); err != nil {
			c.JSON(500, gin.H{"message": "บันทึกไม่สำเร็จ: " + err.Error()})
			return
		}
		updated = len(ids)
		InvalidateMachineIndex()

	case models.LicenseTypeExport:
		// ใบนำออกไม่มีตารางของตัวเอง — วันที่ออกใบเก็บอยู่ในคอลัมน์ export_issue_date
		// ของทะเบียนเครื่อง (เครื่องหนึ่งแถวรู้ทั้งใบนำเข้าและใบนำออกของตัวเอง)
		var rows []models.LicenseItem
		if err := config.DB.Find(&rows).Error; err != nil {
			c.JSON(500, gin.H{"message": "อ่านข้อมูลไม่สำเร็จ: " + err.Error()})
			return
		}
		var ids []uint
		for i := range rows {
			if NormalizeCodeValue(rows[i].ExportLicenseNo) == want {
				ids = append(ids, rows[i].ID)
			}
		}
		if len(ids) == 0 {
			c.JSON(404, gin.H{"message": "ไม่พบรายการเครื่องของใบอนุญาตนี้"})
			return
		}

		// วันหมดอายุของใบนำออกคำนวณจาก export_issue_date ตอนอ่าน จึงไม่ต้องเก็บซ้ำ
		if err := updateByIDs(&models.LicenseItem{}, ids, map[string]interface{}{
			"export_issue_date": issue,
		}); err != nil {
			c.JSON(500, gin.H{"message": "บันทึกไม่สำเร็จ: " + err.Error()})
			return
		}
		updated = len(ids)
	}

	userID, userName := lookupUserName(c)
	source := "IMPORT_LICENSE"
	if licenseType == models.LicenseTypeExport {
		source = "EXPORT_LICENSE"
	}
	CreateAuditLog(source, 0, "set_issue_date",
		strings.ToUpper(licenseNo)+": "+issue.Format("2006-01-02")+" ("+strconv.Itoa(updated)+" เครื่อง)",
		userID, userName)

	c.JSON(200, gin.H{
		"updated": updated,
		"message": "บันทึกวันที่ออกใบอนุญาตแล้ว — ระบบคำนวณวันหมดอายุให้ " + strconv.Itoa(updated) + " เครื่อง",
	})
}

// updateByIDs: อัปเดตหลายแถวเป็นชุด ๆ กันคำสั่ง SQL ยาวเกินขีดจำกัดของฐานข้อมูล
func updateByIDs(model interface{}, ids []uint, values map[string]interface{}) error {
	for _, chunk := range chunkSlice(ids, dbInListChunk) {
		if err := config.DB.Model(model).Where("id IN ?", chunk).Updates(values).Error; err != nil {
			return err
		}
	}
	return nil
}
