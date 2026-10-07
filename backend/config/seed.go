package config

import (
	"log"

	"iconfirm/models"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(plain string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("[seed] hash password ไม่สำเร็จ: %v", err)
	}
	return string(hash)
}

func MigratePlaintextPasswords() {
	var users []models.User
	if err := DB.Find(&users).Error; err != nil {
		log.Println("[migrate] อ่านรายชื่อ user ไม่สำเร็จ:", err)
		return
	}

	migrated := 0
	for _, u := range users {
		if len(u.Password) >= 4 && (u.Password[:4] == "$2a$" || u.Password[:4] == "$2b$" || u.Password[:4] == "$2y$") {
			continue
		}

		hashed := hashPassword(u.Password)
		if err := DB.Model(&models.User{}).Where("id = ?", u.ID).Update("password", hashed).Error; err != nil {
			log.Printf("[migrate] hash password ของ user id=%d ไม่สำเร็จ: %v\n", u.ID, err)
			continue
		}
		migrated++
	}

	if migrated > 0 {
		log.Printf("[migrate] แปลง plaintext password เป็น bcrypt hash แล้ว %d user\n", migrated)
	}
}

type orgUser struct {
	Role     string
	Username string
	Password string
	Name     string
}

func SeedOrgUsers() {
	DB.Model(&models.User{}).Where("role_name = ?", "WH_MANAGER").Update("role_name", "LOG")
	DB.Model(&models.User{}).
		Where("username = ?", "whmanage@kobelco.com").
		Updates(map[string]interface{}{
			"role_name": "LOG",
			"username":  "log@kobelco.com",
			"name":      "LOG User",
			"password":  hashPassword("log.kobelco"),
		})

	list := []orgUser{
		{"ADMIN", "admin", "iconfirm", "Administrator"},
		{"UPLOAD", "upload", "upload", "Upload User"},
		{"LOG", "log@kobelco.com", "log.kobelco", "LOG User"},
		{"LOG", "log@kobelco.com", "log01.kobelco", "Panpaporn Prommapong"},
		{"LOG", "log@kobelco.com", "log02.kobelco", "Sarai Promden"},

		{"WH", "wh@kobelco.com", "wh01.kobelco", "นายวสันต์ มีฤทธิ์"},
		{"WH", "wh@kobelco.com", "wh02.kobelco", "นายอัมรินทร์ สุขแสวง"},
		{"WH", "wh@kobelco.com", "wh03.kobelco", "นายบุญมี บุญทาทอง"},
		{"WH", "wh@kobelco.com", "wh04.kobelco", "นายสุระทิน ชารี"},
		{"WH", "wh@kobelco.com", "wh05.kobelco", "นายกิตติศักดิ์ ศรีบุญเรือง"},
		{"WH", "wh@kobelco.com", "wh06.kobelco", "นายอนันตเดช เอี่ยมสะอาด"},

		{"MFG", "mfg@kobelco.com", "mfg01.kobelco", "นายหนูวิน ใจเรา"},
		{"MFG", "mfg@kobelco.com", "mfg02.kobelco", "นายวิชัย นิลนามะ"},
		{"MFG", "mfg@kobelco.com", "mfg03.kobelco", "นายอนุกูล วงแสนสุข"},
		{"MFG", "mfg@kobelco.com", "mfg04.kobelco", "นายชัยวัฒน์ บัวนาค"},
		{"MFG", "mfg@kobelco.com", "mfg05.kobelco", "นายวิชา จันทร์เส็ง"},
		{"MFG", "mfg@kobelco.com", "mfg06.kobelco", "นายเพชร มุนินทร์"},
		{"MFG", "mfg@kobelco.com", "mfg07.kobelco", "นายวัชรกรณ์ วงเวียน"},
		{"MFG", "mfg@kobelco.com", "mfg08.kobelco", "นางสาววิชุดา นามจันโท"},
		{"MFG", "mfg@kobelco.com", "mfg09.kobelco", "นายนครินทร์ พันที"},
		{"MFG", "mfg@kobelco.com", "mfg10.kobelco", "นายวรภพ มมประโคน"},
		{"MFG", "mfg@kobelco.com", "mfg11.kobelco", "นายอิทธิเดช คำหอม"},
		{"MFG", "mfg@kobelco.com", "mfg12.kobelco", "นายสรวิชญ์ เวียงธิเบต"},
		{"MFG", "mfg@kobelco.com", "mfg13.kobelco", "นายสมบัติ แซ่อึ้ง"},
		{"MFG", "mfg@kobelco.com", "mfg14.kobelco", "นายธวัฒน์ เสนา"},
		{"MFG", "mfg@kobelco.com", "mfg15.kobelco", "นายมงคลวัฒน์ จรัญเสริฐ"},
		{"MFG", "mfg@kobelco.com", "mfg16.kobelco", "นายธงชัย หาญยิ่ง"},
		{"MFG", "mfg@kobelco.com", "mfg17.kobelco", "นายรุ่งทิวา หาประโคน"},
		{"MFG", "mfg@kobelco.com", "mfg18.kobelco", "นายอภิสิทธ์ ชะเทียนรัมย์"},
		{"MFG", "mfg@kobelco.com", "mfg19.kobelco", "นายศักดา เจริญธรรม"},
		{"MFG", "mfg@kobelco.com", "mfg20.kobelco", "นายณัฐพงษ์ เรืองปะคำ"},
		{"MFG", "mfg@kobelco.com", "mfg21.kobelco", "นายกิตติศักดิ์ สร้อยเพชร"},
	}

	created := 0
	for _, u := range list {
		var cnt int64
		DB.Model(&models.User{}).Where("username = ? AND name = ?", u.Username, u.Name).Count(&cnt)
		if cnt > 0 {
			continue
		}
		DB.Create(&models.User{
			RoleName: u.Role,
			Username: u.Username,
			Password: hashPassword(u.Password),
			Status:   "Active",
			Name:     u.Name,
		})
		created++
	}

	if created > 0 {
		log.Printf("[seed] เพิ่มผู้ใช้องค์กรใหม่ %d คน (admin/UPLOAD/LOG/WH/MFG)", created)
	}
}

func SeedData() {

	var count int64

	DB.Model(&models.User{}).Count(&count)

	if count > 0 {
		return
	}

	users := []models.User{
		{
			RoleName: "QA",
			Username: "qa@kobelco.com",
			Password: hashPassword("qa.kobelco"),
			Status:   "Active",
			Name:     "QA User",
		},
		{
			RoleName: "WH",
			Username: "wh@kobelco.com",
			Password: hashPassword("wh.kobelco"),
			Status:   "Active",
			Name:     "Warehouse User",
		},
		{
			RoleName: "LOG",
			Username: "log@kobelco.com",
			Password: hashPassword("log.kobelco"),
			Status:   "Active",
			Name:     "LOG User",
		},
		{
			RoleName: "ADMIN",
			Username: "admin",
			Password: hashPassword("iconfirm"),
			Status:   "Active",
			Name:     "Administrator",
		},
		{
			RoleName: "UPLOAD",
			Username: "upload",
			Password: hashPassword("upload"),
			Status:   "Active",
			Name:     "Upload User",
		},
		{
			RoleName: "MFG",
			Username: "mfg@kobelco.com",
			Password: hashPassword("mfg.kobelco"),
			Status:   "Active",
			Name:     "MFG",
		},
	}

	DB.Create(&users)

}

