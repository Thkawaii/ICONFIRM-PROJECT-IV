package controllers

import (
	"strconv"
	"strings"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

var tagTypeLabels = map[string]string{
	"MC":  "Machine",
	"ITC": "IT Controller",
	"CV":  "Control Valve",
	"SM":  "Swing Motor",
	"MP":  "Motor Propel",
	"PH":  "Pump Assy HYD",
	"EN":  "Engine",
	"CW":  "Counter Weight",
}

func GetPartChecks(c *gin.Context) {

	var rows []models.PartCheck

	query := config.DB.Order("checked_datetime desc")

	if v := strings.TrimSpace(c.Query("invoice_no")); v != "" {
		query = query.Where("invoice_no = ?", v)
	}
	if v := strings.TrimSpace(c.Query("part_type")); v != "" {
		query = query.Where("part_type = ?", strings.ToUpper(v))
	}

	query.Find(&rows)

	enrichPartChecksWithExpected(rows)
	applyCurrentCodeFormat(rows)

	c.JSON(200, rows)
}

func applyCurrentCodeFormat(rows []models.PartCheck) {
	cache := map[string]string{}
	current := func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return v
		}
		if hit, ok := cache[v]; ok {
			return hit
		}
		out := CurrentCodeOf(v)
		cache[v] = out
		return out
	}

	for i := range rows {
		if rows[i].MatchStatus == models.MatchStatusRetiredFormat {
			if msg, blocked := retiredScanMessage(rows[i].PN, rows[i].SN); blocked {
				rows[i].MatchDetail = msg
			}
			continue
		}
		rows[i].PN = current(rows[i].PN)
		rows[i].SN = current(rows[i].SN)
		rows[i].MachineNo = current(rows[i].MachineNo)
		rows[i].ExpectedPN = current(rows[i].ExpectedPN)
	}
}

func enrichPartChecksWithExpected(rows []models.PartCheck) {
	serials := map[string]bool{}
	for _, r := range rows {
		if r.MatchStatus == models.MatchStatusWrongPart && strings.TrimSpace(r.SN) != "" {
			serials[strings.TrimSpace(r.SN)] = true
		}
	}
	if len(serials) == 0 {
		return
	}

	list := make([]string, 0, len(serials))
	for sn := range serials {
		list = append(list, sn)
	}

	// ไม่มีทะเบียนกลาง (master_data) ให้เทียบแล้ว

	pnBySerial := map[string]string{}

	for i := range rows {
		if rows[i].MatchStatus != models.MatchStatusWrongPart {
			continue
		}
		expected := pnBySerial[strings.TrimSpace(rows[i].SN)]
		if expected == "" {
			continue
		}
		rows[i].ExpectedPN = expected
		rows[i].MatchDetail = "S/N " + rows[i].SN + " คู่กับ P/N " + expected +
			" แต่สแกนได้ " + rows[i].PN
	}
}

func DeletePartCheck(c *gin.Context) {

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.PartCheck
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	deletable := map[string]bool{
		models.MatchStatusNotFound:    true,
		models.MatchStatusNotRequired: true,
		models.MatchStatusDuplicate:   true,
	}
	if !deletable[row.MatchStatus] {
		c.JSON(400, gin.H{"message": "ลบได้เฉพาะรายการที่ไม่พบในใบอนุญาต, ไม่ต้องเทียบ หรือยืนยันซ้ำเท่านั้น"})
		return
	}

	if err := config.DB.Delete(&models.PartCheck{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, name := lookupUserName(c)
	CreateAuditLog("PART_CHECK", row.ID, "delete", row.PartType+"/"+row.SN, userID, name)

	c.JSON(200, gin.H{"deleted": true})
}

func dedupeCodes(values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := NormalizeCodeValue(v)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

type ScanPartCheckRequest struct {
	PartType string `json:"partType" binding:"required"`
	PN       string `json:"pn"`
	SN       string `json:"sn" binding:"required"`

	ProductionNo string `json:"productionNo"`
	InvoiceNo    string `json:"invoiceNo"`
}

func checkEnginePart(check *models.PartCheck) {
	oldPN := ResolvePartNo(check.PN)
	oldSN := ResolveComponentSerial(ComponentEN, check.SN)
	check.PN = CurrentCodeOf(oldPN)
	check.SN = CurrentCodeOf(oldSN)

	row, ok := engineRowFor(oldPN, oldSN)
	if !ok {

		if other, found := engineRowByValue(oldSN); found {
			engine := strings.TrimSpace(pickField(other, "ENGINE", "Engine"))
			history := strings.TrimSpace(pickField(other, "History", "Engine History"))

			expected := engine
			if SameCode(engine, oldSN) {
				expected = history
			}
			expected = CurrentCodeOf(expected)

			check.MachineNo = strings.TrimSpace(pickField(other, "Machine No", "Machine"))
			check.MatchStatus = models.MatchStatusWrongPart
			check.MatchMessage = "ข้อมูลไม่ถูกต้อง"
			check.ExpectedPN = expected
			check.MatchDetail = "S/N " + check.SN + " คู่กับ P/N " + expected +
				" แต่สแกนได้ " + check.PN
			return
		}

		check.MatchStatus = models.MatchStatusNotFound
		check.MatchMessage = "ข้อมูลไม่ถูกต้อง"
		check.MatchDetail = "ไม่พบ Engine P/N " + check.PN + " S/N " + check.SN +
			" ในไฟล์ Engine ที่อัปโหลดไว้"
		return
	}

	check.MachineNo = strings.TrimSpace(pickField(row, "Machine No", "Machine"))
	check.MatchStatus = models.MatchStatusMatch
	check.MatchMessage = "ข้อมูลถูกต้อง"
	check.MatchDetail = "ตรงกับไฟล์ Engine ของเครื่อง " + check.MachineNo
}

func checkPlanComponentPart(check *models.PartCheck, component string) {
	scanned := strings.TrimSpace(check.SN)
	label := ComponentLabel(component)

	if resolved := ResolveComponentSerial(component, scanned); !strings.EqualFold(resolved, scanned) {
		scanned = resolved
	}
	check.SN = CurrentCodeOf(scanned)

	otherMachine := ""
	otherComponent := ""

	for machineNo, plan := range loadMachinePlans() {

		if planned := PlannedNoOf(plan, component); planned != "" &&
			SameCode(planned, scanned) {
			check.MachineNo = machineNo
			check.MatchStatus = models.MatchStatusMatch
			check.MatchMessage = "ข้อมูลถูกต้อง"
			check.MatchDetail = label + " " + scanned + " ตรงกับแผนของเครื่อง " + machineNo
			return
		}

		if otherComponent != "" {
			continue
		}
		for _, spec := range componentSpecs {
			if spec.Code == component {
				continue
			}
			if v := PlannedNoOf(plan, spec.Code); v != "" && SameCode(v, scanned) {
				otherMachine = machineNo
				otherComponent = spec.Code
				break
			}
		}
	}

	check.MatchMessage = "ข้อมูลไม่ถูกต้อง"

	if otherComponent != "" {
		check.MatchStatus = models.MatchStatusWrongPart
		check.MatchDetail = scanned + " เป็น " + ComponentLabel(otherComponent) +
			" ของเครื่อง " + otherMachine + " ไม่ใช่ " + label
		return
	}

	check.MatchStatus = models.MatchStatusNotFound
	if MasterTypeOf(component) != "" {
		check.MatchDetail = "ไม่พบ " + label + " " + scanned + " ในทะเบียนพาร์ท (Master Data) และแผนประกอบ"
	} else {
		check.MatchDetail = "ไม่พบ " + label + " " + scanned + " ในแผนประกอบของเครื่องใดเลย"
	}
}

