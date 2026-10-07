package controllers

import (
	"encoding/json"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Scan lock — ข้อมูลที่อัปโหลดแก้ไข/ลบได้ตลอด ยกเว้น "ของที่สแกนผ่านไปแล้ว"
//
// นับเป็นสแกนแล้วเฉพาะผลที่ผ่าน:
//   - WH สแกนพาร์ท (part_checks) ที่ match_status = MATCH
//   - MFG สแกนประกอบ (mfg_assemblies) ที่ status = MATCHED
//   - Import License ที่ confirm_status = CONFIRMED
//
// สแกนแล้ว "ไม่ตรง" (NOT_FOUND / WRONG_PART / NOT_MATCHED ...) ยังแก้ได้
// เพื่อให้แก้ข้อมูลที่ผิดแล้วสแกนใหม่ได้
//
// Export License ไม่มีการสแกนที่อ้างถึงตารางนี้โดยตรง จึงแก้ได้ตลอด
// ---------------------------------------------------------------------------

const scanLockedMessage = "รายการนี้ถูกสแกนผ่านไปแล้ว แก้ไขหรือลบไม่ได้"

type scanLockIndex struct {
	fmt *codeFormatIndex

	// codes: S/N, เลข No. (ITC/SM/PH/MP/CV/CW), Engine ที่ถูกสแกนผ่าน → เหตุผล
	codes map[string]string
	// machines: หมายเลขเครื่องที่มีการสแกนผ่านแล้ว → เหตุผล
	machines map[string]string
	// importItems: license_items.id ที่ถูกยืนยันจากการสแกน
	importItems map[uint]string

	orderToMachine map[string]string
}

func scanReason(where string, at time.Time, by string) string {
	msg := "สแกนผ่านที่ " + where + " แล้ว"
	if !at.IsZero() {
		msg += " · " + at.Format("02/01/2006 15:04")
	}
	if strings.TrimSpace(by) != "" {
		msg += " · " + strings.TrimSpace(by)
	}
	return msg
}

func (x *scanLockIndex) addCode(raw, reason string) {
	for _, k := range x.fmt.scanKeys(raw) {
		if _, ok := x.codes[k]; !ok {
			x.codes[k] = reason
		}
	}
}

func (x *scanLockIndex) addMachine(raw, reason string) {
	for _, k := range x.fmt.scanKeys(raw) {
		if _, ok := x.machines[k]; !ok {
			x.machines[k] = reason
		}
	}
}

func buildScanLockIndex() *scanLockIndex {
	x := &scanLockIndex{
		fmt:            loadCodeFormatIndex(),
		codes:          map[string]string{},
		machines:       map[string]string{},
		importItems:    map[uint]string{},
		orderToMachine: map[string]string{},
	}

	var checks []models.PartCheck
	config.DB.Select("part_type", "pn", "sn", "machine_no", "import_license_item_id", "checked_by", "checked_datetime").
		Where("match_status = ?", models.MatchStatusMatch).
		Order("id asc").Find(&checks)
	for _, ck := range checks {
		reason := scanReason("WH", ck.CheckedDatetime, ck.CheckedBy)
		comp := strings.ToUpper(strings.TrimSpace(ck.PartType))
		x.addCode(ck.SN, reason)
		switch comp {
		case ComponentITC:
			// WH ITC: machine_no เก็บเลข IT Controller No.
			x.addCode(ck.MachineNo, reason)
		case ComponentEN:
			x.addCode(ck.PN, reason)
			x.addMachine(ck.MachineNo, reason)
		default:
			x.addMachine(ck.MachineNo, reason)
		}
		if ck.ImportLicenseItemID != nil {
			if _, ok := x.importItems[*ck.ImportLicenseItemID]; !ok {
				x.importItems[*ck.ImportLicenseItemID] = reason
			}
		}
	}

	var mfg []models.MFGAssembly
	config.DB.Select("machine_no", "no", "serial_no", "created_by", "created_datetime").
		Where("status = ?", models.MFGStatusMatched).
		Order("id asc").Find(&mfg)
	for _, r := range mfg {
		reason := scanReason("MFG", r.CreatedDatetime, r.CreatedBy)
		x.addMachine(r.MachineNo, reason)
		x.addCode(r.ITControllerNo, reason)
		x.addCode(r.SerialNo, reason)
	}

	return x
}

func (x *scanLockIndex) codeLocked(values ...string) (bool, string) {
	for _, v := range values {
		for _, k := range x.fmt.scanKeys(v) {
			if reason, ok := x.codes[k]; ok {
				return true, reason
			}
		}
	}
	return false, ""
}

func (x *scanLockIndex) machineLocked(machineNo string) (bool, string) {
	for _, k := range x.fmt.scanKeys(machineNo) {
		if reason, ok := x.machines[k]; ok {
			return true, reason
		}
	}
	return false, ""
}

// ---- ทะเบียนอะไหล่ (ALL PART) ----

// ---- Import License ----

func (x *scanLockIndex) importLicense(it *models.LicenseItem) (bool, string) {
	if reason, ok := x.importItems[it.ID]; ok && it.ID != 0 {
		return true, reason
	}
	if strings.EqualFold(strings.TrimSpace(it.ConfirmStatus), models.LicenseItemConfirmed) {
		reason := "ยืนยันจากการสแกนแล้ว"
		if it.ConfirmedDatetime != nil {
			reason = scanReason("WH", *it.ConfirmedDatetime, it.ConfirmedBy)
		}
		return true, reason
	}
	return x.codeLocked(it.MachineNo)
}

func (x *scanLockIndex) annotateImportLicense(rows []models.LicenseItem) {
	for i := range rows {
		rows[i].Locked, rows[i].LockReason = x.importLicense(&rows[i])
	}
}

// ---- Planning / WH1 / WH2 / Engine ----

// loadOrderToMachine: เลข Order/LOT ของแผนประกอบ → หมายเลขเครื่อง (ใช้โยง WH1/WH2 เข้าหาเครื่อง)
func (x *scanLockIndex) loadOrderToMachine() {
	if len(x.orderToMachine) > 0 {
		return
	}
	for _, p := range loadUploadRows(models.DatasetPlanning) {
		mc := machineFromRow(p)
		if mc == "" {
			continue
		}
		for _, raw := range []string{p["KCM Order"], p["Work order"], p["Order No"], p["LOT NO."]} {
			for _, k := range joinKeyVariants(raw) {
				if _, ok := x.orderToMachine[k]; !ok {
					x.orderToMachine[k] = mc
				}
			}
		}
	}
}

func (x *scanLockIndex) uploadRow(dataset string, data map[string]string) (bool, string) {
	switch dataset {
	case models.DatasetPlanning:
		if ok, reason := x.machineLocked(machineFromRow(data)); ok {
			return true, reason
		}
		for _, spec := range componentSpecs {
			if v := planValue(data, spec.PlanKeys...); v != "" {
				if ok, reason := x.codeLocked(v); ok {
					return true, reason
				}
			}
		}
	case models.DatasetDailyPlan:
		return x.machineLocked(machineFromRow(data))
	case models.DatasetEngine:
		if ok, reason := x.machineLocked(pickField(data, "Machine No", "Machine")); ok {
			return true, reason
		}
		return x.codeLocked(pickField(data, "ENGINE", "Engine"), pickField(data, "History"))
	case models.DatasetWH1, models.DatasetWH2:
		if mc := machineFromRow(data); mc != "" {
			return x.machineLocked(mc)
		}
		x.loadOrderToMachine()
		for _, k := range orderKeysFromRow(data) {
			if mc, ok := x.orderToMachine[k]; ok {
				if locked, reason := x.machineLocked(mc); locked {
					return true, reason
				}
			}
		}
	}
	return false, ""
}

func (x *scanLockIndex) annotateUploadRows(rows []models.UploadDataRow) {
	for i := range rows {
		data := map[string]string{}
		_ = json.Unmarshal([]byte(rows[i].DataJSON), &data)
		rows[i].Locked, rows[i].LockReason = x.uploadRow(rows[i].Dataset, data)
	}
}

// respondLocked ตอบ 409 เมื่อพยายามแก้/ลบรายการที่สแกนแล้ว
func respondLocked(c *gin.Context, reason string) {
	c.JSON(409, gin.H{
		"message": scanLockedMessage + " (" + reason + ")",
		"locked":  true,
		"reason":  reason,
	})
}
