package controllers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// ตัวช่วยอ่านไฟล์ที่อัปโหลด — ใช้ร่วมกันทุกหน้าอัปโหลด
//
// เดิมอยู่ใน controllers/master_data.go และ master_data_all_parts.go ซึ่งถูกลบไป
// พร้อมตาราง master_data แต่ตัวช่วยพวกนี้ไม่เกี่ยวกับตารางนั้นเลย — มันเป็นเรื่อง
// การอ่าน Excel/CSV ล้วน ๆ และมีไฟล์อื่นเรียกใช้อยู่อีกสิบกว่าไฟล์
// จึงย้ายมาไว้ที่นี่แทนการลบทิ้งไปด้วย
// ---------------------------------------------------------------------------

func normalizeHeader(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func unwrapExcelText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 3 && strings.HasPrefix(s, `="`) && strings.HasSuffix(s, `"`) {
		return s[2 : len(s)-1]
	}
	return s
}

func atoiSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if dot := strings.Index(s, "."); dot >= 0 {
		s = s[:dot]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func readUploadedRows(fileHeader *multipart.FileHeader) ([][]string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("เปิดไฟล์ไม่สำเร็จ")
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == ".csv" {
		return readCSVRows(file)
	}
	return readExcelRows(file)
}

type namedSheetRows struct {
	name string
	rows [][]string
}

func readAllUploadedSheets(fileHeader *multipart.FileHeader) ([]namedSheetRows, error) {
	return readAllUploadedSheetsOpt(fileHeader, false)
}

// readAllUploadedSheetsOpt: เหมือน readAllUploadedSheets แต่เลือกได้ว่าจะขยายเซลล์ที่ merge ลงแนวตั้งหรือไม่
//
// excelize อ่านแบบ iterator แล้วค่าของเซลล์ที่ merge จะอยู่เฉพาะช่องบนสุด ช่องที่เหลือว่าง
// ชีตทะเบียนใบอนุญาต (ต่ออายุ) มักจะ merge คอลัมน์ NO. ("Completed 01"), เลขใบนำเข้า, TOTAL
// ข้ามหลายแถวของโซ่เดียวกัน ถ้าไม่ขยายออก แถวล่าง ๆ ของกลุ่มจะไม่มีเลขใบนำเข้า/ชื่อกลุ่ม
// แล้วหลุดจากการจับคู่ STOCK / คงเหลือ
//
// ใช้เฉพาะกับชีตทะเบียนใบอนุญาต — ชีตอื่นคงพฤติกรรมเดิม (fillMerged = false)
func readAllUploadedSheetsOpt(fileHeader *multipart.FileHeader, fillMerged bool) ([]namedSheetRows, error) {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == ".csv" {
		rows, err := readUploadedRows(fileHeader)
		if err != nil {
			return nil, err
		}
		return []namedSheetRows{{name: "", rows: rows}}, nil
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("เปิดไฟล์ไม่สำเร็จ")
	}
	defer file.Close()

	xl, err := excelize.OpenReader(file)
	if err != nil {
		return nil, errors.New("ไฟล์ไม่ใช่ Excel ที่ถูกต้อง")
	}
	defer xl.Close()

	var out []namedSheetRows
	for _, name := range xl.GetSheetList() {
		rows, err := readSheetAllRows(xl, name)
		if err != nil {
			return nil, errors.New("อ่านชีต '" + name + "' ไม่สำเร็จ")
		}
		if fillMerged {
			rows = fillVerticalMerges(xl, name, rows)
		}
		out = append(out, namedSheetRows{name: name, rows: rows})
	}
	return out, nil
}

// fillVerticalMerges: เติมค่าของเซลล์ที่ merge ข้ามหลายแถว ลงในช่องที่ว่างของแถวล่าง
// ทำเฉพาะ merge แนวตั้ง (r2 > r1) เพื่อไม่ไปแตะหัวตารางที่ merge แนวนอน
func fillVerticalMerges(xl *excelize.File, sheet string, rows [][]string) [][]string {
	merges, err := xl.GetMergeCells(sheet)
	if err != nil {
		return rows
	}
	for _, m := range merges {
		c1, r1, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
		c2, r2, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err1 != nil || err2 != nil || r2 <= r1 {
			continue
		}
		val := m.GetCellValue()
		if strings.TrimSpace(val) == "" {
			continue
		}
		for r := r1 + 1; r <= r2 && r <= len(rows); r++ {
			row := rows[r-1]
			for c := c1; c <= c2; c++ {
				for len(row) < c {
					row = append(row, "")
				}
				if strings.TrimSpace(row[c-1]) == "" {
					row[c-1] = val
				}
			}
			rows[r-1] = row
		}
	}
	return rows
}

func readExcelRows(r io.Reader) ([][]string, error) {
	xl, err := excelize.OpenReader(r)
	if err != nil {
		return nil, errors.New("ไฟล์ไม่ใช่ Excel ที่ถูกต้อง")
	}
	defer xl.Close()

	sheet := xl.GetSheetName(0)
	rows, err := readSheetAllRows(xl, sheet)
	if err != nil {
		return nil, errors.New("อ่านไฟล์ Excel ไม่สำเร็จ")
	}
	return rows, nil
}

func readCSVRows(r io.Reader) ([][]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.New("อ่านไฟล์ CSV ไม่สำเร็จ")
	}
	data = bytes.TrimPrefix(data, []byte("\uFEFF"))

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, errors.New("ไฟล์ CSV ไม่ถูกต้อง อ่านไม่ได้")
	}
	return rows, nil
}
