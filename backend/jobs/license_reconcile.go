package jobs

import (
	"log"
	"time"

	"iconfirm/controllers"
)

// ---------------------------------------------------------------------------
// ตัวตั้งเวลาของงานปรับฐานข้อมูลให้ตรงกับหน้าเว็บ + ปิดใบที่ไม่ได้ต่ออายุ
//
// เดินวันละครั้งก็พอ เพราะทั้งสองเรื่องเปลี่ยนตามวัน ไม่ใช่ตามนาที:
// หมายเหตุในไฟล์เปลี่ยนตอนอัปโหลด (ซึ่งซิงก์ทันทีอยู่แล้ว) ส่วนอายุใบนับเป็นวัน
//
// เดินหนึ่งรอบตอน backend เริ่มทำงานด้วย เพื่อให้ข้อมูลเก่าที่ค้างมาตั้งแต่ก่อนมี
// ฟีเจอร์นี้ ถูกปรับให้ตรงตั้งแต่ deploy ครั้งแรก ไม่ต้องรอข้ามคืน
// ---------------------------------------------------------------------------

const licenseReconcileInterval = 24 * time.Hour

// licenseReconcileStartupDelay = หน่วงให้ฐานข้อมูลพร้อมก่อน
//
// ตอน backend เพิ่งขึ้น การ migrate ยังอาจไม่จบ และงานนี้อ่านหลายตาราง
// ถ้าเดินทันทีอาจเจอตารางที่ยังสร้างไม่เสร็จ
const licenseReconcileStartupDelay = 30 * time.Second

func StartLicenseReconcileScheduler() {
	days := controllers.LicenseAutoCloseDays()
	switch {
	case days > 0:
		log.Printf("[license-reconcile] เปิดการปิดใบอัตโนมัติ — ใบที่หมดอายุเกิน %d วันโดยไม่ได้ต่ออายุ จะถือว่าปิดจบ", days)
		log.Printf("[license-reconcile] ระหว่างนี้ใบยังขึ้นว่า \"หมดอายุแล้ว\" และยังถูกแจ้งเตือนอยู่ — หน้า License Overview จะบอกว่าเหลืออีกกี่วันจึงจะปิด")
		log.Printf("[license-reconcile] ปรับได้ที่ LICENSE_AUTO_CLOSE_DAYS ในไฟล์ .env (0 = ปิดทันทีที่หมดอายุ · off = ไม่ปิดให้อัตโนมัติ)")
	case days == 0:
		log.Printf("[license-reconcile] เปิดการปิดใบอัตโนมัติ — ใบที่หมดอายุแล้วและไม่ได้ต่ออายุ จะถูกปิดจบทันที ไม่มีระยะผ่อนผัน")
	default:
		log.Printf("[license-reconcile] ปิดการปิดใบอัตโนมัติอยู่ (LICENSE_AUTO_CLOSE_DAYS=off) — ยังซิงก์สถานะเสร็จสิ้นกับช่องหมายเหตุให้เหมือนเดิม")
	}

	go func() {
		time.Sleep(licenseReconcileStartupDelay)
		controllers.RunLicenseReconcile("ตอนเริ่มระบบ")

		ticker := time.NewTicker(licenseReconcileInterval)
		defer ticker.Stop()
		for range ticker.C {
			controllers.RunLicenseReconcile("ตามรอบรายวัน")
		}
	}()
}
