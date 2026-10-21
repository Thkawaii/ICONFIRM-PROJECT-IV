package controllers

import (
	"strings"

	"iconfirm/config"
	"iconfirm/models"
)

// ---------------------------------------------------------------------------
// เทียบของที่ WH สแกน กับทะเบียนใบอนุญาตนำเข้า
//
// ไฟล์ "บัญชีแสดงหมายเลขเครื่องใบอนุญาตนำเข้า" มีสองช่องที่ WH สแกนได้จากกล่องจริง
//
//	PART NO.        → P/N ของ IT Controller  เช่น YN22E00849FA
//	หมายเลขเครื่อง  → S/N ของตัวนั้น         เช่น 878270000101
//
// และมีช่อง "ส่งออกไปประเทศ" บอกว่าของกล่องนี้ถูกอนุญาตให้ส่งไปประเทศไหน
//
// เดิม P/N + S/N ที่ถูกต้องมาจากไฟล์ Planning WH ซึ่งระบุไว้รายเครื่อง
// ไฟล์นั้นเลิกใช้แล้ว ทะเบียนใบนำเข้าจึงกลายเป็นแหล่งความจริงแทน
//
// สิ่งที่ตรวจมี 3 ชั้น เรียงจากหยาบไปละเอียด
//
//	1. S/N ที่สแกนมีอยู่ในทะเบียนไหม — ถ้าไม่มี แปลว่าของกล่องนี้ไม่ได้นำเข้าถูกต้อง
//	2. P/N ที่สแกน ตรงกับ PART NO. ของ S/N นั้นไหม — กันหยิบกล่องผิดรุ่น
//	3. ประเทศปลายทางของเครื่องที่กำลังจ่าย ตรงกับ "ส่งออกไปประเทศ" ของกล่องนั้นไหม
//
// ข้อ 3 สำคัญที่สุดและเป็นข้อที่คนมองไม่เห็นด้วยตา — ของที่หน้าตาเหมือนกันเป๊ะ
// แต่ขออนุญาตส่งไปคนละประเทศ ถ้าหยิบสลับกันคือส่งออกผิดใบอนุญาต
// ---------------------------------------------------------------------------

// licenseScanCheck = ผลการเทียบกับทะเบียนใบนำเข้า
type licenseScanCheck struct {
	// Problem = ข้อความบอกว่าไม่ผ่านเพราะอะไร ว่าง = ผ่าน
	Problem string

	// Field = ช่องที่ผิด (P/N · S/N# · ประเทศ) ใช้ลง audit log
	Field string

	// Scanned / Expected = คู่ค่าที่ผิดกับค่าที่ถูก เก็บไว้ตรวจย้อนหลัง
	Scanned  string
	Expected string

	// NotFound = ไม่พบในทะเบียนเลย (ต่างจาก "พบแต่ค่าไม่ตรง")
	// ใช้เลือกข้อความที่จะขึ้นหน้าจอ
	NotFound bool

	// Serial = S/N ที่ยิงมา เก็บไว้ให้หน้าเว็บบอกได้ว่า
	// "S/N ตัวนี้ใช้กับ P/N ตัวไหน" ตอนที่ P/N ไม่ตรง
	Serial string

	// Item = แถวในทะเบียนที่จับคู่ได้ (nil = ไม่เจอ)
	Item *models.LicenseItem
}

// licenseItemBySerial: หาแถวในทะเบียนใบนำเข้าจาก S/N ที่สแกนได้
//
// เทียบแบบ normalize เพราะไฟล์กับสติกเกอร์เว้นวรรค/ขีดไม่เหมือนกัน
func licenseItemBySerial(serial string) *models.LicenseItem {
	want := NormalizeCodeValue(serial)
	if want == "" {
		return nil
	}

	// ลองจับตรง ๆ ก่อน เร็วกว่าและครอบคลุมเกือบทุกกรณี
	var row models.LicenseItem
	if err := config.DB.Where("machine_no = ?", strings.TrimSpace(serial)).First(&row).Error; err == nil {
		return &row
	}

	// ไม่เจอแบบตรงตัว — ไล่เทียบแบบ normalize
	var rows []models.LicenseItem
	if err := config.DB.Find(&rows).Error; err != nil {
		return nil
	}
	for i := range rows {
		if NormalizeCodeValue(rows[i].MachineNo) == want {
			return &rows[i]
		}
	}
	return nil
}

// checkScanAgainstLicense: เทียบ P/N + S/N ที่สแกน และประเทศปลายทาง กับทะเบียนใบนำเข้า
//
// planCountry = ประเทศของเครื่องที่กำลังจ่าย (คอลัมน์ Customer ในแผน Planning PCP)
// ว่างได้ — ถ้าแผนไม่ได้ระบุประเทศไว้ จะข้ามการตรวจข้อ 3 แทนที่จะเดาว่าผิด
func checkScanAgainstLicense(scannedPN, scannedSN, planCountry string) licenseScanCheck {
	scannedSN = strings.TrimSpace(scannedSN)
	scannedPN = strings.TrimSpace(scannedPN)
	if scannedSN == "" {
		return licenseScanCheck{}
	}

	item := licenseItemBySerial(scannedSN)
	if item == nil {
		// ใช้ Field ของตัวเอง ไม่ใช่ "S/N#" เพื่อแยกจากกรณี "มีในทะเบียนแต่ไม่ตรง"
		//
		// สองอย่างนี้คนละเรื่องกันสิ้นเชิง
		//   ไม่พบในทะเบียน   ยังไม่ได้อัปไฟล์ หรือยิงเลขผิดบรรทัดบนสติกเกอร์
		//   มีแต่ไม่ตรง      ของผิดตัวจริง
		// ถ้าใช้ชื่อช่องเดียวกัน หน้าเว็บจะแยกไม่ออกว่าควรบอกคนหน้างานยังไง
		return licenseScanCheck{
			Problem:  "ไม่พบ S/N " + scannedSN + " ในทะเบียนใบอนุญาตนำเข้า — ตรวจว่าอัปโหลดบัญชีแสดงหมายเลขเครื่องของล็อตนี้แล้วหรือยัง",
			Field:    "ไม่พบในทะเบียน",
			Scanned:  scannedSN,
			NotFound: true,
		}
	}

	// ทะเบียนบางแถวอาจยังไม่มี PART NO. (ไฟล์เก่าที่อัปก่อนระบบเก็บช่องนี้)
	// ข้ามไป ดีกว่าปฏิเสธการจ่ายของด้วยข้อมูลที่ยังไม่ครบ
	if scannedPN != "" && strings.TrimSpace(item.PartNo) != "" &&
		!SameCode(scannedPN, item.PartNo) {
		return licenseScanCheck{
			Problem:  "ทะเบียนใบนำเข้าระบุว่า S/N " + item.MachineNo + " คู่กับ P/N " + item.PartNo + " แต่สแกนได้ " + scannedPN,
			Field:    "P/N",
			Scanned:  scannedPN,
			Expected: item.PartNo,
			Serial:   item.MachineNo,
			Item:     item,
		}
	}

	if planCountry != "" && strings.TrimSpace(item.ExportCountry) != "" &&
		!sameCountryName(planCountry, item.ExportCountry) {
		return licenseScanCheck{
			Problem:  "กล่องนี้ขออนุญาตส่งออกไป " + item.ExportCountry + " แต่เครื่องที่กำลังจ่ายตามแผนส่งไป " + planCountry,
			Field:    "ประเทศ",
			Scanned:  item.ExportCountry,
			Expected: planCountry,
			Item:     item,
		}
	}

	return licenseScanCheck{Item: item}
}

// sameCountryName: ชื่อประเทศสองค่านี้หมายถึงที่เดียวกันหรือไม่
//
// ไฟล์สองฝั่งคนละคนกรอก ตัวพิมพ์และช่องว่างจึงไม่ตรงกัน
// ("Indonesia" · "INDONESIA" · "Indonesia ") แต่ไม่แปลงชื่อย่อให้ เช่น
// ANZ-AU กับ Australia ถือว่าคนละค่า เพราะเดาแทนแล้วผิดจะอันตรายกว่าถามคน
func sameCountryName(a, b string) bool {
	norm := func(s string) string {
		return strings.ToUpper(strings.Join(strings.Fields(s), " "))
	}
	return norm(a) == norm(b)
}
