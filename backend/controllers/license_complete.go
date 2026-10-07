package controllers

import (
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type completeRequest struct {
	IDs       []uint `json:"ids"`
	Completed *bool  `json:"completed"`

	LicenseNo *string `json:"licenseNo"`
	InvoiceNo *string `json:"invoiceNo"`

	ExportLicenseNo *string `json:"exportLicenseNo"`
}

func (r completeRequest) wantCompleted() bool {
	if r.Completed == nil {
		return true
	}
	return *r.Completed
}

func completeFields(completed bool, userName string, now time.Time) map[string]interface{} {
	if !completed {
		return map[string]interface{}{
			"completed":    false,
			"completed_by": "",
			"completed_at": nil,
		}
	}
	return map[string]interface{}{
		"completed":    true,
		"completed_by": userName,
		"completed_at": now,
	}
}

func updateCompleteByIDs(model interface{}, ids []uint, fields map[string]interface{}) (int64, error) {
	var affected int64
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, part := range chunkSlice(ids, dbInListChunk) {
			res := tx.Model(model).Where("id IN ?", part).Updates(fields)
			if res.Error != nil {
				return res.Error
			}
			affected += res.RowsAffected
		}
		return nil
	})
	return affected, err
}

func completeActionName(completed bool) string {
	if completed {
		return "mark_complete"
	}
	return "unmark_complete"
}

func SetImportLicenseComplete(c *gin.Context) {
	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}

	completed := req.wantCompleted()
	userID, userName := lookupUserName(c)
	now := time.Now()

	tx := config.DB.Model(&models.LicenseItem{})
	target := ""
	byIDs := false

	switch {
	case len(req.IDs) > 0:
		byIDs = true
		target = strconv.Itoa(len(req.IDs)) + " รายการ"

	case req.LicenseNo != nil || req.InvoiceNo != nil:
		licenseNo := ""
		invoiceNo := ""
		if req.LicenseNo != nil {
			licenseNo = strings.TrimSpace(*req.LicenseNo)
			tx = tx.Where("license_no = ?", licenseNo)
		}
		if req.InvoiceNo != nil {
			invoiceNo = strings.TrimSpace(*req.InvoiceNo)
			tx = tx.Where("invoice_no = ?", invoiceNo)
		}
		target = "license_no=" + licenseNo + " invoice_no=" + invoiceNo

	default:
		c.JSON(400, gin.H{"message": "กรุณาเลือกรายการที่ต้องการก่อน (ส่ง ids หรือระบุใบอนุญาต)"})
		return
	}

	var (
		affected int64
		err      error
	)
	if byIDs {
		affected, err = updateCompleteByIDs(&models.LicenseItem{}, req.IDs, completeFields(completed, userName, now))
	} else {
		res := tx.Updates(completeFields(completed, userName, now))
		affected, err = res.RowsAffected, res.Error
	}
	if err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	if affected == 0 {
		c.JSON(404, gin.H{"message": "ไม่พบรายการที่ต้องการอัปเดต"})
		return
	}

	CreateAuditLog("IMPORT_LICENSE", 0, completeActionName(completed), target, userID, userName)

	c.JSON(200, gin.H{
		"updated":     affected,
		"completed":   completed,
		"completedBy": userName,
		"completedAt": now,
	})
}

