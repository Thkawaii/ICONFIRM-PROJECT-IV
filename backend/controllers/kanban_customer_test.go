package controllers

import "testing"

func TestSameCustomerName(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		// ตรงกัน
		{"Singapore", "Singapore", true},
		{"Singapore", "SINGAPORE", true},
		{"Singapore", " singapore ", true},
		{"Singapore", "KOBELCO (SINGAPORE) PTE LTD", true},
		{"Saudi Arabia", "SAUDI ARABIA", true},
		{"Hong Kong", "HONGKONG", true},
		{"U.A.E.", "UAE", true},
		{"Thailand", "Thailand (KCMT)", true},

		// ไม่ตรงกัน
		{"Singapore", "Malaysia", false},
		{"Indonesia", "India", false},
		{"Australia", "Austria", false},
		{"Japan", "Korea", false},

		// ไม่มีข้อมูล
		{"", "Singapore", false},
		{"Singapore", "", false},
	}
	for _, tc := range cases {
		if got := SameCustomerName(tc.a, tc.b); got != tc.want {
			t.Errorf("SameCustomerName(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCheckKanbanCustomer(t *testing.T) {
	cases := []struct {
		name      string
		qr, plan  string
		wantState string
		blocked   bool
	}{
		{"ตรงกัน", "Singapore", "Singapore", CustomerCheckMatch, false},
		{"ตรงกันแบบชื่อเต็ม", "Singapore", "KOBELCO (SINGAPORE) PTE LTD", CustomerCheckMatch, false},
		{"คนละประเทศ", "Singapore", "Malaysia", CustomerCheckMismatch, true},
		{"QR ไม่มีลูกค้า", "", "Singapore", CustomerCheckNoQR, false},
		{"Planning WH เว้นว่าง", "Singapore", "", CustomerCheckNoPlan, false},
		{"Planning WH เป็นขีด", "Singapore", "-", CustomerCheckNoPlan, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := checkKanbanCustomer("YN15438324", tc.qr, tc.plan)
			if res.State != tc.wantState {
				t.Fatalf("state = %q, want %q", res.State, tc.wantState)
			}
			if res.Blocked() != tc.blocked {
				t.Fatalf("blocked = %v, want %v", res.Blocked(), tc.blocked)
			}
			if res.Detail == "" {
				t.Fatalf("detail ต้องไม่ว่าง")
			}
		})
	}
}

// ลูกค้า / ประเทศต้องอ่านจากคอลัมน์ Customer name ของไฟล์ Planning WH
func TestCheckKanbanCustomerRow(t *testing.T) {
	allocRow := map[string]string{
		"Machine S/N":   "YN15438324",
		"Customer name": "Malaysia",
		"Lot no.":       "L001",
	}

	bad := checkKanbanCustomerRow("YN15438324", "Singapore", allocRow)
	if !bad.Blocked() {
		t.Fatalf("Kanban Singapore กับแผน Malaysia ต้องไม่ผ่าน (state=%s)", bad.State)
	}

	ok := checkKanbanCustomerRow("YN15438324", "Malaysia", allocRow)
	if ok.Blocked() {
		t.Fatalf("Kanban Malaysia กับแผน Malaysia ต้องผ่าน (state=%s)", ok.State)
	}

	// ไม่มีแถวในไฟล์ Planning WH → ข้ามการตรวจ ไม่บล็อก
	none := checkKanbanCustomerRow("YN15438324", "Singapore", nil)
	if none.Blocked() {
		t.Fatalf("ไม่มีแถว Planning WH ต้องไม่บล็อก (state=%s)", none.State)
	}
}

// QR บน Kanban ช่องที่ 3 ต้องอ่านเป็นลูกค้า / ประเทศ
func TestParseSpecQRCustomerField(t *testing.T) {
	raw := "YN15438324,YN15-0QD7BG131001,Singapore,YN02B10321F1,YN12B10983F1," +
		"600mm HD grouser shoe,YN60C00942P1_4.3T,0.93m3 w/REINF bucket(SEA T)," +
		"Southeast Asia A(less EGR),IT(Mobile4G  normal speed),,,,sk200-10, sea-a"

	q, ok := ParseSpecQR(raw)
	if !ok {
		t.Fatal("อ่าน QR ไม่สำเร็จ")
	}
	if q.MachineNo != "YN15438324" {
		t.Fatalf("machineNo = %q", q.MachineNo)
	}
	if q.Customer != "Singapore" {
		t.Fatalf("customer = %q, want Singapore", q.Customer)
	}
}
