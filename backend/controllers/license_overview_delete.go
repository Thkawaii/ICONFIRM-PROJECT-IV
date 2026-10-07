package controllers

import (
	"strconv"
	"strings"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// ลบใบอนุญาต 1 รายการจากหน้า License Overview
//
// 1 แถวในหน้านั้น = ใบอนุญาต 1 โซ่ (ใบต้นฉบับ → ต่ออายุ → ใบปัจจุบัน)
// การลบจึงต้องเก็บกวาดให้ครบทั้งโซ่ ไม่งั้นแถวเดิมจะโผล่กลับมาใหม่
// เพราะระบบสร้างแถวภาพรวมจากทั้งไฟล์ License และตารางประวัติการต่ออายุ
//
//	1. รายการเครื่องในไฟล์ Import / Export ของทุกเลขใบในโซ่
//	2. ประวัติการต่ออายุทุกครั้งในโซ่เดียวกัน
//	3. แถวในทะเบียนใบอนุญาต (ชีตต่ออายุ) ที่อ้างถึงเลขใบเหล่านั้น
//
// แถวที่สแกนผ่านไปแล้ว (locked) จะไม่ถูกลบ และรายงานกลับใน skipped
// ---------------------------------------------------------------------------

// licenseChainNumbers: เลขใบทั้งหมดในโซ่เดียวกัน (normalize แล้ว) สำหรับใช้เทียบ
func licenseChainNumbers(licenseType, licenseNo string) map[string]bool {
	// ใช้โซ่ชุดเดียวกับหน้าภาพรวม ไม่งั้นการลบจะเก็บกวาดไม่ครบโซ่
	history := loadRenewalHistoryWithLedger()
	idx := buildRenewalLinkIndex(history)
	chain := buildLicenseChainWith(idx, licenseType, licenseNo)

	wanted := map[string]bool{}
	add := func(v string) {
		if n := NormalizeCodeValue(v); n != "" {
			wanted[n] = true
		}
	}
	add(licenseNo)
	add(chain.OriginalNo)
	add(chain.CurrentNo)
	for _, p := range chain.PreviousNos {
		add(p)
	}
	for _, s := range chain.Steps {
		add(s.OldLicenseNo)
		add(s.NewLicenseNo)
	}
	return wanted
}

// ---------------------------------------------------------------------------
// ล้างข้อมูลใบอนุญาตทั้งหมด
//
// ใช้ตอนอยากเริ่มใหม่จากไฟล์ชุดใหม่ ไม่ต้องไล่ลบทีละใบ
// แยก scope ให้เลือกได้ เพราะบางครั้งอยากล้างเฉพาะฝั่งเดียว
// ---------------------------------------------------------------------------

const (
	licenseClearImport  = "import"
	licenseClearExport  = "export"
	licenseClearRenewal = "renewal"
	licenseClearAll     = "all"
)

// ClearLicenseOverview: DELETE /license-overview/all?scope=import|export|renewal|all
func ClearLicenseOverview(c *gin.Context) {
	scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	switch scope {
	case licenseClearImport, licenseClearExport, licenseClearRenewal, licenseClearAll:
	default:
		c.JSON(400, gin.H{"message": "scope ต้องเป็น import, export, renewal หรือ all"})
		return
	}

	wantImport := scope == licenseClearImport || scope == licenseClearAll
	wantExport := scope == licenseClearExport || scope == licenseClearAll
	wantRenewal := scope == licenseClearRenewal || scope == licenseClearAll

	counts := gin.H{}
	var total int64

	wipe := func(model interface{}) (int64, error) {
		res := config.DB.Where("1 = 1").Delete(model)
		return res.RowsAffected, res.Error
	}

	// ประวัติการต่ออายุของฝั่งนั้นต้องไปด้วย ไม่งั้นแถวเดิมจะยังอยู่ในตาราง
	// เพราะระบบสร้างแถวจากประวัติการต่ออายุได้เองแม้ไม่มีรายการเครื่องแล้ว
	wipeHistoryOfType := func(licenseType string) (int64, error) {
		res := config.DB.Where("UPPER(license_type) = ?", licenseType).
			Delete(&models.LicenseRenewalHistory{})
		return res.RowsAffected, res.Error
	}

	if wantImport {
		n, err := wipe(&models.LicenseItem{})
		if err != nil {
			c.JSON(500, gin.H{"message": "ลบใบนำเข้าไม่สำเร็จ: " + err.Error()})
			return
		}
		ResetIdentityIfEmpty(config.DB, &models.LicenseItem{})
		InvalidateMachineIndex()
		counts["import"] = n
		total += n

		if !wantRenewal {
			h, err := wipeHistoryOfType(models.LicenseTypeImport)
			if err != nil {
				c.JSON(500, gin.H{"message": "ลบประวัติการต่ออายุฝั่งนำเข้าไม่สำเร็จ: " + err.Error()})
				return
			}
			counts["importHistory"] = h
			total += h
		}
	}

	if wantExport {
		// ใบนำออกไม่มีตารางของตัวเอง — เครื่องเดียวกันเป็นของใบนำเข้าด้วย
		// จึงล้างเฉพาะคอลัมน์ฝั่งนำออก ถ้าลบทั้งแถวจะพาใบนำเข้าหายไปด้วย
		res := config.DB.Model(&models.LicenseItem{}).
			Where("export_license_no IS NOT NULL AND export_license_no <> ''").
			Updates(clearExportColumns())
		if res.Error != nil {
			c.JSON(500, gin.H{"message": "ลบใบนำออกไม่สำเร็จ: " + res.Error.Error()})
			return
		}
		n := res.RowsAffected
		counts["export"] = n
		total += n

		if !wantRenewal {
			h, err := wipeHistoryOfType(models.LicenseTypeExport)
			if err != nil {
				c.JSON(500, gin.H{"message": "ลบประวัติการต่ออายุฝั่งนำออกไม่สำเร็จ: " + err.Error()})
				return
			}
			counts["exportHistory"] = h
			total += h
		}
	}

	if wantRenewal {
		// ประวัติการต่ออายุกับตารางทะเบียนมาจากไฟล์เดียวกัน จึงล้างคู่กันเสมอ
		// ถ้าล้างแค่ตัวใดตัวหนึ่ง ข้อมูลสองฝั่งจะไม่ตรงกัน
		n, err := wipe(&models.LicenseRenewalHistory{})
		if err != nil {
			c.JSON(500, gin.H{"message": "ลบประวัติการต่ออายุไม่สำเร็จ: " + err.Error()})
			return
		}
		ResetIdentityIfEmpty(config.DB, &models.LicenseRenewalHistory{})
		counts["renewalHistory"] = n
		total += n

		m, err := wipe(&models.LicenseRenewal{})
		if err != nil {
			c.JSON(500, gin.H{"message": "ลบทะเบียนใบอนุญาตไม่สำเร็จ: " + err.Error()})
			return
		}
		ResetIdentityIfEmpty(config.DB, &models.LicenseRenewal{})
		counts["renewalLedger"] = m
		total += m
	}

	userID, userName := lookupUserName(c)
	if wantImport {
		CreateAuditLog("IMPORT_LICENSE", 0, "clear_all", "ล้างทั้งหมดจากหน้าใบอนุญาต", userID, userName)
	}
	if wantExport {
		CreateAuditLog("EXPORT_LICENSE", 0, "clear_all", "ล้างทั้งหมดจากหน้าใบอนุญาต", userID, userName)
	}
	if wantRenewal {
		CreateAuditLog("LICENSE_RENEWAL_HISTORY", 0, "clear_all", "ล้างทั้งหมดจากหน้าใบอนุญาต", userID, userName)
		CreateAuditLog("LICENSE_RENEWAL", 0, "clear_all", "ล้างทั้งหมดจากหน้าใบอนุญาต", userID, userName)
	}

	// แถวที่ยังเหลือในตารางหลังล้าง — ส่วนใหญ่มาจากตารางทะเบียน (ชีตต่ออายุ)
	// ที่ยังไม่ได้ล้าง ต้องบอกให้ผู้ใช้รู้ ไม่ใช่ปล่อยให้งงว่าทำไมลบแล้วไม่หาย
	remaining := len(buildLicenseOverview())

	msg := "ล้างข้อมูลแล้ว " + strconv.FormatInt(total, 10) + " รายการ"
	if remaining > 0 {
		msg += " · ยังเหลือในตาราง " + strconv.Itoa(remaining) + " ใบ"
	}

	c.JSON(200, gin.H{
		"scope":     scope,
		"deleted":   total,
		"counts":    counts,
		"remaining": remaining,
		"sources":   licenseDataSourceCounts(),
		"message":   msg,
	})
}

// DeleteLicenseOverviewEntry: DELETE /license-overview?type=EXPORT&licenseNo=EXP-004
func DeleteLicenseOverviewEntry(c *gin.Context) {
	licenseType := models.NormalizeLicenseType(c.Query("type"))
	licenseNo := strings.TrimSpace(c.Query("licenseNo"))

	if licenseType == "" || licenseNo == "" {
		c.JSON(400, gin.H{"message": "กรุณาระบุ type (Import/Export) และ licenseNo"})
		return
	}

	wanted := licenseChainNumbers(licenseType, licenseNo)
	if len(wanted) == 0 {
		c.JSON(400, gin.H{"message": "เลขใบอนุญาตไม่ถูกต้อง"})
		return
	}
	hit := func(v string) bool { return wanted[NormalizeCodeValue(v)] }

	var (
		itemIDs []uint
		skipped = make([]gin.H, 0)
	)

	// --- 1. รายการเครื่องในไฟล์ License ---
	switch licenseType {
	case models.LicenseTypeImport:
		var rows []models.LicenseItem
		if err := config.DB.Find(&rows).Error; err != nil {
			c.JSON(500, gin.H{"message": "อ่านข้อมูลเดิมไม่สำเร็จ: " + err.Error()})
			return
		}
		locks := buildScanLockIndex()
		for i := range rows {
			if !hit(rows[i].LicenseNo) {
				continue
			}
			if locked, reason := locks.importLicense(&rows[i]); locked {
				skipped = append(skipped, gin.H{
					"machineNo": rows[i].MachineNo,
					"reason":    reason,
				})
				continue
			}
			itemIDs = append(itemIDs, rows[i].ID)
		}
	case models.LicenseTypeExport:
		var rows []models.LicenseItem
		if err := config.DB.Find(&rows).Error; err != nil {
			c.JSON(500, gin.H{"message": "อ่านข้อมูลเดิมไม่สำเร็จ: " + err.Error()})
			return
		}
		for i := range rows {
			if hit(rows[i].ExportLicenseNo) {
				itemIDs = append(itemIDs, rows[i].ID)
			}
		}
	}

	var deletedItems int64
	if len(itemIDs) > 0 {
		var err error
		switch licenseType {
		case models.LicenseTypeImport:
			deletedItems, err = deleteByIDs(&models.LicenseItem{}, itemIDs)
		case models.LicenseTypeExport:
			// ล้างเฉพาะคอลัมน์ฝั่งนำออก เครื่องยังเป็นของใบนำเข้าอยู่
			res := config.DB.Model(&models.LicenseItem{}).
				Where("id IN ?", itemIDs).Updates(clearExportColumns())
			deletedItems, err = res.RowsAffected, res.Error
		}
		if err != nil {
			c.JSON(500, gin.H{"message": "ลบข้อมูลไม่สำเร็จ: " + err.Error()})
			return
		}
	}

	// --- 2. ประวัติการต่ออายุของโซ่นี้ ---
	var histIDs []uint
	for _, h := range loadRenewalHistory() {
		if strings.ToUpper(strings.TrimSpace(h.LicenseType)) != licenseType {
			continue
		}
		if hit(h.OldLicenseNo) || hit(h.NewLicenseNo) {
			histIDs = append(histIDs, h.ID)
		}
	}
	var deletedHistory int64
	if len(histIDs) > 0 {
		n, err := deleteByIDs(&models.LicenseRenewalHistory{}, histIDs)
		if err != nil {
			c.JSON(500, gin.H{"message": "ลบประวัติการต่ออายุไม่สำเร็จ: " + err.Error()})
			return
		}
		deletedHistory = n
	}

	// --- 3. ทะเบียนใบอนุญาต (ชีตต่ออายุ) ---
	var ledger []models.LicenseRenewal
	config.DB.Find(&ledger)
	var ledgerIDs []uint
	for i := range ledger {
		match := false
		switch licenseType {
		case models.LicenseTypeImport:
			match = hit(ledger[i].ImportLicenseNo)
		case models.LicenseTypeExport:
			match = hit(ledger[i].ExportLicenseNo)
		}
		if match {
			ledgerIDs = append(ledgerIDs, ledger[i].ID)
		}
	}
	var deletedLedger int64
	if len(ledgerIDs) > 0 {
		n, err := deleteByIDs(&models.LicenseRenewal{}, ledgerIDs)
		if err != nil {
			c.JSON(500, gin.H{"message": "ลบทะเบียนใบอนุญาตไม่สำเร็จ: " + err.Error()})
			return
		}
		deletedLedger = n
	}

	total := deletedItems + deletedHistory + deletedLedger
	if total == 0 && len(skipped) == 0 {
		c.JSON(404, gin.H{"message": "ไม่พบข้อมูลของใบอนุญาตนี้"})
		return
	}

	// เคลียร์ของที่ค้างอยู่หลังลบ เพื่อให้หน้าอื่นเห็นข้อมูลตรงกันทันที
	switch licenseType {
	case models.LicenseTypeImport:
		if deletedItems > 0 {
			InvalidateMachineIndex()
		}
		ResetIdentityIfEmpty(config.DB, &models.LicenseItem{})
	case models.LicenseTypeExport:
		if deletedItems > 0 {
			InvalidateMachineIndex()
		}
	}

	userID, userName := lookupUserName(c)
	source := "IMPORT_LICENSE"
	if licenseType == models.LicenseTypeExport {
		source = "EXPORT_LICENSE"
	}
	CreateAuditLog(source, 0, "delete_license",
		strings.ToUpper(licenseNo)+": รายการ "+strconv.FormatInt(deletedItems, 10)+
			" · ประวัติต่ออายุ "+strconv.FormatInt(deletedHistory, 10)+
			" · ทะเบียน "+strconv.FormatInt(deletedLedger, 10),
		userID, userName)

	c.JSON(200, gin.H{
		"licenseType":    licenseType,
		"licenseNo":      licenseNo,
		"deleted":        total,
		"deletedItems":   deletedItems,
		"deletedHistory": deletedHistory,
		"deletedLedger":  deletedLedger,
		"skipped":        skipped,
		"message": "ลบใบอนุญาต " + strings.ToUpper(licenseNo) + " แล้ว (" +
			strconv.FormatInt(total, 10) + " รายการ)",
	})
}

// clearExportColumns: ล้างข้อมูลฝั่งใบอนุญาตนำออกออกจากแถวทะเบียนเครื่อง
//
// ใช้แทนการลบแถว เพราะเครื่องแถวเดียวกันยังเป็นของใบอนุญาตนำเข้าอยู่
// ถ้าลบทั้งแถว ใบนำเข้าจะหายไปด้วยทั้งที่ผู้ใช้สั่งลบแค่ใบนำออก
func clearExportColumns() map[string]interface{} {
	return map[string]interface{}{
		"export_license_no": "",
		"export_issue_date": nil,
		"export_country":    "",
	}
}
