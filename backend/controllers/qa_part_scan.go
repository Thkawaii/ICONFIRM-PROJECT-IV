package controllers

import (
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

var qaScanComponentOrder = []string{
	ComponentITC,
	ComponentCV,
	ComponentSM,
	ComponentMP,
	ComponentPH,
	ComponentEN,
	ComponentCW,
}

type QAScanUnit struct {
	MachineNo      string `json:"machineNo"`
	Model          string `json:"model"`
	Component      string `json:"component"`
	ComponentLabel string `json:"componentLabel"`

	PlannedNo       string   `json:"plannedNo"`
	PlannedNoFormer []string `json:"plannedNoFormer,omitempty"`
	MachineNoFormer []string `json:"machineNoFormer,omitempty"`

	Scanned       bool   `json:"scanned"`
	ScanAttempted bool   `json:"scanAttempted"`
	ScannedNo     string `json:"scannedNo"`
	ScannedPN     string `json:"scannedPN"`
	ScannedAt     string `json:"scannedAt"`
	ScannedBy     string `json:"scannedBy"`
	MatchStatus   string `json:"matchStatus"`
	MatchMessage  string `json:"matchMessage"`

	Assembled         bool   `json:"assembled"`
	AssembleAttempted bool   `json:"assembleAttempted"`
	AssembledAt       string `json:"assembledAt"`
	AssembledBy       string `json:"assembledBy"`
	AssembledStatus   string `json:"assembledStatus"`

	LicenseNo string `json:"licenseNo"`
	InvoiceNo string `json:"invoiceNo"`

	SpecCode string `json:"specCode"`
	ITDevice string `json:"itDevice"`
	Country  string `json:"country"`
}

type QAScanComponentMeta struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type QAPartScanSummaryResponse struct {
	Components  []QAScanComponentMeta `json:"components"`
	Units       []QAScanUnit          `json:"units"`
	Machines    int                   `json:"machines"`
	GeneratedAt string                `json:"generatedAt"`
}

func qaScanKey(s string) string {
	s = strings.ToUpper(strings.TrimSpace(unwrapExcelText(s)))
	if s == "" || s == "-" {
		return ""
	}
	return strings.NewReplacer(" ", "", "-", "", "_", "", "/", "", ".", "").Replace(s)
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func rfc3339Ptr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return rfc3339(*t)
}

func GetQAPartScanSummary(c *gin.Context) {

	plans := loadMachinePlans()

	enginePlanByMachine := map[string]string{}
	for _, row := range loadUploadRows(models.DatasetEngine) {
		mc := strings.TrimSpace(pickField(row, "Machine No", "Machine"))
		if mc == "" {
			continue
		}
		if _, ok := enginePlanByMachine[mc]; ok {
			continue
		}
		if v := strings.TrimSpace(pickField(row, "ENGINE", "Engine")); v != "" {
			enginePlanByMachine[mc] = v
		}
	}

	fmtIdx := loadCodeFormatIndex()

	var checks []models.PartCheck
	config.DB.Order("checked_datetime asc").Find(&checks)

	matchedCheckByMachine := map[string]models.PartCheck{}
	matchedCheckByNo := map[string]models.PartCheck{}
	latestCheckByMachine := map[string]models.PartCheck{}
	latestCheckByNo := map[string]models.PartCheck{}

	for _, ck := range checks {
		comp := strings.ToUpper(strings.TrimSpace(ck.PartType))
		if comp == "" {
			continue
		}

		isMatch := strings.EqualFold(strings.TrimSpace(ck.MatchStatus), models.MatchStatusMatch)
		mcKeys := fmtIdx.scanKeys(ck.MachineNo)

		for _, raw := range []string{ck.SN, ck.PN, ck.MachineNo} {
			for _, k := range fmtIdx.scanKeys(raw) {
				latestCheckByNo[comp+"|"+k] = ck
				if isMatch {
					matchedCheckByNo[comp+"|"+k] = ck
				}
				for _, mcKey := range mcKeys {
					latestCheckByMachine[comp+"|"+mcKey+"|"+k] = ck
					if isMatch {
						matchedCheckByMachine[comp+"|"+mcKey+"|"+k] = ck
					}
				}
			}
		}
	}

	lookupCheck := func(comp, machineNo, planned string) (models.PartCheck, bool, bool) {
		numKeys := fmtIdx.scanKeys(planned)
		if len(numKeys) == 0 {
			return models.PartCheck{}, false, false
		}
		mcKeys := fmtIdx.scanKeys(machineNo)

		byMachine := func(idx map[string]models.PartCheck) (models.PartCheck, bool) {
			for _, mk := range mcKeys {
				for _, nk := range numKeys {
					if ck, ok := idx[comp+"|"+mk+"|"+nk]; ok {
						return ck, true
					}
				}
			}
			return models.PartCheck{}, false
		}
		byNo := func(idx map[string]models.PartCheck) (models.PartCheck, bool) {
			for _, nk := range numKeys {
				if ck, ok := idx[comp+"|"+nk]; ok {
					return ck, true
				}
			}
			return models.PartCheck{}, false
		}

		if ck, ok := byMachine(matchedCheckByMachine); ok {
			return ck, true, true
		}
		if ck, ok := byNo(matchedCheckByNo); ok {
			return ck, true, true
		}
		if ck, ok := byMachine(latestCheckByMachine); ok {
			return ck, false, true
		}
		if ck, ok := byNo(latestCheckByNo); ok {
			return ck, false, true
		}
		return models.PartCheck{}, false, false
	}

	var mfgRows []models.MFGAssembly
	config.DB.Order("id asc").Find(&mfgRows)

	matchedMFGByKey := map[string]models.MFGAssembly{}
	latestMFGByKey := map[string]models.MFGAssembly{}
	for _, m := range mfgRows {
		status := strings.ToUpper(strings.TrimSpace(m.Status))
		if status == models.MFGStatusDuplicate || status == models.MFGStatusRetiredFormat {
			continue
		}
		serialKeys := fmtIdx.scanKeys(m.ITControllerNo)
		if len(serialKeys) == 0 {
			continue
		}
		mcKeys := fmtIdx.scanKeys(m.MachineNo)
		if len(mcKeys) == 0 {
			mcKeys = []string{""}
		}

		for _, mk := range mcKeys {
			for _, sk := range serialKeys {
				k := mk + "|" + sk
				latestMFGByKey[k] = m
				if status == models.MFGStatusMatched {
					matchedMFGByKey[k] = m
				}
			}
		}
	}

	lookupMFG := func(machineNo, planned string) (models.MFGAssembly, bool, bool) {
		mcKeys := fmtIdx.scanKeys(machineNo)
		numKeys := fmtIdx.scanKeys(planned)
		find := func(idx map[string]models.MFGAssembly) (models.MFGAssembly, bool) {
			for _, mk := range mcKeys {
				for _, nk := range numKeys {
					if m, ok := idx[mk+"|"+nk]; ok {
						return m, true
					}
				}
			}
			return models.MFGAssembly{}, false
		}
		if m, ok := find(matchedMFGByKey); ok {
			return m, true, true
		}
		if m, ok := find(latestMFGByKey); ok {
			return m, false, true
		}
		return models.MFGAssembly{}, false, false
	}

	modelByITC := map[string]string{}

	// ไม่มีทะเบียนกลางให้ดึงชื่อรุ่นแล้ว modelByITC จึงว่างเสมอ

	var licItems []models.LicenseItem
	config.DB.Select("machine_no, model, license_no, invoice_no").Find(&licItems)
	licByITC := map[string]models.LicenseItem{}
	for _, it := range licItems {
		for _, k := range fmtIdx.scanKeys(it.MachineNo) {
			if _, ok := licByITC[k]; !ok {
				licByITC[k] = it
			}
			if model := strings.TrimSpace(it.Model); model != "" && modelByITC[k] == "" {
				modelByITC[k] = model
			}
		}
	}

	firstModel := func(keys []string) string {
		for _, k := range keys {
			if v := modelByITC[k]; v != "" {
				return v
			}
		}
		return ""
	}
	firstLicense := func(keys []string) (models.LicenseItem, bool) {
		for _, k := range keys {
			if it, ok := licByITC[k]; ok {
				return it, true
			}
		}
		return models.LicenseItem{}, false
	}

	units := make([]QAScanUnit, 0, len(plans)*len(qaScanComponentOrder))
	machines := 0

	for machineNo, plan := range plans {
		itcKeys := fmtIdx.scanKeys(PlannedITCOf(plan))

		model := firstModel(itcKeys)
		if model == "" {
			model = planValue(plan, "Model", "MODEL", "Machine Model", "Assembly_Parts_Name", "Assembly Parts Name")
		}
		if model == "" {
			model = "ไม่ระบุ Model"
		}

		specCode := planValue(plan, "Spec Code", "Product Spec 1")
		itDevice := plannedDeviceOf(plan)
		country := plannedCountryOf(plan)

		counted := false

		for _, comp := range qaScanComponentOrder {
			planned := PlannedNoOf(plan, comp)
			if comp == ComponentEN && planned == "" {
				planned = enginePlanByMachine[machineNo]
			}
			if planned == "" {

				continue
			}
			counted = true

			u := QAScanUnit{
				MachineNo:       fmtIdx.current(machineNo),
				MachineNoFormer: fmtIdx.formerCodes(machineNo),
				Model:           model,
				Component:       comp,
				ComponentLabel:  ComponentLabel(comp),
				PlannedNo:       fmtIdx.current(planned),
				PlannedNoFormer: fmtIdx.formerCodes(planned),
				SpecCode:        specCode,
				ITDevice:        itDevice,
				Country:         country,
			}

			if ck, matched, attempted := lookupCheck(comp, machineNo, planned); attempted {
				u.Scanned = matched
				u.ScanAttempted = true
				u.ScannedNo = strings.TrimSpace(ck.SN)
				u.ScannedPN = strings.TrimSpace(ck.PN)
				if ck.MatchStatus != models.MatchStatusRetiredFormat {
					u.ScannedNo = fmtIdx.current(u.ScannedNo)
					u.ScannedPN = fmtIdx.current(u.ScannedPN)
				}
				u.ScannedAt = rfc3339(ck.CheckedDatetime)
				u.ScannedBy = strings.TrimSpace(ck.CheckedBy)
				u.MatchStatus = ck.MatchStatus
				u.MatchMessage = ck.MatchMessage
				u.LicenseNo = strings.TrimSpace(ck.LicenseNo)
				u.InvoiceNo = strings.TrimSpace(ck.InvoiceNo)
			}

			if u.LicenseNo == "" || u.InvoiceNo == "" {
				if lic, ok := firstLicense(itcKeys); ok {
					if u.LicenseNo == "" {
						u.LicenseNo = strings.TrimSpace(lic.LicenseNo)
					}
					if u.InvoiceNo == "" {
						u.InvoiceNo = strings.TrimSpace(lic.InvoiceNo)
					}
				}
			}

			if m, assembled, attempted := lookupMFG(machineNo, planned); attempted {
				u.Assembled = assembled
				u.AssembleAttempted = true
				u.AssembledAt = rfc3339(m.CreatedDatetime)
				if m.DateAssembly != nil {
					u.AssembledAt = rfc3339Ptr(m.DateAssembly)
				}
				u.AssembledBy = strings.TrimSpace(m.CreatedBy)
				u.AssembledStatus = strings.TrimSpace(m.Status)
			}

			units = append(units, u)
		}

		if counted {
			machines++
		}
	}

	comps := make([]QAScanComponentMeta, 0, len(qaScanComponentOrder))
	for _, code := range qaScanComponentOrder {
		comps = append(comps, QAScanComponentMeta{Code: code, Label: ComponentLabel(code)})
	}

	c.JSON(200, QAPartScanSummaryResponse{
		Components:  comps,
		Units:       units,
		Machines:    machines,
		GeneratedAt: time.Now().Format(time.RFC3339),
	})
}
