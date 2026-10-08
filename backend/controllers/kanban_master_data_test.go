package controllers

import (
	"strings"
	"testing"
)

// QR บน Kanban ของจริง (ช่องที่ 2 = Product Spec, ช่องที่ 7 = CW P/N + น้ำหนักถ่วง)
const kanbanQRSample = "YN15435865,YN15-0TD6LG111001,Indonesia,YN02B10084F1,YN12B20016F1," +
	"800mm HD grouser shoe,YN60C00942P1_4.3T,0.93m3 w/REINF bucket(SEA T)," +
	"Southeast Asia F(less EGR),IT(Satellite  iridium),YN02P00133F2G1,J05ETG59275," +
	"Logging guard,SK200XDL-10, SEA-F Logging"

func TestParseKanbanQRSample(t *testing.T) {
	q, ok := ParseSpecQR(kanbanQRSample)
	if !ok {
		t.Fatal("อ่าน QR บน Kanban ไม่ได้")
	}
	if q.MachineNo != "YN15435865" {
		t.Errorf("MachineNo = %q, want YN15435865", q.MachineNo)
	}
	if q.SpecCode != "YN15-0TD6LG111001" {
		t.Errorf("Product Spec = %q, want YN15-0TD6LG111001", q.SpecCode)
	}
	if q.Customer != "Indonesia" {
		t.Errorf("Customer = %q, want Indonesia", q.Customer)
	}
	if q.CWPN != "YN60C00942P1_4.3T" {
		t.Errorf("CW = %q, want YN60C00942P1_4.3T", q.CWPN)
	}
}

func TestSplitKanbanPartNo(t *testing.T) {
	cases := []struct {
		in, code, suffix string
	}{
		{"YN60C00942P1_4.3T", "YN60C00942P1", "4.3T"},
		{"YN60C00942P1", "YN60C00942P1", ""},
		{" YN60C00942P1_4.3T ", "YN60C00942P1", "4.3T"},
		{"", "", ""},
	}
	for _, tc := range cases {
		code, suffix := splitKanbanPartNo(tc.in)
		if code != tc.code || suffix != tc.suffix {
			t.Errorf("splitKanbanPartNo(%q) = %q, %q; want %q, %q", tc.in, code, suffix, tc.code, tc.suffix)
		}
	}
}

func TestParseTons(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"4.3T", 4.3, true},
		{"4.3", 4.3, true},
		{"4.30", 4.3, true},
		{"4,3", 4.3, true},
		{"4.3 Tons", 4.3, true},
		{"4.3 t", 4.3, true},
		{"4", 4, true},
		{"", 0, false},
		{"-", 0, false},
		{"N/A", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseTons(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseTons(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestSameTons(t *testing.T) {
	same := [][2]string{{"4.3T", "4.3"}, {"4.3T", "4.30"}, {"4.3", "4.3 Tons"}, {"4,3", "4.3T"}}
	for _, p := range same {
		if !SameTons(p[0], p[1]) {
			t.Errorf("SameTons(%q, %q) = false, want true", p[0], p[1])
		}
	}
	diff := [][2]string{{"4.3T", "4.5"}, {"4.3T", "5.0T"}, {"4.3T", ""}, {"", ""}}
	for _, p := range diff {
		if SameTons(p[0], p[1]) {
			t.Errorf("SameTons(%q, %q) = true, want false", p[0], p[1])
		}
	}
}

// เทียบน้ำหนักข้ามหน่วย — แปลงหน่วยก่อนเทียบ
// (เคสจริง: master_data เขียน 6.2T แต่ Kanban เขียน 6.2KG = คนละน้ำหนัก 1000 เท่า)
func TestSameTonsUnits(t *testing.T) {

	// คนละหน่วย แต่เป็นน้ำหนักเดียวกัน → ต้องผ่าน
	same := [][2]string{
		{"6200KG", "6.2T"},
		{"6.2T", "6200KG"},
		{"6.2TON", "6.2T"},
		{"6.2 TONNES", "6.2"},
		{"6.2KGS", "6.2KG"},
		{"4300KG", "4.3"},
	}
	for _, p := range same {
		if !SameTons(p[0], p[1]) {
			t.Errorf("SameTons(%q, %q) = false, want true", p[0], p[1])
		}
	}

	// ตัวเลขเท่ากันแต่คนละหน่วย = คนละน้ำหนักจริง ๆ → ต้องไม่ผ่าน
	diff := [][2]string{
		{"6.2KG", "6.2T"},
		{"6.2T", "6.2KG"},
		{"4.3LB", "4.3T"},
		{"6300KG", "6.2T"},
	}
	for _, p := range diff {
		if SameTons(p[0], p[1]) {
			t.Errorf("SameTons(%q, %q) = true, want false", p[0], p[1])
		}
	}

	// ตัวเลขดิบเท่ากันไหม (ใช้เลือกข้อความแจ้งเตือน ไม่ได้ใช้ตัดสินถูกผิด)
	if !SameWeightNumber("6.2KG", "6.2T") {
		t.Error(`SameWeightNumber("6.2KG", "6.2T") = false, want true`)
	}
	if SameWeightNumber("6.2KG", "6.3T") {
		t.Error(`SameWeightNumber("6.2KG", "6.3T") = true, want false`)
	}
}

// น้ำหนักเขียนคั่นหลักพัน ("6,200KG") ต้องอ่านได้เป็น 6200 กก. ไม่ใช่ 6.2 หรือ 6
func TestWeightThousandSeparator(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		unit string
	}{
		{"6,200KG", 6200, "KG"},
		{"6,200 kg", 6200, "KG"},
		{"4,3T", 4.3, "T"},
		{"4,30", 4.3, "T"},
		{"1,250KG", 1250, "KG"},
	}
	for _, tc := range cases {
		got, unit, ok := parseWeight(tc.in)
		if !ok || got != tc.want || unit != tc.unit {
			t.Errorf("parseWeight(%q) = %v %q, %v; want %v %q", tc.in, got, unit, ok, tc.want, tc.unit)
		}
	}

	if !SameTons("6,200KG", "6.2T") {
		t.Error(`SameTons("6,200KG", "6.2T") = false, want true`)
	}
}

// จุลภาคในน้ำหนักบน Kanban ไปชนกับตัวคั่นช่องของ QR — ต้องต่อช่องกลับให้ถูก
func TestParseSpecQRThousandSeparator(t *testing.T) {
	raw := "YN30100010,LB08-0TOCBG1VU110,Indonesia,YN02B10321F1,YN12B20016F1," +
		"800mm HD grouser shoe,LB60C00210P1_6,200KG,none,Indonesia,IT(Satellite)"

	q, ok := ParseSpecQR(raw)
	if !ok {
		t.Fatal("ParseSpecQR = not ok, want ok")
	}
	if q.CWPN != "LB60C00210P1_6200KG" {
		t.Errorf("CWPN = %q, want LB60C00210P1_6200KG", q.CWPN)
	}
	// ช่องหลัง CW ต้องไม่เลื่อน
	if q.ITDevice != "IT(Satellite)" {
		t.Errorf("ITDevice = %q, want IT(Satellite)", q.ITDevice)
	}

	pn, weight := splitKanbanPartNo(q.CWPN)
	if pn != "LB60C00210P1" || weight != "6200KG" {
		t.Errorf("splitKanbanPartNo = %q / %q, want LB60C00210P1 / 6200KG", pn, weight)
	}
	if !SameTons(weight, "6.2T") {
		t.Errorf("SameTons(%q, 6.2T) = false, want true", weight)
	}
}

// ช่องอื่นที่ขึ้นต้นด้วยตัวเลข 3 หลักต้องไม่ถูกต่อเข้าด้วยกันโดยพลาด
func TestParseSpecQRKeepsShoeField(t *testing.T) {
	raw := "YN15438324,YN15-0QD7BG131001,Singapore,YN02B10321F1,YN12B20016F1," +
		"800mm HD grouser shoe,YN60C00942P1_4.3T,none,Singapore,IT(iridium)"

	q, ok := ParseSpecQR(raw)
	if !ok {
		t.Fatal("ParseSpecQR = not ok, want ok")
	}
	if q.Shoe != "800mm HD grouser shoe" {
		t.Errorf("Shoe = %q, want 800mm HD grouser shoe", q.Shoe)
	}
	if q.CWPN != "YN60C00942P1_4.3T" {
		t.Errorf("CWPN = %q, want YN60C00942P1_4.3T", q.CWPN)
	}
	if q.ITDevice != "IT(iridium)" {
		t.Errorf("ITDevice = %q, want IT(iridium)", q.ITDevice)
	}
}

func TestSamePartNo(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		// Kanban ต่อท้ายด้วยน้ำหนักถ่วง ส่วนคอลัมน์ P/N ใน master_data เก็บแค่รหัส
		{"YN60C00942P1_4.3T", "YN60C00942P1", true},
		{"YN60C00942P1", "YN60C00942P1_4.3T", true},
		{"YN60C00942P1_4.3T", "YN60C00942P1_4.3T", true},
		{"YN60-C00942P1", "YN60C00942P1", true},
		{"YN60C00942P1", "YN60C00943P1", false},
		{"YN60C00942P1_4.3T", "YN60C00943P1", false},
		{"", "YN60C00942P1", false},
		{"YN60C00942P1", "", false},
	}
	for _, tc := range cases {
		if got := SamePartNo(tc.a, tc.b); got != tc.want {
			t.Errorf("SamePartNo(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestMasterDataPartNoOf(t *testing.T) {
	row := map[string]string{
		"CW Part No":             "YN60C00942P1",
		"CV Part No":             "YN30V00001F1",
		"SM Part No":             "YN15V00001F1",
		"Motor Propel Part No":   "YN15V00002F1",
		"Pump Hydrolics Part No": "YN10V00001F1",
		"Engine Part No":         "J05ETG59275",
	}
	want := map[string]string{
		ComponentCW: "YN60C00942P1",
		ComponentCV: "YN30V00001F1",
		ComponentSM: "YN15V00001F1",
		ComponentMP: "YN15V00002F1",
		ComponentPH: "YN10V00001F1",
		ComponentEN: "J05ETG59275",
	}
	for comp, exp := range want {
		if got := masterDataPartNoOf(row, comp); got != exp {
			t.Errorf("%s: P/N = %q, want %q", comp, got, exp)
		}
	}

	// IT Controller ไม่มี P/N ใน master_data (ผูกกับเครื่องในไฟล์ Planning WH)
	if got := masterDataPartNoOf(row, ComponentITC); got != "" {
		t.Errorf("ITC ไม่ควรมี P/N ใน master_data แต่ได้ %q", got)
	}
	if got := masterDataPartNoOf(nil, ComponentCW); got != "" {
		t.Errorf("แถวว่าง = %q, want \"\"", got)
	}

	// คอลัมน์ที่ถูกเติม prefix ตอนอัปโหลด และชื่อคอลัมน์แบบอื่น
	if got := masterDataPartNoOf(map[string]string{extraColumnPrefix + "CW Part No": "A1"}, ComponentCW); got != "A1" {
		t.Errorf("คอลัมน์ที่มี prefix = %q, want A1", got)
	}
	if got := masterDataPartNoOf(map[string]string{"Pump Hydraulics Part No": "E9"}, ComponentPH); got != "E9" {
		t.Errorf("ชื่อคอลัมน์สะกดอีกแบบ = %q, want E9", got)
	}
}

func TestMasterDataWeightTonsOf(t *testing.T) {
	if got := masterDataWeightTonsOf(map[string]string{"Weight (Tons)": "4.3"}); got != "4.3" {
		t.Errorf("Weight (Tons) = %q, want 4.3", got)
	}
	if got := masterDataWeightTonsOf(map[string]string{extraColumnPrefix + "Weight (Tons)": "4.3"}); got != "4.3" {
		t.Errorf("คอลัมน์ที่มี prefix = %q, want 4.3", got)
	}
	if got := masterDataWeightTonsOf(nil); got != "" {
		t.Errorf("แถวว่าง = %q, want \"\"", got)
	}
}

func TestCheckKanbanPartNos(t *testing.T) {
	q, ok := ParseSpecQR(kanbanQRSample)
	if !ok {
		t.Fatal("อ่าน QR บน Kanban ไม่ได้")
	}
	const spec = "YN15-0TD6LG111001"
	// "YN60C00942P1_4.3T" บน Kanban = P/N YN60C00942P1 + น้ำหนักถ่วง 4.3 ตัน
	masterRow := func(cw, weight string) map[string]string {
		return map[string]string{"Product Spec": spec, "CW Part No": cw, "Weight (Tons)": weight}
	}

	t.Run("P/N และน้ำหนักตรงกับ master_data", func(t *testing.T) {
		res := checkKanbanPartNos(q, spec, masterRow("YN60C00942P1", "4.3"))
		if res.State != KanbanPartCheckMatch || res.Blocked() {
			t.Fatalf("state = %s, blocked = %v (want MATCH / false)", res.State, res.Blocked())
		}
		if len(res.Items) != 2 {
			t.Fatalf("ต้องเทียบทั้ง P/N และ Weight (Tons) — items = %+v", res.Items)
		}
		for _, it := range res.Items {
			if !it.OK {
				t.Fatalf("item %s ไม่ผ่าน: %+v", it.Field, it)
			}
		}
	})

	t.Run("P/N ไม่ตรงกับ CW Part No", func(t *testing.T) {
		res := checkKanbanPartNos(q, spec, masterRow("YN60C00999P1", "4.3"))
		if res.State != KanbanPartCheckMismatch || !res.Blocked() {
			t.Fatalf("state = %s, blocked = %v (want MISMATCH / true)", res.State, res.Blocked())
		}
		if res.Detail == "" {
			t.Fatal("ต้องบอกสาเหตุเสมอ")
		}
	})

	t.Run("น้ำหนักถ่วงไม่ตรงกับ Weight (Tons)", func(t *testing.T) {
		res := checkKanbanPartNos(q, spec, masterRow("YN60C00942P1", "5.5"))
		if res.State != KanbanPartCheckMismatch || !res.Blocked() {
			t.Fatalf("state = %s, blocked = %v (want MISMATCH / true)", res.State, res.Blocked())
		}
	})

	t.Run("น้ำหนักเขียนคนละรูปแบบ แต่ค่าเท่ากัน", func(t *testing.T) {
		res := checkKanbanPartNos(q, spec, masterRow("YN60C00942P1", "4.30"))
		if res.State != KanbanPartCheckMatch {
			t.Fatalf("state = %s (want MATCH) — 4.3T ต้องเท่ากับ 4.30", res.State)
		}
	})

	t.Run("Kanban ไม่ได้เขียนน้ำหนักถ่วงต่อท้าย P/N — ต้องไม่ผ่าน", func(t *testing.T) {
		// ตัด "_4.3T" ออกจากช่อง CW ให้เหลือแต่รหัส P/N
		noWeight := strings.Replace(kanbanQRSample, "YN60C00942P1_4.3T", "YN60C00942P1", 1)
		nq, ok := ParseSpecQR(noWeight)
		if !ok {
			t.Fatal("อ่าน QR ไม่ได้")
		}
		res := checkKanbanPartNos(nq, spec, masterRow("YN60C00942P1", "4.3"))
		if res.State != KanbanPartCheckMismatch || !res.Blocked() {
			t.Fatalf("state = %s, blocked = %v (want MISMATCH / true) — "+
				"CW บน Kanban ต้องมีน้ำหนักถ่วงต่อท้ายเสมอ", res.State, res.Blocked())
		}
	})

	t.Run("master_data ไม่มีคอลัมน์น้ำหนัก — เทียบแค่ P/N", func(t *testing.T) {
		res := checkKanbanPartNos(q, spec, map[string]string{"CW Part No": "YN60C00942P1"})
		if res.State != KanbanPartCheckMatch || len(res.Items) != 1 {
			t.Fatalf("state = %s, items = %d (want MATCH / 1)", res.State, len(res.Items))
		}
	})

	t.Run("คอลัมน์ที่ถูกเติม prefix ตอนอัปโหลด", func(t *testing.T) {
		res := checkKanbanPartNos(q, spec, map[string]string{
			extraColumnPrefix + "CW Part No":    "YN60C00942P1",
			extraColumnPrefix + "Weight (Tons)": "4.30",
		})
		if res.State != KanbanPartCheckMatch || len(res.Items) != 2 {
			t.Fatalf("state = %s, items = %d (want MATCH / 2)", res.State, len(res.Items))
		}
	})

	t.Run("ไม่มี Product Spec นี้ใน master_data", func(t *testing.T) {
		res := checkKanbanPartNos(q, spec, nil)
		if res.State != KanbanPartCheckNoMaster || res.Blocked() {
			t.Fatalf("state = %s, blocked = %v (want NO_MASTER / false)", res.State, res.Blocked())
		}
	})

	t.Run("master_data ไม่มีคอลัมน์ที่เทียบได้", func(t *testing.T) {
		res := checkKanbanPartNos(q, spec, map[string]string{"Product Spec": spec})
		if res.State != KanbanPartCheckSkip || res.Blocked() {
			t.Fatalf("state = %s, blocked = %v (want SKIP / false)", res.State, res.Blocked())
		}
	})
}