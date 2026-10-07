package routes

import (
	"iconfirm/controllers"
	"iconfirm/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {

	r.POST("/login", controllers.Login)

	auth := r.Group("/")
	auth.Use(middleware.AuthMiddleware())

	uploadData := auth.Group("/upload-data")
	{
		uploadData.GET("", controllers.GetUploadData)
		uploadData.GET("/export", controllers.ExportUploadData)

		manage := uploadData.Group("")
		manage.Use(middleware.RoleMiddleware("UPLOAD", "ADMIN"))
		{
			manage.POST("/upload/:dataset", controllers.UploadDataFile)
			manage.POST("/preview/:dataset", controllers.PreviewUploadDataMapping)
			manage.DELETE("/:id", controllers.DeleteUploadDataRow)
			manage.DELETE("", controllers.ClearUploadData)
		}

		edit := uploadData.Group("")
		edit.Use(middleware.RoleMiddleware("ADMIN"))
		{
			edit.PUT("/:id", controllers.UpdateUploadDataRow)
			edit.PATCH("/:id", controllers.UpdateUploadDataRow)
		}
	}

	formatConfig := auth.Group("/format-config")
	{
		formatConfig.GET("/column-alias", controllers.GetColumnAliases)
		formatConfig.GET("/code-alias", controllers.GetCodeAliases)

		manage := formatConfig.Group("")
		manage.Use(middleware.RoleMiddleware("ADMIN"))
		{
			manage.POST("/column-alias", controllers.CreateColumnAlias)
			manage.DELETE("/column-alias/:id", controllers.DeleteColumnAlias)

			manage.POST("/code-alias", controllers.CreateCodeAlias)
			manage.POST("/code-alias/upload", controllers.UploadCodeAliases)
			manage.DELETE("/code-alias/:id", controllers.DeleteCodeAlias)
		}
	}

	partCheck := auth.Group("/part-check")
	partCheck.Use(middleware.RoleMiddleware("WH", "LOG"))
	{
		partCheck.GET("", controllers.GetPartChecks)
		partCheck.DELETE("/:id", controllers.DeletePartCheck)

	}

	// ขั้นตอนใหม่: WH เลือก MC# แล้วสแกนของที่จ่าย (เทียบ Engine_IT allocation / CW_CV_ITS)
	whFlow := auth.Group("/wh-flow")
	whFlow.Use(middleware.RoleMiddleware("WH", "LOG", "MFG", "QA", "ADMIN"))
	{
		whFlow.GET("/machines", controllers.GetFlowMachines)
		whFlow.GET("/issues", controllers.GetFlowIssues)
		whFlow.GET("/machines/:machineNo", controllers.GetFlowMachine)

		issue := whFlow.Group("")
		issue.Use(middleware.RoleMiddleware("WH", "LOG"))
		{
			issue.POST("/issue", controllers.IssueFlowPart)
			issue.DELETE("/issue/:id", controllers.CancelFlowIssue)
		}
	}

	// ขั้นตอนใหม่: MFG สแกน MC# จาก QR บน Kanban แล้วยืนยันของที่ WH จ่ายมา
	mfgFlow := auth.Group("/mfg-flow")
	mfgFlow.Use(middleware.RoleMiddleware("MFG"))
	{
		mfgFlow.POST("/kanban", controllers.ScanFlowKanban)
		mfgFlow.POST("/confirm", controllers.ConfirmFlowAssembly)
	}

	// Import / Export License: ไม่มีการแก้ไขในตาราง — LOG แก้/ลบข้อมูลใน Excel แล้วอัปโหลดไฟล์เดิมอีกครั้ง
	importLicense := auth.Group("/import-license")
	importLicense.Use(middleware.RoleMiddleware("WH", "LOG"))
	{
		importLicense.GET("", controllers.GetImportLicenseItems)
		importLicense.GET("/summary", controllers.GetImportLicenseSummary)
		importLicense.GET("/alerts", controllers.GetImportLicenseAlerts)

		manage := importLicense.Group("")
		manage.Use(middleware.RoleMiddleware("LOG"))
		{
			manage.POST("/upload", controllers.UploadImportLicenseItems)
			manage.POST("/preview", controllers.PreviewImportLicenseMapping)
			manage.POST("/verify", controllers.VerifyImportLicenseCode)
			manage.POST("/renew", controllers.RenewImportLicense)
			manage.POST("/complete", controllers.SetImportLicenseComplete)
			manage.POST("/bulk-delete", controllers.BulkDeleteImportLicenseItems)
			manage.DELETE("/:id", controllers.DeleteImportLicenseItem)
			manage.DELETE("", controllers.ClearImportLicenseItems)
		}
	}

	// ทะเบียนใบอนุญาต (ชีต "ต่ออายุ" เดิม) — ของเดิมที่ยังไม่เคยผูก route ไว้
	licenseRenewal := auth.Group("/license-renewal")
	licenseRenewal.Use(middleware.RoleMiddleware("LOG", "ADMIN"))
	{
		licenseRenewal.GET("", controllers.GetLicenseRenewals)
		licenseRenewal.GET("/alerts", controllers.GetLicenseRenewalAlerts)
		licenseRenewal.POST("/upload", controllers.UploadLicenseRenewals)
		licenseRenewal.DELETE("", controllers.ClearLicenseRenewals)

		// แก้ / ลบแถวในตารางโดยตรง ไม่ต้องแก้ไฟล์ Excel แล้วอัปใหม่
		//
		// ไม่มีปลายทางสำหรับ "เพิ่มใบอนุญาต" แล้ว — ไฟล์ Excel คือความจริงของตารางนี้
		// การเพิ่มแถวจากในระบบทำให้มีแถวที่ไฟล์ไม่รู้จัก แล้วชนกับไฟล์ตอนอัปรอบถัดไป
		licenseRenewal.PATCH("/:id", controllers.UpdateLicenseRenewal)
		licenseRenewal.DELETE("/:id", controllers.DeleteLicenseRenewal)
	}

	// อัปโหลดไฟล์เดียว ลงให้ครบทั้ง 3 ชีต ในคำขอเดียว
	// ของเดิมหน้าเว็บยิง 3 request ด้วยไฟล์เดียวกัน ไฟล์ใหญ่จึงเสียเวลาส่งคูณสาม
	// ต้องมีสิทธิ์ครบทั้งสามปลายทาง = LOG
	licenseUpload := auth.Group("/license-upload")
	licenseUpload.Use(middleware.RoleMiddleware("LOG"))
	{
		licenseUpload.POST("/workbook", controllers.UploadLicenseWorkbook)
	}

	// ไฟล์ Renewal (ต่ออายุ) — อัปโหลดแยกจาก Import / Export
	// เก็บเป็นประวัติ "เลขใบเดิม → เลขใบใหม่" ไม่แตะข้อมูล License เดิม
	licenseHistory := auth.Group("/license-renewal-history")
	licenseHistory.Use(middleware.RoleMiddleware("LOG", "ADMIN"))
	{
		licenseHistory.GET("", controllers.GetLicenseRenewalHistory)
		licenseHistory.POST("/preview", controllers.PreviewLicenseRenewalHistory)
		licenseHistory.POST("/upload", controllers.UploadLicenseRenewalHistory)
		licenseHistory.DELETE("", controllers.ClearLicenseRenewalHistory)
	}

	// License Overview — ดู Import + Export ในหน้าเดียว พร้อมประวัติการต่ออายุ
	licenseOverview := auth.Group("/license-overview")
	licenseOverview.Use(middleware.RoleMiddleware("LOG", "ADMIN"))
	{
		licenseOverview.GET("", controllers.GetLicenseOverview)
		licenseOverview.GET("/detail", controllers.GetLicenseDetail)
		licenseOverview.GET("/upload-log", controllers.GetLicenseUploadLog)
		// ลบใบอนุญาต 1 รายการทั้งโซ่ (รายการเครื่อง + ประวัติต่ออายุ + ทะเบียน)
		licenseOverview.DELETE("", controllers.DeleteLicenseOverviewEntry)
		// ล้างข้อมูลทั้งหมด — เลือกได้ว่าเฉพาะนำเข้า / นำออก / ต่ออายุ หรือทั้งหมด
		licenseOverview.DELETE("/all", controllers.ClearLicenseOverview)
		// กรอกวันที่ออกใบอนุญาตเอง สำหรับไฟล์ที่ไม่มีคอลัมน์วันที่
		licenseOverview.PATCH("/issue-date", controllers.SetLicenseIssueDate)
	}

	auth.POST("/uploads", controllers.UploadPhoto)

	auth.GET("/users", controllers.GetUsers)

	admin := auth.Group("/admin/users")
	admin.Use(middleware.RoleMiddleware("ADMIN"))
	{
		admin.GET("", controllers.GetAdminUsers)
		admin.POST("", controllers.CreateUser)
		admin.PATCH("/:id", controllers.UpdateUser)
		admin.DELETE("/:id", controllers.DeleteUser)
	}

	adminMail := auth.Group("/admin/mail-recipients")
	adminMail.Use(middleware.RoleMiddleware("ADMIN"))
	{
		adminMail.GET("", controllers.GetMailRecipients)
		adminMail.POST("", controllers.CreateMailRecipient)
		adminMail.PATCH("/:id", controllers.UpdateMailRecipient)
		adminMail.DELETE("/:id", controllers.DeleteMailRecipient)
	}

	// แจ้งเตือนใบอนุญาตรายสัปดาห์ — ดูสถานะ/ตัวอย่างอีเมล และกดส่งเดี๋ยวนี้
	// เป็นงานตั้งค่าระบบ ไม่ใช่งานประจำวันของ LOG จึงเปิดให้เฉพาะ ADMIN
	weeklyAlert := auth.Group("/weekly-alert")
	weeklyAlert.Use(middleware.RoleMiddleware("ADMIN"))
	{
		weeklyAlert.GET("/status", controllers.GetWeeklyAlertStatus)
		weeklyAlert.GET("/preview", controllers.GetWeeklyAlertPreview)
		weeklyAlert.POST("/send", controllers.SendWeeklyAlertNow)
	}

	mfgAssembly := auth.Group("/mfg-assembly")
	mfgAssembly.Use(middleware.RoleMiddleware("MFG"))
	{
		mfgAssembly.GET("", controllers.GetMFGAssemblies)
		mfgAssembly.POST("/scan", controllers.ScanMFGAssembly)
		mfgAssembly.POST("/spec-check", controllers.CheckMFGSpecQR)
		mfgAssembly.POST("", controllers.CreateMFGAssembly)
		mfgAssembly.PATCH("/:id", controllers.UpdateMFGAssembly)
		mfgAssembly.DELETE("/:id", controllers.DeleteMFGAssembly)

		mfgAssembly.POST("/:id/photo", controllers.UploadMFGAssemblyPhoto)
	}

	qa := auth.Group("/qa")
	qa.Use(middleware.RoleMiddleware("QA"))
	{
		qa.GET("/confirmed", controllers.GetQAConfirmedTable)
		qa.GET("/part-scan-summary", controllers.GetQAPartScanSummary)
		// QA ดูรายการที่ Matched ของ WH / MFG (อ่านอย่างเดียว)
		qa.GET("/wh", controllers.GetPartChecks)
		qa.GET("/mfg", controllers.GetMFGAssemblies)
	}

	auditLog := auth.Group("/audit-log")
	{
		auditLog.GET("", controllers.GetAuditLog)
	}
}
