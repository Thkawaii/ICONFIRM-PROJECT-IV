package controllers

import (
	"encoding/json"
	"strings"

	"iconfirm/config"
	"iconfirm/models"
)

// ---------------------------------------------------------------------------
// อัปโหลดไฟล์เดิมที่ user แก้ข้อมูลใน Excel
//
// หาแถวเดิมในระบบของแต่ละแถวในไฟล์:
//   1) คีย์หลักตรง (ทะเบียน = ชนิด + Serial No., Import = หมายเลขเครื่อง, Export = IT Controller S/N)
//   2) ถ้าคีย์หลักไม่ตรง (user แก้คีย์ใน Excel เช่นพิมพ์ Serial ผิดแล้วแก้) ให้หาจากคีย์รอง
//      (ทะเบียน = No. หรือ IMEI, Import = หมายเลขการผลิต, Export = Machine No)
//      ใช้ได้เฉพาะเมื่อ: ตรงกับแถวเดิม "แถวเดียว", คีย์หลักเดิมของแถวนั้นไม่มีอยู่ในไฟล์นี้แล้ว
//      และยังไม่ถูกแถวอื่นในไฟล์จับคู่ไป — กันจับคู่ผิดชิ้น
//
// แถวที่จับคู่ได้แต่สแกนผ่านแล้ว ผู้เรียกจะไม่อัปเดต (ดู scan_lock.go)
// ---------------------------------------------------------------------------

func normKey(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// extraJSONEqual เทียบคอลัมน์เพิ่ม โดยถือว่า "", "{}", "null" คือว่างเหมือนกัน
func extraJSONEqual(a, b string) bool {
	norm := func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" || s == "{}" || s == "null" {
			return ""
		}
		var m map[string]interface{}
		if json.Unmarshal([]byte(s), &m) == nil {
			out, _ := json.Marshal(m)
			return string(out)
		}
		return s
	}
	return norm(a) == norm(b)
}

// ---- ทะเบียนอะไหล่ ----

// ---- Import License ----

func matchImportLicenseExisting(items []models.LicenseItem) ([]*models.LicenseItem, error) {
	out := make([]*models.LicenseItem, len(items))
	if len(items) == 0 {
		return out, nil
	}
	machineNos := make([]string, 0, len(items))
	inFile := map[string]bool{}
	for _, it := range items {
		machineNos = append(machineNos, it.MachineNo)
		inFile[normKey(it.MachineNo)] = true
	}
	var rows []models.LicenseItem
	if err := findWhereInChunks(config.DB, "machine_no", machineNos, &rows); err != nil {
		return nil, err
	}
	byMachine := map[string]models.LicenseItem{}
	for _, r := range rows {
		byMachine[r.MachineNo] = r
	}

	claimed := map[uint]bool{}
	var prods []string
	for i, it := range items {
		if old, ok := byMachine[it.MachineNo]; ok {
			o := old
			out[i] = &o
			claimed[old.ID] = true
		} else if strings.TrimSpace(it.ProductionNo) != "" {
			prods = append(prods, it.ProductionNo)
		}
	}
	if len(prods) == 0 {
		return out, nil
	}

	var cand []models.LicenseItem
	if err := findWhereInChunks(config.DB, "production_no", prods, &cand); err != nil {
		return nil, err
	}
	byProd := map[string][]models.LicenseItem{}
	for _, r := range cand {
		if inFile[normKey(r.MachineNo)] {
			continue
		}
		k := normKey(r.ProductionNo)
		byProd[k] = append(byProd[k], r)
	}
	for i, it := range items {
		if out[i] != nil || strings.TrimSpace(it.ProductionNo) == "" {
			continue
		}
		list := byProd[normKey(it.ProductionNo)]
		if len(list) != 1 || claimed[list[0].ID] {
			continue
		}
		o := list[0]
		out[i] = &o
		claimed[o.ID] = true
	}
	return out, nil
}

// ---- Export License ----

// importLicenseSyncDeletes: รายการของไฟล์ชื่อเดียวกันที่ไม่อยู่ในไฟล์รอบนี้
func importLicenseSyncDeletes(matches []*models.LicenseItem, fileName string) ([]syncDelete, error) {
	if normFileName(fileName) == "" {
		return nil, nil
	}
	present := map[uint]bool{}
	for _, m := range matches {
		if m != nil {
			present[m.ID] = true
		}
	}
	var rows []models.LicenseItem
	if err := config.DB.Where("file_name <> '' AND LOWER(TRIM(file_name)) = ?", normFileName(fileName)).
		Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	var out []syncDelete
	var locks *scanLockIndex
	for i := range rows {
		r := rows[i]
		if present[r.ID] {
			continue
		}
		if locks == nil {
			locks = buildScanLockIndex()
		}
		locked, reason := locks.importLicense(&r)
		out = append(out, syncDelete{id: r.ID, label: r.MachineNo, locked: locked, reason: reason})
	}
	return out, nil
}

