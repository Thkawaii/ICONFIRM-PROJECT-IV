package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func allModels() []interface{} {
	return []interface{}{
		&models.User{},
		&models.AuditLog{},
		&models.PartCheck{},
		&models.LicenseItem{},
		&models.UploadDataRow{},
		&models.MFGAssembly{},
		&models.MatchingAssembly{},
		&models.ColumnAlias{},
		&models.CodeAlias{},
	}
}

var testDBCounter int64

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:memtest_%d?mode=memory&cache=shared",
		atomic.AddInt64(&testDBCounter, 1))

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if sqlDB, e := db.DB(); e == nil {
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetConnMaxLifetime(0)
	}

	if err := db.AutoMigrate(allModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	config.DB = db
	return db
}

func makeUser(t *testing.T, db *gorm.DB, username, password, name, role string) models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	u := models.User{
		Username: username,
		Password: string(hash),
		Name:     name,
		RoleName: role,
		Status:   "Active",
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func strptr(s string) *string { return &s }

func newContext(method, body string, userID uint, username string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, "/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	if userID != 0 || username != "" {
		c.Set("user_id", userID)
		c.Set("username", username)
	}
	return c, rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode json (%s): %v", rec.Body.String(), err)
	}
	return out
}

func mustStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, want, rec.Body.String())
	}
}

// csvUploadContext: ย้ายมาจาก export_license_reupload_test.go ที่ถูกลบไปพร้อมฟีเจอร์ใบนำออก
// ยังมีเทสต์อื่นใช้อยู่ จึงเก็บไว้ในไฟล์กลางของชุดเทสต์แทน
func csvUploadContext(t *testing.T, name, body string, userID uint, username string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := part.Write([]byte(body)); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	w.Close()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	c.Request = req
	c.Set("user_id", userID)
	c.Set("username", username)
	return c, rec
}
