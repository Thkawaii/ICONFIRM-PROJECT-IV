package models

import "time"

// ---------------------------------------------------------------------------
// LicenseRenewal = ทะเบียนใบอนุญาต (ชีต "ต่ออายุ")
//
// 1 แถว = ใบอนุญาตนำออก 1 ใบ ที่ออกจากโควต้าของใบอนุญาตนำเข้าใบหนึ่ง
// ใบนำเข้า 1 ใบจึงมีได้หลายแถว = ต่ออายุหลายครั้ง (ในข้อมูลจริงมีถึง 16 ครั้ง)
//
// ไฟล์ Excel เป็นแหล่งข้อมูลหลัก — ผู้ใช้ทำงานใน Excel ตามเดิม
// ระบบอ่านมาเพื่อ "เช็คสถานะ" อย่างเดียว: หมดอายุยัง / ใกล้หมดยัง / ต่ออายุไปกี่ครั้งแล้ว
// การอัปโหลดซิงก์รายแถวตามคีย์ (ดู controllers/license_register_merge.go)
// ---------------------------------------------------------------------------

type LicenseRenewal struct {
	ID uint `gorm:"primaryKey"`

	// GroupNo = คอลัมน์ NO. ในชีต เช่น "Completed 01" / "117"
	GroupNo string `gorm:"column:group_no;size:50;index"`

	// ITControllerModel = คอลัมน์ IT CONTROLLER MODEL (เป็น P/N ของ IT controller)
	ITControllerModel string `gorm:"column:it_controller_model;size:60;index"`

	ImportLicenseNo string `gorm:"column:import_license_no;size:60;index"`
	ExportLicenseNo string `gorm:"column:export_license_no;size:60;index"`

	Country string `gorm:"column:country;size:120"`

	// Note = ข้อความที่ผู้ใช้จดไว้ในช่องเลขใบอนุญาตแทนตัวเลขใบ
	// เช่น "วันที่ส่ง E-mail : Thu 19/12/2024 13:54 / เจ้าหน้าที่หลุดเมลล์..."
	// ไม่ใช่เลขใบ จึงเก็บแยกไว้เป็นหมายเหตุ แล้วนำไปแสดงในประวัติการต่ออายุตามลำดับเดิมในไฟล์
	Note string `gorm:"column:note;type:text"`

	// Total = โควต้าของใบนำเข้า (คอลัมน์ TOTAL)
	Total int `gorm:"column:total"`

	IssueDate  *time.Time `gorm:"column:issue_date;index"`
	ExpireDate *time.Time `gorm:"column:expire_date;index"`

	// Stock = จำนวนที่อยู่บนใบนำออกใบนี้ · Remain = ยอดคงเหลือที่ยังใช้ได้
	Stock  int `gorm:"column:stock"`
	Remain int `gorm:"column:remain"`

	// HasRemain = ช่อง REMAIN มีตัวเลขกรอกไว้จริงหรือไม่
	//
	// จำเป็นเพราะช่องว่างกับเลข 0 เก็บลงคอลัมน์ remain เหมือนกันหมด
	// แต่ความหมายต่างกันคนละเรื่อง: 0 = นำออกหมดแล้ว ส่วนว่าง = ยังไม่ได้กรอก
	// ถ้าไม่แยกไว้ แถวที่ยังไม่กรอกจะถูกนับว่าปิดงานแล้วทั้งหมด
	HasRemain bool `gorm:"column:has_remain;default:false"`

	// ขั้นตอนขอต่ออายุ: ส่งอีเมล → จ่ายเงิน → รับเอกสาร
	EmailDate    *time.Time `gorm:"column:email_date"`
	PaymentDate  *time.Time `gorm:"column:payment_date"`
	ReceivedDate *time.Time `gorm:"column:received_date"`

	// ManualEntry = แถวนี้ผู้ใช้กรอกในระบบเอง ไม่ได้มาจากไฟล์ Excel
	//
	// การซิงก์ไฟล์ไม่ลบแถวที่กรอกเอง (ยกเว้นคีย์ตรงกับไฟล์ ซึ่งไฟล์ชนะ)
	ManualEntry bool `gorm:"column:manual_entry;index;default:false"`

	// ExtraJSON = คอลัมน์ในไฟล์ที่ระบบไม่รู้จัก เก็บไว้ทั้งหมดเป็น JSON
	//
	// ชื่อคอลัมน์ติดป้าย "[+] " นำหน้าไว้ เพื่อแยกจากคอลัมน์มาตรฐานเวลาแสดงผล
	// เก็บเป็นข้อความตามที่พิมพ์มา ไม่ได้แปลงชนิด จึงนำไปคำนวณต่อไม่ได้
	//
	// ของเดิมชีตต่ออายุทิ้งคอลัมน์ที่ไม่รู้จักไปเงียบ ๆ ต่างจากชีตนำเข้า/นำออก
	// ที่เก็บไว้ให้ ทั้งที่เป็นไฟล์เล่มเดียวกัน ผู้ใช้จึงเดาไม่ถูกว่าอันไหนเก็บอันไหนไม่เก็บ
	ExtraJSON string `gorm:"type:text" json:"extra_json"`

	FileName   string `gorm:"column:file_name;size:255"`
	UploadDate time.Time

	SortOrder int64 `gorm:"column:sort_order;index;default:0"`

	UserID uint
	User   User

	// ---- คำนวณตอนอ่าน ไม่ได้เก็บในฐานข้อมูล ----

	// RenewalRound = ใบนี้เป็นการต่ออายุครั้งที่เท่าไหร่ของใบนำเข้านั้น (1 = ใบแรก)
	RenewalRound int `gorm:"-" json:"RenewalRound"`
	// RenewalCount = ใบนำเข้านั้นต่ออายุไปแล้วทั้งหมดกี่ครั้ง
	RenewalCount int `gorm:"-" json:"RenewalCount"`

	DaysLeft *int   `gorm:"-" json:"DaysLeft"`
	Status   string `gorm:"-" json:"Status"`
	// RenewStep = ขั้นตอนขอต่ออายุที่ไปถึงแล้ว
	RenewStep string `gorm:"-" json:"RenewStep"`
}

// สถานะของใบอนุญาตนำออก 1 ใบ
const (
	LicenseStatusExpired   = "EXPIRED"    // หมดอายุแล้ว
	LicenseStatusExpiring  = "EXPIRING"   // ใกล้หมดอายุ
	LicenseStatusValid     = "VALID"      // ยังใช้ได้
	LicenseStatusNoLicense = "NO_LICENSE" // ยังไม่ได้ออกใบนำออก (มีแต่โควต้าใบนำเข้า)
	LicenseStatusNoDate    = "NO_DATE"    // มีเลขใบแต่ไม่มีวันหมดอายุ
	LicenseStatusUsedUp    = "USED_UP"    // โควต้าหมด (REMAIN <= 0) ทั้งที่ยังไม่หมดอายุ
	LicenseStatusOverUsed  = "OVER_USED"  // ใช้เกินโควต้า (REMAIN ติดลบ)
)

// ขั้นตอนขอต่ออายุ
const (
	RenewStepNone     = "NONE"     // ยังไม่ได้เริ่ม
	RenewStepEmail    = "EMAIL"    // ส่งอีเมลแล้ว รอจ่ายเงิน
	RenewStepPayment  = "PAYMENT"  // จ่ายเงินแล้ว รอเอกสาร
	RenewStepReceived = "RECEIVED" // ได้รับเอกสารแล้ว
)

// LicenseExpiringWithinDays = ช่วงที่ถือว่า "ใกล้หมดอายุ"
// ใบนำออกอายุประมาณ 29 วัน จึงใช้ 14 วันเป็นเกณฑ์เตือน
const LicenseExpiringWithinDays = 14

// LicenseRemainLowThreshold = โควต้าคงเหลือที่ถือว่าใกล้หมด
const LicenseRemainLowThreshold = 5
