package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func UploadPhoto(c *gin.Context) {

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": "กรุณาแนบไฟล์รูปภาพ (field name: file)"})
		return
	}

	if err := os.MkdirAll("./uploads", 0755); err != nil {
		c.JSON(500, gin.H{"message": "สร้างโฟลเดอร์ uploads ไม่สำเร็จ"})
		return
	}

	ext := filepath.Ext(fileHeader.Filename)
	safeName := fmt.Sprintf("%d%s", time.Now().UnixNano(), strings.ToLower(ext))
	dest := filepath.Join("uploads", safeName)

	if err := c.SaveUploadedFile(fileHeader, dest); err != nil {
		c.JSON(500, gin.H{"message": "บันทึกไฟล์ไม่สำเร็จ"})
		return
	}

	c.JSON(201, gin.H{
		"url":       "/uploads/" + safeName,
		"file_name": fileHeader.Filename,
	})
}
