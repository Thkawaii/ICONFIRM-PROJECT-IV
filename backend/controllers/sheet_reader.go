package controllers

import (
	"errors"
	"mime/multipart"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

var errUploadNoFile = errors.New("กรุณาแนบไฟล์ Excel หรือ CSV (field name: file)")

const sheetSparseTailStop = 20000

func readSheetAllRows(xl *excelize.File, sheet string) ([][]string, error) {
	it, err := xl.Rows(sheet)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	results := make([][]string, 0, 256)
	lastFilled := 0
	lastDense := 0
	wide := false
	stopped := false

	for it.Next() {
		row, err := it.Columns()
		if err != nil {
			return nil, err
		}
		results = append(results, row)
		n := len(results)

		filled := 0
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				filled++
				if filled >= 2 {
					break
				}
			}
		}
		if filled > 0 {
			lastFilled = n
		}
		if filled >= 2 {
			lastDense = n
			wide = true
		}
		if wide && n-lastDense >= sheetSparseTailStop {
			stopped = true
			break
		}
	}

	if stopped {
		return results[:lastDense], nil
	}
	return results[:lastFilled], nil
}

// ---------------------------------------------------------------------------
// เลือกชีตให้อัตโนมัติ — ไฟล์ที่มีหลายชีต (เช่น Excel_Form ที่รวมทุกประเภทไว้ไฟล์เดียว)
// จะเลือกชีตที่ "ชื่อชีต" ตรงกับประเภทที่อัปโหลดก่อน แล้วค่อยดูชีตที่หาหัวตารางเจอ
// และมีคอลัมน์ที่รู้จักมากที่สุด; ถ้าไม่มีชีตไหนเข้าเลย จะคืนชีตแรก (ให้ข้อความ error เดิมทำงาน)
// ---------------------------------------------------------------------------

// sheetScore คืนคะแนนของชีต (< 0 = ชีตนี้ใช้ไม่ได้)
type sheetScore func(sheetName string, rows [][]string) int

const sheetNameMatchBonus = 1000

func sheetNameHas(sheetName string, keys ...string) bool {
	n := normalizeHeader(sheetName)
	if n == "" {
		return false
	}
	for _, k := range keys {
		if strings.Contains(n, k) {
			return true
		}
	}
	return false
}

// readBestSheet: เลือกชีตที่ใช่ แล้วอ่านเต็มเฉพาะชีตนั้น
//
// การให้คะแนนดูแค่หัวตาราง (ทุก findHeader มองไม่เกิน 40 แถวแรก) จึงให้คะแนน
// จากหัว ๆ ชีตได้เลย ไม่ต้องแกะทั้งเล่มก่อนเหมือนเดิม
func readBestSheet(fileHeader *multipart.FileHeader, score sheetScore) ([][]string, string, error) {
	wb, err := openUploadedWorkbook(fileHeader)
	if err != nil {
		return nil, "", err
	}
	defer wb.Close()

	names := wb.Names()
	if len(names) == 0 {
		return nil, "", nil
	}

	pick := names[0]
	if len(names) > 1 && score != nil {
		best, bestScore := -1, -1
		for i, name := range names {
			probe := wb.Probe(name)
			if len(probe) < 2 {
				continue
			}
			if s := score(name, probe); s > bestScore {
				best, bestScore = i, s
			}
		}
		// ไม่มีชีตไหนเข้าเกณฑ์ → คืนชีตแรก ให้ข้อความ error เดิมทำงานต่อ
		if best >= 0 {
			pick = names[best]
		}
	}

	rows, err := wb.Rows(pick, false)
	if err != nil {
		return nil, pick, err
	}
	return rows, pick, nil
}

func readBestSheetFromForm(c *gin.Context, score sheetScore) ([][]string, string, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, "", errUploadNoFile
	}
	rows, _, err := readBestSheet(fileHeader, score)
	if err != nil {
		return nil, fileHeader.Filename, err
	}
	return rows, fileHeader.Filename, nil
}

func countKnownHeaders(headers []string, known func(string) bool) int {
	n := 0
	for _, h := range headers {
		if h != "" && known(h) {
			n++
		}
	}
	return n
}
