package models

import (
	"math"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// กติกาของใบอนุญาตนำออก
//
// เดิมไฟล์นี้มีทั้งตาราง export_license_items และกติกาเรื่องวันที่/สถานะ
// ตารางถูกถอดออกแล้ว เพราะข้อมูลใบนำออกทุกอย่างอยู่ในตาราง license_items
// (บัญชีแสดงหมายเลขเครื่อง มีคอลัมน์เลขใบอนุญาตนำออกอยู่ในแถวเดียวกับใบนำเข้า)
// และชีตต่ออายุใน license_renewals
//
// ที่เหลือในไฟล์นี้คือกติกาล้วน ๆ ซึ่งยังถูกใช้ทั้งโปรเจค
// ---------------------------------------------------------------------------

const ExportLicenseValidityMonths = 1

// ExportLicenseFirstRound = รอบแรก (ยังไม่เคยต่ออายุ)
const ExportLicenseFirstRound = 1

// ExportLicenseMaxRound = กันเลขรอบเพี้ยนจากไฟล์ (เช่นเผลอใส่ปี ค.ศ. ลงช่องนี้)
const ExportLicenseMaxRound = 9999

// exportNoteDoneWords = คำในช่อง Note / หมายเหตุ ที่ถือว่า "เสร็จแล้ว"
var exportNoteDoneWords = []string{"เสร็จแล้ว", "เสร็จสิ้น", "เสร็จเรียบร้อย", "completed", "complete", "done", "finish"}

// exportNoteNegateThai = คำปฏิเสธภาษาไทย ถ้าอยู่ใกล้ ๆ คำว่า "เสร็จแล้ว" ถือว่ายังไม่เสร็จ
// ครอบคลุมทั้ง "ยังไม่เสร็จ" และ "ไม่เสร็จ"
const exportNoteNegateThai = "ไม่"

// exportNoteNegatePrefix = คำนำหน้าภาษาอังกฤษที่พลิกความหมาย เช่น not done, incomplete, unfinished
var exportNoteNegatePrefix = []string{"not", "non", "un", "in"}

// NoteMeansCompleted บอกว่าข้อความในช่อง Remark / Note ถือเป็นสถานะเสร็จแล้วหรือไม่
//
// ช่องนี้เป็นข้อความอิสระ ผู้ใช้มักพิมพ์ยาว ๆ เช่น
//
//	"เนื่องจากเจ้าหน้าที่ยังไม่ออกใบอนุญาต จึงใช้ใบเดิมไปก่อน ... เสร็จแล้ว 26/06/24"
//
// จึงเช็คแบบ "มีคำว่าเสร็จแล้วอยู่ในข้อความ" แต่ดูคำปฏิเสธเฉพาะที่ติดกับคำนั้นจริง ๆ
// (ภายใน 12 ตัวอักษรก่อนหน้า) เพื่อไม่ให้คำว่า "ยังไม่" ที่อยู่คนละประโยคมาทำให้สถานะเพี้ยน
func NoteMeansCompleted(note string) bool {
	s := strings.ToLower(strings.TrimSpace(note))
	if s == "" {
		return false
	}
	for _, w := range exportNoteDoneWords {
		if hasUnnegatedWord(s, w) {
			return true
		}
	}
	return false
}

// negationLookBehind = จำนวนตัวอักษรก่อนหน้าคำว่า "เสร็จแล้ว" ที่ถือว่าเป็นประโยคเดียวกัน
const negationLookBehind = 12

// hasUnnegatedWord: มีคำ word อยู่ในข้อความ และอย่างน้อยหนึ่งตำแหน่งไม่ได้ถูกปฏิเสธ
func hasUnnegatedWord(s, word string) bool {
	runes := []rune(s)
	from := 0
	for {
		idx := strings.Index(s[from:], word)
		if idx < 0 {
			return false
		}
		abs := from + idx
		// แปลงตำแหน่ง byte เป็นตำแหน่ง rune เพื่อตัดหน้าต่างภาษาไทยได้ถูกต้อง
		head := len([]rune(s[:abs]))
		start := head - negationLookBehind
		if start < 0 {
			start = 0
		}
		if !isNegated(string(runes[start:head])) {
			return true
		}
		from = abs + len(word)
		if from >= len(s) {
			return false
		}
	}
}

// isNegated: ข้อความสั้น ๆ ที่อยู่หน้าคำว่า "เสร็จแล้ว" เป็นคำปฏิเสธหรือไม่
//
//	ไทย    — มีคำว่า "ไม่" อยู่ในช่วงนั้น (ยังไม่เสร็จ / ไม่เสร็จ)
//	อังกฤษ — ลงท้ายด้วย not / non / un / in แบบติดกับคำ (not done / incomplete / unfinished)
//	         เช็คแบบ "ลงท้าย" เพื่อไม่ให้คำอย่าง invoice ที่บังเอิญมี in อยู่ข้างหน้าถูกนับเป็นปฏิเสธ
func isNegated(window string) bool {
	if strings.Contains(window, exportNoteNegateThai) {
		return true
	}
	trimmed := strings.TrimRight(window, " \t-_./")
	for _, p := range exportNoteNegatePrefix {
		if strings.HasSuffix(trimmed, p) {
			return true
		}
	}
	return false
}

// NormalizeRenewalRound บังคับให้ครั้งที่ต่ออายุอยู่ในช่วงที่ใช้งานได้ (ว่าง/0 = รอบแรก)
func NormalizeRenewalRound(n int) int {
	if n < ExportLicenseFirstRound {
		return ExportLicenseFirstRound
	}
	if n > ExportLicenseMaxRound {
		return ExportLicenseMaxRound
	}
	return n
}

// ExportLicenseLeadDays = นับกี่วันปฏิทินจาก "วันออกใบ" ถึงวันครบกำหนดยื่นต่ออายุ
//
// กติกาที่ตกลงกับหน้างาน
//     วันหมดอายุ         = วันออกใบ + 1 เดือนปฏิทิน
//     ครบกำหนดยื่นต่ออายุ = วันออกใบ + 15 วันปฏิทิน
//
// นับจากวันออกใบ ไม่ใช่ถอยหลังจากวันหมดอายุ เพราะเดือนยาวไม่เท่ากัน
// สองวิธีให้ผลต่างกันทุกเดือนที่ไม่ใช่ 30 วัน
const ExportLicenseLeadDays = 15

const ExportLicenseLeadWarnDays = 7


const (
	ExportLeadOverdue = "LEAD_OVERDUE"
	ExportLeadDue     = "LEAD_DUE"
	ExportLeadNoDate  = "LEAD_NO_DATE"
)

func AddMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	target := first.AddDate(0, months, 0)

	last := time.Date(target.Year(), target.Month()+1, 1, 0, 0, 0, 0, target.Location()).AddDate(0, 0, -1).Day()
	if d > last {
		d = last
	}
	return time.Date(target.Year(), target.Month(), d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

func DaysBetween(from, to time.Time) int {
	a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	b := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, from.Location())
	return int(math.Round(b.Sub(a).Hours() / 24))
}

