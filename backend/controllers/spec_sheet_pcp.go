package controllers

import (
	"errors"
	"mime/multipart"
	"strings"

	"iconfirm/models"
)

// ---------------------------------------------------------------------------
// ชีต "Spec sheet_pcp" (Planning MFG) — ผูก MC# เข้ากับ Product Spec
//
// ชีตนี้มีแถวหัวตารางซ้ำกันหลายแถวติดกัน (2–5) แล้วค่อยเป็นข้อมูล และมีคอลัมน์
// "Product Spec" ซ้ำสองช่อง (ช่องแรก = spec code, ช่องสอง = คำอธิบาย)
// ตัวอ่านนี้เลือกเฉพาะคอลัมน์ที่ใช้ แล้วคัดเอาเฉพาะแถวที่ Machine เป็นเลขเครื่องจริง
//
// WH ใช้ข้อมูลนี้ตอนจ่าย CW / CV / SM / MP / PH เพราะ P/N ผูกกับ spec code
// ไม่ได้ผูกกับเลขเครื่อง
// ---------------------------------------------------------------------------

var errNoSpecSheetPCP = errors.New(
	"ไม่พบข้อมูล Planning MFG ในไฟล์นี้ — ต้องมีหัวตารางที่มี Machine และ Product Spec")

// specPCPColumns: ลำดับคอลัมน์ของตารางที่แปลงเสร็จแล้ว
var specPCPColumns = []string{
	"Line", "LOT NO.", "Machine", "Product Spec", "Domestic/Exp", "Selling comp",
	"KCM Order", "Brand", "Destination", "Base machine spec.", "Boom", "Arm",
	"Shoe", "Counter weight", "Front ATT", "Cab", "Cab guard", "IT device", "Note1",
}

// specPCPHeaders: หัวคอลัมน์ในชีต (normalize แล้ว) → ชื่อคอลัมน์มาตรฐาน
var specPCPHeaders = map[string]string{
	"line":            "Line",
	"lotno":           "LOT NO.",
	"machine":         "Machine",
	"machineno":       "Machine",
	"productspec":     "Product Spec",
	"speccode":        "Product Spec",
	"domesticexp":     "Domestic/Exp",
	"sellingcomp":     "Selling comp",
	"sellingcompany":  "Selling comp",
	"kcmorder":        "KCM Order",
	"brand":           "Brand",
	"destination":     "Destination",
	"basemachinespec": "Base machine spec.",
	"boom":            "Boom",
	"arm":             "Arm",
	"shoe":            "Shoe",
	"counterweight":   "Counter weight",
	"frontatt":        "Front ATT",
	"cab":             "Cab",
	"cabguard":        "Cab guard",
	"itdevice":        "IT device",
	"note1":           "Note1",
}

// findSpecPCPHeader: หาแถวหัวตาราง แล้วคืน map ดัชนีคอลัมน์ → ชื่อคอลัมน์มาตรฐาน
// หัวคอลัมน์ที่ซ้ำ (เช่น Product Spec สองช่อง) เก็บเฉพาะช่องแรก
func findSpecPCPHeader(rows [][]string) (int, map[int]string) {
	limit := 40
	if len(rows) < limit {
		limit = len(rows)
	}

	for i := 0; i < limit; i++ {
		hasMachine, hasSpec := false, false
		for c := range rows[i] {
			switch normalizeHeader(cellAt(rows, i, c)) {
			case "machine", "machineno":
				hasMachine = true
			case "productspec", "speccode":
				hasSpec = true
			}
		}
		if !hasMachine || !hasSpec {
			continue
		}

		out := map[int]string{}
		taken := map[string]bool{}
		for c := 0; c < len(rows[i]); c++ {
			label, ok := specPCPHeaders[normalizeHeader(cellAt(rows, i, c))]
			if !ok || label == "" || taken[label] {
				continue
			}
			taken[label] = true
			out[c] = label
		}
		if len(out) < 3 {
			continue
		}
		return i, out
	}
	return -1, nil
}

// specPCPRows: แปลงแถวข้อมูลในชีตเป็น map ชื่อคอลัมน์ → ค่า
// ข้ามแถวหัวตารางที่ซ้ำกัน โดยดูว่าช่อง Machine เป็นเลขเครื่องจริงหรือไม่
func specPCPRows(rows [][]string) []map[string]string {
	headerIdx, layout := findSpecPCPHeader(rows)
	if headerIdx < 0 {
		return nil
	}

	machineCol := -1
	for c, label := range layout {
		if label == "Machine" {
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
		data["Machine"] = mc
		out = append(out, data)
	}
	return out
}

func specPCPTable(blocks []map[string]string) [][]string {
	table := make([][]string, 0, len(blocks)+1)
	table = append(table, append([]string{}, specPCPColumns...))
	for _, b := range blocks {
		row := make([]string, len(specPCPColumns))
		for i, label := range specPCPColumns {
			row[i] = b[label]
		}
		table = append(table, row)
	}
	return table
}

// readSpecSheetPCPSheets: อ่านทุกชีตในไฟล์ รวมแถวที่เป็น Spec_sheet เข้าด้วยกัน
func readSpecSheetPCPSheets(fileHeader *multipart.FileHeader) ([][]string, bool, error) {
	sheets, err := readAllUploadedSheets(fileHeader)
	if err != nil {
		return nil, false, err
	}
	var blocks []map[string]string
	seen := map[string]bool{}
	found := false
	for _, sh := range sheets {
		b := specPCPRows(sh.rows)
		if b == nil {
			continue
		}
		found = true
		for _, row := range b {
			key := NormalizeCodeValue(row["Machine"])
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
	return specPCPTable(blocks), true, nil
}

// flowSpecSheetIndex: MC# (normalize) → แถวข้อมูลจากชีต Planning MFG ที่อัปโหลดไว้
func flowSpecSheetIndex() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, row := range loadUploadRows(models.DatasetSpecSheet) {
		mc := pickField(row, "Machine", extraColumnPrefix+"Machine")
		key := NormalizeCodeValue(mc)
		if key == "" {
			continue
		}
		if _, ok := out[key]; !ok {
			out[key] = row
		}
	}
	return out
}
