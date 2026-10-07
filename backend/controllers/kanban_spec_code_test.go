package controllers

import "testing"

// masterDataForTest: ตาราง Master Data (ชีต CW_CV_ITS) จำลอง key = Product Spec แบบ normalize
func masterDataForTest() map[string]map[string]string {
	return map[string]map[string]string{
		NormalizeCodeValue("YN15-0TD6BG111001"): {
			"Product Spec":           "YN15-0TD6BG111001",
			"CW Part No":             "YN60C00942P1",
			"CV Part No":             "YN30V00001F1",
			"SM Part No":             "YN15V00001F1",
			"Motor Propel Part No":   "YN15V00002F1",
			"Pump Hydrolics Part No": "YN10V00001F1",
		},
		NormalizeCodeValue("YN15-0QD7BG131001"): {
			"Product Spec": "YN15-0QD7BG131001",
			"CW Part No":   "YN60C00943P1",
		},
	}
}

func TestCheckKanbanSpecCode(t *testing.T) {
	specs := masterDataForTest()

	cases := []struct {
		name      string
		qr, plan  string
		wantState string
		blocked   bool
		warned    bool
		wantUsed  string
	}{
		{
			name:      "ตรงกับแผนและมีใน Master Data",
			qr:        "YN15-0TD6BG111001",
			plan:      "YN15-0TD6BG111001",
			wantState: SpecCodeCheckMatch,
			blocked:   false,
			wantUsed:  "YN15-0TD6BG111001",
		},
		{
			name:      "ตรงกันแม้รูปแบบขีดต่างกัน",
			qr:        "YN15-0TD6BG111001",
			plan:      "YN150TD6BG111001",
			wantState: SpecCodeCheckMatch,
			blocked:   false,
			wantUsed:  "YN15-0TD6BG111001",
		},
		{
			name:      "Kanban คนละ spec กับแผน",
			qr:        "YN15-0TD6BG111001",
			plan:      "YN15-0QD7BG131001",
			wantState: SpecCodeCheckMismatch,
			blocked:   true,
		},
		{
			// Master Data ยังไม่มีรุ่นนี้ → เตือน แต่ไม่บล็อก
			// (IT / Engine ยังต้องจ่ายและยืนยันได้ตามปกติ)
			name:      "spec ไม่มีใน Master Data",
			qr:        "LX10-0TD3LG911001",
			plan:      "LX10-0TD3LG911001",
			wantState: SpecCodeCheckNotInMaster,
			blocked:   false,
			warned:    true,
			wantUsed:  "LX10-0TD3LG911001",
		},
		{
			name:      "แผนเว้นว่าง ใช้ค่าจาก Kanban",
			qr:        "YN15-0TD6BG111001",
			plan:      "",
			wantState: SpecCodeCheckNoPlan,
			blocked:   false,
			wantUsed:  "YN15-0TD6BG111001",
		},
		{
			name:      "QR ไม่มี spec code ใช้ค่าตามแผน",
			qr:        "",
			plan:      "YN15-0TD6BG111001",
			wantState: SpecCodeCheckNoQR,
			blocked:   false,
			wantUsed:  "YN15-0TD6BG111001",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := checkKanbanSpecCode("YN15438295", tc.qr, tc.plan, specs)
			if res.State != tc.wantState {
				t.Fatalf("state = %q, want %q", res.State, tc.wantState)
			}
			if res.Blocked() != tc.blocked {
				t.Fatalf("blocked = %v, want %v", res.Blocked(), tc.blocked)
			}
			if res.Warned() != tc.warned {
				t.Fatalf("warned = %v, want %v", res.Warned(), tc.warned)
			}
			if tc.wantUsed != "" && res.Used != tc.wantUsed {
				t.Fatalf("used = %q, want %q", res.Used, tc.wantUsed)
			}
			if res.Detail == "" {
				t.Fatalf("detail ว่าง — ต้องบอกสาเหตุให้ผู้ใช้เสมอ")
			}
		})
	}
}

func TestKanbanSpecCodeOverride(t *testing.T) {
	specs := masterDataForTest()

	// ผ่าน → ใช้ค่าจาก Kanban หา P/N
	ok := checkKanbanSpecCode("YN15438295", "YN15-0TD6BG111001", "YN15-0TD6BG111001", specs)
	if got := kanbanSpecCodeOverride(ok); got != "YN15-0TD6BG111001" {
		t.Fatalf("override = %q, want spec code จาก Kanban", got)
	}

	// QR ไม่มี spec code → ไม่ override ใช้ค่าตามแผนเหมือนเดิม
	noQR := checkKanbanSpecCode("YN15438295", "", "YN15-0TD6BG111001", specs)
	if got := kanbanSpecCodeOverride(noQR); got != "" {
		t.Fatalf("override = %q, want \"\"", got)
	}

	// Kanban คนละ spec กับแผน → ถูกบล็อก ไม่ override
	bad := checkKanbanSpecCode("YN15438295", "YN15-0TD6BG111001", "YN15-0QD7BG131001", specs)
	if got := kanbanSpecCodeOverride(bad); got != "" {
		t.Fatalf("override = %q, want \"\"", got)
	}

	// Master Data ยังไม่มีรุ่นนี้ → ไม่บล็อก และยังใช้ค่าจาก Kanban
	// (CW / CV / SM / MP / PH จะขึ้น NO_PLAN พร้อมบอก spec code ที่หาไม่เจอ)
	warn := checkKanbanSpecCode("LX10400740", "LX10-0TD3LG911001", "LX10-0TD3LG911001", specs)
	if warn.Blocked() {
		t.Fatalf("ไม่ควรบล็อกเมื่อ Master Data ยังไม่มีรุ่นนี้")
	}
	if got := kanbanSpecCodeOverride(warn); got != "LX10-0TD3LG911001" {
		t.Fatalf("override = %q, want spec code จาก Kanban", got)
	}
}

func TestBlockedForPart(t *testing.T) {
	specs := masterDataForTest()
	cases := []struct {
		name, qr, plan string
		want           bool
	}{
		{"ตรงแผนและมีใน Master Data", "YN15-0TD6BG111001", "YN15-0TD6BG111001", false},
		{"แผนว่าง แต่มีใน Master Data", "YN15-0TD6BG111001", "", false},
		{"Kanban ไม่ตรงแผน", "YN15-0TD6BG111001", "YN15-0QD7BG131001", true},
		{"ไม่มีใน Master Data", "LX10-0TD3LG911001", "LX10-0TD3LG911001", true},
		{"Kanban ไม่มี Product Spec", "", "YN15-0TD6BG111001", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := checkKanbanSpecCode("YN15438295", tc.qr, tc.plan, specs)
			if got := res.BlockedForPart(); got != tc.want {
				t.Fatalf("BlockedForPart = %v, want %v (state %s)", got, tc.want, res.State)
			}
			if tc.want {
				if msg, detail := partBlockMessage(res, "Counter Weight"); msg == "" || detail == "" {
					t.Fatalf("ต้องมีข้อความบอกสาเหตุ")
				}
			}
		})
	}
}
