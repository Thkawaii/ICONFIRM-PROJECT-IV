package models

import (
	"testing"
)

func TestNoteMeansCompleted(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"เสร็จแล้ว", true},
		{"  เสร็จแล้ว  ", true},
		{"ส่งของแล้ว เสร็จแล้ว 12/05", true},
		{"เสร็จสิ้น", true},
		{"เสร็จเรียบร้อย", true},
		{"Completed", true},
		{"completed 01", true},
		{"DONE", true},
		{"", false},
		{"รอเจ้าหน้าที่ออกใบอนุญาต", false},
		{"ยังไม่เสร็จ", false},
		{"ยังไม่เสร็จแล้ว", false}, // คำปฏิเสธต้องชนะ
		{"not completed", false},
		{"เนื่องจากเจ้าหน้าที่ยังไม่ออกใบอนุญาต จึงใช้ใบเดิมไปก่อน เสร็จแล้ว 26/06/24", true},
		{"ทำเรื่องต่ออายุแล้ว แต่ยังไม่เสร็จ", false},
		{"ออก invoice เสร็จแล้ว", true}, // คำว่า invoice มี "in" อยู่ ต้องไม่ถูกนับเป็นคำปฏิเสธ
		{"incomplete", false},
		{"unfinished", false},
		{"W/H ไม่ Frist in & Frist Out จึงต่ออายุไม่ทัน", false},
	}
	for _, tc := range cases {
		got := NoteMeansCompleted(tc.in)
		if got != tc.want {
			t.Errorf("NoteMeansCompleted(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeRenewalRound(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, 1},
		{-5, 1},
		{1, 1},
		{2, 2},
		{9999, 9999},
		{100000, 9999},
	}
	for _, tc := range cases {
		if got := NormalizeRenewalRound(tc.in); got != tc.want {
			t.Errorf("NormalizeRenewalRound(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

