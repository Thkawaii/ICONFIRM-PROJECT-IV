package controllers

import (
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// บรรทัดสรุปการอัปโหลดไฟล์ License ที่เก็บลง Audit Log
//
// ต้องตอบได้ว่า "ใคร / เมื่อไร / ไฟล์อะไร / กี่รายการ / สำเร็จกี่ / error กี่"
// ส่วนใคร-เมื่อไร Audit Log เก็บไว้ในคอลัมน์ของตัวเองอยู่แล้ว (Name, ActionDatetime)
// ที่นี่จึงเก็บเฉพาะชื่อไฟล์กับตัวเลข ในรูปแบบเดียวกันทั้ง 3 ประเภท:
//
//	import.xlsx | total=178 new=178 update=0 delete=0 error=0
//
// ฝั่งหน้าเว็บอ่านบรรทัดนี้ไปแสดงในตาราง Log การอัปโหลดได้ตรง ๆ
// ---------------------------------------------------------------------------

const licenseUploadLogMaxLen = 500

// licenseUploadLogLine: ประกอบบรรทัดสรุปการอัปโหลด
//
//	total  = แถวที่อ่านได้จากไฟล์ทั้งหมด
//	added  = เพิ่มใหม่ · updated = แก้ของเดิม · deleted = ลบเพราะไม่มีในไฟล์แล้ว
//	errors = แถวที่มีปัญหา (ข้ามไป)
func licenseUploadLogLine(fileName string, total, added, updated, deleted, errors int) string {
	parts := []string{
		"total=" + strconv.Itoa(total),
		"new=" + strconv.Itoa(added),
		"update=" + strconv.Itoa(updated),
		"delete=" + strconv.Itoa(deleted),
		"error=" + strconv.Itoa(errors),
	}
	line := strings.TrimSpace(fileName) + " | " + strings.Join(parts, " ")
	if len(line) > licenseUploadLogMaxLen {
		line = line[:licenseUploadLogMaxLen]
	}
	return line
}

// licenseUploadLogFileName: แยกชื่อไฟล์กลับออกมาจากบรรทัดสรุป
// (บรรทัดเก่าที่เก็บแค่ชื่อไฟล์ล้วน ๆ ก็ยังอ่านได้)
func licenseUploadLogFileName(line string) string {
	if i := strings.Index(line, " | "); i >= 0 {
		return strings.TrimSpace(line[:i])
	}
	return strings.TrimSpace(line)
}

// licenseUploadLogCounts: ดึงตัวเลขจากบรรทัดสรุป (ไม่มี = 0, ok = false)
func licenseUploadLogCounts(line string) (map[string]int, bool) {
	i := strings.Index(line, " | ")
	if i < 0 {
		return nil, false
	}
	out := map[string]int{}
	for _, f := range strings.Fields(line[i+3:]) {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if n, err := strconv.Atoi(kv[1]); err == nil {
			out[kv[0]] = n
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}
