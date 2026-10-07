package controllers

import "testing"

func TestLooks12Digit(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"878250022802", true},
		{"1234567890", true},
		{"123456789012345", true},
		{"  878250022802  ", true},
		{"123456789", false},
		{"1234567890123456", false},
		{"87825002280A", false},
		{"8782-5002-2802", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := looks12Digit(tc.in); got != tc.want {
			t.Errorf("looks12Digit(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeDigitCell(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"878180022402.0", "878180022402"},
		{"878180022402.00", "878180022402"},
		{"878180022402.000", "878180022402"},
		{"  878180022402  ", "878180022402"},
		{"878180022402", "878180022402"},
		{"878180022402.5", "878180022402.5"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeDigitCell(tc.in); got != tc.want {
			t.Errorf("normalizeDigitCell(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	if got := normalizeDigitCell("1.23E5"); got != "123000" {
		t.Errorf("normalizeDigitCell(1.23E5) = %q, want 123000", got)
	}
}

func TestNormalizeCodeValue(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"YN22E00849FA", "YN22E00849FA"},
		{"yn22e00849fa", "YN22E00849FA"},
		{"YN22-E008 49FA", "YN22E00849FA"},
		{" MC-LC14405563 ", "MCLC14405563"},
		{`="878250022802"`, "878250022802"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeCodeValue(tc.in); got != tc.want {
			t.Errorf("NormalizeCodeValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUnwrapExcelText(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`="878250022802"`, "878250022802"},
		{`="ABC"`, "ABC"},
		{"plain", "plain"},
		{`"quoted"`, `"quoted"`},
		{"", ""},
	}
	for _, tc := range cases {
		if got := unwrapExcelText(tc.in); got != tc.want {
			t.Errorf("unwrapExcelText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAtoiSafe(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"12", 12},
		{"12.0", 12},
		{"  7 ", 7},
		{"abc", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := atoiSafe(tc.in); got != tc.want {
			t.Errorf("atoiSafe(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestPickField(t *testing.T) {
	m := map[string]string{
		"Machine": "LX10400690",
		"empty":   "   ",
		"Product": "SK75",
	}
	if got := pickField(m, "missing", "empty", "Machine"); got != "LX10400690" {
		t.Errorf("pickField priority = %q, want LX10400690", got)
	}
	if got := pickField(m, "missing1", "missing2"); got != "" {
		t.Errorf("pickField none = %q, want empty", got)
	}
}

func TestMachineFromRow(t *testing.T) {
	if got := machineFromRow(map[string]string{"Machine No": "A1"}); got != "A1" {
		t.Errorf("machineFromRow standard = %q, want A1", got)
	}
	if got := machineFromRow(map[string]string{"[+] Machine": "B2"}); got != "B2" {
		t.Errorf("machineFromRow plus-prefixed = %q, want B2", got)
	}
	if got := machineFromRow(map[string]string{"nothing": "x"}); got != "" {
		t.Errorf("machineFromRow none = %q, want empty", got)
	}
}
