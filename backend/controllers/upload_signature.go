package controllers

import (
	"sort"
	"strings"
)

// uploadDataSignature สร้างลายเซ็นของแถว (คอลัมน์ที่มีค่า เรียงตามชื่อ) ใช้กันแถวซ้ำตอนอัปโหลด
//   - only != nil  → นับเฉพาะคอลัมน์มาตรฐานที่ระบุ
//   - ignore       → ข้ามคอลัมน์ที่เลิกใช้แล้ว (เช่น Note เดิมของ Planning/Engine)
func uploadDataSignature(data map[string]string, only map[string]bool, ignore ...map[string]bool) string {
	var skip map[string]bool
	if len(ignore) > 0 {
		skip = ignore[0]
	}
	keys := make([]string, 0, len(data))
	for k, v := range data {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if only != nil && !only[k] {
			continue
		}
		if skip != nil && skip[normalizeHeader(strings.TrimPrefix(k, extraColumnPrefix))] {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(0)
		b.WriteString(strings.TrimSpace(data[k]))
		b.WriteByte(1)
	}
	return b.String()
}
