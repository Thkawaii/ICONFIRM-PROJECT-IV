package config

import (
	"fmt"
	"iconfirm/models"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ConnectDB() {

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			getenv("DB_HOST", "localhost"),
			getenv("DB_USER", "postgres"),
			getenv("DB_PASSWORD", "Kobelco.com"),
			getenv("DB_NAME", "iconfirm"),
			getenv("DB_PORT", "5432"),
			getenv("DB_SSLMODE", "disable"),
		)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	DB = db

	RenameLegacyTables()
	DropRetiredTables()

	RenameCodeAliasColumns()
	MigrateCodeAliasOldValue()


	db.AutoMigrate(

		&models.User{},


		&models.AuditLog{},

		&models.PartCheck{},

		&models.LicenseItem{},


		&models.LicenseRenewal{},
		&models.LicenseRenewalHistory{},

		&models.UploadDataRow{},

		&models.MFGAssembly{},

		&models.ColumnAlias{},
		&models.CodeAlias{},

		&models.WeeklyAlertLog{},

		&models.MailRecipient{},
	)

	DropLegacyAssemblyDataset()

	DropRedundantItemColumns()

	DropUploadNoteColumns()

	BackfillUploadSortOrder()



	SeedData()

	SeedOrgUsers()

	SeedMailRecipients()

	MigratePlaintextPasswords()

	if os.Getenv("SEED_SAMPLE_ITC") == "1" {
	}

	log.Println("Database Connected")
}

func RenameCodeAliasColumns() {
	if DB == nil {
		return
	}

	rename := func(table, oldCol, newCol string) {
		var count int64
		if err := DB.Raw(
			`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`,
			table, oldCol,
		).Scan(&count).Error; err != nil {
			log.Println("check column for rename:", err)
			return
		}
		if count == 0 {
			return
		}
		if err := DB.Exec(`ALTER TABLE ` + table + ` RENAME COLUMN "` + oldCol + `" TO "` + newCol + `"`).Error; err != nil {
			log.Println("rename column", table, oldCol, "->", newCol, ":", err)
			return
		}
		log.Printf("Renamed column %s.%s -> %s", table, oldCol, newCol)
	}

	rename("change_format_parts", "from_code", "new")
	rename("change_format_parts", "from_norm", "old")
	rename("column_aliases", "scope", "table")
	rename("column_aliases", "source", "new")
	rename("column_aliases", "target", "old")
}

func MigrateCodeAliasOldValue() {
	if DB == nil {
		return
	}

	columnExists := func(table, col string) bool {
		var count int64
		if err := DB.Raw(
			`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`,
			table, col,
		).Scan(&count).Error; err != nil {
			log.Println("check column for drop:", err)
			return false
		}
		return count > 0
	}

	if columnExists("change_format_parts", "to_serial_no") {
		if err := DB.Exec(
			`UPDATE change_format_parts SET "old" = "to_serial_no" WHERE "to_serial_no" IS NOT NULL AND "to_serial_no" <> ''`,
		).Error; err != nil {
			log.Println("migrate change_format_parts.old <- to_serial_no:", err)
		} else {
			log.Println("Migrated change_format_parts.old <- to_serial_no")
		}
		if err := DB.Exec(`ALTER TABLE change_format_parts DROP COLUMN "to_serial_no"`).Error; err != nil {
			log.Println("drop column change_format_parts.to_serial_no:", err)
		} else {
			log.Println("Dropped column change_format_parts.to_serial_no")
		}
	}

	if columnExists("change_format_parts", "to_part_no") {
		if err := DB.Exec(`ALTER TABLE change_format_parts DROP COLUMN "to_part_no"`).Error; err != nil {
			log.Println("drop column change_format_parts.to_part_no:", err)
		} else {
			log.Println("Dropped column change_format_parts.to_part_no")
		}
	}
}

func DropLegacyAssemblyDataset() {
	if DB == nil {
		return
	}
	res := DB.Where("dataset = ?", "assembly").Delete(&models.UploadDataRow{})
	if res.Error != nil {
		log.Println("drop legacy assembly dataset:", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("Removed %d legacy assembly rows from upload_data_rows", res.RowsAffected)
	}
}

func DropRedundantItemColumns() {
	if DB == nil {
		return
	}

	columns := []struct{ table, column string }{
		{"mfg_assemblies", "item"},
		{"matching_assemblies", "item"},
		{"license_items", "item_no"},
	}

	for _, c := range columns {
		var count int64
		if err := DB.Raw(
			`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`,
			c.table, c.column,
		).Scan(&count).Error; err != nil {
			log.Println("check column", c.table+"."+c.column, ":", err)
			continue
		}
		if count == 0 {
			continue
		}
		if err := DB.Exec(`ALTER TABLE ` + c.table + ` DROP COLUMN "` + c.column + `"`).Error; err != nil {
			log.Println("drop column", c.table+"."+c.column, ":", err)
			continue
		}
		log.Printf("Dropped redundant column %s.%s", c.table, c.column)
	}
}

// BackfillUploadSortOrder ตั้งลำดับแสดงผลให้แถวเดิมที่ยังไม่มี (ตามลำดับที่เข้าระบบ)
// ใช้บล็อกละ 1,000,000 ต่อแถว เพื่อให้ไฟล์ที่อัปซ้ำเรียงแถวของตัวเองในบล็อกนั้นได้
func BackfillUploadSortOrder() {
	if DB == nil {
		return
	}
	for _, table := range []string{"upload_data_rows", "license_items"} {
		if !DB.Migrator().HasColumn(table, "sort_order") {
			continue
		}
		res := DB.Exec("UPDATE " + table + " SET sort_order = id * 1000000 WHERE sort_order IS NULL OR sort_order = 0")
		if res.Error != nil {
			log.Println("backfill sort_order", table, ":", res.Error)
		} else if res.RowsAffected > 0 {
			log.Printf("Backfilled %s.sort_order (%d rows)", table, res.RowsAffected)
		}
	}
}

// DropUploadNoteColumns ลบคอลัมน์ note ที่เคยเพิ่มไว้สำหรับ "ลบข้อมูลด้วยคำ Delete/ลบ ในช่อง Note"
// ฟีเจอร์นั้นเลิกใช้แล้ว (เปลี่ยนเป็นแก้ไข/ลบในตารางหน้าอัปโหลดโดยตรง) จึงไม่ต้องเก็บคอลัมน์นี้อีก
func DropUploadNoteColumns() {
	if DB == nil {
		return
	}
	// หมายเหตุ: export_license_items ไม่อยู่ในรายการนี้แล้ว
	// เพราะคอลัมน์ note ถูกนำกลับมาใช้เป็น "ตัวกำหนดสถานะ Checklist" (เสร็จแล้ว / ยังไม่เสร็จ)
	for _, table := range []string{"upload_data_rows", "license_items"} {
		if !DB.Migrator().HasColumn(table, "note") {
			continue
		}
		if err := DB.Migrator().DropColumn(table, "note"); err != nil {
			log.Println("drop column", table+".note", ":", err)
			continue
		}
		log.Printf("Dropped column %s.note (เลิกใช้ลบข้อมูลผ่านช่อง Note)", table)
	}
}

// ---------------------------------------------------------------------------
// เปลี่ยนชื่อตารางเก่าให้ตรงกับชื่อใหม่ในโค้ด
//
// ต้องทำ "ก่อน" AutoMigrate เสมอ เพราะ AutoMigrate เห็นว่ายังไม่มีตารางชื่อใหม่
// มันจะสร้างตารางเปล่าขึ้นมาให้ แล้วข้อมูลเดิมจะค้างอยู่ในตารางชื่อเก่าโดยไม่มีใครอ่าน
// ผู้ใช้จะเห็นเป็น "ข้อมูลหายทั้งหมด" ทั้งที่ยังอยู่ครบในฐานข้อมูล
//
// ทำงานได้ครั้งเดียว ถ้ามีตารางชื่อใหม่อยู่แล้วจะข้ามไป เรียกซ้ำกี่รอบก็ปลอดภัย
// ---------------------------------------------------------------------------

// legacyTableRenames = ชื่อเก่า -> ชื่อใหม่
//
// import_license_items -> license_items
//
//	ไฟล์บัญชีแสดงหมายเลขเครื่องมีทั้งเลขใบอนุญาตนำเข้าและเลขใบอนุญาตนำออก
//	อยู่ในแถวเดียวกัน ตารางนี้จึงเป็นทะเบียน "เครื่อง" ไม่ใช่ทะเบียนใบนำเข้า
var legacyTableRenames = [][2]string{
	{"import_license_items", "license_items"},
}

func RenameLegacyTables() {
	if DB == nil {
		return
	}
	m := DB.Migrator()

	for _, pair := range legacyTableRenames {
		oldName, newName := pair[0], pair[1]

		if !m.HasTable(oldName) {
			continue
		}
		if m.HasTable(newName) {
			// มีทั้งสองตาราง = เคยเปลี่ยนชื่อไปแล้ว แล้วมีอะไรสร้างตารางเก่าขึ้นมาใหม่
			// ไม่รวมให้เอง เพราะเดาไม่ได้ว่าแถวไหนใหม่กว่ากัน ปล่อยให้คนตัดสินใจ
			log.Printf("[migrate] มีทั้งตาราง %s และ %s — ข้ามการเปลี่ยนชื่อ กรุณาตรวจสอบด้วยตนเอง", oldName, newName)
			continue
		}

		if err := m.RenameTable(oldName, newName); err != nil {
			log.Printf("[migrate] เปลี่ยนชื่อตาราง %s เป็น %s ไม่สำเร็จ: %v", oldName, newName, err)
			continue
		}
		log.Printf("[migrate] เปลี่ยนชื่อตาราง %s เป็น %s แล้ว (ข้อมูลเดิมอยู่ครบ)", oldName, newName)
	}
}

// ---------------------------------------------------------------------------
// ลบตารางที่เลิกใช้แล้ว
//
// export_license_items ถูกถอดออก เพราะข้อมูลใบอนุญาตนำออกทุกอย่างอยู่ในตาราง
// license_items อยู่แล้ว — ไฟล์บัญชีแสดงหมายเลขเครื่องมีคอลัมน์เลขใบอนุญาตนำออก
// อยู่ในแถวเดียวกับเลขใบอนุญาตนำเข้า เครื่องหนึ่งเครื่องจึงรู้ทั้งสองฝั่งด้วยตัวเอง
// ส่วนข้อมูลการต่ออายุอยู่ในตาราง license_renewals
//
// ตารางนี้จึงไม่มีทางมีข้อมูลที่หาจากที่อื่นไม่ได้ และไม่มีหน้าจอไหนเขียนลงมันแล้ว
// ---------------------------------------------------------------------------

var retiredTables = []string{
	"export_license_items",

	// master_data ถูกถอดออก เพราะไฟล์ที่อัปจริงมีสามไฟล์ — Planning WH, Planning MFG,
	// Master Data — และทั้งสามลงตาราง upload_data_rows ไม่ใช่ตารางนี้
	// ตอน WH/MFG สแกนก็เทียบกับสามชุดนั้น ตารางนี้จึงไม่มีทางถูกเติมและไม่มีใครอ่าน
	"master_data",
}

func DropRetiredTables() {
	if DB == nil {
		return
	}
	m := DB.Migrator()

	for _, table := range retiredTables {
		if !m.HasTable(table) {
			continue
		}

		// นับแถวไว้ใน log ก่อนลบ เผื่อต้องย้อนไปดูว่าตอนลบมีข้อมูลค้างอยู่เท่าไร
		var n int64
		DB.Table(table).Count(&n)

		if err := m.DropTable(table); err != nil {
			log.Printf("[migrate] ลบตาราง %s ไม่สำเร็จ: %v", table, err)
			continue
		}
		log.Printf("[migrate] ลบตาราง %s ที่เลิกใช้แล้ว (มี %d แถวตอนลบ)", table, n)
	}
}
