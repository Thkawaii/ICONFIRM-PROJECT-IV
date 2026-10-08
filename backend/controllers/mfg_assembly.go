package controllers

import (
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

func GetMFGAssemblies(c *gin.Context) {
	var rows []models.MFGAssembly
	config.DB.Order("id asc").Find(&rows)

	resolver := newMFGPlanResolver()

	// Planning MFG: ใช้เติม Spec code / IT device ให้รายการของขั้นตอนใหม่ (โหลดครั้งเดียว)
	var specSheet map[string]map[string]string
	for i := range rows {
		if rows[i].FlowVersion == FlowVersionV2 {
			specSheet = flowSpecSheetIndex()
			break
		}
	}

	serialOwner := map[string]string{}
	matchedPair := map[string]bool{}

	for i := range rows {
		// รายการจากขั้นตอนใหม่ (WH จ่ายตามแผน allocation → MFG ยืนยันจาก Kanban)
		// ใช้ผลตรวจที่บันทึกไว้ตอนยืนยัน ไม่คำนวณใหม่จากตาราง Planning เดิม
		if rows[i].FlowVersion == FlowVersionV2 {
			rows[i].ComponentLabel = ComponentLabel(rows[i].Component)
			if row := specSheet[NormalizeCodeValue(rows[i].MachineNo)]; row != nil {
				rows[i].SpecCode = pickField(row, "Product Spec", extraColumnPrefix+"Product Spec")
				rows[i].ITDevice = pickField(row, "IT device", extraColumnPrefix+"IT device")
			}
			rows[i].PlanComponent = rows[i].Component
			rows[i].PlanComponentLabel = rows[i].ComponentLabel
			rows[i].PlanState = PlanStateMatch
			rows[i].PlanMatched = true
			rows[i].WHRequired = true
			rows[i].WHPartType = rows[i].Component
			rows[i].WHMatchStatus = models.MatchStatusMatch
			continue
		}

		if strings.EqualFold(strings.TrimSpace(rows[i].Status), models.MFGStatusRetiredFormat) {
			rows[i].Status = models.MFGStatusRetiredFormat
			continue
		}

		rows[i].MachineNo = CurrentCodeOf(rows[i].MachineNo)
		rows[i].ITControllerNo = CurrentCodeOf(rows[i].ITControllerNo)

		plan, spec := resolver.evaluateWithSpec(
			rows[i].MachineNo, rows[i].ITControllerNo, rows[i].Component, rows[i].QRCode)
		applyMFGSpec(&rows[i], spec)

		if strings.TrimSpace(rows[i].Component) == "" {
			rows[i].Component = plan.Component
		}

		enrichMFGWithWH(&rows[i])
		applyMFGPlan(&rows[i], plan)

		duplicate := false
		savedMatched := strings.EqualFold(strings.TrimSpace(rows[i].Status), models.MFGStatusMatched)
		if serial := strings.TrimSpace(rows[i].ITControllerNo); serial != "" {
			mc := strings.TrimSpace(rows[i].MachineNo)
			comp := strings.ToUpper(strings.TrimSpace(rows[i].Component))
			ownerKey := comp + "|" + serial
			key := ownerKey + "|" + mc

			if owner, ok := serialOwner[ownerKey]; ok && owner != mc {
				duplicate = true
			} else if savedMatched && matchedPair[key] {
				duplicate = true
			}
			if savedMatched {
				if _, ok := serialOwner[ownerKey]; !ok {
					serialOwner[ownerKey] = mc
				}
				matchedPair[key] = true
			}
		}

		if strings.EqualFold(strings.TrimSpace(rows[i].Status), models.MFGStatusDuplicate) {
			rows[i].Status = models.MFGStatusDuplicate
			continue
		}

		rows[i].Status = mfgDisplayStatus(
			rows[i].Status,
			mfgStatusFor(plan.Component, duplicate, plan.State, rows[i].WHMatched),
		)
	}

	c.JSON(200, rows)
}

func applyMFGPlan(row *models.MFGAssembly, plan MFGPlanResult) {
	row.PlanITControllerNo = plan.PlannedITC
	row.PlanState = plan.State
	row.PlanComponent = plan.Component
	row.PlanComponentLabel = plan.Label
	row.PlanMatched = plan.OK()
	row.PlanMessage = plan.Message
	row.PlanDetail = plan.Detail
	row.PlanOwnerMachineNo = plan.OwnerMachine
}

// applyMFGSpec: เก็บผลเทียบ QR กับ Specification sheet ไว้ในแถว (SpecDetail = JSON ของผลเทียบ)
func applyMFGSpec(row *models.MFGAssembly, spec SpecCheckResult) {
	if strings.TrimSpace(row.QRCode) == "" && spec.State == SpecStateNoQR {
		row.SpecState = ""
		row.SpecDetail = ""
		return
	}
	row.SpecState = spec.State
	row.SpecDetail = specResultJSON(spec)
}

func parseMFGDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, l := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(l, s); err == nil {
			return &t
		}
	}
	return nil
}

func lookupMFGCountry(itcNo string) string {
	itcNo = strings.TrimSpace(itcNo)
	if itcNo == "" {
		return ""
	}
	var item models.LicenseItem
	if err := config.DB.Where("machine_no = ?", itcNo).First(&item).Error; err == nil {
		return item.ExportCountry
	}
	return ""
}

func itcUsedOnOtherMachine(machineNo, itcNo string, excludeID uint) bool {
	itcNo = strings.TrimSpace(itcNo)
	if itcNo == "" {
		return false
	}

	q := config.DB.Model(&models.MFGAssembly{}).
		Where("no IN ? AND machine_no <> ? AND status = ?",
			CodeVariants(itcNo), strings.TrimSpace(machineNo), models.MFGStatusMatched)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}

	var count int64
	q.Count(&count)
	return count > 0
}

func findMFGRowForPair(machineNo, itcNo string) *models.MFGAssembly {
	machineNo = strings.TrimSpace(machineNo)
	itcNo = strings.TrimSpace(itcNo)
	if machineNo == "" || itcNo == "" {
		return nil
	}

	var row models.MFGAssembly
	err := config.DB.Where("machine_no IN ? AND no IN ? AND status NOT IN ?",
		CodeVariants(machineNo), CodeVariants(itcNo),
		[]string{models.MFGStatusDuplicate, models.MFGStatusRetiredFormat}).
		Order("id desc").First(&row).Error
	if err != nil {
		return nil
	}
	return &row
}

func mfgCountryFor(machineNo, itcNo string, plan map[string]string) string {

	if v := lookupMFGCountry(itcNo); v != "" {
		return v
	}
	if v := plannedCountryOf(plan); v != "" {
		return v
	}

	machineNo = strings.TrimSpace(machineNo)
	if machineNo == "" {
		return ""
	}
	var exp models.LicenseItem
	if err := config.DB.Where("machine_no = ?", machineNo).Order("id desc").
		First(&exp).Error; err == nil {
		return strings.TrimSpace(exp.ExportCountry)
	}
	return ""
}

func looks12Digit(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 10 || len(s) > 15 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func resolveITControllerNo(machineNo, preferred string) (itcNo, country string) {
	machineNo = strings.TrimSpace(machineNo)

	if p := strings.TrimSpace(preferred); looks12Digit(p) {
		itcNo = p
	}

	if plan := planForMachine(machineNo); plan != nil {
		if itcNo == "" {
			itcNo = PlannedITCOf(plan)
		}
		country = plannedCountryOf(plan)
	}

	if machineNo != "" {
		var mfgRow models.MFGAssembly
		if err := config.DB.Where("machine_no = ?", machineNo).Order("id desc").
			First(&mfgRow).Error; err == nil {
			if itcNo == "" {
				itcNo = strings.TrimSpace(mfgRow.ITControllerNo)
			}
			if country == "" {
				country = strings.TrimSpace(mfgRow.Country)
			}
		}
	}

	// ทะเบียนเครื่องไม่มีเลข IT controller เก็บไว้ (ตารางใบนำออกเดิมมี แต่ถูกถอดไปแล้ว)
	// จึงเติมได้เฉพาะประเทศปลายทาง ส่วนเลข IT controller ต้องมาจากแหล่งอื่นด้านบน
	if machineNo != "" && country == "" {
		var exp models.LicenseItem
		if err := config.DB.Where("machine_no = ?", machineNo).
			Order("id desc").First(&exp).Error; err == nil {
			if country == "" {
				country = strings.TrimSpace(exp.ExportCountry)
			}
		}
	}

	return itcNo, country
}

func findWHPartCheck(component, serial string) *models.PartCheck {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return nil
	}
	component = strings.ToUpper(strings.TrimSpace(component))

	v := CodeVariants(serial)
	q := config.DB.Model(&models.PartCheck{}).
		Where("match_status = ?", models.MatchStatusMatch).
		Where("(machine_no IN ? OR sn IN ? OR pn IN ?)", v, v, v)

	if component != "" {
		q = q.Where("part_type = ?", component)
	}

	var pc models.PartCheck
	if err := q.Order("checked_datetime desc").First(&pc).Error; err != nil {
		return nil
	}
	return &pc
}

func latestWHPartCheckAnyStatus(component, serial string) *models.PartCheck {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return nil
	}
	component = strings.ToUpper(strings.TrimSpace(component))

	v := CodeVariants(serial)
	q := config.DB.Model(&models.PartCheck{}).
		Where("(machine_no IN ? OR sn IN ? OR pn IN ?)", v, v, v)
	if component != "" {
		q = q.Where("part_type = ?", component)
	}

	var pc models.PartCheck
	if err := q.Order("checked_datetime desc").First(&pc).Error; err != nil {
		return nil
	}
	return &pc
}

func mfgComponentOf(row *models.MFGAssembly) string {
	if c := strings.ToUpper(strings.TrimSpace(row.Component)); c != "" {
		return c
	}
	serial := strings.TrimSpace(row.ITControllerNo)
	if serial == "" {
		return ""
	}
	if c := DetectComponentFromPlan(planForMachine(row.MachineNo), serial); c != "" {
		return c
	}
	return DetectComponentType(serial)
}

func enrichMFGWithWH(row *models.MFGAssembly) {
	row.WHMatched = false
	row.WHLicenseNo = ""
	row.WHInvoiceNo = ""
	row.WHProductionNo = ""
	row.WHModel = ""
	row.WHCheckedBy = ""
	row.WHCheckedDatetime = nil
	row.WHMatchStatus = ""
	row.WHMessage = ""

	component := mfgComponentOf(row)
	row.WHPartType = component
	row.WHRequired = ComponentNeedsWHScan(component)
	row.ComponentLabel = ComponentLabel(component)

	serial := strings.TrimSpace(row.ITControllerNo)
	if serial == "" {
		return
	}

	pc := findWHPartCheck(component, serial)
	if pc == nil {
		if other := latestWHPartCheckAnyStatus(component, serial); other != nil {
			row.WHMatchStatus = other.MatchStatus
			row.WHMessage = other.MatchMessage
		}
		return
	}

	row.WHMatched = true
	row.WHMatchStatus = pc.MatchStatus
	row.WHMessage = pc.MatchMessage
	row.WHLicenseNo = pc.LicenseNo
	row.WHInvoiceNo = pc.InvoiceNo
	row.WHProductionNo = pc.ProductionNo
	row.WHCheckedBy = pc.CheckedBy
	t := pc.CheckedDatetime
	row.WHCheckedDatetime = &t

	if component != "" && component != ComponentITC {
		return
	}

	var lic models.LicenseItem
	found := false
	if pc.ImportLicenseItemID != nil {
		if config.DB.First(&lic, *pc.ImportLicenseItemID).Error == nil {
			found = true
		}
	}
	if !found {
		if config.DB.Where("machine_no = ?", serial).First(&lic).Error == nil {
			found = true
		}
	}
	if found {
		row.WHModel = lic.Model
		if strings.TrimSpace(row.Country) == "" && strings.TrimSpace(lic.ExportCountry) != "" {
			row.Country = lic.ExportCountry
		}
	}
}

type MFGScanRequest struct {
	MachineNo      string `json:"machineNo" binding:"required"`
	ITControllerNo string `json:"itControllerNo"`

	SerialNo string `json:"serialNo"`

	PartNo string `json:"partNo"`

	PartType string `json:"partType"`

	// QRCode: ข้อความจาก QR บน Specification sheet (Machine No, Spec code, ลูกค้า, ...)
	QRCode string `json:"qrCode"`
}

type mfgITCPartCheck struct {
	PartNo   string
	SerialNo string
	ITCNo    string
	Message  string
	Detail   string
}

func (r mfgITCPartCheck) Failed() bool {
	return r.Message != ""
}

func checkMFGITCPart(pn, sn string) mfgITCPartCheck {
	pn = strings.TrimSpace(pn)
	sn = strings.TrimSpace(sn)
	out := mfgITCPartCheck{PartNo: pn, SerialNo: sn}

	if pn == "" || sn == "" {
		out.Message = "IT Controller ต้องสแกน Machine + P/N + S/N"
		return out
	}
	if SameCode(pn, sn) {
		out.Message = "ค่า S/N ซ้ำกับ P/N"
		out.Detail = "P/N และ S/N เป็นค่าเดียวกัน (" + sn + ")"
		return out
	}

	// ทะเบียนกลาง (ตาราง master_data) ถูกถอดออกแล้ว — ไฟล์ที่อัปจริงมีสามชุด
	// Planning WH / Planning MFG / Master Data ซึ่งทั้งหมดอยู่ในตาราง upload_data_rows
	// และไม่มีชุดไหนเก็บคู่ P/N–S/N ของ IT Controller รายตัว
	//
	// จึงเหลือการตรวจเท่าที่ตรวจได้จากตัวรหัสเอง (กรอกครบ และ P/N ไม่ซ้ำกับ S/N)
	// ส่วนการยืนยันว่า S/N นี้คู่กับ P/N นี้จริงไหม ทำไม่ได้แล้ว
	out.PartNo = CurrentCodeOf(ResolvePartNo(pn))
	out.SerialNo = CurrentCodeOf(ResolveComponentSerial(ComponentITC, sn))
	return out
}

func mfgCodeIsPart(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	if looks12Digit(code) {
		return true
	}
	// ไม่มีทะเบียนกลางให้ถามแล้ว — ตัดสินจากรูปแบบรหัสอย่างเดียว
	return false
}

func (r MFGScanRequest) scannedSerial() string {
	if v := strings.TrimSpace(r.SerialNo); v != "" {
		return v
	}
	return strings.TrimSpace(r.ITControllerNo)
}

func resolveMachineNo(raw string) string {
	return ResolveMachineNo(raw)
}

func ScanMFGAssembly(c *gin.Context) {
	var req MFGScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	machineNo := strings.TrimSpace(req.MachineNo)
	qrCode := strings.TrimSpace(req.QRCode)
	if q, ok := ParseSpecQR(qrCode); ok {
		machineNo = q.MachineNo
	} else if LooksLikeSpecQR(machineNo) {
		// เครื่องสแกนส่ง QR มาในช่อง Machine No
		if q, ok := ParseSpecQR(machineNo); ok {
			qrCode = machineNo
			machineNo = q.MachineNo
		}
	}
	itcNo := req.scannedSerial()
	partType := strings.ToUpper(strings.TrimSpace(req.PartType))
	if machineNo == "" {
		c.JSON(400, gin.H{"message": "ต้องมี Machine No"})
		return
	}

	resolver := newMFGPlanResolver()

	if resolver.planOf(resolveMachineNo(machineNo)) == nil && mfgCodeIsPart(machineNo) {

		// สแกนบาร์โค้ดพาร์ทมาในช่อง Machine No — เก็บไว้ตรวจย้อนหลัง
		LogInvalidData(c, InvalidData{
			SourceTable: "MFG_ASSEMBLY",
			Action:      "scan_invalid",
			MachineNo:   machineNo,
			Component:   partType,
			Field:       "Machine No",
			WrongValue:  machineNo,
			Reason:      "ค่าที่สแกนเป็นรหัสพาร์ท ไม่ใช่ Machine No และไม่มีในแผนประกอบ",
		})

		c.JSON(422, gin.H{
			"message":        "ข้อมูลไม่ถูกต้อง",
			"invalidMachine": true,
		})
		return
	}

	scanPartNo := strings.TrimSpace(req.PartNo)
	scanSerialNo := strings.TrimSpace(req.SerialNo)
	if scanSerialNo == "" {
		scanSerialNo = strings.TrimSpace(req.ITControllerNo)
	}

	var itcCheck *mfgITCPartCheck
	if partType == ComponentITC && scanPartNo != "" {
		check := checkMFGITCPart(scanPartNo, scanSerialNo)
		itcCheck = &check
		scanPartNo = check.PartNo
		scanSerialNo = check.SerialNo
		if check.ITCNo != "" {
			itcNo = check.ITCNo
		}
	}
	partFailed := itcCheck != nil && itcCheck.Failed()

	if msg, blocked := retiredScanMessage(machineNo, itcNo); blocked {
		userID, name := lookupUserName(c)
		now := time.Now()

		row := models.MFGAssembly{
			DateAssembly:    &now,
			MachineNo:       machineNo,
			ITControllerNo:  itcNo,
			PartNo:          scanPartNo,
			SerialNo:        scanSerialNo,
			Status:          models.MFGStatusRetiredFormat,
			RetiredDetail:   msg,
			QRCode:          qrCode,
			CheckDate:       &now,
			CreatedBy:       name,
			CreatedDatetime: now,
			UpdatedDatetime: now,
			UserID:          userID,
		}

		if err := config.DB.Create(&row).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		LogInvalidData(c, InvalidData{
			SourceTable:  "MFG_ASSEMBLY",
			SourceID:     row.ID,
			Action:       "scan_retired_format",
			MachineNo:    machineNo,
			Component:    partType,
			Field:        "รูปแบบรหัส",
			WrongValue:   firstNonEmpty(itcNo, machineNo),
			CorrectValue: CurrentCodeOf(firstNonEmpty(itcNo, machineNo)),
			Reason:       msg,
		})

		c.JSON(201, gin.H{
			"row":           row,
			"status":        models.MFGStatusRetiredFormat,
			"matched":       false,
			"retiredFormat": true,
			"message":       "รูปแบบเดิมถูกยกเลิกแล้ว",
			"detail":        msg,
		})
		return
	}

	// Scan Kanban ขั้นตอนเดียว (ไม่มีการสแกนพาร์ท)
	if qrCode != "" && itcNo == "" && strings.TrimSpace(req.PartNo) == "" {
		scanMFGKanban(c, resolver, machineNo, qrCode)
		return
	}

	machineNo = resolveMachineNo(machineNo)

	plan, spec := resolver.evaluateWithSpec(machineNo, itcNo, req.PartType, qrCode)

	if v := strings.TrimSpace(plan.ScannedITC); v != "" {
		itcNo = CurrentCodeOf(v)
	}
	machineNo = CurrentCodeOf(machineNo)

	userID, name := lookupUserName(c)
	now := time.Now()

	existing := findMFGRowForPair(machineNo, itcNo)
	if partFailed && existing != nil && existing.Status == models.MFGStatusMatched {
		existing = nil
	}

	if !partFailed && existing != nil && existing.Status == models.MFGStatusMatched {
		component := strings.TrimSpace(existing.Component)
		if component == "" {
			component = plan.Component
		}

		dup := models.MFGAssembly{
			DateAssembly:    &now,
			MachineNo:       machineNo,
			ITControllerNo:  itcNo,
			PartNo:          scanPartNo,
			SerialNo:        scanSerialNo,
			Component:       component,
			Country:         existing.Country,
			QRCode:          qrCode,
			CheckDate:       &now,
			CreatedBy:       name,
			CreatedDatetime: now,
			UpdatedDatetime: now,
			UserID:          userID,
		}

		enrichMFGWithWH(&dup)
		applyMFGSpec(&dup, spec)
		dup.Status = models.MFGStatusDuplicate

		if err := config.DB.Create(&dup).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		applyMFGPlan(&dup, plan)

		CreateAuditLog("MFG_ASSEMBLY", dup.ID, "scan_repeat",
			machineNo+"/"+models.MFGStatusDuplicate, userID, name)

		c.JSON(200, gin.H{
			"row":                   dup,
			"status":                models.MFGStatusDuplicate,
			"matched":               false,
			"duplicate":             true,
			"originalID":            existing.ID,
			"whMatched":             dup.WHMatched,
			"whRequired":            dup.WHRequired,
			"whMissing":             false,
			"component":             dup.Component,
			"componentLabel":        dup.ComponentLabel,
			"message":               "รายการนี้เคยบันทึกไปแล้ว",
			"plan":                  plan,
			"plannedITControllerNo": plan.PlannedITC,
			"plannedState":          plan.State,
			"plannedMatch":          plan.OK(),
			"spec":                  spec,
		})
		return
	}

	duplicate := itcUsedOnOtherMachine(machineNo, itcNo, 0)

	row := models.MFGAssembly{
		DateAssembly:    &now,
		MachineNo:       machineNo,
		ITControllerNo:  itcNo,
		PartNo:          scanPartNo,
		SerialNo:        scanSerialNo,
		Component:       plan.Component,
		Country:         mfgCountryFor(machineNo, itcNo, resolver.planOf(machineNo)),
		QRCode:          qrCode,
		CheckDate:       &now,
		CreatedBy:       name,
		CreatedDatetime: now,
		UpdatedDatetime: now,
		UserID:          userID,
	}
	if partFailed && row.Component == "" {
		row.Component = ComponentITC
	}
	if existing != nil {

		row.ID = existing.ID
		row.CreatedBy = existing.CreatedBy
		row.CreatedDatetime = existing.CreatedDatetime
		row.PhotoURL = existing.PhotoURL
	}

	enrichMFGWithWH(&row)
	applyMFGSpec(&row, spec)
	row.Status = mfgStatusFor(plan.Component, duplicate, plan.State, row.WHMatched)
	if partFailed {
		row.Status = models.MFGStatusNotMatched
	}

	action := "scan_create"
	if existing != nil {
		action = "scan_update"
		if err := config.DB.Save(&row).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
	} else {
		if err := config.DB.Create(&row).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
	}

	applyMFGPlan(&row, plan)

	if row.Status == models.MFGStatusMatched {
		CreateAuditLog("MFG_ASSEMBLY", row.ID, action, machineNo+"/"+row.Status, userID, name)
	} else {

		// สแกนแล้วไม่ตรงแผน — เก็บทั้งค่าที่สแกนได้และค่าที่ถูกต้อง
		field := "IT Controller No"
		wrong := firstNonEmpty(plan.ScannedITC, itcNo)
		correct := plan.PlannedITC
		reason := firstNonEmpty(plan.Detail, plan.Message)

		if partFailed {
			field = "P/N"
			wrong = firstNonEmpty(scanPartNo, scanSerialNo)
			correct = ""
			reason = firstNonEmpty(itcCheck.Detail, itcCheck.Message)
		}
		if reason == "" {
			reason = "สแกนไม่ตรงกับแผนประกอบของเครื่อง " + machineNo
		}

		LogInvalidData(c, InvalidData{
			SourceTable:  "MFG_ASSEMBLY",
			SourceID:     row.ID,
			Action:       action + "_invalid",
			MachineNo:    machineNo,
			Component:    firstNonEmpty(row.Component, plan.Component, partType),
			Field:        field,
			WrongValue:   wrong,
			CorrectValue: correct,
			Reason:       reason,
		})
	}

	message := mfgFinalMessage(row.Status, plan, row.WHLicenseNo)
	detail := plan.Detail
	if partFailed {
		message = itcCheck.Message
		if itcCheck.Detail != "" {
			detail = itcCheck.Detail
		}
	}

	c.JSON(201, gin.H{
		"row":                   row,
		"status":                row.Status,
		"matched":               row.Status == models.MFGStatusMatched,
		"whMatched":             row.WHMatched,
		"retried":               existing != nil,
		"component":             plan.Component,
		"componentLabel":        plan.Label,
		"message":               message,
		"detail":                detail,
		"partMismatch":          partFailed,
		"whRequired":            row.WHRequired,
		"whMissing":             row.WHRequired && !row.WHMatched,
		"whMatchStatus":         row.WHMatchStatus,
		"whMessage":             row.WHMessage,
		"plan":                  plan,
		"plannedITControllerNo": plan.PlannedITC,
		"plannedState":          plan.State,
		"plannedMatch":          plan.OK(),
		"spec":                  spec,
	})
}

// findMFGKanbanRows: รายการของเครื่องนี้ที่บันทึกไว้แล้ว (ไม่นับรายการซ้ำ / รูปแบบเดิมถูกยกเลิก)
// matched = รายการที่ประกอบผ่านแล้ว, retry = รายการ Kanban ที่เคยไม่ผ่าน (สแกนใหม่จะอัปเดตแถวนี้)
func findMFGKanbanRows(machineNo string) (matched, retry *models.MFGAssembly) {
	variants := CodeVariants(machineNo)
	if len(variants) == 0 {
		variants = []string{machineNo}
	}
	var rows []models.MFGAssembly
	config.DB.Where("machine_no IN ? AND status NOT IN ?", variants,
		[]string{models.MFGStatusDuplicate, models.MFGStatusRetiredFormat}).
		Order("id desc").Find(&rows)
	for i := range rows {
		if strings.EqualFold(strings.TrimSpace(rows[i].Status), models.MFGStatusMatched) {
			if matched == nil {
				matched = &rows[i]
			}
			continue
		}
		if retry == nil && strings.TrimSpace(rows[i].ITControllerNo) == "" {
			retry = &rows[i]
		}
	}
	return matched, retry
}

// scanMFGKanban: MFG สแกน Kanban (QR บน Specification sheet) → เทียบกับ Daily Plan → ตรง = MATCHED
func scanMFGKanban(c *gin.Context, resolver *mfgPlanResolver, machineNo, qrCode string) {
	machineNo = CurrentCodeOf(resolveMachineNo(machineNo))
	plan, spec := resolver.evaluateWithSpec(machineNo, "", "", qrCode)

	userID, name := lookupUserName(c)
	now := time.Now()

	country := ""
	qrCustomer := ""
	if q, ok := ParseSpecQR(qrCode); ok {
		country = q.Customer
		qrCustomer = q.Customer
	}
	if country == "" {
		country = mfgCountryFor(machineNo, "", resolver.planOf(machineNo))
	}

	// ลูกค้า / ประเทศบน Kanban ต้องตรงกับไฟล์ Planning WH ของเครื่องนี้
	customer := checkKanbanCustomerForMachine(machineNo, qrCustomer)

	row := models.MFGAssembly{
		DateAssembly:    &now,
		MachineNo:       machineNo,
		Country:         country,
		QRCode:          qrCode,
		CheckDate:       &now,
		CreatedBy:       name,
		CreatedDatetime: now,
		UpdatedDatetime: now,
		UserID:          userID,
	}
	enrichMFGWithWH(&row)
	applyMFGSpec(&row, spec)

	matched, retry := findMFGKanbanRows(machineNo)

	// เครื่องนี้ประกอบผ่านไปแล้ว → บันทึกเป็นรายการซ้ำ
	// (ถ้าลูกค้า / ประเทศไม่ตรงกับ Planning WH ให้ตกไปทางไม่ผ่านแทน)
	if matched != nil && plan.OK() && !customer.Blocked() {
		row.Status = models.MFGStatusDuplicate
		if err := config.DB.Create(&row).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		applyMFGPlan(&row, plan)
		CreateAuditLog("MFG_ASSEMBLY", row.ID, "scan_repeat", machineNo+"/"+models.MFGStatusDuplicate, userID, name)
		c.JSON(200, gin.H{
			"row":        row,
			"status":     models.MFGStatusDuplicate,
			"matched":    false,
			"duplicate":  true,
			"originalID": matched.ID,
			"message":    "รายการนี้เคยบันทึกไปแล้ว",
			"plan":       plan,
			"spec":       spec,
		})
		return
	}

	row.Status = mfgStatusFor("", false, plan.State, false)
	if customer.Blocked() {
		row.Status = models.MFGStatusNotMatched
	}

	action := "scan_kanban"
	if retry != nil {
		action = "scan_kanban_retry"
		row.ID = retry.ID
		row.CreatedBy = retry.CreatedBy
		row.CreatedDatetime = retry.CreatedDatetime
		row.PhotoURL = retry.PhotoURL
		if err := config.DB.Save(&row).Error; err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
	} else if err := config.DB.Create(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	applyMFGPlan(&row, plan)

	if row.Status == models.MFGStatusMatched {
		CreateAuditLog("MFG_ASSEMBLY", row.ID, action, machineNo+"/"+row.Status, userID, name)
	} else {

		// สแกน Kanban แล้วไม่ผ่าน — เก็บค่าบน Kanban คู่กับค่าตามแผน
		field := "Kanban"
		wrong := firstNonEmpty(qrCustomer, qrCode)
		correct := ""
		reason := firstNonEmpty(plan.Detail, plan.Message)

		if customer.Blocked() {
			field = "ลูกค้า / ประเทศ"
			wrong = customer.QR
			correct = customer.Plan
			reason = customer.Detail
		}
		if reason == "" {
			reason = "Kanban ของเครื่อง " + machineNo + " ไม่ตรงกับแผน"
		}

		LogInvalidData(c, InvalidData{
			SourceTable:  "MFG_ASSEMBLY",
			SourceID:     row.ID,
			Action:       action + "_invalid",
			MachineNo:    machineNo,
			Field:        field,
			WrongValue:   wrong,
			CorrectValue: correct,
			Reason:       reason,
		})
	}

	message := mfgFinalMessage(row.Status, plan, "")
	detail := plan.Detail
	if customer.Blocked() {
		message = customer.Message
		detail = customer.Detail
	}

	c.JSON(201, gin.H{
		"row":              row,
		"status":           row.Status,
		"matched":          row.Status == models.MFGStatusMatched,
		"retried":          retry != nil,
		"message":          message,
		"detail":           detail,
		"plan":             plan,
		"spec":             spec,
		"customerCheck":    customer,
		"customerMismatch": customer.Blocked(),
	})
}

type MFGAssemblyRequest struct {
	DateAssembly   string `json:"dateAssembly"`
	MachineNo      string `json:"machineNo"`
	ITControllerNo string `json:"itControllerNo"`
	PartNo         string `json:"partNo"`
	SerialNo       string `json:"serialNo"`
	Country        string `json:"country"`
	CheckDate      string `json:"checkDate"`
	Status         string `json:"status"`
	// QRCode: (ไม่บังคับ) QR บน Specification sheet — ส่ง nil = ไม่เปลี่ยนค่าเดิม
	QRCode *string `json:"qrCode"`
}

func applyManualStatus(systemStatus, requested string) string {
	requested = strings.ToUpper(strings.TrimSpace(requested))
	if requested == "" || requested == models.MFGStatusMatched {
		return systemStatus
	}
	if systemStatus == models.MFGStatusMatched {
		return requested
	}
	return systemStatus
}

func CreateMFGAssembly(c *gin.Context) {
	var req MFGAssemblyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	now := time.Now()

	dateAss := parseMFGDate(req.DateAssembly)
	if dateAss == nil {
		dateAss = &now
	}
	checkDate := parseMFGDate(req.CheckDate)
	if checkDate == nil {
		checkDate = &now
	}

	machineNo := resolveMachineNo(strings.TrimSpace(req.MachineNo))
	itcNo := strings.TrimSpace(req.ITControllerNo)

	qrCode := ""
	if req.QRCode != nil {
		qrCode = strings.TrimSpace(*req.QRCode)
	}

	resolver := newMFGPlanResolver()
	plan, spec := resolver.evaluateWithSpec(machineNo, itcNo, "", qrCode)

	if v := strings.TrimSpace(plan.ScannedITC); v != "" {
		itcNo = v
	}

	country := strings.TrimSpace(req.Country)
	if country == "" {
		country = mfgCountryFor(machineNo, itcNo, resolver.planOf(machineNo))
	}

	duplicate := itcUsedOnOtherMachine(machineNo, itcNo, 0)

	row := models.MFGAssembly{
		DateAssembly:    dateAss,
		MachineNo:       machineNo,
		ITControllerNo:  itcNo,
		PartNo:          strings.TrimSpace(req.PartNo),
		SerialNo:        strings.TrimSpace(req.SerialNo),
		Component:       plan.Component,
		Country:         country,
		QRCode:          qrCode,
		CheckDate:       checkDate,
		CreatedBy:       name,
		CreatedDatetime: now,
		UpdatedDatetime: now,
		UserID:          userID,
	}

	enrichMFGWithWH(&row)
	applyMFGSpec(&row, spec)
	row.Status = applyManualStatus(
		mfgStatusFor(plan.Component, duplicate, plan.State, row.WHMatched),
		req.Status,
	)

	if err := config.DB.Create(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	applyMFGPlan(&row, plan)

	CreateAuditLog("MFG_ASSEMBLY", row.ID, "create", row.MachineNo, userID, name)
	c.JSON(201, row)
}

func UpdateMFGAssembly(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.MFGAssembly
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	var req MFGAssemblyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	row.MachineNo = strings.TrimSpace(req.MachineNo)
	row.ITControllerNo = strings.TrimSpace(req.ITControllerNo)
	row.PartNo = strings.TrimSpace(req.PartNo)
	row.SerialNo = strings.TrimSpace(req.SerialNo)
	row.Country = strings.TrimSpace(req.Country)
	if d := parseMFGDate(req.DateAssembly); d != nil {
		row.DateAssembly = d
	}
	if d := parseMFGDate(req.CheckDate); d != nil {
		row.CheckDate = d
	}
	row.UpdatedDatetime = time.Now()

	row.MachineNo = resolveMachineNo(row.MachineNo)
	if req.QRCode != nil {
		row.QRCode = strings.TrimSpace(*req.QRCode)
	}

	resolver := newMFGPlanResolver()
	plan, spec := resolver.evaluateWithSpec(row.MachineNo, row.ITControllerNo, row.Component, row.QRCode)
	row.Component = plan.Component
	if v := strings.TrimSpace(plan.ScannedITC); v != "" {
		row.ITControllerNo = v
	}

	duplicate := itcUsedOnOtherMachine(row.MachineNo, row.ITControllerNo, row.ID)

	enrichMFGWithWH(&row)
	applyMFGSpec(&row, spec)
	row.Status = applyManualStatus(
		mfgStatusFor(plan.Component, duplicate, plan.State, row.WHMatched),
		req.Status,
	)

	if err := config.DB.Save(&row).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	applyMFGPlan(&row, plan)

	userID, name := lookupUserName(c)
	CreateAuditLog("MFG_ASSEMBLY", row.ID, "edit", row.MachineNo, userID, name)
	c.JSON(200, row)
}

func DeleteMFGAssembly(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.MFGAssembly
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	if err := config.DB.Delete(&models.MFGAssembly{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	CreateAuditLog("MFG_ASSEMBLY", row.ID, "delete", row.MachineNo, userID, name)
	c.JSON(200, gin.H{"deleted": true})
}
