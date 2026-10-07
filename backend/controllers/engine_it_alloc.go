package controllers

import (
	"errors"
	"mime/multipart"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// ชีต "Engine_IT allocation" — แผนจ่าย Engine / IT# รายเครื่อง
//
// หัวตารางมี 2 ชั้น: แถวบนคือชื่อกลุ่ม (Engine / Delivery / IT#) แถวล่างคือชื่อคอลัมน์จริง
// ซึ่งซ้ำกันระหว่างกลุ่ม (Part no. / Serial no. / Tag no. / 1st Invioce no. / Remark)
//
//	A         B             C               D           E         F           G                   H         I                 J       ... U         V           ...
//	          (Kobelco)                                 Engine                                                                            IT#
//	Lot no.   Machine S/N   Customer name   Main line   Part no.  Serial no.  Serial no. mistake  Tag no.   1st Invioce no.   Remark     Part no.  Serial no.  ...
//
// ตัวอ่านนี้แปลงเป็นตารางหัวเดียว โดยเติมชื่อกลุ่มไว้ข้างหน้า (Engine Part no. / IT Part no. ...)
// เพื่อให้ขั้นตอนอัปโหลดปกติ (findUploadDataHeader → buildStandardRowData) ใช้ต่อได้เลย
// ---------------------------------------------------------------------------

var errNoEngineITAllocSheet = errors.New(
	"ไม่พบข้อมูล Planning WH ในไฟล์นี้ — ต้องมีหัวตารางที่มี Lot no. และ Machine S/N " +
		"พร้อมกลุ่ม Engine / IT# อยู่แถวบน")

// allocMachineNoRe: รูปแบบเลขเครื่อง เช่น YN15438178 / LC11400123
var allocMachineNoRe = regexp.MustCompile(`^[A-Z]{1,4}[0-9]{5,}[A-Z0-9]*$`)

// engineITAllocColumns: ลำดับคอลัมน์ของตารางที่แปลงเสร็จแล้ว
var engineITAllocColumns = []string{
	"Lot no.", "Machine S/N", "Customer name", "Main line", "Product Spec",
	"Engine Part no.", "Engine Serial no.", "Engine Serial no. mistake",
	"Engine Tag no.", "Engine Invoice no.", "Engine Remark",
	"IT Part no.", "IT Serial no.", "IT Serial no. mistake",
	"IT Tag no.", "IT Invoice no.", "IT Remark",
}

// allocSharedHeaders: ชื่อคอลัมน์ที่ซ้ำกันทั้งกลุ่ม Engine และ IT# (normalize แล้ว) → ชื่อท้าย
var allocSharedHeaders = map[string]string{
	"partno":          "Part no.",
	"serialno":        "Serial no.",
	"serialnomistake": "Serial no. mistake",
	"tagno":           "Tag no.",
	"1stinvioceno":    "Invoice no.",
	"1stinvoiceno":    "Invoice no.",
	"invoiceno":       "Invoice no.",
	"remark":          "Remark",
	"remarks":         "Remark",
}

// allocPlainHeaders: คอลัมน์ที่ไม่ขึ้นกับกลุ่ม (normalize แล้ว) → ชื่อมาตรฐาน
var allocPlainHeaders = map[string]string{
	"lotno":             "Lot no.",
	"lot":               "Lot no.",
	"machinesn":         "Machine S/N",
	"machineno":         "Machine S/N",
	"machine":           "Machine S/N",
	"mcno":              "Machine S/N",
	"customername":      "Customer name",
	"customer":          "Customer name",
	"mainline":          "Main line",
	"productspec":       "Product Spec",
	"speccode":          "Product Spec",
	"specificationcode": "Product Spec",
}

// allocGroupOf: แปลงชื่อกลุ่มที่อยู่แถวบนเป็นคำนำหน้า ("" = ไม่ใช่กลุ่มที่สนใจ)
func allocGroupOf(raw string) string {
	switch normalizeHeader(raw) {
	case "engine":
		return "Engine"
	case "it", "it#", "itcontroller", "itno":
		return "IT"
	}
	return ""
}

// findEngineITAllocHeader: หาแถวหัวตาราง แล้วคืน map ดัชนีคอลัมน์ → ชื่อคอลัมน์มาตรฐาน
// คืน -1 ถ้าชีตนี้ไม่ใช่ Engine_IT allocation
func findEngineITAllocHeader(rows [][]string) (int, map[int]string) {
	limit := 40
	if len(rows) < limit {
		limit = len(rows)
	}

	for i := 0; i < limit; i++ {
		hasLot, hasMachine := false, false
		for c := range rows[i] {
			switch normalizeHeader(cellAt(rows, i, c)) {
			case "lotno":
				hasLot = true
			case "machinesn":
				hasMachine = true
			}
		}
		if !hasLot || !hasMachine {
			continue
		}

		// ไล่หากลุ่มจากแถวบน: กลุ่มเขียนไว้เฉพาะคอลัมน์แรกของกลุ่ม แล้วมีผลต่อไปทางขวา
		group := map[int]string{}
		current := ""
		width := len(rows[i])
		if i > 0 && len(rows[i-1]) > width {
			width = len(rows[i-1])
		}
		for c := 0; c < width; c++ {
			if i > 0 {
				if g := allocGroupOf(cellAt(rows, i-1, c)); g != "" {
					current = g
				} else if strings.TrimSpace(cellAt(rows, i-1, c)) != "" {
					// กลุ่มอื่น (เช่น Delivery / Kobelco) → ออกจากกลุ่มที่สนใจ
					current = ""
				}
			}
			group[c] = current
		}

		out := map[int]string{}
		hits := 0
		for c := 0; c < len(rows[i]); c++ {
			key := normalizeHeader(cellAt(rows, i, c))
			if key == "" {
				continue
			}
			if label, ok := allocPlainHeaders[key]; ok {
				if _, taken := labelTaken(out, label); !taken {
					out[c] = label
					hits++
				}
				continue
			}
			if tail, ok := allocSharedHeaders[key]; ok {
				g := group[c]
				if g == "" {
					continue // อยู่ในกลุ่มที่ไม่ได้ใช้ (เช่น Delivery)
				}
				label := g + " " + tail
				if _, taken := labelTaken(out, label); !taken {
					out[c] = label
					hits++
				}
			}
		}

		if hits < 4 {
			continue
		}
		return i, out
	}
	return -1, nil
}

func labelTaken(out map[int]string, label string) (int, bool) {
	for idx, l := range out {
		if l == label {
			return idx, true
		}
	}
	return 0, false
}

// engineITAllocRows: แปลงแถวข้อมูลในชีตเป็น map ชื่อคอลัมน์ → ค่า
func engineITAllocRows(rows [][]string) []map[string]string {
	headerIdx, layout := findEngineITAllocHeader(rows)
	if headerIdx < 0 {
		return nil
	}

	machineCol := -1
	for c, label := range layout {
		if label == "Machine S/N" {
			machineCol = c
			break
		}
	}
	if machineCol < 0 {
		return nil
	}

	var out []map[string]string
	for r := headerIdx + 1; r < len(rows); r++ {
		mc := strings.ToUpper(strings.TrimSpace(unwrapExcelText(cellAt(rows, r, machineCol))))
		if !allocMachineNoRe.MatchString(mc) {
			continue
		}
		data := map[string]string{}
		for c, label := range layout {
			v := strings.TrimSpace(unwrapExcelText(cellAt(rows, r, c)))
			if v != "" && v != "-" {
				data[label] = v
			}
		}
		data["Machine S/N"] = mc
		out = append(out, data)
	}
	return out
}

// engineITAllocTable: สร้างตารางปกติ (หัวคอลัมน์ + แถวข้อมูล) ให้ขั้นตอนอัปโหลดเดิมใช้ต่อ
func engineITAllocTable(blocks []map[string]string) [][]string {
	table := make([][]string, 0, len(blocks)+1)
	table = append(table, append([]string{}, engineITAllocColumns...))
	for _, b := range blocks {
		row := make([]string, len(engineITAllocColumns))
		for i, label := range engineITAllocColumns {
			row[i] = b[label]
		}
		table = append(table, row)
	}
	return table
}

// readEngineITAllocSheets: อ่านทุกชีตในไฟล์ รวมแถวที่เป็น Engine_IT allocation เข้าด้วยกัน
// ok=false = ไม่ใช่ไฟล์รูปแบบนี้
func readEngineITAllocSheets(fileHeader *multipart.FileHeader) ([][]string, bool, error) {
	sheets, err := readAllUploadedSheets(fileHeader)
	if err != nil {
		return nil, false, err
	}
	var blocks []map[string]string
	seen := map[string]bool{}
	found := false
	for _, sh := range sheets {
		b := engineITAllocRows(sh.rows)
		if b == nil {
			continue
		}
		found = true
		for _, row := range b {
			key := NormalizeCodeValue(row["Machine S/N"])
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			blocks = append(blocks, row)
		}
	}
	if !found {
		return nil, false, nil
	}
	return engineITAllocTable(blocks), true, nil
}
