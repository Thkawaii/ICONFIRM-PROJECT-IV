package controllers

import (
	"log"
	"time"

	"iconfirm/config"
	"iconfirm/models"
)

// FixOverflowedExpireDates: ซ่อมวันหมดอายุที่เคยคำนวณล้นเดือน
//
// ของเดิมบวกเดือนด้วย AddDate ตรง ๆ ซึ่งไม่ clamp สิ้นเดือน
// ออกใบ 31 ส.ค. บวก 6 เดือนได้ "31 ก.พ." ซึ่งไม่มีจริง แล้วล้นเป็น 3 มี.ค.
// ค่าผิดพวกนี้ถูกเขียนลงฐานข้อมูลไปแล้ว รีสตาร์ทเฉย ๆ จึงไม่หาย
//
// ตัวนี้ไล่ซ่อมให้ตอนระบบเริ่มทำงาน จะได้ไม่ต้องแตะฐานข้อมูลเอง
//
// แก้เฉพาะแถวที่ "ค่าในฐานข้อมูลตรงกับผลของสูตรเก่าเป๊ะ" เท่านั้น
//
//	ค่าเก่าที่คำนวณผิด   แก้
//	ค่าที่กรอกมาจากไฟล์   ไม่แตะ
//
// เงื่อนไขนี้แคบมากโดยตั้งใจ เพราะถ้าไล่แก้ทุกแถวที่ไม่เท่ากับสูตรใหม่
// จะไปทับวันที่กรมออกให้จริงซึ่งอาจไม่ใช่ 6 เดือนพอดี
func FixOverflowedExpireDates(trigger string) {
	var rows []models.LicenseItem
	if err := config.DB.
		Where("issue_date IS NOT NULL AND expire_date IS NOT NULL").
		Find(&rows).Error; err != nil {
		log.Printf("[expire-backfill] (%s) อ่านข้อมูลไม่ได้: %v", trigger, err)
		return
	}

	fixed := 0
	for i := range rows {
		iss := rows[i].IssueDate
		cur := rows[i].ExpireDate
		if iss == nil || cur == nil {
			continue
		}

		good := models.AddMonthsClamped(*iss, models.ImportLicenseValidityMonths)
		bad := iss.AddDate(0, models.ImportLicenseValidityMonths, 0)

		// สูตรเก่ากับสูตรใหม่ให้ผลเท่ากัน = แถวนี้ไม่เคยมีปัญหา
		if sameDay(good, bad) {
			continue
		}
		// ค่าในฐานข้อมูลไม่ใช่ผลของสูตรเก่า = คนกรอกมาเอง ไม่แตะ
		if !sameDay(*cur, bad) {
			continue
		}

		g := good
		if err := config.DB.Model(&models.LicenseItem{}).
			Where("id = ?", rows[i].ID).
			Update("expire_date", &g).Error; err != nil {
			log.Printf("[expire-backfill] แก้ id=%d ไม่สำเร็จ: %v", rows[i].ID, err)
			continue
		}
		fixed++
	}

	if fixed == 0 {
		log.Printf("[expire-backfill] (%s) ตรวจแล้ว ไม่มีวันหมดอายุที่คำนวณล้นเดือนค้างอยู่", trigger)
		return
	}
	log.Printf("[expire-backfill] (%s) ซ่อมวันหมดอายุที่เคยคำนวณล้นเดือน %d แถว", trigger, fixed)
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
