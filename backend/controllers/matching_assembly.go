package controllers

import (
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

func GetMatchingAssemblies(c *gin.Context) {

	var rows []models.MatchingAssembly
	config.DB.Order("id asc").Find(&rows)
	c.JSON(200, rows)
}

type MatchingAssemblyRequest struct {
	MachineNo         string `json:"machineNo"`
	ITControllerSN    string `json:"itControllerSN"`
	Country           string `json:"country"`
	Classification    string `json:"classification"`
	AssemblyPartsNo   string `json:"assemblyPartsNo"`
	AssemblyPartsName string `json:"assemblyPartsName"`
}

func CreateMatchingAssembly(c *gin.Context) {

	var req MatchingAssemblyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	now := time.Now()

	row := models.MatchingAssembly{
		MachineNo:         strings.TrimSpace(req.MachineNo),
		ITControllerSN:    strings.TrimSpace(req.ITControllerSN),
		Country:           strings.TrimSpace(req.Country),
		Classification:    strings.TrimSpace(req.Classification),
		AssemblyPartsNo:   strings.TrimSpace(req.AssemblyPartsNo),
		AssemblyPartsName: strings.TrimSpace(req.AssemblyPartsName),
		CreatedBy:         name,
		CreatedDatetime:   now,
		UpdatedDatetime:   now,
		UserID:            userID,
	}

	if err := config.DB.Create(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	CreateAuditLog("MATCHING_ASSEMBLY", row.ID, "create", row.MachineNo, userID, name)
	c.JSON(201, row)
}

func UpdateMatchingAssembly(c *gin.Context) {

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.MatchingAssembly
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	var req MatchingAssemblyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"machine_no":          strings.TrimSpace(req.MachineNo),
		"it_controller_sn":    strings.TrimSpace(req.ITControllerSN),
		"country":             strings.TrimSpace(req.Country),
		"classification":      strings.TrimSpace(req.Classification),
		"assembly_parts_no":   strings.TrimSpace(req.AssemblyPartsNo),
		"assembly_parts_name": strings.TrimSpace(req.AssemblyPartsName),
		"updated_datetime":    time.Now(),
	}

	if err := config.DB.Model(&row).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	config.DB.First(&row, id)

	userID, name := lookupUserName(c)
	CreateAuditLog("MATCHING_ASSEMBLY", row.ID, "edit", row.MachineNo, userID, name)
	c.JSON(200, row)
}

func DeleteMatchingAssembly(c *gin.Context) {

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.MatchingAssembly
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	if err := config.DB.Delete(&models.MatchingAssembly{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	CreateAuditLog("MATCHING_ASSEMBLY", row.ID, "delete", row.MachineNo, userID, name)
	c.JSON(200, gin.H{"deleted": true})
}

func upsertMatchingAssemblyFromScan(machineNo, serialNo, partsNo, partsName, country string, when time.Time, userID uint, name string) {

	machineNo = strings.TrimSpace(machineNo)
	if machineNo == "" {
		return
	}

	serialNo = strings.TrimSpace(serialNo)
	partsNo = strings.TrimSpace(partsNo)
	partsName = strings.TrimSpace(partsName)
	country = strings.TrimSpace(country)

	var existing models.MatchingAssembly
	if err := config.DB.Where("machine_no = ?", machineNo).First(&existing).Error; err == nil {
		updates := map[string]interface{}{}
		if strings.TrimSpace(existing.Country) == "" && country != "" {
			updates["country"] = country
		}
		if strings.TrimSpace(existing.ITControllerSN) == "" && serialNo != "" {
			updates["it_controller_sn"] = serialNo
		}
		if strings.TrimSpace(existing.AssemblyPartsName) == "" && partsName != "" {
			updates["assembly_parts_name"] = partsName
		}
		if len(updates) > 0 {
			updates["updated_datetime"] = time.Now()
			config.DB.Model(&existing).Updates(updates)
		}
		return
	}

	row := models.MatchingAssembly{
		MachineNo:         machineNo,
		ITControllerSN:    serialNo,
		Country:           country,
		Classification:    "",
		AssemblyPartsNo:   partsNo,
		AssemblyPartsName: partsName,
		CreatedBy:         name,
		CreatedDatetime:   time.Now(),
		UpdatedDatetime:   time.Now(),
		UserID:            userID,
	}

	if err := config.DB.Create(&row).Error; err == nil {
		CreateAuditLog("MATCHING_ASSEMBLY", row.ID, "scan_create", machineNo, userID, name)
	}
}
