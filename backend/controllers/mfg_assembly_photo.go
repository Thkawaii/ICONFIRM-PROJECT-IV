package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

func UploadMFGAssemblyPhoto(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.MFGAssembly
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการสแกนนี้"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": "กรุณาแนบไฟล์รูปภาพ (field name: file)"})
		return
	}

	if err := os.MkdirAll("./uploads", 0755); err != nil {
		c.JSON(500, gin.H{"message": "สร้างโฟลเดอร์ uploads ไม่สำเร็จ"})
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".png" && ext != ".webp" && ext != ".jpg" && ext != ".jpeg" {
		ext = ".jpg"
	}
	safeName := fmt.Sprintf("mfg_%d_%d%s", row.ID, time.Now().UnixNano(), ext)
	dest := filepath.Join("uploads", safeName)

	if err := c.SaveUploadedFile(fileHeader, dest); err != nil {
		c.JSON(500, gin.H{"message": "บันทึกไฟล์ไม่สำเร็จ"})
		return
	}

	row.PhotoURL = "/uploads/" + safeName
	row.UpdatedDatetime = time.Now()

	if err := config.DB.Save(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	CreateAuditLog("MFG_ASSEMBLY_PHOTO", row.ID, "upload_photo", row.PhotoURL, userID, name)

	c.JSON(200, gin.H{
		"row":     row,
		"saved":   true,
		"message": "บันทึกรูปถ่ายแล้ว",
	})
}
