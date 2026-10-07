package controllers

import (
	"strings"
	"testing"
)

// ไม่มีคอลัมน์แปลกปลอม = ไม่ต้องรบกวนผู้ใช้
func TestExtraColumnNoticeEmpty(t *testing.T) {
	var tr extraColumnTracker
	if got := tr.notice(); got != "" {
		t.Fatalf("ไม่มีคอลัมน์ใหม่ต้องไม่มีข้อความ ได้ %q", got)
	}
	if len(tr.labels()) != 0 {
		t.Fatalf("ต้องไม่มีรายชื่อคอลัมน์")
	}
}

// ข้อความต้องบอกจำนวน ตามด้วยชื่อคอลัมน์ตามลำดับในไฟล์ และบอกว่าไม่ได้เอาไปคำนวณ
func TestExtraColumnNoticeText(t *testing.T) {
	var tr extraColumnTracker
	tr.add("PO NUMBER")
	tr.add("VESSEL")
	tr.add("ETA")

	want := "พบคอลัมน์ใหม่ 3 คอลัมน์: PO NUMBER, VESSEL, ETA — จะเก็บไว้แต่ไม่ถูกนำไปคำนวณ"
	if got := tr.notice(); got != want {
		t.Fatalf("ข้อความผิด\nได้     %q\nต้องการ %q", got, want)
	}
}

// ชื่อซ้ำ (เพราะไล่ทุกแถว) ต้องนับครั้งเดียว และช่องว่างต้องไม่ถูกนับ
func TestExtraColumnTrackerDedupe(t *testing.T) {
	var tr extraColumnTracker
	for i := 0; i < 500; i++ {
		tr.add("PO NUMBER")
		tr.add("  VESSEL  ")
		tr.add("")
		tr.add("   ")
	}
	labels := tr.labels()
	if len(labels) != 2 {
		t.Fatalf("ต้องเหลือ 2 คอลัมน์ ได้ %v", labels)
	}
	if labels[0] != "PO NUMBER" || labels[1] != "VESSEL" {
		t.Fatalf("ชื่อหรือลำดับผิด: %v", labels)
	}
}

// คอลัมน์เยอะเกินไป ให้ยุบท้ายข้อความ ไม่ใช่พิมพ์ออกมาทั้งหมด
func TestExtraColumnNoticeCapsList(t *testing.T) {
	var tr extraColumnTracker
	for _, c := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"} {
		tr.add(c)
	}

	got := tr.notice()
	if !strings.HasPrefix(got, "พบคอลัมน์ใหม่ 12 คอลัมน์: ") {
		t.Fatalf("ต้องรายงานจำนวนเต็มทั้ง 12 คอลัมน์: %q", got)
	}
	if !strings.Contains(got, "และอีก 2 คอลัมน์") {
		t.Fatalf("ส่วนที่เกินต้องถูกยุบ: %q", got)
	}
	if strings.Contains(got, "K") || strings.Contains(got, "L") {
		t.Fatalf("ต้องไม่พิมพ์ชื่อคอลัมน์ที่เกินออกมา: %q", got)
	}
	// ถึงจะยุบรายชื่อ ก็ยังต้องบอกว่าเก็บไว้แต่ไม่ได้คำนวณ
	if !strings.HasSuffix(got, "— จะเก็บไว้แต่ไม่ถูกนำไปคำนวณ") {
		t.Fatalf("ท้ายข้อความผิด: %q", got)
	}
}

// labels() ต้องคืนสำเนา ไม่ใช่ตัวจริง ไม่งั้นผู้เรียกแก้ของภายในได้
func TestExtraColumnLabelsIsCopy(t *testing.T) {
	var tr extraColumnTracker
	tr.add("PO NUMBER")

	got := tr.labels()
	got[0] = "เปลี่ยนแล้ว"

	if tr.labels()[0] != "PO NUMBER" {
		t.Fatal("labels() ต้องคืนสำเนา")
	}
}
