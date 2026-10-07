package controllers

import (
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

func GetAuditLog(c *gin.Context) {

	var logs []models.AuditLog

	config.DB.Order("action_datetime desc").Find(&logs)

	c.JSON(200, logs)
}

func CreateAuditLog(sourceTable string, sourceID uint, action string, resultStatus string, userID uint, name string) {

	entry := models.AuditLog{
		SourceTable:    sourceTable,
		SourceID:       sourceID,
		Action:         action,
		ResultStatus:   resultStatus,
		ActionDatetime: time.Now(),
		UserID:         userID,
		Name:           name,
	}

	config.DB.Create(&entry)
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
