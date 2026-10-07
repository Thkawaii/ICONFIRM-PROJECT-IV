package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"iconfirm/config"
	"iconfirm/jobs"
	"iconfirm/middleware"
	"iconfirm/routes"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	_ = godotenv.Load()

	config.ConnectDB()
	config.MigratePlaintextPasswords()

	jobs.StartWeeklyAlertScheduler()
	jobs.StartLicenseReconcileScheduler()

	r := gin.Default()

	r.Use(middleware.CORSMiddleware())

	r.Static("/uploads", "./uploads")

	routes.SetupRoutes(r)

	candidates := []string{"../frontend/dist", "./frontend/dist", "frontend/dist"}
	frontendDist := ""
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			frontendDist = c
			break
		}
	}

	if frontendDist != "" {
		abs, _ := filepath.Abs(frontendDist)
		log.Printf("[frontend] เสิร์ฟหน้าเว็บจาก: %s (เข้าเว็บที่ http://<ip>:PORT ได้เลย)", abs)

		r.Static("/assets", filepath.Join(frontendDist, "assets"))
		r.StaticFile("/", filepath.Join(frontendDist, "index.html"))

		r.NoRoute(func(c *gin.Context) {
			c.File(filepath.Join(frontendDist, "index.html"))
		})
	} else {
		log.Printf("[frontend] ⚠️  หา frontend/dist ไม่เจอ — เข้า :PORT จะได้หน้าโล่ง/404")
		log.Printf("[frontend]     ต้อง build ก่อน: cd frontend && npm run build")
		log.Printf("[frontend]     แล้วรัน go จากในโฟลเดอร์ backend: cd backend && go run .")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	certFile := strings.TrimSpace(os.Getenv("TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("TLS_KEY_FILE"))

	if certFile != "" && keyFile != "" {
		log.Printf("[server] เปิด HTTPS ที่พอร์ต %s (ใบรับรอง: %s)", port, certFile)
		if err := r.RunTLS(":"+port, certFile, keyFile); err != nil {
			log.Fatalf("[server] เปิด HTTPS ไม่สำเร็จ: %v", err)
		}
		return
	}

	if certFile != "" || keyFile != "" {
		log.Println("[server] ⚠️  ต้องตั้งทั้ง TLS_CERT_FILE และ TLS_KEY_FILE คู่กัน — ตอนนี้จะเสิร์ฟเป็น HTTP ไปก่อน")
	}

	log.Printf("[server] เปิด HTTP ที่พอร์ต %s (ยังไม่ได้เข้ารหัส — ดูวิธีเปิด HTTPS ใน .env.example)", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[server] เปิดเซิร์ฟเวอร์ไม่สำเร็จ: %v", err)
	}
}
