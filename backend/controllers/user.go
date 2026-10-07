package controllers

import (
	"fmt"
	"strings"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const UserStatusDeleted = "Deleted"

var userHistoryModels = []interface{}{
	&models.AuditLog{},
	&models.PartCheck{},
	&models.MFGAssembly{},
	&models.LicenseItem{},
	&models.UploadDataRow{},
	&models.MatchingAssembly{},
	&models.ColumnAlias{},
	&models.CodeAlias{},
}

func userHasHistory(db *gorm.DB, userID uint) (bool, error) {
	for _, m := range userHistoryModels {
		if !db.Migrator().HasTable(m) {
			continue
		}
		var n int64
		if err := db.Model(m).Where("user_id = ?", userID).Limit(1).Count(&n).Error; err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

func archiveUser(db *gorm.DB, user models.User) error {
	username := fmt.Sprintf("deleted-%d-%s", user.ID, user.Username)
	if r := []rune(username); len(r) > 100 {
		username = string(r[:100])
	}
	return db.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"status":   UserStatusDeleted,
		"username": username,
		"password": "",
	}).Error
}

type UserSummary struct {
	ID       uint   `json:"ID"`
	Name     string `json:"Name"`
	RoleName string `json:"RoleName"`
}

func GetUsers(c *gin.Context) {

	var users []models.User

	query := config.DB.Where("status = ? OR status = ''", "Active")
	if role := c.Query("role"); role != "" {
		query = query.Where("role_name = ?", role)
	}
	query.Find(&users)

	summaries := make([]UserSummary, 0, len(users))
	for _, u := range users {
		summaries = append(summaries, UserSummary{ID: u.ID, Name: u.Name, RoleName: u.RoleName})
	}

	c.JSON(200, summaries)
}

type AdminUserView struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	RoleName string `json:"role_name"`
	Status   string `json:"status"`
	Email    string `json:"email"`

	// ReceivesAlerts = อีเมลนี้อยู่ในรายชื่อผู้รับรายงานรายสัปดาห์ (และยังเปิดใช้งานอยู่)
	ReceivesAlerts bool `json:"receives_alerts"`

	// WelcomeMail = ผลของการส่งรายงานฉบับล่าสุดให้ผู้ใช้ใหม่ (เฉพาะตอนสร้าง/แก้ไข)
	//   queued   = เข้าคิวส่งแล้ว
	//   disabled = ปิดการแจ้งเตือนรายสัปดาห์อยู่ จึงยังไม่ส่ง
	//   none     = ไม่ได้ขอให้ส่ง หรือไม่ได้กรอกอีเมล
	WelcomeMail string `json:"welcome_mail,omitempty"`
}

func toAdminUserView(u models.User) AdminUserView {
	return AdminUserView{
		ID:             u.ID,
		Name:           u.Name,
		Username:       u.Username,
		RoleName:       u.RoleName,
		Status:         u.Status,
		Email:          u.Email,
		ReceivesAlerts: mailRecipientActive(u.Email),
	}
}

func GetAdminUsers(c *gin.Context) {
	var users []models.User
	q := config.DB.Model(&models.User{}).Where("status IS NULL OR status <> ?", UserStatusDeleted)

	if role := strings.TrimSpace(c.Query("role")); role != "" {
		q = q.Where("role_name = ?", role)
	}
	if kw := strings.TrimSpace(c.Query("q")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR username LIKE ?", like, like)
	}
	q.Order("role_name asc, name asc").Find(&users)

	out := make([]AdminUserView, 0, len(users))
	for _, u := range users {
		out = append(out, toAdminUserView(u))
	}
	c.JSON(200, out)
}

type CreateUserRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	RoleName string `json:"role_name"`
	Status   string `json:"status"`
	Email    string `json:"email"`

	// ReceiveAlerts = ติ๊กให้ผู้ใช้คนนี้รับรายงานใบอนุญาตรายสัปดาห์ด้วย
	// ไม่ส่งค่ามา = ไม่รับ (ผู้ใช้หน้างานส่วนใหญ่ไม่ต้องรับรายงานฉบับนี้)
	ReceiveAlerts bool `json:"receive_alerts"`
}

func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Username = strings.TrimSpace(req.Username)
	req.RoleName = strings.TrimSpace(req.RoleName)
	req.Email = normalizeUserEmail(req.Email)
	if req.Name == "" || req.Username == "" || req.Password == "" || req.RoleName == "" {
		c.JSON(400, gin.H{"message": "กรุณากรอกชื่อ, username, password และแผนก/role ให้ครบ"})
		return
	}
	if msg := validateUserEmail(req.Email, req.ReceiveAlerts); msg != "" {
		c.JSON(400, gin.H{"message": msg})
		return
	}
	if req.Status == "" {
		req.Status = "Active"
	}

	var same []models.User
	config.DB.Where("username = ?", req.Username).Find(&same)
	for _, u := range same {
		if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) == nil {
			c.JSON(409, gin.H{"message": "รหัสผ่านนี้ถูกใช้กับ username นี้แล้ว — กรุณาตั้งรหัสผ่านอื่น"})
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"message": "สร้างรหัสผ่านไม่สำเร็จ"})
		return
	}

	user := models.User{
		Name:     req.Name,
		Username: req.Username,
		Password: string(hash),
		RoleName: req.RoleName,
		Status:   req.Status,
		Email:    req.Email,
	}
	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	adminID, adminName := lookupUserName(c)
	CreateAuditLog("USER", user.ID, "create", req.Name, adminID, adminName)

	view := toAdminUserView(user)
	view.WelcomeMail = applyUserAlertSubscription(user, req.ReceiveAlerts, adminName)
	view.ReceivesAlerts = req.ReceiveAlerts && user.Email != ""

	c.JSON(201, view)
}

type UpdateUserRequest struct {
	Name     string `json:"name"`
	RoleName string `json:"role_name"`
	Status   string `json:"status"`
	Password string `json:"password"`

	// Email / ReceiveAlerts เป็น pointer เพื่อแยก "ไม่ได้ส่งค่ามา" ออกจาก "ส่งค่าว่าง"
	// ไม่งั้นการแก้แค่ชื่อ จะลบอีเมลของผู้ใช้ทิ้งไปด้วย
	Email         *string `json:"email"`
	ReceiveAlerts *bool   `json:"receive_alerts"`
}

func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := config.DB.First(&user, id).Error; err != nil || user.Status == UserStatusDeleted {
		c.JSON(404, gin.H{"message": "ไม่พบผู้ใช้"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "ข้อมูลไม่ถูกต้อง"})
		return
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(req.Name) != "" {
		updates["name"] = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.RoleName) != "" {
		updates["role_name"] = strings.TrimSpace(req.RoleName)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		if st == UserStatusDeleted {
			c.JSON(400, gin.H{"message": "สถานะไม่ถูกต้อง — ใช้ปุ่มลบเพื่อลบผู้ใช้"})
			return
		}
		updates["status"] = st
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(500, gin.H{"message": "สร้างรหัสผ่านไม่สำเร็จ"})
			return
		}
		updates["password"] = string(hash)
	}

	// อีเมลเดิมต้องจำไว้ก่อนเขียนทับ เพราะถ้าผู้ใช้เปลี่ยนอีเมล
	// ต้องถอนชื่ออีเมลเก่าออกจากรายชื่อผู้รับด้วย ไม่งั้นรายงานจะยังส่งไปที่เดิม
	previousEmail := user.Email

	if req.Email != nil {
		email := normalizeUserEmail(*req.Email)
		wantAlerts := req.ReceiveAlerts != nil && *req.ReceiveAlerts
		if msg := validateUserEmail(email, wantAlerts); msg != "" {
			c.JSON(400, gin.H{"message": msg})
			return
		}
		updates["email"] = email
	}

	if len(updates) == 0 && req.ReceiveAlerts == nil {
		c.JSON(400, gin.H{"message": "ไม่มีข้อมูลที่จะแก้ไข"})
		return
	}

	if len(updates) > 0 {
		config.DB.Model(&user).Updates(updates)
	}

	adminID, adminName := lookupUserName(c)
	CreateAuditLog("USER", user.ID, "update", user.Name, adminID, adminName)

	config.DB.First(&user, id)

	view := toAdminUserView(user)

	// อีเมลเปลี่ยน → เลิกส่งไปที่อยู่เดิม
	if previousEmail != "" && previousEmail != user.Email {
		setMailRecipientActive(previousEmail, "", false)
	}

	// ไม่ได้ส่ง receive_alerts มา = ไม่ได้ตั้งใจแก้เรื่องรายชื่อผู้รับ ปล่อยไว้ตามเดิม
	if req.ReceiveAlerts != nil {
		view.WelcomeMail = applyUserAlertSubscription(user, *req.ReceiveAlerts, adminName)
		view.ReceivesAlerts = *req.ReceiveAlerts && user.Email != ""
	}

	c.JSON(200, view)
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	adminID, adminName := lookupUserName(c)

	var user models.User
	if err := config.DB.First(&user, id).Error; err != nil || user.Status == UserStatusDeleted {
		c.JSON(404, gin.H{"message": "ไม่พบผู้ใช้"})
		return
	}
	if user.ID == adminID {
		c.JSON(400, gin.H{"message": "ลบบัญชีตัวเองไม่ได้"})
		return
	}

	hasHistory, err := userHasHistory(config.DB, user.ID)
	if err != nil {
		c.JSON(500, gin.H{"message": "ตรวจสอบประวัติการใช้งานของผู้ใช้ไม่สำเร็จ"})
		return
	}

	// ลบหรือเก็บประวัติก็ตาม ต้องหยุดส่งรายงานให้คนที่ไม่อยู่แล้ว
	unsubscribeUserAlerts(user)

	if !hasHistory {
		if err := config.DB.Delete(&models.User{}, user.ID).Error; err == nil {
			CreateAuditLog("USER", user.ID, "delete", user.Name, adminID, adminName)
			c.JSON(200, gin.H{"deleted": 1, "archived": false})
			return
		}
	}

	if err := archiveUser(config.DB, user); err != nil {
		c.JSON(500, gin.H{"message": "ลบผู้ใช้ไม่สำเร็จ"})
		return
	}
	CreateAuditLog("USER", user.ID, "delete", user.Name+" (เก็บประวัติการใช้งานไว้)", adminID, adminName)
	c.JSON(200, gin.H{
		"deleted":  1,
		"archived": true,
		"message":  "ลบผู้ใช้แล้ว — ผู้ใช้นี้มีประวัติการใช้งานในระบบ จึงเก็บประวัติไว้ และปิดการเข้าสู่ระบบถาวร",
	})
}
