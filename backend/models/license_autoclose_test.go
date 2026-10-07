package models

import (
	"testing"
	"time"
)

var autoCloseNow = time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

func daysAgo(n int) *time.Time {
	d := autoCloseNow.AddDate(0, 0, -n)
	return &d
}

// ใบที่หมดอายุเกินระยะผ่อนผันแล้วเท่านั้นที่ถือว่าปิดจบ
func TestLicenseAutoCloseDueOnlyPastGrace(t *testing.T) {
	cases := []struct {
		name   string
		expiry *time.Time
		want   bool
	}{
		{"ยังไม่หมดอายุ", daysAgo(-5), false},
		{"หมดอายุวันนี้", daysAgo(0), false},
		{"หมดอายุมา 10 วัน ยังอยู่ในระยะผ่อนผัน", daysAgo(10), false},
		{"หมดอายุมา 30 วัน พอดีเส้น ยังไม่ปิด", daysAgo(30), false},
		{"หมดอายุมา 31 วัน เกินแล้ว", daysAgo(31), true},
		{"หมดอายุมาเป็นปี", daysAgo(400), true},
	}

	for _, c := range cases {
		if got := LicenseAutoCloseDue(c.expiry, DefaultLicenseAutoCloseDays, autoCloseNow); got != c.want {
			t.Errorf("%s: ได้ %v ต้องการ %v", c.name, got, c.want)
		}
	}
}

// ค่าติดลบ = ปิดการทำงาน ใบเก่าแค่ไหนก็ต้องไม่ถูกปิดให้เอง
func TestLicenseAutoCloseDisabledWithNegativeDays(t *testing.T) {
	if LicenseAutoCloseDue(daysAgo(999), LicenseAutoCloseDisabled, autoCloseNow) {
		t.Fatal("ค่าปิดการทำงานต้องไม่ปิดใบให้อัตโนมัติ")
	}
}

// ตั้ง 0 = พอหมดอายุก็ปิดเลย ต้องแยกออกจาก "ปิดการทำงาน" ให้ได้
func TestLicenseAutoCloseZeroMeansCloseOnExpiry(t *testing.T) {
	if !LicenseAutoCloseDue(daysAgo(1), 0, autoCloseNow) {
		t.Fatal("ตั้ง 0 แล้วใบที่หมดอายุไปแล้ว 1 วัน ต้องถูกปิด")
	}
	if LicenseAutoCloseDue(daysAgo(0), 0, autoCloseNow) {
		t.Fatal("ใบที่หมดอายุวันนี้ยังใช้ได้ถึงสิ้นวัน ต้องยังไม่ถูกปิด")
	}
	if LicenseAutoCloseDue(daysAgo(-3), 0, autoCloseNow) {
		t.Fatal("ใบที่ยังไม่หมดอายุต้องไม่ถูกปิด")
	}
}

// นับถอยหลังที่โชว์บนหน้าเว็บ ต้องตรงกับวันที่ใบจะถูกปิดจริง
func TestLicenseAutoCloseInDays(t *testing.T) {
	// หมดอายุมา 27 วัน ระยะผ่อนผัน 30 วัน → ถูกปิดตอนเลยมา 31 วัน = อีก 4 วัน
	if got := LicenseAutoCloseInDays(daysAgo(27), 30, autoCloseNow); got == nil || *got != 4 {
		t.Fatalf("เหลืออีกกี่วัน = %v ต้องการ 4", got)
	}
	// ยังไม่หมดอายุ ยังไม่ต้องนับถอยหลัง
	if got := LicenseAutoCloseInDays(daysAgo(-5), 30, autoCloseNow); got != nil {
		t.Fatalf("ใบที่ยังไม่หมดอายุต้องไม่มีตัวนับถอยหลัง ได้ %v", *got)
	}
	// เลยคิวปิดไปแล้ว ไม่ต้องนับถอยหลังอีก
	if got := LicenseAutoCloseInDays(daysAgo(40), 30, autoCloseNow); got != nil {
		t.Fatalf("ใบที่ถึงคิวปิดแล้วต้องไม่มีตัวนับถอยหลัง ได้ %v", *got)
	}
	// ปิดการทำงานอยู่ ไม่มีวันถูกปิด จึงไม่ต้องนับถอยหลัง
	if got := LicenseAutoCloseInDays(daysAgo(27), LicenseAutoCloseDisabled, autoCloseNow); got != nil {
		t.Fatalf("ตอนปิดการทำงานต้องไม่มีตัวนับถอยหลัง ได้ %v", *got)
	}
}

// ใบที่ยังไม่ระบุวันหมดอายุ อาจเป็นใบที่เพิ่งยื่นและรอเลขจริง ห้ามปิดให้
func TestLicenseAutoCloseSkipsMissingExpiry(t *testing.T) {
	if LicenseAutoCloseDue(nil, DefaultLicenseAutoCloseDays, autoCloseNow) {
		t.Fatal("ใบที่ไม่มีวันหมดอายุต้องไม่ถูกปิดให้อัตโนมัติ")
	}
}

func TestLicenseAutoCloseOverdueDays(t *testing.T) {
	if got := LicenseAutoCloseOverdueDays(daysAgo(38), autoCloseNow); got != 38 {
		t.Fatalf("หมดอายุมาแล้ว %d วัน ต้องการ 38", got)
	}
	if got := LicenseAutoCloseOverdueDays(daysAgo(-7), autoCloseNow); got != 0 {
		t.Fatalf("ใบที่ยังไม่หมดอายุต้องได้ 0 ได้ %d", got)
	}
	if got := LicenseAutoCloseOverdueDays(nil, autoCloseNow); got != 0 {
		t.Fatalf("ใบที่ไม่มีวันหมดอายุต้องได้ 0 ได้ %d", got)
	}
}

// ข้อความที่ระบบเขียนไว้ตอนปิดใบ ต้องถูกตีความว่า "เสร็จสิ้น"
// ไม่งั้นหน้าเว็บกับฐานข้อมูลจะกลับมาเห็นไม่ตรงกันอีก
func TestLicenseAutoCloseNoteReadsAsCompleted(t *testing.T) {
	if !NoteMeansCompleted(LicenseAutoCloseNote) {
		t.Fatalf("ข้อความปิดอัตโนมัติต้องถูกอ่านว่าเสร็จสิ้น: %q", LicenseAutoCloseNote)
	}
}
