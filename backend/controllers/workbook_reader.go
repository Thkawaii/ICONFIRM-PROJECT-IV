package controllers

import (
	"errors"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// อ่านไฟล์ที่อัปโหลดแบบ lazy — อ่านเต็มเฉพาะชีตที่ใช้จริง
//
// ของเดิม readAllUploadedSheets อ่านทุกชีตให้ครบก่อน แล้วค่อยเลือกว่าจะใช้ชีตไหน
// ไฟล์งานจริงมี 7 ชีต และชีต Export ชีตเดียวมีเก้าพันแถว ยี่สิบแปดคอลัมน์
// การ์ด Import จึงเสียเวลาแกะชีต Export ทั้งชีตทิ้งทุกครั้ง ทั้งที่ไม่ได้ใช้เลย
//
// ยิ่งกว่านั้น หน้าเว็บอัปไฟล์เดียวกัน 3 รอบ (Import / Export / ต่ออายุ)
// และฝั่ง Import กับ Export ยังเรียก AbsorbLedgerSheet ซึ่งเปิดไฟล์อ่านใหม่ทั้งเล่มอีก
// ไฟล์เล่มเดียวจึงถูกแกะซ้ำประมาณ 5 รอบต่อการอัปโหลด 1 ครั้ง
//
// ตัวอ่านนี้แยกเป็นสองจังหวะ:
//
//	Probe(name) — อ่านแค่หัว ๆ ชีต (sheetProbeRows แถวแรก) พอให้หาหัวตาราง/ให้คะแนนได้
//	Rows(name)  — อ่านเต็มชีต เรียกเมื่อเลือกชีตได้แล้วเท่านั้น และจำผลไว้
// ---------------------------------------------------------------------------

// sheetProbeRows = จำนวนแถวที่อ่านมาใช้หาหัวตาราง
//
// ตัวหาหัวตารางทุกตัวในระบบมองไม่เกิน 40 แถวแรก (engine_it_alloc มากสุดที่ 40)
// เผื่อไว้ที่ 60 เพื่อให้ผลการให้คะแนนเหมือนกับตอนอ่านทั้งชีตเป๊ะ
const sheetProbeRows = 60

type workbook struct {
	xl    *excelize.File
	file  multipart.File
	names []string

	probes map[string][][]string
	full   map[string][][]string
	merged map[string]bool
}

// openUploadedWorkbook: เปิดไฟล์ที่อัปโหลดค้างไว้ ยังไม่อ่านเนื้อชีต
// ไฟล์ CSV ไม่มีหลายชีต จึงอ่านทีเดียวจบแล้วทำเป็นชีตเดียวชื่อว่าง
func openUploadedWorkbook(fileHeader *multipart.FileHeader) (*workbook, error) {
	if fileHeader == nil {
		return nil, errUploadNoFile
	}

	if strings.ToLower(filepath.Ext(fileHeader.Filename)) == ".csv" {
		rows, err := readUploadedRows(fileHeader)
		if err != nil {
			return nil, err
		}
		return &workbook{
			names:  []string{""},
			probes: map[string][][]string{"": rows},
			full:   map[string][][]string{"": rows},
			merged: map[string]bool{"": true},
		}, nil
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("เปิดไฟล์ไม่สำเร็จ")
	}
	xl, err := excelize.OpenReader(file)
	if err != nil {
		file.Close()
		return nil, errors.New("ไฟล์ไม่ใช่ Excel ที่ถูกต้อง")
	}

	return &workbook{
		xl:     xl,
		file:   file,
		names:  xl.GetSheetList(),
		probes: map[string][][]string{},
		full:   map[string][][]string{},
		merged: map[string]bool{},
	}, nil
}

func (w *workbook) Close() {
	if w == nil {
		return
	}
	if w.xl != nil {
		w.xl.Close()
	}
	if w.file != nil {
		w.file.Close()
	}
}

func (w *workbook) Names() []string {
	if w == nil {
		return nil
	}
	return w.names
}

// Probe: หัว ๆ ของชีต ใช้หาหัวตาราง / ให้คะแนนว่าใช่ชีตที่ต้องการไหม
// ถ้าอ่านชีตนั้นเต็มไปแล้วก็ใช้ของเต็มเลย ไม่ต้องอ่านซ้ำ
func (w *workbook) Probe(name string) [][]string {
	if w == nil {
		return nil
	}
	if rows, ok := w.full[name]; ok {
		return rows
	}
	if rows, ok := w.probes[name]; ok {
		return rows
	}
	rows, err := readSheetRowsLimit(w.xl, name, sheetProbeRows)
	if err != nil {
		rows = nil
	}
	w.probes[name] = rows
	return rows
}

// Rows: เนื้อชีตทั้งชีต จำผลไว้ให้เรียกซ้ำได้ฟรี
// fillMerged = ขยายเซลล์ที่ merge แนวตั้งลงแถวล่าง (ใช้กับชีตทะเบียนใบอนุญาต)
func (w *workbook) Rows(name string, fillMerged bool) ([][]string, error) {
	if w == nil {
		return nil, errUploadNoFile
	}
	if rows, ok := w.full[name]; ok && (!fillMerged || w.merged[name]) {
		return rows, nil
	}
	rows, err := readSheetAllRows(w.xl, name)
	if err != nil {
		return nil, errors.New("อ่านชีต '" + name + "' ไม่สำเร็จ")
	}
	if fillMerged {
		rows = fillVerticalMerges(w.xl, name, rows)
		w.merged[name] = true
	}
	w.full[name] = rows
	return rows, nil
}

// readSheetRowsLimit: อ่านชีตแค่ limit แถวแรก แล้วตัดแถวว่างท้ายทิ้ง
// ตัดแถวว่างท้ายแบบเดียวกับ readSheetAllRows เพื่อให้ผลการให้คะแนนตรงกัน
func readSheetRowsLimit(xl *excelize.File, sheet string, limit int) ([][]string, error) {
	if xl == nil {
		return nil, errors.New("ไม่มีไฟล์ Excel")
	}
	it, err := xl.Rows(sheet)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	results := make([][]string, 0, limit)
	lastFilled := 0
	for it.Next() {
		row, err := it.Columns()
		if err != nil {
			return nil, err
		}
		results = append(results, row)
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				lastFilled = len(results)
				break
			}
		}
		if len(results) >= limit {
			break
		}
	}
	return results[:lastFilled], nil
}
