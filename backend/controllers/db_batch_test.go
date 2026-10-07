package controllers

import (
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestChunkSliceKeepsEveryItemInOrder(t *testing.T) {
	list := make([]int, 23)
	for i := range list {
		list[i] = i
	}
	parts := chunkSlice(list, 5)
	if len(parts) != 5 {
		t.Fatalf("expected 5 chunks, got %d", len(parts))
	}
	if len(parts[4]) != 3 {
		t.Fatalf("last chunk should hold 3 items, got %d", len(parts[4]))
	}
	next := 0
	for _, p := range parts {
		if len(p) > 5 {
			t.Fatalf("chunk larger than size: %d", len(p))
		}
		for _, v := range p {
			if v != next {
				t.Fatalf("expected %d, got %d", next, v)
			}
			next++
		}
	}
	if chunkSlice([]int{}, 5) != nil {
		t.Fatalf("empty input should give nil")
	}
}

func TestChunkSliceAppendDoesNotClobberNextChunk(t *testing.T) {
	list := []int{1, 2, 3, 4}
	parts := chunkSlice(list, 2)
	_ = append(parts[0], 99)
	if parts[1][0] != 3 {
		t.Fatalf("next chunk was overwritten: %v", parts[1])
	}
}

func TestInsertBatchStaysUnderPostgresParamLimit(t *testing.T) {
	const pgMaxParams = 65535
	const generousColumnCount = 60
	if dbInsertBatch*generousColumnCount > pgMaxParams {
		t.Fatalf("dbInsertBatch=%d too large", dbInsertBatch)
	}
	if dbInListChunk > pgMaxParams {
		t.Fatalf("dbInListChunk=%d too large", dbInListChunk)
	}
}

func TestClampRunesCountsThaiCharacters(t *testing.T) {
	s := strings.Repeat("ก", 300)
	got := clampRunes(s, 255)
	if n := len([]rune(got)); n != 255 {
		t.Fatalf("expected 255 runes, got %d", n)
	}
	if clampRunes("abc", 10) != "abc" {
		t.Fatalf("short string should be unchanged")
	}
}

func TestCapProblems(t *testing.T) {
	var many []string
	for i := 0; i < maxUploadProblems+42; i++ {
		many = append(many, "p"+strconv.Itoa(i))
	}
	got := capProblems(many)
	if len(got) != maxUploadProblems+1 {
		t.Fatalf("expected %d lines, got %d", maxUploadProblems+1, len(got))
	}
	if !strings.Contains(got[len(got)-1], "42") {
		t.Fatalf("last line should mention remaining count: %q", got[len(got)-1])
	}
}

func TestReadSheetAllRowsStopsAtSparseTail(t *testing.T) {
	xl := excelize.NewFile()
	defer xl.Close()
	sheet := xl.GetSheetName(0)

	sw, err := xl.NewStreamWriter(sheet)
	if err != nil {
		t.Fatal(err)
	}
	rowNo := 1
	write := func(vals ...interface{}) {
		cell, _ := excelize.CoordinatesToCellName(1, rowNo)
		if err := sw.SetRow(cell, vals); err != nil {
			t.Fatal(err)
		}
		rowNo++
	}
	write("Item", "Machine No", "IT Controller Serial No.")
	for i := 1; i <= 100; i++ {
		write(i, "M"+strconv.Itoa(i), "S"+strconv.Itoa(i))
	}
	for i := 101; i <= 100+sheetSparseTailStop+5000; i++ {
		write(i)
	}
	if err := sw.Flush(); err != nil {
		t.Fatal(err)
	}

	rows, err := readSheetAllRows(xl, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 101 {
		t.Fatalf("expected header + 100 data rows, got %d", len(rows))
	}
	if rows[100][2] != "S100" {
		t.Fatalf("last data row lost: %v", rows[100])
	}
}

func TestReadSheetAllRowsKeepsSingleColumnFiles(t *testing.T) {
	xl := excelize.NewFile()
	defer xl.Close()
	sheet := xl.GetSheetName(0)

	sw, err := xl.NewStreamWriter(sheet)
	if err != nil {
		t.Fatal(err)
	}
	total := sheetSparseTailStop + 3000
	for i := 1; i <= total; i++ {
		cell, _ := excelize.CoordinatesToCellName(1, i)
		if err := sw.SetRow(cell, []interface{}{"SN" + strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := sw.Flush(); err != nil {
		t.Fatal(err)
	}

	rows, err := readSheetAllRows(xl, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != total {
		t.Fatalf("expected %d rows, got %d", total, len(rows))
	}
}
