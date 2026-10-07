package controllers

import (
	"strconv"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// bulkDeleteRequest: ลบหลายรายการในครั้งเดียว (จากการติ๊กเลือกในตาราง)
type bulkDeleteRequest struct {
	IDs []uint `json:"ids"`
}

// maxBulkDelete: กันการส่ง id มามากผิดปกติในคำขอเดียว
const maxBulkDelete = 50000

func uniqueIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func bindBulkDelete(c *gin.Context) ([]uint, bool) {
	var req bulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return nil, false
	}
	ids := uniqueIDs(req.IDs)
	if len(ids) == 0 {
		c.JSON(400, gin.H{"message": "กรุณาเลือกรายการที่ต้องการลบก่อน"})
		return nil, false
	}
	if len(ids) > maxBulkDelete {
		c.JSON(400, gin.H{"message": "เลือกรายการมากเกินไปในครั้งเดียว"})
		return nil, false
	}
	return ids, true
}

func deleteByIDs(model interface{}, ids []uint) (int64, error) {
	var affected int64
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, part := range chunkSlice(ids, dbInListChunk) {
			res := tx.Where("id IN ?", part).Delete(model)
			if res.Error != nil {
				return res.Error
			}
			affected += res.RowsAffected
		}
		return nil
	})
	return affected, err
}

// BulkDeleteImportLicenseItems: POST /import-license/bulk-delete  {ids:[...]}
// แถวที่สแกนผ่านแล้ว (ล็อก) จะถูกข้าม ไม่ลบ และแจ้งกลับใน skipped
func BulkDeleteImportLicenseItems(c *gin.Context) {
	ids, ok := bindBulkDelete(c)
	if !ok {
		return
	}

	var rows []models.LicenseItem
	for _, part := range chunkSlice(ids, dbInListChunk) {
		var chunk []models.LicenseItem
		if err := config.DB.Where("id IN ?", part).Find(&chunk).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		rows = append(rows, chunk...)
	}
	if len(rows) == 0 {
		c.JSON(404, gin.H{"message": "ไม่พบรายการที่ต้องการลบ"})
		return
	}

	lockIdx := buildScanLockIndex()
	deletable := make([]uint, 0, len(rows))
	skipped := make([]gin.H, 0)
	for i := range rows {
		if locked, reason := lockIdx.importLicense(&rows[i]); locked {
			skipped = append(skipped, gin.H{"id": rows[i].ID, "machineNo": rows[i].MachineNo, "reason": reason})
			continue
		}
		deletable = append(deletable, rows[i].ID)
	}

	var deleted int64
	if len(deletable) > 0 {
		n, err := deleteByIDs(&models.LicenseItem{}, deletable)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		deleted = n
		ResetIdentityIfEmpty(config.DB, &models.LicenseItem{})
		userID, userName := lookupUserName(c)
		CreateAuditLog("IMPORT_LICENSE", 0, "bulk_delete", strconv.FormatInt(deleted, 10)+" รายการ", userID, userName)
	}

	c.JSON(200, gin.H{
		"deleted": deleted,
		"skipped": skipped,
	})
}

