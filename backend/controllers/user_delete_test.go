package controllers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func deleteUserReq(t *testing.T, admin models.User, id uint) map[string]interface{} {
	t.Helper()
	c, rec := newContext("DELETE", "", admin.ID, admin.Username)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	DeleteUser(c)
	mustStatus(t, rec, 200)
	return decodeJSON(t, rec)
}

func TestDeleteUserWithoutHistoryIsRemoved(t *testing.T) {
	db := newTestDB(t)
	admin := makeUser(t, db, "admin", "pw", "Admin", "ADMIN")
	u := makeUser(t, db, "wh-new", "pw", "WH ใหม่", "WH")

	out := deleteUserReq(t, admin, u.ID)
	if out["archived"] != false {
		t.Fatalf("ผู้ใช้ที่ไม่มีประวัติต้องถูกลบจริง: %v", out)
	}

	var n int64
	db.Model(&models.User{}).Where("id = ?", u.ID).Count(&n)
	if n != 0 {
		t.Fatal("แถวผู้ใช้ต้องหายไป")
	}
}

func TestDeleteUserWithHistoryIsArchived(t *testing.T) {
	db := newTestDB(t)
	admin := makeUser(t, db, "admin", "pw", "Admin", "ADMIN")
	u := makeUser(t, db, "wh01", "secret", "สุภาพร ก.", "WH")

	db.Create(&models.AuditLog{SourceTable: "PART_CHECK", Action: "scan", ActionDatetime: time.Now(), UserID: u.ID, Name: u.Name})
	db.Create(&models.PartCheck{PartType: "CV", SN: "CV-0001", MatchStatus: models.MatchStatusMatch, CheckedDatetime: time.Now(), UserID: u.ID})

	out := deleteUserReq(t, admin, u.ID)
	if out["archived"] != true {
		t.Fatalf("ผู้ใช้ที่มีประวัติต้องถูกปิดบัญชีแทนการลบ: %v", out)
	}

	var got models.User
	if err := db.First(&got, u.ID).Error; err != nil {
		t.Fatalf("แถวผู้ใช้ต้องยังอยู่ (ประวัติอ้างถึง): %v", err)
	}
	if got.Status != UserStatusDeleted {
		t.Errorf("status = %q ต้องเป็น %q", got.Status, UserStatusDeleted)
	}
	if got.Username == "wh01" || got.Password != "" {
		t.Errorf("ต้องปล่อย username เดิมและล้างรหัสผ่าน: username=%q", got.Username)
	}
	if got.Name != "สุภาพร ก." {
		t.Errorf("ชื่อต้องคงไว้เพื่อแสดงในประวัติ: %q", got.Name)
	}

	var logs, checks int64
	db.Model(&models.AuditLog{}).Where("user_id = ?", u.ID).Count(&logs)
	db.Model(&models.PartCheck{}).Where("user_id = ?", u.ID).Count(&checks)
	if logs == 0 || checks == 0 {
		t.Error("ประวัติการใช้งานต้องไม่หาย")
	}

	c, rec := newContext("GET", "", admin.ID, admin.Username)
	GetAdminUsers(c)
	mustStatus(t, rec, 200)
	var list []AdminUserView
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	for _, v := range list {
		if v.ID == u.ID {
			t.Error("ผู้ใช้ที่ลบแล้วต้องไม่แสดงในรายชื่อ")
		}
	}

	c, rec = newContext("POST", `{"username":"wh01","password":"secret"}`, 0, "")
	Login(c)
	if rec.Code == 200 {
		t.Error("ผู้ใช้ที่ลบแล้วต้องเข้าสู่ระบบไม่ได้")
	}

	c, rec = newContext("POST", `{"name":"สุภาพร ก.","username":"wh01","password":"secret","role_name":"WH"}`, admin.ID, admin.Username)
	CreateUser(c)
	mustStatus(t, rec, 201)

	c, rec = newContext("DELETE", "", admin.ID, admin.Username)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(u.ID), 10)}}
	DeleteUser(c)
	mustStatus(t, rec, 404)

	c, rec = newContext("PATCH", `{"name":"x"}`, admin.ID, admin.Username)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(u.ID), 10)}}
	UpdateUser(c)
	mustStatus(t, rec, 404)
}

func TestUpdateUserCannotSetDeletedStatus(t *testing.T) {
	db := newTestDB(t)
	admin := makeUser(t, db, "admin", "pw", "Admin", "ADMIN")
	u := makeUser(t, db, "mfg01", "pw", "MFG", "MFG")

	c, rec := newContext("PATCH", `{"status":"Deleted"}`, admin.ID, admin.Username)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(u.ID), 10)}}
	UpdateUser(c)
	mustStatus(t, rec, 400)
}

var fkDBCounter int64

func TestDeleteUserWithForeignKeysEnforced(t *testing.T) {
	dsn := fmt.Sprintf("file:fktest_%d?mode=memory&cache=shared&_pragma=foreign_keys(1)", atomic.AddInt64(&fkDBCounter, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(allModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	config.DB = db

	admin := makeUser(t, db, "admin", "pw", "Admin", "ADMIN")
	u := makeUser(t, db, "qa01", "pw", "QA", "QA")
	if err := db.Create(&models.AuditLog{SourceTable: "USER", Action: "scan", ActionDatetime: time.Now(), UserID: u.ID}).Error; err != nil {
		t.Fatalf("create audit log: %v", err)
	}

	if err := db.Delete(&models.User{}, u.ID).Error; err == nil {
		t.Skip("sqlite ไม่ได้บังคับ foreign key ในสภาพแวดล้อมนี้ ข้ามการทดสอบ")
	}

	out := deleteUserReq(t, admin, u.ID)
	if out["archived"] != true {
		t.Fatalf("ต้องปิดบัญชีแทนการลบ: %v", out)
	}
}
