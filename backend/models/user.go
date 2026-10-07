package models

import "time"

type User struct {
	ID uint `gorm:"primaryKey"`

	RoleName string `gorm:"size:50"`

	Username string `gorm:"size:100;index"`

	Password string `gorm:"size:255"`

	Status string `gorm:"size:20"`

	Name string `gorm:"size:100"`

	// Email = ที่อยู่อีเมลของผู้ใช้ (ว่างได้)
	//
	// ไม่บังคับ เพราะผู้ใช้ฝั่งหน้างาน (QA / Warehouse) เข้าระบบด้วย username
	// และหลายคนไม่มีอีเมลบริษัท
	//
	// ถ้ากรอกไว้ ผู้ดูแลจะติ๊กให้รับรายงานใบอนุญาตรายสัปดาห์ได้ตั้งแต่ตอนสร้างบัญชี
	// โดยระบบจะเพิ่มชื่อเข้าตาราง mail_recipients ให้เอง ไม่ต้องไปเพิ่มซ้ำอีกหน้า
	Email string `gorm:"size:190;index"`

	CreatedAt time.Time
}
