package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// อัปโหลดไฟล์ทะเบียนใบอนุญาต "รอบเดียว" แล้วลงให้ครบทั้งใบนำเข้าและทะเบียนต่ออายุ
//
// ของเดิมหน้าเว็บยิงหลาย request ด้วยไฟล์เดียวกัน
// ไฟล์จึงถูกส่งขึ้นเซิร์ฟเวอร์ 3 รอบ และถูกแกะ multipart ใหม่ 3 รอบ
// ไฟล์เล็กไม่รู้สึก แต่พอไฟล์ใหญ่ขึ้น เวลาอัปก็คูณสามตรง ๆ
//
// endpoint นี้รับไฟล์ครั้งเดียว แล้วเรียก handler เดิมทั้งสามตัวต่อกันในคำขอเดียว
// ทุกตัวอ่านไฟล์ผ่าน c.FormFile("file") ซึ่ง net/http แกะ multipart ครั้งแรก
// แล้วเก็บผลไว้ใน Request.MultipartForm — เรียกซ้ำจึงได้ของเดิม ไม่ได้อ่านไฟล์ใหม่
//
// ตัว handler ไม่ถูกแก้เลยสักบรรทัด ตรรกะการนำเข้าทั้งหมดจึงเหมือนเดิมเป๊ะ
// และ endpoint เดิมทั้งสามตัวยังอยู่ครบ ใช้แยกทีละชีตได้เหมือนเดิม
// ---------------------------------------------------------------------------

type workbookPart struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	OK    bool   `json:"ok"`
	// Skipped = ไฟล์นี้ไม่มีชีตของส่วนนี้ ไม่ใช่ความผิดพลาด
	// ไฟล์งานจริงมักแยกเล่ม เล่มละชีต การขึ้นเป็นสีแดงทุกครั้งทำให้ตกใจเปล่า ๆ
	Skipped bool   `json:"skipped"`
	Count   *int   `json:"count"`
	Message string `json:"message"`

	// ExtraNotice = ชีตนี้มีคอลัมน์ที่ระบบไม่รู้จัก เก็บไว้แล้วแต่ไม่ได้นำไปคำนวณ
	// แยกรายชีต เพราะไฟล์เล่มเดียวมีได้ทั้งชีตต่ออายุและชีตนำเข้า คนละชุดคอลัมน์กัน
	ExtraNotice string `json:"extraNotice,omitempty"`
}

// workbookUploadParts: ลำดับการนำเข้า — ทะเบียนต่ออายุก่อน แล้วค่อยใบอนุญาตนำเข้า
var workbookUploadParts = []struct {
	key     string
	label   string
	handler func(*gin.Context)
	// present = ไฟล์เล่มนี้มีชีตของส่วนนี้ไหม ใช้หัวตารางเป็นตัวตัดสิน
	present func(*workbook) bool
}{
	{"renewal", "ทะเบียนใบอนุญาต (ต่ออายุ)", UploadLicenseRenewals, workbookHasRenewalSheet},
	{"import", "ใบอนุญาตนำเข้า (Import)", UploadImportLicenseItems, workbookHasImportSheet},
}

// ไม่มีส่วน Export ในชุดนี้แล้ว
//
// ไฟล์ Serial Allocation เป็นรายการรายเครื่องหลายพันแถว แต่หน้าทะเบียนใบอนุญาต
// ไม่ได้ใช้มันเลย — ประเทศ วันออก วันหมดอายุ STOCK REMAIN และโซ่การต่ออายุ
// มาจากชีตต่ออายุทั้งหมด ส่วนดัชนีเครื่องก็สร้างจาก Planning / Engine / WH / MFG
// โดยมีไฟล์นี้เป็นแค่คอลัมน์อ้างอิงเสริม ไม่ใช่ต้นทางของเลขเครื่อง
//
// ยังอัปแยกได้ที่หน้าใบอนุญาตนำออกเหมือนเดิม (POST /export-license/upload)

// ชีตไหนเป็นของใคร ตัดสินจากหัวตารางเหมือนตอนอัปโหลดจริงทุกประการ
// อ่านแค่หัว ๆ ชีต (Probe) จึงไม่ต้องแกะทั้งเล่มเพื่อมาตรวจ

func workbookHasRenewalSheet(wb *workbook) bool {
	for _, name := range wb.Names() {
		if idx, _ := findRenewalHeader(wb.Probe(name)); idx >= 0 {
			return true
		}
	}
	return false
}

func workbookHasImportSheet(wb *workbook) bool {
	for _, name := range wb.Names() {
		if idx, _ := findImportLicenseHeader(wb.Probe(name)); idx >= 0 {
			return true
		}
	}
	return false
}

// UploadLicenseWorkbook: POST /license-upload/workbook
func UploadLicenseWorkbook(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"message": errUploadNoFile.Error()})
		return
	}

	wb, err := openUploadedWorkbook(fileHeader)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	defer wb.Close()

	parts := make([]workbookPart, 0, len(workbookUploadParts))
	okCount, ranCount := 0, 0
	for _, p := range workbookUploadParts {
		var res workbookPart
		if p.present(wb) {
			res = runWorkbookPart(c, p.handler)
			ranCount++
			if res.OK {
				okCount++
			}
		} else {
			res = workbookPart{Skipped: true, Message: "ไฟล์นี้ไม่มีชีตของส่วนนี้ — ข้ามไป"}
		}
		res.Key, res.Label = p.key, p.label
		parts = append(parts, res)
	}

	status := 200
	message := "อัปเดตแล้ว " + itoa(okCount) + " จาก " + itoa(ranCount) + " ชีตที่พบในไฟล์"
	if ranCount == 0 {
		status = 400
		message = "ไฟล์นี้ไม่มีชีตที่ระบบอ่านได้เลย (ใบอนุญาตนำเข้า · ทะเบียนต่ออายุ)"
	} else if okCount == 0 {
		status = 400
		message = "พบชีตในไฟล์ แต่อ่านข้อมูลเข้าระบบไม่ได้เลย"
	}

	c.JSON(status, gin.H{
		"fileName": fileHeader.Filename,
		"parts":    parts,
		"ok":       okCount,
		"message":  message,
	})
}

// runWorkbookPart: เรียก handler เดิม 1 ตัว โดยใช้ request เดิม แล้วเก็บคำตอบของมันไว้
//
// handler เดิมเขียนคำตอบลง c.Writer ตรง ๆ จึงต้องให้มันเขียนลงที่พักก่อน
// ไม่งั้นทั้งสามตัวจะแย่งกันเขียน response ของคำขอเดียว
func runWorkbookPart(c *gin.Context, handler func(*gin.Context)) workbookPart {
	rec := httptest.NewRecorder()
	sub, _ := gin.CreateTestContext(rec)

	// ใช้ Request ตัวเดิม — multipart ที่แกะไว้แล้วติดไปด้วย ไฟล์จึงไม่ถูกอ่านซ้ำ
	sub.Request = c.Request
	// Keys เป็น map ตัวเดียวกัน ข้อมูลผู้ใช้จาก JWT middleware จึงส่งต่อไปครบ
	sub.Keys = c.Keys

	handler(sub)

	return readWorkbookPartResult(rec)
}

func readWorkbookPartResult(rec *httptest.ResponseRecorder) workbookPart {
	out := workbookPart{OK: rec.Code >= 200 && rec.Code < 300}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		if !out.OK {
			out.Message = strings.TrimSpace(http.StatusText(rec.Code))
			if out.Message == "" {
				out.Message = "อัปโหลดไม่สำเร็จ"
			}
		}
		return out
	}

	if msg, ok := body["message"].(string); ok {
		out.Message = msg
	}
	if !out.OK && out.Message == "" {
		out.Message = "อัปโหลดไม่สำเร็จ"
	}

	if v, ok := body["extraNotice"].(string); ok {
		out.ExtraNotice = v
	}

	// จำนวนที่นำเข้าได้ แต่ละ handler เรียกชื่อไม่เหมือนกัน
	for _, key := range []string{"imported", "created", "inserted"} {
		if v, ok := body[key].(float64); ok {
			n := int(v)
			out.Count = &n
			break
		}
	}
	return out
}
