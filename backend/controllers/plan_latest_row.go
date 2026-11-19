package controllers

import "strings"

// ---------------------------------------------------------------------------
// กติกาของแผนผลิต: "หนึ่งเครื่อง = หนึ่งแถว และเอาอันล่าสุด"
//
// Machine No. (MC#) เป็นตัวระบุเครื่องจริง เครื่องหนึ่งเครื่องมีแผนจ่ายได้แผนเดียว
// แต่ไฟล์แผนเขียนเครื่องเดียวกันซ้ำได้หลายแถว เช่น
//
//	1  7.154  YN15437980  Laos(X)  1  21-Jul-26  22-Jul-26
//	2  7.154  YN15437980  Laos(X)  1  21-Jul-26  22-Jul-26
//	3  7.154  YN15437980  Laos(X)  1  21-Jul-26  22-Jul-26
//
// ของเดิมอ่านแบบ "เจอครั้งแรกชนะ" (เจอแถวบนแล้วข้ามแถวที่ซ้ำทั้งหมด) ซึ่งผิด
// เพราะคนทำแผนแก้ไฟล์โดยพิมพ์แถวใหม่ต่อท้าย/ทับแถวล่าง แถวล่างสุดคือข้อมูลที่ใช่
// ถ้าเอาแถวแรก ระบบจะค้างอยู่กับวันเดิมและประเทศเดิมทั้งที่ไฟล์เปลี่ยนไปแล้ว
//
// กติกาที่ใช้ตอนนี้: ยุบเหลือแถวเดียวต่อเครื่อง โดย "แถวล่างสุดในไฟล์ชนะ"
//
//	1  7.154  YN15437980  Malaysia  1  22-Jul-26  24-Jul-26
//
// ตำแหน่งแถวยึดที่ "ครั้งแรกที่เจอเครื่องนั้น" แต่ค่าที่เก็บเอาจากแถวล่างสุด
// (ลำดับในตารางจะได้ไม่สลับไปมาทุกครั้งที่มีคนเพิ่มแถวซ้ำ ส่วนค่าเป็นของล่าสุด)
// ---------------------------------------------------------------------------

// keepLatestByKey: ยุบรายการซ้ำตามคีย์ ให้รายการที่เจอ "ล่าสุด" ชนะ
//
//	keyOf  คืนคีย์ของรายการ (คีย์ว่าง = ข้ามรายการนั้นทิ้ง)
//	merge  วิธีรวมของเก่ากับของใหม่ (nil = ของใหม่ทับทั้งแถว)
func keepLatestByKey[T any](list []T, keyOf func(T) string, merge func(prev, latest T) T) []T {
	at := make(map[string]int, len(list))
	out := make([]T, 0, len(list))
	for _, item := range list {
		key := keyOf(item)
		if key == "" {
			continue
		}
		if i, ok := at[key]; ok {
			if merge != nil {
				out[i] = merge(out[i], item)
			} else {
				out[i] = item
			}
			continue
		}
		at[key] = len(out)
		out = append(out, item)
	}
	return out
}

// mergeLatestFields: รวมข้อมูลสองแถวของเครื่องเดียวกัน — ช่องที่แถวล่าสุดกรอกมาชนะ
//
// ช่องที่แถวล่าสุดเว้นว่างจะใช้ค่าเดิมต่อ เพราะไฟล์แผนกรอกไม่ครบทุกแถวเป็นเรื่องปกติ
// (เช่น เขียนวันที่ไว้แถวหัวกลุ่มแถวเดียว) ถ้าปล่อยให้ช่องว่างลบค่าเดิม
// ข้อมูลที่ไฟล์ยังมีอยู่จะหายไปเฉย ๆ
//
// แลกมาด้วยข้อจำกัดอย่างหนึ่ง: ลบค่าทิ้งด้วยการ "ลบข้อความในเซลล์" ไม่ได้
// ต้องลบแถวนั้นออกจากไฟล์ (ซึ่งระบบจะลบแถวในระบบตามให้)
func mergeLatestFields(prev, latest map[string]string) map[string]string {
	out := make(map[string]string, len(prev)+len(latest))
	for k, v := range prev {
		out[k] = v
	}
	for k, v := range latest {
		if strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// machineKeyOfBlock: คีย์ของบล็อก/แถวสเปก = MC# แบบ normalize
func machineKeyOfBlock(b map[string]string) string {
	return NormalizeCodeValue(b["Machine"])
}

// keepLatestMachineBlocks: ยุบแถวสเปกที่เป็นเครื่องเดียวกัน เอาแถวล่างสุดของไฟล์
func keepLatestMachineBlocks(blocks []map[string]string) []map[string]string {
	return keepLatestByKey(blocks, machineKeyOfBlock, mergeLatestFields)
}
