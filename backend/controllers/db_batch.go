package controllers

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
)

const (
	dbInListChunk = 10000

	dbInsertBatch = 500

	maxUploadProblems = 300

	// dbRepositionBatch = จำนวนแถวต่อ 1 คำสั่ง UPDATE ตอนย้ายลำดับ
	// 500 แถว = ~1,001 พารามิเตอร์ ยังห่างจากเพดาน 65,535 ของ Postgres มาก
	dbRepositionBatch = 500
)

// rowReposition = แถวเดิมที่ข้อมูลไม่เปลี่ยน แค่ย้ายตำแหน่งในไฟล์
type rowReposition struct {
	id   uint
	sort int64
}

// applyRepositions: อัปเดต sort_order (และชื่อไฟล์) ของหลายแถวในคำสั่งเดียว
//
// ของเดิมยิง UPDATE ทีละแถว ซึ่งเป็นคอขวดหลักของการอัปโหลด:
// เคสที่เจอบ่อยที่สุดคือผู้ใช้แก้ไม่กี่แถวแล้วอัปไฟล์เดิมซ้ำ แถวที่เหลือทั้งหมด
// "ข้อมูลเหมือนเดิม แต่ต้องยืนยันลำดับ" จึงตกมาอยู่ใน moves ทั้งก้อน
// ไฟล์ชีต Export ที่มีเก้าพันแถวจึงกลายเป็น UPDATE เก้าพันครั้ง = เก้าพัน round-trip
// ทั้งที่ข้อมูลไม่ได้เปลี่ยนสักแถว
//
// รวมเป็น CASE เดียวต่อ 500 แถว ลดจำนวนคำสั่งลงประมาณ 500 เท่า
// ใช้ได้ทั้ง Postgres (ของจริง) และ SQLite (ในเทสต์) จึงไม่ต้องแยกโค้ดตาม dialect
func applyRepositions(db *gorm.DB, model interface{}, moves []rowReposition, fileName string) error {
	if db == nil || len(moves) == 0 {
		return nil
	}
	for _, part := range chunkSlice(moves, dbRepositionBatch) {
		var expr strings.Builder
		args := make([]interface{}, 0, len(part)*2)
		ids := make([]uint, 0, len(part))

		expr.WriteString("CASE id")
		for _, m := range part {
			expr.WriteString(" WHEN ? THEN ?")
			args = append(args, m.id, m.sort)
			ids = append(ids, m.id)
		}
		// ELSE กันไว้เฉย ๆ — WHERE id IN คัดให้แล้วว่าทุกแถวต้องเข้าเงื่อนไขใดเงื่อนไขหนึ่ง
		expr.WriteString(" ELSE sort_order END")

		updates := map[string]interface{}{
			"sort_order": gorm.Expr(expr.String(), args...),
		}
		if fileName != "" {
			updates["file_name"] = fileName
		}
		if err := db.Model(model).Where("id IN ?", ids).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func chunkSlice[T any](list []T, size int) [][]T {
	if size <= 0 {
		size = 1
	}
	if len(list) == 0 {
		return nil
	}
	out := make([][]T, 0, (len(list)+size-1)/size)
	for start := 0; start < len(list); start += size {
		end := start + size
		if end > len(list) {
			end = len(list)
		}
		out = append(out, list[start:end:end])
	}
	return out
}

func findWhereInChunks[T any, V any](db *gorm.DB, column string, values []V, out *[]T) error {
	for _, part := range chunkSlice(values, dbInListChunk) {
		var batch []T
		if err := db.Where(column+" IN ?", part).Find(&batch).Error; err != nil {
			return err
		}
		*out = append(*out, batch...)
	}
	return nil
}

func capProblems(problems []string) []string {
	if len(problems) <= maxUploadProblems {
		return problems
	}
	rest := len(problems) - maxUploadProblems
	out := make([]string, 0, maxUploadProblems+1)
	out = append(out, problems[:maxUploadProblems]...)
	out = append(out, "… และอีก "+strconv.Itoa(rest)+" รายการ")
	return out
}

func clampRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
