package controllers

import (
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// คอลัมน์ที่ระบบไม่รู้จักในไฟล์ที่อัปโหลด
//
// ไฟล์ของผู้ใช้โตขึ้นเรื่อย ๆ — มีคนเพิ่มคอลัมน์ใหม่เข้าไปเองโดยไม่ได้บอกใคร
// ระบบเก็บค่าพวกนี้ไว้ในช่อง extra_json อยู่แล้ว ข้อมูลไม่หาย แต่เก็บแบบเงียบ ๆ
// ผู้ใช้จึงไม่มีทางรู้ว่าคอลัมน์ที่เพิ่งใส่เข้าไปนั้นระบบรับรู้หรือเปล่า และที่สำคัญกว่า
// คือไม่รู้ว่ามัน "เก็บไว้เฉย ๆ" ไม่ได้ถูกเอาไปคำนวณวันหมดอายุหรือยอดคงเหลือให้
//
// ตัวนี้เก็บชื่อคอลัมน์ที่เจอไว้ตามลำดับในไฟล์ แล้วสรุปเป็นข้อความเดียวส่งกลับ
// ไปบอกผู้ใช้ตอนอัปโหลดเสร็จ
// ---------------------------------------------------------------------------

// extraColumnNoticeLimit = รายชื่อคอลัมน์ที่เอ่ยในข้อความ เกินกว่านี้ยุบเป็น "และอีก N คอลัมน์"
//
// ไฟล์บางเล่มมีคอลัมน์แปลกปลอมเป็นสิบ ถ้าพิมพ์ออกมาหมดจะกลายเป็นข้อความยาวเหยียด
// ที่ไม่มีใครอ่าน — ข้อมูลที่ถูกเก็บยังครบเหมือนเดิม แค่ข้อความสรุปสั้นลง
const extraColumnNoticeLimit = 10

// extraColumnTracker = ชื่อคอลัมน์ที่ไม่รู้จัก เรียงตามลำดับที่เจอในไฟล์
//
// ค่าศูนย์ใช้งานได้ทันที ไม่ต้อง new
type extraColumnTracker struct {
	seen  map[string]bool
	order []string
}

// add: จดชื่อคอลัมน์ไว้ (ชื่อซ้ำนับครั้งเดียว)
//
// รับชื่อคอลัมน์ดิบจากหัวตาราง ไม่ใช่ชื่อที่ติดป้าย "[+] " แล้ว
func (t *extraColumnTracker) add(label string) {
	label = strings.TrimSpace(label)
	if label == "" {
		return
	}
	if t.seen == nil {
		t.seen = map[string]bool{}
	}
	if t.seen[label] {
		return
	}
	t.seen[label] = true
	t.order = append(t.order, label)
}

// labels: ชื่อคอลัมน์ทั้งหมดตามลำดับในไฟล์
func (t *extraColumnTracker) labels() []string {
	if len(t.order) == 0 {
		return []string{}
	}
	out := make([]string, len(t.order))
	copy(out, t.order)
	return out
}

// notice: ข้อความแจ้งผู้ใช้ — คืนค่าว่างเมื่อไม่มีคอลัมน์ใหม่
//
//	พบคอลัมน์ใหม่ 3 คอลัมน์: PO NUMBER, VESSEL, ETA — จะเก็บไว้แต่ไม่ถูกนำไปคำนวณ
func (t *extraColumnTracker) notice() string {
	n := len(t.order)
	if n == 0 {
		return ""
	}

	shown := t.order
	suffix := ""
	if n > extraColumnNoticeLimit {
		shown = t.order[:extraColumnNoticeLimit]
		suffix = " และอีก " + strconv.Itoa(n-extraColumnNoticeLimit) + " คอลัมน์"
	}

	return "พบคอลัมน์ใหม่ " + strconv.Itoa(n) + " คอลัมน์: " +
		strings.Join(shown, ", ") + suffix +
		" — จะเก็บไว้แต่ไม่ถูกนำไปคำนวณ"
}
