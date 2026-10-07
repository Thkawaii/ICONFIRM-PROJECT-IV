package controllers

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"
)

// ---------------------------------------------------------------------------
// ปรับฐานข้อมูลให้ตรงกับสิ่งที่หน้าเว็บแสดง + ปิดใบที่หมดอายุแล้วไม่ได้ต่ออายุ
//
// ก่อนหน้านี้ความจริงของคำว่า "เสร็จสิ้น" อยู่คนละที่กันสามแห่ง:
//
//	ฐานข้อมูล   คอลัมน์ completed
//	หน้าเว็บ     isLicenseCompleted() — ดูคอลัมน์ completed ก่อน ถ้าไม่จริงค่อยอ่านหมายเหตุ
//	อีเมล        importItemDone()/exportItemDone() — กติกาเดียวกับหน้าเว็บ
//
// สองอันหลังอ่านหมายเหตุเป็นทางสำรอง ฐานข้อมูลจึงตามไม่ทัน: แถวที่ผู้ใช้พิมพ์คำว่า
// "เสร็จสิ้น" ไว้ในไฟล์ตั้งแต่ก่อนระบบจะอ่านหมายเหตุเป็น ยังมี completed = false
// อยู่ในฐานข้อมูล ทั้งที่บนเว็บขึ้นว่าเสร็จแล้ว — รายงานที่ query ตรงจากคอลัมน์
// (เช่นยอดนับในหน้า Dashboard) จึงได้ตัวเลขคนละชุดกับที่คนเห็นบนตาราง
//
// งานนี้เดินสองเรื่องในรอบเดียว:
//
//	1. ซิงก์คอลัมน์ completed ให้ตรงกับหมายเหตุ (ทั้งขาปิดและขาเปิดกลับ)
//	2. ปิดใบที่หมดอายุเกินกำหนดโดยไม่มีใครต่ออายุ
//
// ทำงานอัตโนมัติวันละครั้ง และกดสั่งเองได้จากหน้า Admin
// ---------------------------------------------------------------------------

// licenseAutoCloseDaysEnv = ตัวแปรใน .env ที่ตั้งระยะผ่อนผันก่อนปิดใบอัตโนมัติ
const licenseAutoCloseDaysEnv = "LICENSE_AUTO_CLOSE_DAYS"

// completedByNoteMarker = ร่องรอยของการปิดใบโดยอ่านจากช่องหมายเหตุ
const completedByNoteMarker = "ระบบ (อ่านจากหมายเหตุ)"

// systemCompletedBy: การปิดใบครั้งนั้นเป็นฝีมือระบบ ไม่ใช่คนกดเอง
//
// สำคัญตอน "เปิดกลับ": ถ้าผู้ใช้กดปิดงานเองในหน้าเว็บโดยไม่ได้พิมพ์หมายเหตุ
// แถวนั้นต้องคงสถานะเสร็จสิ้นไว้ ห้ามให้งานนี้ไปเปิดกลับให้
func systemCompletedBy(by string) bool {
	switch strings.TrimSpace(by) {
	case completedByNoteMarker, models.LicenseAutoCloseBy:
		return true
	}
	return false
}

// LicenseAutoCloseDays: ระยะผ่อนผันก่อนถือว่าใบที่ไม่ได้ต่ออายุปิดจบ
//
//	30        = หมดอายุเกิน 30 วันแล้วค่อยปิด (ค่าเริ่มต้น)
//	0         = พอหมดอายุก็ปิดเลย ไม่ต้องรอ
//	off/-1    = ไม่ปิดให้อัตโนมัติ
//
// ค่าที่พิมพ์ผิดจะถอยไปใช้ค่าเริ่มต้น ไม่ใช่ปิดการทำงาน — เพราะการพิมพ์ผิดแล้ว
// ระบบเงียบไปเฉย ๆ อันตรายกว่าการที่มันยังทำงานตามค่าเริ่มต้น
func LicenseAutoCloseDays() int {
	raw := strings.TrimSpace(os.Getenv(licenseAutoCloseDaysEnv))
	if raw == "" {
		return models.DefaultLicenseAutoCloseDays
	}
	switch strings.ToLower(raw) {
	case "off", "false", "no", "never", "disabled":
		return models.LicenseAutoCloseDisabled
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return models.DefaultLicenseAutoCloseDays
	}
	if n < 0 {
		return models.LicenseAutoCloseDisabled
	}
	return n
}

// LicenseAutoClosedRow = ใบที่ถูกปิดอัตโนมัติในรอบนี้ (ไว้รายงานให้ผู้ดูแลดู)
type LicenseAutoClosedRow struct {
	LicenseType  string `json:"licenseType"`
	LicenseNo    string `json:"licenseNo"`
	OverdueDays  int    `json:"overdueDays"`
	ClosedItems  int    `json:"closedItems"`
	RenewalCount int    `json:"renewalCount"`
}

// LicenseReconcileResult = สรุปผลของการเดินงานหนึ่งรอบ
type LicenseReconcileResult struct {
	RanAt         time.Time `json:"ranAt"`
	AutoCloseDays int       `json:"autoCloseDays"`

	// ซิงก์คอลัมน์ completed ให้ตรงกับหมายเหตุ
	ItemsClosedByNote int `json:"itemsClosedByNote"`
	ItemsReopened     int `json:"itemsReopened"`

	// ปิดใบที่ไม่ได้ต่ออายุ
	AutoClosedLicenses int                    `json:"autoClosedLicenses"`
	AutoClosedItems    int                    `json:"autoClosedItems"`
	AutoClosed         []LicenseAutoClosedRow `json:"autoClosed"`
}

// Changed: รอบนี้มีอะไรเปลี่ยนในฐานข้อมูลไหม
func (r LicenseReconcileResult) Changed() int {
	return r.ItemsClosedByNote + r.ItemsReopened + r.AutoClosedItems
}

// ReconcileLicenseCompletion เดินงานทั้งสองเรื่องหนึ่งรอบ แล้วคืนสรุปผล
func ReconcileLicenseCompletion(now time.Time) LicenseReconcileResult {
	result := LicenseReconcileResult{
		RanAt:         now,
		AutoCloseDays: LicenseAutoCloseDays(),
		AutoClosed:    []LicenseAutoClosedRow{},
	}
	if config.DB == nil {
		return result
	}

	syncLicenseItemNoteStatus(&result, now)
	autoCloseUnrenewedLicenses(&result, now)

	return result
}

// syncLicenseItemNoteStatus: ทำให้คอลัมน์ completed ของทะเบียนเครื่อง ตรงกับช่องหมายเหตุ
//
// เครื่องหนึ่งแถวมีสถานะเสร็จสิ้นเดียว ใช้ร่วมกันทั้งฝั่งใบนำเข้าและใบนำออก
// เพราะในทางปฏิบัติเครื่องจะ "จบงาน" ก็ต่อเมื่อถูกส่งออกไปแล้ว
func syncLicenseItemNoteStatus(result *LicenseReconcileResult, now time.Time) {
	var items []models.LicenseItem
	if err := config.DB.Find(&items).Error; err != nil {
		return
	}

	for i := range items {
		it := items[i]
		wantDone := models.NoteMeansCompleted(it.Remark)

		switch {
		case wantDone && !it.Completed:
			at := now
			config.DB.Model(&models.LicenseItem{}).Where("id = ?", it.ID).
				Updates(map[string]interface{}{
					"completed":    true,
					"completed_by": firstNonEmpty(it.CompletedBy, completedByNoteMarker),
					"completed_at": &at,
				})
			result.ItemsClosedByNote++

		case !wantDone && it.Completed && systemCompletedBy(it.CompletedBy):
			// หมายเหตุถูกลบคำว่า "เสร็จสิ้น" ออกแล้ว — ต้องย้อนสถานะกลับให้ตรงกับไฟล์
			config.DB.Model(&models.LicenseItem{}).Where("id = ?", it.ID).
				Updates(map[string]interface{}{
					"completed":    false,
					"completed_by": "",
					"completed_at": nil,
				})
			result.ItemsReopened++
		}
	}
}

// autoCloseUnrenewedLicenses: ปิดใบที่หมดอายุเกินกำหนดโดยไม่มีใบต่ออายุใบใหม่
//
// ใช้ buildLicenseOverview() เป็นต้นทาง = ความจริงชุดเดียวกับที่คนเห็นบนหน้าเว็บ
// ใบที่ถูกต่ออายุไปแล้วจะไม่โผล่ในรายการนี้อยู่แล้ว (ไปเป็นประวัติของใบใหม่แทน)
// ที่เหลือจึงเป็นใบปลายโซ่จริง ๆ — หมดอายุแล้วไม่มีใครต่อ = งานจบ
func autoCloseUnrenewedLicenses(result *LicenseReconcileResult, now time.Time) {
	days := result.AutoCloseDays
	if days < 0 {
		return
	}

	for _, row := range buildLicenseOverview() {
		if row.Completed {
			continue
		}
		if !models.LicenseAutoCloseDue(row.ExpiryDate, days, now) {
			continue
		}

		// ปิดทุกเลขใบในโซ่ เพราะรายการเครื่องอาจผูกอยู่กับใบเก่าก่อนต่ออายุ
		nos := append([]string{row.CurrentLicenseNo}, row.PreviousLicenseNos...)
		closed := closeLicenseItems(row.LicenseType, nos, now)

		result.AutoClosedLicenses++
		result.AutoClosedItems += closed
		result.AutoClosed = append(result.AutoClosed, LicenseAutoClosedRow{
			LicenseType:  row.LicenseType,
			LicenseNo:    row.CurrentLicenseNo,
			OverdueDays:  models.LicenseAutoCloseOverdueDays(row.ExpiryDate, now),
			ClosedItems:  closed,
			RenewalCount: row.RenewalCount,
		})
	}
}

// closeLicenseItems: ติ๊กเสร็จสิ้นให้ทุกเครื่องที่ยังค้างอยู่บนเลขใบเหล่านี้
//
// เทียบเลขใบแบบ normalize เพราะไฟล์ที่อัปโหลดเข้ามาคนละรอบ เขียนเว้นวรรค
// และตัวพิมพ์ไม่เหมือนกัน ถ้าเทียบตรง ๆ จะปิดไม่ครบ
func closeLicenseItems(licenseType string, licenseNos []string, now time.Time) int {
	keys := map[string]bool{}
	for _, no := range licenseNos {
		if k := NormalizeCodeValue(no); k != "" {
			keys[k] = true
		}
	}
	if len(keys) == 0 {
		return 0
	}

	at := now
	updates := map[string]interface{}{
		"completed":    true,
		"completed_by": models.LicenseAutoCloseBy,
		"completed_at": &at,
	}

	n := 0

	var items []models.LicenseItem
	if err := config.DB.Where("completed IS NOT TRUE").Find(&items).Error; err != nil {
		return 0
	}

	// ฝั่งนำเข้าเทียบคอลัมน์ license_no ฝั่งนำออกเทียบ export_license_no
	// สองฝั่งอยู่ในแถวเดียวกัน เพราะเครื่องหนึ่งเครื่องมีทั้งใบนำเข้าและใบนำออกของตัวเอง
	for _, it := range items {
		no := it.LicenseNo
		if licenseType == models.LicenseTypeExport {
			no = it.ExportLicenseNo
		}
		if !keys[NormalizeCodeValue(no)] {
			continue
		}
		if config.DB.Model(&models.LicenseItem{}).Where("id = ?", it.ID).
			Updates(updates).Error == nil {
			n++
		}
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// RunLicenseReconcile: เดินงานหนึ่งรอบแล้วเขียน log — ใช้โดยตัวตั้งเวลา
func RunLicenseReconcile(trigger string) LicenseReconcileResult {
	result := ReconcileLicenseCompletion(time.Now())

	if result.Changed() == 0 {
		log.Printf("[license-reconcile] (%s) ตรวจแล้ว ฐานข้อมูลตรงกับหน้าเว็บอยู่แล้ว ไม่มีอะไรต้องแก้", trigger)
		return result
	}

	log.Printf("[license-reconcile] (%s) ปรับให้ตรงกับหน้าเว็บแล้ว — ปิดตามหมายเหตุ %d เครื่อง · เปิดกลับ %d เครื่อง",
		trigger, result.ItemsClosedByNote, result.ItemsReopened)

	if result.AutoClosedLicenses > 0 {
		log.Printf("[license-reconcile] ปิดใบที่หมดอายุเกิน %d วันโดยไม่ได้ต่ออายุ %d ใบ (%d เครื่อง)",
			result.AutoCloseDays, result.AutoClosedLicenses, result.AutoClosedItems)
		for _, r := range result.AutoClosed {
			log.Printf("[license-reconcile]   - %s %s หมดอายุมาแล้ว %d วัน → ปิดจบ",
				models.LicenseTypeLabel(r.LicenseType), r.LicenseNo, r.OverdueDays)
		}
	}
	return result
}

