package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type udColumn struct {
	Label   string
	Aliases []string
}

type udDataset struct {
	Columns []udColumn
	MinHits int
	Anchors []string
	// Ignore: หัวคอลัมน์ที่เลิกใช้แล้ว (normalize แล้ว) — ข้ามเงียบ ๆ ไม่เก็บเป็นคอลัมน์เพิ่ม
	Ignore map[string]bool
}

// legacyNoteHeaders: คอลัมน์ NOTE ที่เคยใช้สั่งลบข้อมูลตอนอัปโหลด (เลิกใช้แล้ว)
var legacyNoteHeaders = map[string]bool{"note": true, "notes": true}

// planningIgnoredHeaders: หัวคอลัมน์ที่ตาราง Planning ไม่ใช้แล้ว — ข้ามเงียบ ๆ ไม่เก็บ ไม่แสดง
// (หมายเลขพาร์ทรายเครื่อง: IT Controller / Swing Motor / Pump Assy HYD / Motor Propel / Control Valve)
var planningIgnoredHeaders = map[string]bool{
	"note": true, "notes": true,
	"itcontrollerno": true, "itcontroller": true, "itcontrollernumber": true, "itcno": true, "itcontrollerserial": true,
	"swingmotorno": true, "swingmotor": true, "swno": true, "swingno": true, "swingmotornumber": true,
	"pumpassyhydno": true, "pumpassyno": true, "pumpno": true, "pumpassy": true, "pumpassyhyd": true,
	"motorpropelno": true, "propelno": true, "propelmotorno": true, "propel": true,
	"controlvalveno": true, "cvno": true, "valveno": true, "controlvalve": true,
}

func col(label string, aliases ...string) udColumn {
	all := make([]string, 0, len(aliases)+1)
	all = append(all, normalizeHeader(label))
	for _, a := range aliases {
		all = append(all, normalizeHeader(a))
	}
	return udColumn{Label: label, Aliases: dedupStrings(all)}
}

func dedupStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

var udDatasets = map[string]udDataset{

	models.DatasetPlanning: {
		MinHits: 4,
		Anchors: []string{"machine", "lotno", "kcmorder", "line"},
		Columns: []udColumn{
			col("Line"),
			col("LOT NO.", "lotno", "lot"),
			col("Machine", "machineno", "machinenumber", "machineno1", "mcno", "mcnumber", "machineid"),
			col("Product Spec 1", "productspec"),
			col("Product Spec 2", "productspec2"),
			col("Domestic/Exp", "domesticexp", "domexp"),
			col("Assembly Status", "assemblystetu", "assemblystatus", "assemblystet"),
			col("Shipping Status", "shippingstetu", "shippingstatus", "shippingstet"),
			col("LINE ON", "lineon"),
			col("Assembly P", "assemblyp"),
			col("Assemb", "assemb"),
			col("Shipping P", "shippingp"),
			col("Etd.Shipment", "etdshipment", "etdshipme", "etd"),
			col("Shipping", "shippi"),
			col("Sell"),
			col("Division", "divisi"),
			col("Selling Company", "sellingcompany", "sellingcomp"),
			col("KCM Order", "kcmorder"),
			col("Delivery", "delive"),
			col("User"),
			col("Country"),
			col("Country Name", "countryname"),
			col("Brand"),
			col("Destination"),
			col("Purpose"),
			col("Upp,lower spec.", "upplowerspec", "upperlowerspec"),
			col("Front ATT piping", "frontattpiping"),
			col("Other piping", "otherpiping"),
			col("Base machine spec.", "basemachinespec"),
			col("Cab base", "cabbase"),
			col("Cab"),
			col("Boom"),
			col("Piping, boom", "pipingboom"),
			col("Arm"),
			col("Piping, arm", "pipingarm"),
			col("Shoe"),
			col("Counter weight", "counterweight"),
			col("CW No", "cwno", "cwpartno", "counterweightno", "counterweightpartno"),
			col("Lower ATT", "loweratt"),
			col("Lever"),
			col("Multi control", "multicontrol"),
			col("Air conditioner", "airconditioner"),
			col("Cold region spec.", "coldregionspec"),
			col("Auto greasing system", "autogreasingsystem"),
			col("Seat"),
			col("Travel alarm", "travelalarm"),
			col("Gauge cluster", "gaugecluster"),
			col("Radio"),
			col("Engine start key", "enginestartkey"),
			col("DigNavi", "dignavi"),
			col("Paint"),
			col("Front ATT", "frontatt"),
			col("Cab guard", "cabguard"),
			col("IT device", "itdevice"),
			col("Other option", "otheroption"),
			col("Additional ATT", "additionalatt"),
			col("Cold region spec(HYD oil)", "coldregionspechydoil", "hydoil"),
			col("Shoe option", "shoeoption"),
			col("Manufacturer options", "manufactureroptions", "manufactureroption"),
			col("Note1", "note1"),
			col("Note2", "note2"),
			col("Note3", "note3"),
		},
		Ignore: planningIgnoredHeaders,
	},

	// Daily Plan: อ่านจากชีต "Specification sheet" (เครื่องละ 5 แถว) — ดู spec_sheet.go
	models.DatasetDailyPlan: {
		MinHits: 4,
		Anchors: []string{"machine", "machineno", "lotno"},
		Columns: []udColumn{
			col("LOT NO.", "lotno", "lot"),
			col("Machine", "machineno", "machinenumber", "machineno1", "mcno", "mcnumber", "machineid"),
			col("Spec Code", "speccode", "specificationcode"),
			col("Country Name", "countryname", "customer", "customername"),
			col("Main line", "mainline", "mainlineplan"),
			col("Stand by shipping", "standbyshipping"),
			col("Brand"),
			col("Destination"),
			col("Purpose"),
			col("Upp,lower spec.", "upplowerspec", "upperlowerspec"),
			col("Base machine spec.", "basemachinespec"),
			col("Front ATT piping", "frontattpiping"),
			col("Other piping", "otherpiping"),
			col("Cab base", "cabbase"),
			col("Cab"),
			col("Cab guard", "cabguard"),
			col("Boom"),
			col("Piping, boom", "pipingboom"),
			col("Arm"),
			col("Piping, arm", "pipingarm"),
			col("Shoe"),
			col("Counter weight", "counterweight"),
			col("Lower ATT", "loweratt"),
			col("Lever"),
			col("Multi control", "multicontrol"),
			col("Air conditioner", "airconditioner"),
			col("Cold region spec.", "coldregionspec"),
			col("Auto greasing system", "autogreasingsystem"),
			col("Seat"),
			col("Paint"),
			col("Front ATT", "frontatt"),
			col("Other option", "otheroption"),
			col("Additional ATT", "additionalatt"),
			col("Radio"),
			col("Engine start key", "enginestartkey"),
			col("IT device", "itdevice"),
			col("Cold region spec(HYD oil)", "coldregionspechydoil", "hydoil"),
			col("Remark", "remarks"),
		},
		Ignore: planningIgnoredHeaders,
	},

	models.DatasetWH1: {
		MinHits: 3,
		Anchors: []string{"partsno", "orderno", "workorder", "warehouse"},
		Columns: []udColumn{
			col("Warehouse"),
			col("Forwarding Warehouse", "forwardingwarehouse"),
			col("Stock out Inst date", "stockoutinstdate"),
			col("ST/LC", "stlc"),
			col("Order No", "orderno"),
			col("Shipping finish", "shippingfinish"),
			col("Work order", "workorder"),
			col("W-Detail No.", "wdetailno"),
			col("Work order finish", "workorderfinish", "workorderfnish"),
			col("Stock out No.", "stockoutno"),
			col("Stock out finish", "stockoutfinish"),
			col("Parts No", "partsno", "partno", "partnumber", "pn"),
			col("Name"),
			col("Pick"),
			col("Inst"),
			col("Ship"),
			col("Remain"),
			col("Shortage"),
			col("Mismatch"),
			col("Pr"),
			col("Sp"),
			col("AB"),
			col("Standard cost", "standardcost"),
			col("Shelf-1", "shelf1"),
			col("Shelf-2", "shelf2"),
			col("Note"),
			col("Assembly Parts Number", "assemblypartsnumber"),
			col("Assembly Parts Name", "assemblypartsname"),
			col("DL"),
			col("Reservation No.", "reservationno"),
			col("R-Detail No.", "rdetailno"),
			col("Final Color", "finalcolor"),
		},
	},

	models.DatasetWH2: {
		MinHits: 3,
		Anchors: []string{"orderno", "partsno", "order", "partsname"},
		Columns: []udColumn{
			col("Order"),
			col("ORDER No.", "orderno"),
			col("Parts No", "partsno", "partno", "partnumber", "pn"),
			col("PARTS NAME", "partsname", "partsna"),
			col("Quantity", "quantity", "quan"),
			col("#1", "1"),
			col("#2", "2"),
			col("#3", "3"),
			col("#4", "4"),
			col("WG"),
			col("OP"),
			col("STOCK", "stoc"),
			col("Upd.Stock", "updstock", "updstoc"),
			col("LOCATION", "location"),
			col("Work a", "worka"),
			col("FINISH", "finish"),
			col("STOCK 2", "stock2"),
			col("Note"),
		},
	},

	models.DatasetEngine: {
		MinHits: 2,
		Anchors: []string{"machineno", "history", "engine"},
		Columns: []udColumn{
			col("Machine No", "machineno", "machinenumber", "mcno", "mcnumber", "machineid"),
			col("History", "history"),
			col("ENGINE", "engine"),
		},
		Ignore: legacyNoteHeaders,
	},

	// Planning WH (ชีต Engine_IT allocation) — ชีตมีหัวตาราง 2 ชั้น (กลุ่ม Engine / IT# อยู่แถวบน)
	// ตัวอ่านเฉพาะใน engine_it_alloc.go จะแปลงเป็นหัวคอลัมน์เดี่ยวตามรายการนี้ก่อนเข้าขั้นตอนปกติ
	models.DatasetEngineITAlloc: {
		MinHits: 3,
		Anchors: []string{"machinesn", "lotno", "itpartno", "itserialno"},
		Columns: []udColumn{
			col("Lot no.", "lotno", "lot"),
			col("Machine S/N", "machinesn", "machineno", "mcno", "mcnumber", "machine"),
			col("Customer name", "customername", "customer", "countryname"),
			col("Main line", "mainline", "mainlineplan"),
			col("Product Spec", "productspec", "speccode", "specificationcode", "specification"),
			col("Engine Part no.", "enginepartno"),
			col("Engine Serial no.", "engineserialno"),
			col("Engine Serial no. mistake", "engineserialnomistake"),
			col("Engine Tag no.", "enginetagno"),
			col("Engine Invoice no.", "engineinvoiceno", "engine1stinvioceno", "engine1stinvoiceno"),
			col("Engine Remark", "engineremark"),
			col("IT Part no.", "itpartno"),
			col("IT Serial no.", "itserialno"),
			col("IT Serial no. mistake", "itserialnomistake"),
			col("IT Tag no.", "ittagno"),
			col("IT Invoice no.", "itinvoiceno", "it1stinvioceno", "it1stinvoiceno"),
			col("IT Remark", "itremark"),
		},
		Ignore: legacyNoteHeaders,
	},

	// Master Data (ชีต CW_CV_ITS) — P/N ของอะไหล่หลักรายรุ่น (Product Spec) ที่ WH ใช้เทียบตอนจ่ายของ
	models.DatasetCWCVITS: {
		MinHits: 3,
		Anchors: []string{"productspec", "cwpartno"},
		Columns: []udColumn{
			col("Product Spec", "productspec", "speccode", "specificationcode", "spec"),
			col("CW Part No", "cwpartno", "cwno", "counterweightpartno", "counterweightno"),
			col("Name", "name", "cwname"),
			col("Weight (g)", "weightg", "weight"),
			col("Weight (Tons)", "weighttons", "weightton"),
			col("BOOM Part No", "boompartno", "boomno"),
			col("BOOM Part Name", "boompartname", "boomname"),
			col("ARM Part No", "armpartno", "armno"),
			col("ARM Part Name", "armpartname", "armname"),
			col("CV Part No", "cvpartno", "cvno", "controlvalvepartno", "controlvalveno"),
			col("SM Part No", "smpartno", "smno", "swingmotorpartno", "swingmotorno"),
			col("Motor Propel Part No", "motorpropelpartno", "motorpropelno", "mppartno", "mpno"),
			col("Pump Hydrolics Part No", "pumphydrolicspartno", "pumphydraulicspartno", "pumpassyhydpartno", "pumpassyhydno", "phpartno", "phno"),
			col("Engine Part No", "enginepartno", "engineno"),
		},
		Ignore: legacyNoteHeaders,
	},

	// Planning MFG (ชีต Spec sheet_pcp) — ผูก MC# เข้ากับ Product Spec
	// ใช้ตอน WH จ่าย CW / CV / SM / MP / PH เพราะ P/N ผูกกับ spec code ไม่ใช่เลขเครื่อง
	models.DatasetSpecSheet: {
		MinHits: 3,
		Anchors: []string{"machine", "productspec", "lotno"},
		Columns: []udColumn{
			col("Line"),
			col("LOT NO.", "lotno", "lot"),
			col("Machine", "machineno", "machinenumber", "mcno", "mcnumber", "machineid", "machinesn"),
			col("Product Spec", "productspec", "speccode", "specificationcode", "spec"),
			col("Domestic/Exp", "domesticexp", "domexp"),
			col("Selling comp", "sellingcomp", "sellingcompany"),
			col("KCM Order", "kcmorder"),
			col("Brand"),
			col("Destination"),
			col("Base machine spec.", "basemachinespec"),
			col("Boom"),
			col("Arm"),
			col("Shoe"),
			col("Counter weight", "counterweight"),
			col("Front ATT", "frontatt"),
			col("Cab"),
			col("Cab guard", "cabguard"),
			col("IT device", "itdevice"),
			col("Note1", "note1"),
		},
		Ignore: legacyNoteHeaders,
	},
}

var udDatasetLabels = map[string]string{
	models.DatasetPlanning:      "Planning",
	models.DatasetDailyPlan:     "Daily Plan",
	models.DatasetWH1:           "WH1",
	models.DatasetWH2:           "WH2",
	models.DatasetEngine:        "Engine",
	models.DatasetEngineITAlloc: "Planning WH",
	models.DatasetCWCVITS:       "Master Data",
	models.DatasetSpecSheet:     "Planning MFG",
}

var udDatasetKeyFields = map[string][]string{
	models.DatasetPlanning:      {"Machine"},
	models.DatasetDailyPlan:     {"Machine"},
	models.DatasetWH1:           {"Order No", "Parts No", "Work order"},
	models.DatasetWH2:           {"ORDER No.", "Parts No"},
	models.DatasetEngine:        {"Machine No"},
	models.DatasetEngineITAlloc: {"Machine S/N"},
	models.DatasetCWCVITS:       {"Product Spec"},
	models.DatasetSpecSheet:     {"Machine"},
}

var udDatasetCoreFields = map[string][]string{
	models.DatasetPlanning:      {"Product Spec 1", "Product Spec 2", "KCM Order", "Country Name"},
	models.DatasetDailyPlan:     {"Spec Code", "Country Name", "Base machine spec.", "Destination", "Shoe", "Front ATT", "IT device"},
	models.DatasetWH1:           {"Assembly Parts Number", "Name"},
	models.DatasetWH2:           {"PARTS NAME", "Quantity"},
	models.DatasetEngine:        {"ENGINE"},
	models.DatasetEngineITAlloc: {"Customer name", "Engine Part no.", "Engine Serial no.", "IT Part no.", "IT Serial no."},
	models.DatasetCWCVITS:       {"CW Part No", "CV Part No", "SM Part No", "Motor Propel Part No", "Pump Hydrolics Part No"},
	models.DatasetSpecSheet:     {"Product Spec", "LOT NO.", "Destination", "IT device"},
}

var udDatasetKeyLabel = map[string]string{
	models.DatasetPlanning:      "Machine No",
	models.DatasetDailyPlan:     "Machine No",
	models.DatasetWH1:           "Order · Parts · WO",
	models.DatasetWH2:           "Order · Parts",
	models.DatasetEngine:        "Machine No",
	models.DatasetEngineITAlloc: "Machine S/N (MC#)",
	models.DatasetCWCVITS:       "Product Spec",
	models.DatasetSpecSheet:     "Machine No (MC#)",
}

func uploadDataDiffKey(dataset string, data map[string]string) string {
	fields := udDatasetKeyFields[dataset]
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, strings.ToLower(strings.TrimSpace(data[f])))
	}
	return strings.Join(parts, "|")
}

func buildStandardRowData(ds udDataset, headerMap map[string]int, raw []string) (map[string]string, bool) {
	data := map[string]string{}
	occSeen := map[string]int{}
	anyValue := false
	for _, cdef := range ds.Columns {
		occSeen[cdef.Aliases[0]]++
		occ := occSeen[cdef.Aliases[0]]
		j, found := resolveColumn(headerMap, cdef, occ)
		val := ""
		if found && j < len(raw) {
			val = strings.TrimSpace(unwrapExcelText(raw[j]))
		}
		data[cdef.Label] = val
		if val != "" {
			anyValue = true
		}
	}
	return data, anyValue
}

func buildHeaderIndex(headerRow []string) map[string]int {
	idx := map[string]int{}
	count := map[string]int{}
	for j, cell := range headerRow {
		key := normalizeHeader(cell)
		if key == "" {
			continue
		}
		count[key]++
		if count[key] == 1 {
			idx[key] = j
		} else {
			idx[fmt.Sprintf("%s#%d", key, count[key])] = j
		}
	}
	return idx
}

func resolveColumn(headerIdx map[string]int, c udColumn, occ int) (int, bool) {
	for _, a := range c.Aliases {
		key := a
		if occ > 1 {
			key = fmt.Sprintf("%s#%d", a, occ)
		}
		if j, ok := headerIdx[key]; ok {
			return j, true
		}
	}
	return -1, false
}

func withRuntimeAliases(ds udDataset, dataset string) udDataset {
	extra := loadColumnAliases(dataset)
	if len(extra) == 0 {
		return ds
	}
	cols := make([]udColumn, len(ds.Columns))
	copy(cols, ds.Columns)
	for i := range cols {
		if adds, ok := extra[cols[i].Label]; ok {
			merged := append([]string{}, cols[i].Aliases...)
			merged = append(merged, adds...)
			cols[i].Aliases = dedupStrings(merged)
		}
	}
	ds.Columns = cols
	return ds
}

const extraColumnPrefix = "[+] "

func knownAliasSet(ds udDataset) map[string]bool {
	known := map[string]bool{}
	for k := range ds.Ignore {
		known[k] = true
	}
	for _, cdef := range ds.Columns {
		for _, a := range cdef.Aliases {
			known[a] = true
		}
	}
	return known
}

func captureExtraColumns(ds udDataset, headerRow, raw []string, data map[string]string) {
	known := knownAliasSet(ds)
	for j, cell := range headerRow {
		key := normalizeHeader(cell)
		if key == "" || known[key] {
			continue
		}
		if j >= len(raw) {
			continue
		}
		val := unwrapExcelText(raw[j])
		if val == "" {
			continue
		}
		label := extraColumnPrefix + strings.TrimSpace(cell)
		if _, exists := data[label]; !exists {
			data[label] = val
		}
	}
}

func collectExtraLabels(rows []models.UploadDataRow, standard []string, ignore ...map[string]bool) []string {
	var skip map[string]bool
	if len(ignore) > 0 {
		skip = ignore[0]
	}
	std := map[string]bool{}
	for _, l := range standard {
		std[l] = true
	}
	seen := map[string]bool{}
	var extras []string
	for _, r := range rows {
		var m map[string]string
		if json.Unmarshal([]byte(r.DataJSON), &m) != nil {
			continue
		}
		for k := range m {
			if std[k] || seen[k] {
				continue
			}
			if skip != nil && skip[normalizeHeader(strings.TrimPrefix(k, extraColumnPrefix))] {
				continue
			}
			seen[k] = true
			extras = append(extras, k)
		}
	}
	sort.Strings(extras)
	return extras
}

func PreviewUploadDataMapping(c *gin.Context) {
	dataset := strings.ToLower(strings.TrimSpace(c.Param("dataset")))
	ds, ok := udDatasets[dataset]
	if !ok {
		c.JSON(400, gin.H{"message": "dataset ไม่ถูกต้อง (engine_it_allocation | cw_cv_its | spec_sheet)"})
		return
	}
	if !models.IsUploadableDataset(dataset) {
		c.JSON(400, gin.H{"message": "ชุดข้อมูล " + udDatasetLabels[dataset] + " ไม่เปิดให้อัปโหลดแล้ว — ใช้ Planning WH / Master Data / Planning MFG แทน"})
		return
	}
	ds = withRuntimeAliases(ds, dataset)

	rows, fileName, err := readUploadDataRows(c, dataset, ds)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if len(rows) < 1 {
		c.JSON(400, gin.H{"message": "ไฟล์ไม่มีข้อมูล หรืออ่านไม่ได้"})
		return
	}

	headerIdx, headerMap := findUploadDataHeader(rows, ds)
	if headerIdx < 0 {
		c.JSON(200, gin.H{
			"file":        fileName,
			"headerFound": false,
			"message":     "หาหัวตารางไม่เจอ — ตรวจว่าไฟล์ตรงกับชนิด " + udDatasetLabels[dataset],
		})
		return
	}

	type matchInfo struct {
		Label  string `json:"label"`
		Source string `json:"source"`
	}
	var matched []matchInfo
	var missing []string
	specTable := dataset == models.DatasetDailyPlan && isSpecSheetTable(rows[headerIdx])
	specColumnSet := map[string]bool{}
	for _, l := range specSheetColumns {
		specColumnSet[l] = true
	}

	occSeen := map[string]int{}
	for _, cdef := range ds.Columns {
		occSeen[cdef.Aliases[0]]++
		occ := occSeen[cdef.Aliases[0]]
		if j, found := resolveColumn(headerMap, cdef, occ); found && j < len(rows[headerIdx]) {
			matched = append(matched, matchInfo{Label: cdef.Label, Source: strings.TrimSpace(rows[headerIdx][j])})
		} else if !specTable || specColumnSet[cdef.Label] {
			// ไฟล์ Specification sheet: ไม่นับคอลัมน์ของ Planning รูปแบบเดิมเป็น "ขาด"
			missing = append(missing, cdef.Label)
		}
	}

	known := knownAliasSet(ds)
	var extra []string
	for _, cell := range rows[headerIdx] {
		key := normalizeHeader(cell)
		if key == "" || known[key] {
			continue
		}
		extra = append(extra, strings.TrimSpace(cell))
	}

	coreSet := map[string]bool{}
	for _, f := range udDatasetCoreFields[dataset] {
		coreSet[f] = true
	}

	var existingRows []models.UploadDataRow
	config.DB.Select("id", "dataset", "data_json", "file_name", "sort_order").
		Where("dataset = ?", dataset).Order("id asc").Find(&existingRows)

	type fieldDiff struct {
		Field string `json:"field"`
		Old   string `json:"old"`
		New   string `json:"new"`
	}
	type rowResult struct {
		Key    string      `json:"key"`
		Status string      `json:"status"`
		Diffs  []fieldDiff `json:"diffs,omitempty"`
	}

	var fileData []map[string]string
	for i := headerIdx + 1; i < len(rows); i++ {
		raw := rows[i]
		if len(raw) > 0 && len([]rune(raw[0])) > 60 {
			continue
		}
		data, anyValue := buildStandardRowData(ds, headerMap, raw)
		captureExtraColumns(ds, rows[headerIdx], raw, data)
		if !anyValue {
			for _, v := range data {
				if strings.TrimSpace(v) != "" {
					anyValue = true
					break
				}
			}
		}
		if !anyValue {
			continue
		}
		fileData = append(fileData, data)
	}

	plan := planUploadDataRows(dataset, ds, existingRows, func(i int) map[string]string { return fileData[i] }, len(fileData))

	counts := map[string]int{"NEW": 0, "UPDATED": 0, "CHANGED": 0, "UNCHANGED": 0, "LOCKED": 0}
	preview := make([]rowResult, 0, 300)

	deletes := uploadDataSyncDeletes(dataset, existingRows, plan, fileName)
	deleteCount, deleteLocked := countSyncDeletes(deletes)
	for _, d := range deletes {
		if len(preview) >= 300 {
			break
		}
		status := "DELETE"
		if d.locked {
			status = "DELETE_LOCKED"
		}
		preview = append(preview, rowResult{Key: d.label, Status: status})
	}

	for i, d := range plan {
		data := fileData[i]
		keyLabel := d.keyLabel
		if keyLabel == "" {
			keyLabel = "(ไม่มีคีย์)"
		}

		var diffs []fieldDiff
		coreChanged := false
		if d.oldData != nil {
			for _, cdef := range ds.Columns {
				o := strings.TrimSpace(d.oldData[cdef.Label])
				n := strings.TrimSpace(data[cdef.Label])
				if o != n {
					diffs = append(diffs, fieldDiff{Field: cdef.Label, Old: o, New: n})
					if coreSet[cdef.Label] {
						coreChanged = true
					}
				}
			}
		}

		var status string
		switch d.action {
		case udActionInsert:
			status = "NEW"
		case udActionLocked:
			status = "LOCKED"
		case udActionUpdate:
			if coreChanged {
				status = "CHANGED"
			} else {
				status = "UPDATED"
			}
		default:
			status = "UNCHANGED"
		}
		counts[status]++
		if status != "UNCHANGED" && len(preview) < 300 {
			preview = append(preview, rowResult{Key: keyLabel, Status: status, Diffs: diffs})
		}
	}

	total := counts["NEW"] + counts["UPDATED"] + counts["CHANGED"] + counts["UNCHANGED"] + counts["LOCKED"]

	c.JSON(200, gin.H{
		"file":        fileName,
		"dataset":     dataset,
		"headerFound": true,
		"headerRow":   headerIdx + 1,
		"matched":     matched,
		"missing":     missing,
		"extra":       extra,
		"keyLabel":    udDatasetKeyLabel[dataset],
		"coreFields":  udDatasetCoreFields[dataset],
		"summary": gin.H{
			"total":        total,
			"new":          counts["NEW"],
			"updated":      counts["UPDATED"],
			"changed":      counts["CHANGED"],
			"unchanged":    counts["UNCHANGED"],
			"locked":       counts["LOCKED"],
			"deleted":      deleteCount,
			"deleteLocked": deleteLocked,
		},
		"rows": preview,
	})
}

func udStandardLabelSet(ds udDataset) map[string]bool {
	out := make(map[string]bool, len(ds.Columns))
	for _, cdef := range ds.Columns {
		out[cdef.Label] = true
	}
	return out
}

func findUploadDataHeader(rows [][]string, ds udDataset) (int, map[string]int) {
	limit := 30
	if len(rows) < limit {
		limit = len(rows)
	}

	known := map[string]bool{}
	for _, c := range ds.Columns {
		for _, a := range c.Aliases {
			known[a] = true
		}
	}

	for i := 0; i < limit; i++ {
		hits := 0
		anchor := false
		for _, cell := range rows[i] {
			key := normalizeHeader(cell)
			if known[key] {
				hits++
			}
			for _, a := range ds.Anchors {
				if key == a {
					anchor = true
				}
			}
		}
		if hits >= ds.MinHits && anchor {
			return i, buildHeaderIndex(rows[i])
		}
	}
	return -1, nil
}

// uploadDataHeaderHits นับคอลัมน์ที่รู้จักในแถวหัวตาราง (ใช้ให้คะแนนตอนเลือกชีต)
func uploadDataHeaderHits(row []string, ds udDataset) int {
	known := map[string]bool{}
	for _, c := range ds.Columns {
		for _, a := range c.Aliases {
			known[a] = true
		}
	}
	hits := 0
	for _, cell := range row {
		if known[normalizeHeader(cell)] {
			hits++
		}
	}
	return hits
}

var udDatasetSheetNames = map[string][]string{
	models.DatasetPlanning:      {"planning", "plan"},
	models.DatasetDailyPlan:     {"specification", "dailyplan"},
	models.DatasetWH1:           {"wh1"},
	models.DatasetWH2:           {"wh2"},
	models.DatasetEngine:        {"engine"},
	models.DatasetEngineITAlloc: {"engineitallocation", "itallocation", "allocation"},
	models.DatasetCWCVITS:       {"cwcvits", "cwcv"},
	models.DatasetSpecSheet:     {"specsheetpcp", "specsheet", "pcp"},
}

func uploadDataSheetScore(dataset string, ds udDataset) sheetScore {
	return func(name string, rows [][]string) int {
		idx, _ := findUploadDataHeader(rows, ds)
		if idx < 0 {
			return -1
		}
		score := uploadDataHeaderHits(rows[idx], ds)
		if sheetNameHas(name, udDatasetSheetNames[dataset]...) {
			score += sheetNameMatchBonus
		}
		return score
	}
}

// readUploadDataRows: อ่านไฟล์ที่อัปโหลด
// Daily Plan: แปลงบล็อกเครื่อง (5 แถว/เครื่อง) ในชีต "Specification sheet" ทุกชีต (20T / 30T) เป็นตารางเดียว
// ชนิดอื่น: เลือกชีตที่เหมาะที่สุดตามเดิม
func readUploadDataRows(c *gin.Context, dataset string, ds udDataset) ([][]string, string, error) {
	if dataset == models.DatasetSpecSheet {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return nil, "", errUploadNoFile
		}
		rows, ok, err := readSpecSheetPCPSheets(fileHeader)
		if err != nil {
			return nil, fileHeader.Filename, err
		}
		if !ok {
			return nil, fileHeader.Filename, errNoSpecSheetPCP
		}
		return rows, fileHeader.Filename, nil
	}
	if dataset == models.DatasetEngineITAlloc {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return nil, "", errUploadNoFile
		}
		rows, ok, err := readEngineITAllocSheets(fileHeader)
		if err != nil {
			return nil, fileHeader.Filename, err
		}
		if !ok {
			return nil, fileHeader.Filename, errNoEngineITAllocSheet
		}
		return rows, fileHeader.Filename, nil
	}
	if dataset == models.DatasetDailyPlan {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return nil, "", errUploadNoFile
		}
		rows, ok, err := readPlanningSpecSheets(fileHeader)
		if err != nil {
			return nil, fileHeader.Filename, err
		}
		if !ok {
			return nil, fileHeader.Filename, errNoSpecSheet
		}
		return rows, fileHeader.Filename, nil
	}
	return readBestSheetFromForm(c, uploadDataSheetScore(dataset, ds))
}

func GetUploadData(c *gin.Context) {

	dataset := strings.ToLower(strings.TrimSpace(c.Query("dataset")))
	if _, ok := udDatasets[dataset]; !ok {
		c.JSON(400, gin.H{"message": "dataset ไม่ถูกต้อง (planning | daily_plan | wh1 | wh2 | engine)"})
		return
	}

	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	kw := strings.TrimSpace(c.Query("keyword"))
	applyFilter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("dataset = ?", dataset)
		if kw != "" {
			like := "%" + kw + "%"
			q = q.Where(
				"machine_no ILIKE ? OR lot_no ILIKE ? OR order_no ILIKE ? OR parts_no ILIKE ?",
				like, like, like, like,
			)
		}
		return q
	}

	var total int64
	applyFilter(config.DB.Model(&models.UploadDataRow{})).Count(&total)

	var rows []models.UploadDataRow
	applyFilter(config.DB.Preload("User")).
		Order("sort_order asc, id asc").
		Limit(limit).Offset((page - 1) * limit).
		Find(&rows)

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}

	buildScanLockIndex().annotateUploadRows(rows)

	columns := udDatasetColumnLabels(dataset)
	columns = append(columns, collectExtraLabels(rows, columns, udDatasets[dataset].Ignore)...)

	c.JSON(200, gin.H{
		"dataset":    dataset,
		"columns":    columns,
		"rows":       rows,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	})
}

func udDatasetColumnLabels(dataset string) []string {
	ds := udDatasets[dataset]
	labels := make([]string, 0, len(ds.Columns))
	for _, c := range ds.Columns {
		labels = append(labels, c.Label)
	}
	return labels
}

func UploadDataFile(c *gin.Context) {

	dataset := strings.ToLower(strings.TrimSpace(c.Param("dataset")))
	ds, ok := udDatasets[dataset]
	if !ok {
		c.JSON(400, gin.H{"message": "dataset ไม่ถูกต้อง (engine_it_allocation | cw_cv_its | spec_sheet)"})
		return
	}
	if !models.IsUploadableDataset(dataset) {
		c.JSON(400, gin.H{"message": "ชุดข้อมูล " + udDatasetLabels[dataset] + " ไม่เปิดให้อัปโหลดแล้ว — ใช้ Planning WH / Master Data / Planning MFG แทน"})
		return
	}
	ds = withRuntimeAliases(ds, dataset)

	rows, fileName, err := readUploadDataRows(c, dataset, ds)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if len(rows) < 2 {
		c.JSON(400, gin.H{"message": "ไฟล์ไม่มีข้อมูล หรืออ่านไม่ได้"})
		return
	}

	headerIdx, headerMap := findUploadDataHeader(rows, ds)
	if headerIdx < 0 {
		c.JSON(400, gin.H{
			"message": "หาหัวตารางไม่เจอ — ตรวจว่าไฟล์ตรงกับชนิด " + udDatasetLabels[dataset] + " และมีหัวคอลัมน์ครบ",
		})
		return
	}

	userID, userName := lookupUserName(c)
	now := time.Now()

	type parsedUpload struct {
		row  models.UploadDataRow
		data map[string]string
		line int
	}

	var parsed []parsedUpload
	skipped := 0
	componentCells := 0

	for i := headerIdx + 1; i < len(rows); i++ {
		raw := rows[i]

		empty := true
		for _, cell := range raw {
			if strings.TrimSpace(cell) != "" {
				empty = false
				break
			}
		}
		if empty {
			skipped++
			continue
		}

		if len(raw) > 0 && len([]rune(raw[0])) > 60 {
			skipped++
			continue
		}

		data := map[string]string{}
		occSeen := map[string]int{}
		anyValue := false

		for _, cdef := range ds.Columns {
			occSeen[cdef.Aliases[0]]++
			occ := occSeen[cdef.Aliases[0]]

			j, found := resolveColumn(headerMap, cdef, occ)
			val := ""
			if found && j < len(raw) {
				val = unwrapExcelText(raw[j])
			}
			data[cdef.Label] = val
			if val != "" {
				anyValue = true
			}
		}

		captureExtraColumns(ds, rows[headerIdx], raw, data)
		if !anyValue {
			for _, v := range data {
				if v != "" {
					anyValue = true
					break
				}
			}
		}

		if !anyValue {
			skipped++
			continue
		}

		jsonBytes, _ := json.Marshal(data)

		row := models.UploadDataRow{
			Dataset:    dataset,
			DataJSON:   string(jsonBytes),
			FileName:   fileName,
			UploadDate: now,
			UserID:     userID,
		}
		fillUploadDataKeys(&row, dataset, data)

		if dataset == models.DatasetPlanning {
			componentCells += len(planComponentsFilled(data))
		}

		parsed = append(parsed, parsedUpload{row: row, data: data, line: i + 1})
	}

	if len(parsed) == 0 {
		c.JSON(400, gin.H{"message": "ไม่พบแถวข้อมูลที่นำเข้าได้ในไฟล์นี้"})
		return
	}

	var existingRows []models.UploadDataRow
	if err := config.DB.Select("id", "dataset", "data_json", "file_name", "sort_order").
		Where("dataset = ?", dataset).Order("id asc").
		Find(&existingRows).Error; err != nil {
		c.JSON(500, gin.H{"message": "อ่านข้อมูลเดิมไม่สำเร็จ: " + err.Error()})
		return
	}

	plan := planUploadDataRows(dataset, ds, existingRows, func(i int) map[string]string { return parsed[i].data }, len(parsed))
	deletes := uploadDataSyncDeletes(dataset, existingRows, plan, fileName)
	sortBase := uploadSortBase(func() *gorm.DB {
		return config.DB.Model(&models.UploadDataRow{}).Where("dataset = ?", dataset)
	}, fileName)

	var moves []rowReposition

	var (
		toInsert  []models.UploadDataRow
		toUpdate  []models.UploadDataRow
		duplicate int
		locked    int
		problems  []string
	)
	for i, d := range plan {
		p := parsed[i]
		sortOrder := sortBase + int64(i)
		switch d.action {
		case udActionInsert:
			row := p.row
			row.SortOrder = sortOrder
			toInsert = append(toInsert, row)
		case udActionUpdate:
			row := p.row
			row.ID = d.existing.ID
			row.SortOrder = sortOrder
			toUpdate = append(toUpdate, row)
		case udActionLocked:
			locked++
			problems = append(problems, fmt.Sprintf("แถว %d (%s): %s (%s) — ไม่อัปเดตแถวนี้", p.line, d.keyLabel, scanLockedMessage, d.reason))
			moves = append(moves, rowReposition{d.existing.ID, sortOrder})
		default:
			duplicate++
			if d.existing != nil {
				moves = append(moves, rowReposition{d.existing.ID, sortOrder})
			}
		}
	}
	deleteIDs := syncDeleteIDs(deletes)
	_, lockedKept := countSyncDeletes(deletes)
	for _, d := range deletes {
		if d.locked {
			problems = append(problems, fmt.Sprintf("%s: ไม่มีในไฟล์แล้ว แต่%s (%s) — ไม่ลบแถวนี้", d.label, scanLockedMessage, d.reason))
		}
	}

	if len(toInsert) == 0 && len(toUpdate) == 0 && len(deleteIDs) == 0 {
		// ลำดับอาจเปลี่ยน (ย้ายแถวใน Excel) แม้ข้อมูลเหมือนเดิม
		applyRepositions(config.DB, &models.UploadDataRow{}, moves, fileName)
		msg := "ไม่มีข้อมูลใหม่หรือข้อมูลที่เปลี่ยน — ข้อมูลในไฟล์ตรงกับที่มีอยู่แล้ว"
		if locked > 0 {
			msg = "ไม่มีข้อมูลที่อัปเดตได้ — แถวที่เปลี่ยนถูกสแกนผ่านไปแล้ว"
		}
		c.JSON(200, gin.H{
			"dataset":    dataset,
			"imported":   0,
			"updated":    0,
			"locked":     locked,
			"deleted":    0,
			"lockedKept": lockedKept,
			"skipped":    skipped,
			"duplicate":  duplicate,
			"components": componentCells,
			"problems":   capProblems(problems),
			"file":       fileName,
			"message":    msg,
		})
		return
	}

	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if len(toInsert) > 0 {
			if err := tx.CreateInBatches(&toInsert, 1000).Error; err != nil {
				return err
			}
		}
		for _, row := range toUpdate {
			if err := tx.Model(&models.UploadDataRow{}).Where("id = ?", row.ID).Updates(map[string]interface{}{
				"data_json":   row.DataJSON,
				"machine_no":  row.MachineNo,
				"lot_no":      row.LotNo,
				"order_no":    row.OrderNo,
				"parts_no":    row.PartsNo,
				"kcm_order":   row.KCMOrder,
				"work_order":  row.WorkOrder,
				"file_name":   row.FileName,
				"upload_date": row.UploadDate,
				"user_id":     row.UserID,
				"sort_order":  row.SortOrder,
			}).Error; err != nil {
				return err
			}
		}
		if err := applyRepositions(tx, &models.UploadDataRow{}, moves, fileName); err != nil {
			return err
		}
		for _, part := range chunkSlice(deleteIDs, dbInsertBatch) {
			if err := tx.Where("id IN ?", part).Delete(&models.UploadDataRow{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"message": "บันทึกข้อมูลไม่สำเร็จ: " + err.Error()})
		return
	}

	InvalidateMachineIndex()
	CreateAuditLog("UPLOAD_DATA", 0, "upload_"+dataset, fileName, userID, userName)
	if len(deleteIDs) > 0 {
		CreateAuditLog("UPLOAD_DATA", 0, "sync_delete_"+dataset, fmt.Sprintf("%s: %d", fileName, len(deleteIDs)), userID, userName)
	}

	c.JSON(201, gin.H{
		"dataset":    dataset,
		"imported":   len(toInsert),
		"updated":    len(toUpdate),
		"deleted":    len(deleteIDs),
		"lockedKept": lockedKept,
		"locked":     locked,
		"skipped":    skipped,
		"duplicate":  duplicate,
		"components": componentCells,
		"problems":   capProblems(problems),
		"file":       fileName,
	})
}

// ---------------------------------------------------------------------------
// อัปโหลดไฟล์ Planning / WH1 / WH2 / Engine ซ้ำ (ไฟล์เดิมที่เพิ่ม/แก้ข้อมูล)
//
//   - แถวใหม่                                  → เพิ่ม
//   - แถวเดิม (คีย์ตรง) เหมือนเดิมทุกช่อง          → ข้าม (ซ้ำ)
//   - แถวเดิม (คีย์ตรง) แก้ข้อมูลใน Excel         → อัปเดตแถวเดิม (ไม่เพิ่มแถวซ้ำ)
//   - แถวเดิมที่สแกนผ่านแล้ว แต่ Excel เปลี่ยนค่า   → ไม่อัปเดต + แจ้งในรายการปัญหา
//
// คีย์ของแต่ละชุดข้อมูลดู udDatasetKeyFields (Planning = Machine, Engine = Machine No,
// WH1 = Order No + Parts No + Work order, WH2 = ORDER No. + Parts No)
// ถ้าคีย์ว่าง หรือคีย์เดียวกันมีหลายแถว (ในไฟล์หรือในระบบ) จะใช้วิธีเดิม: เทียบทั้งแถว
// ---------------------------------------------------------------------------

const (
	udActionDuplicate = iota
	udActionInsert
	udActionUpdate
	udActionLocked
)

type udDecision struct {
	action      int
	existingIdx int
	existing    *models.UploadDataRow
	oldData     map[string]string
	keyLabel    string
	reason      string
}

func planUploadDataRows(dataset string, ds udDataset, existingRows []models.UploadDataRow, dataAt func(int) map[string]string, n int) []udDecision {
	existingData := make([]map[string]string, len(existingRows))
	sigIdx := map[string][]int{}
	byKey := map[string][]int{}
	for i, r := range existingRows {
		m := map[string]string{}
		_ = json.Unmarshal([]byte(r.DataJSON), &m)
		existingData[i] = m
		sig := uploadDataSignature(m, nil, ds.Ignore)
		sigIdx[sig] = append(sigIdx[sig], i)
		if k := uploadDataDiffKey(dataset, m); strings.Trim(k, "|") != "" {
			byKey[k] = append(byKey[k], i)
		}
	}

	fileKeyCount := map[string]int{}
	for i := 0; i < n; i++ {
		if k := uploadDataDiffKey(dataset, dataAt(i)); strings.Trim(k, "|") != "" {
			fileKeyCount[k]++
		}
	}

	var locks *scanLockIndex
	claimed := map[int]bool{}
	out := make([]udDecision, n)
	seenSig := map[string]bool{}
	for i := 0; i < n; i++ {
		data := dataAt(i)
		sig := uploadDataSignature(data, nil, ds.Ignore)
		key := uploadDataDiffKey(dataset, data)
		label := strings.TrimSpace(strings.ReplaceAll(strings.Trim(key, "|"), "|", " / "))
		d := udDecision{keyLabel: label, existingIdx: -1}

		// เหมือนแถวเดิมทุกช่อง → จับคู่กับแถวเดิมที่ยังไม่ถูกจับ
		if idxs, ok := sigIdx[sig]; ok || seenSig[sig] {
			d.action = udActionDuplicate
			for _, j := range idxs {
				if !claimed[j] {
					claimed[j] = true
					old := existingRows[j]
					d.existing = &old
					d.existingIdx = j
					break
				}
			}
			seenSig[sig] = true
			out[i] = d
			continue
		}
		seenSig[sig] = true

		idx := byKey[key]
		if strings.Trim(key, "|") == "" || len(idx) != 1 || fileKeyCount[key] != 1 || claimed[idx[0]] {
			d.action = udActionInsert
			out[i] = d
			continue
		}

		claimed[idx[0]] = true
		old := existingRows[idx[0]]
		d.existing = &old
		d.existingIdx = idx[0]
		d.oldData = existingData[idx[0]]
		if locks == nil {
			locks = buildScanLockIndex()
		}
		if isLocked, reason := locks.uploadRow(dataset, d.oldData); isLocked {
			d.action = udActionLocked
			d.reason = reason
		} else {
			d.action = udActionUpdate
		}
		out[i] = d
	}
	return out
}

// uploadDataSyncDeletes: แถวของไฟล์ชื่อเดียวกันที่ไม่อยู่ในไฟล์ที่อัปรอบนี้แล้ว (ถูกลบใน Excel)
func uploadDataSyncDeletes(dataset string, existingRows []models.UploadDataRow, plan []udDecision, fileName string) []syncDelete {
	present := map[uint]bool{}
	for _, d := range plan {
		if d.existing != nil {
			present[d.existing.ID] = true
		}
	}
	var out []syncDelete
	var locks *scanLockIndex
	for _, r := range existingRows {
		if present[r.ID] || !sameUploadFile(r.FileName, fileName) {
			continue
		}
		m := map[string]string{}
		_ = json.Unmarshal([]byte(r.DataJSON), &m)
		label := strings.TrimSpace(strings.ReplaceAll(strings.Trim(uploadDataDiffKey(dataset, m), "|"), "|", " / "))
		if label == "" {
			label = fmt.Sprintf("#%d", r.ID)
		}
		if locks == nil {
			locks = buildScanLockIndex()
		}
		locked, reason := locks.uploadRow(dataset, m)
		out = append(out, syncDelete{id: r.ID, label: label, locked: locked, reason: reason})
	}
	return out
}

func fillUploadDataKeys(row *models.UploadDataRow, dataset string, data map[string]string) {
	switch dataset {
	case models.DatasetPlanning:
		row.MachineNo = normalizeDigitCell(data["Machine"])
		row.LotNo = data["LOT NO."]
		row.KCMOrder = data["KCM Order"]
	case models.DatasetDailyPlan:
		row.MachineNo = normalizeDigitCell(data["Machine"])
		row.LotNo = data["LOT NO."]
	case models.DatasetWH1:
		row.OrderNo = data["Order No"]
		row.PartsNo = data["Parts No"]
		row.WorkOrder = data["Work order"]
	case models.DatasetWH2:
		row.OrderNo = data["ORDER No."]
		row.PartsNo = data["Parts No"]
	case models.DatasetEngine:
		row.MachineNo = normalizeDigitCell(data["Machine No"])
	case models.DatasetEngineITAlloc:
		row.MachineNo = normalizeDigitCell(data["Machine S/N"])
		row.LotNo = data["Lot no."]
	case models.DatasetCWCVITS:
		row.PartsNo = strings.TrimSpace(data["Product Spec"])
	case models.DatasetSpecSheet:
		row.MachineNo = normalizeDigitCell(data["Machine"])
		row.LotNo = data["LOT NO."]
		row.PartsNo = strings.TrimSpace(data["Product Spec"])
	}
}

func DeleteUploadDataRow(c *gin.Context) {

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var row models.UploadDataRow
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	if locked, reason := uploadRowLocked(&row); locked {
		respondLocked(c, reason)
		return
	}

	if err := config.DB.Delete(&models.UploadDataRow{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	userID, userName := lookupUserName(c)
	InvalidateMachineIndex()
	CreateAuditLog("UPLOAD_DATA", row.ID, "delete_"+row.Dataset, row.MachineNo, userID, userName)

	c.JSON(200, gin.H{"deleted": true})
}

func uploadRowLocked(row *models.UploadDataRow) (bool, string) {
	data := map[string]string{}
	_ = json.Unmarshal([]byte(row.DataJSON), &data)
	return buildScanLockIndex().uploadRow(row.Dataset, data)
}

// UpdateUploadDataRow แก้ไขแถวจากตารางหน้าอัปโหลด
//   - ส่งมาเฉพาะช่องที่แก้ก็ได้ ({"data": {"Machine": "..."}}) ระบบจะรวมกับค่าเดิม
//   - ส่ง "replace": true เพื่อแทนทั้งแถว
//   - แถวที่สแกนผ่านแล้วจะได้ 409 (locked)
func UpdateUploadDataRow(c *gin.Context) {

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}

	var body struct {
		Data    map[string]string `json:"data"`
		Replace bool              `json:"replace"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if body.Data == nil {
		c.JSON(400, gin.H{"message": "ต้องส่ง data (คอลัมน์ → ค่า) มาด้วย"})
		return
	}

	var row models.UploadDataRow
	if err := config.DB.First(&row, id).Error; err != nil {
		c.JSON(404, gin.H{"message": "ไม่พบรายการนี้"})
		return
	}

	if locked, reason := uploadRowLocked(&row); locked {
		respondLocked(c, reason)
		return
	}

	merged := map[string]string{}
	if !body.Replace {
		_ = json.Unmarshal([]byte(row.DataJSON), &merged)
	}
	for k, v := range body.Data {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		merged[k] = strings.TrimSpace(v)
	}

	jsonBytes, _ := json.Marshal(merged)
	row.DataJSON = string(jsonBytes)

	row.MachineNo = ""
	row.LotNo = ""
	row.OrderNo = ""
	row.PartsNo = ""
	row.KCMOrder = ""
	row.WorkOrder = ""
	fillUploadDataKeys(&row, row.Dataset, merged)

	userID, userName := lookupUserName(c)
	if err := config.DB.Model(&models.UploadDataRow{}).Where("id = ?", row.ID).Updates(map[string]interface{}{
		"data_json":  row.DataJSON,
		"machine_no": row.MachineNo,
		"lot_no":     row.LotNo,
		"order_no":   row.OrderNo,
		"parts_no":   row.PartsNo,
		"kcm_order":  row.KCMOrder,
		"work_order": row.WorkOrder,
	}).Error; err != nil {
		log.Printf("update upload_data_rows id=%d: %v", row.ID, err)
		c.JSON(500, gin.H{"message": "อัปเดตไม่สำเร็จ"})
		return
	}

	InvalidateMachineIndex()
	CreateAuditLog("UPLOAD_DATA", row.ID, "edit_"+row.Dataset, row.MachineNo, userID, userName)

	row.Locked, row.LockReason = uploadRowLocked(&row)
	c.JSON(200, gin.H{"updated": true, "row": row})
}

func ClearUploadData(c *gin.Context) {

	dataset := strings.ToLower(strings.TrimSpace(c.Query("dataset")))
	if _, ok := udDatasets[dataset]; !ok {
		c.JSON(400, gin.H{"message": "ต้องระบุ dataset ที่ต้องการลบ (planning | wh1 | wh2 | engine)"})
		return
	}

	res := config.DB.Where("dataset = ?", dataset).Delete(&models.UploadDataRow{})
	if res.Error != nil {
		c.JSON(500, gin.H{"message": res.Error.Error()})
		return
	}

	userID, userName := lookupUserName(c)
	ResetIdentityIfEmpty(config.DB, &models.UploadDataRow{})
	InvalidateMachineIndex()
	CreateAuditLog("UPLOAD_DATA", 0, "clear_"+dataset, dataset, userID, userName)

	c.JSON(200, gin.H{"deleted": res.RowsAffected})
}

func ExportUploadData(c *gin.Context) {

	dataset := strings.ToLower(strings.TrimSpace(c.Query("dataset")))
	if _, ok := udDatasets[dataset]; !ok {
		c.JSON(400, gin.H{"message": "dataset ไม่ถูกต้อง (planning | daily_plan | wh1 | wh2 | engine)"})
		return
	}

	var rows []models.UploadDataRow
	config.DB.Where("dataset = ?", dataset).
		Order("sort_order asc, id asc").
		Find(&rows)

	labels := udDatasetColumnLabels(dataset)
	labels = append(labels, collectExtraLabels(rows, labels, udDatasets[dataset].Ignore)...)

	xl := excelize.NewFile()
	sheet := udDatasetLabels[dataset]
	if sheet == "" {
		sheet = "Data"
	}
	xl.SetSheetName("Sheet1", sheet)

	textStyle, _ := xl.NewStyle(&excelize.Style{NumFmt: 49})

	for col, h := range labels {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		xl.SetCellValue(sheet, cell, h)
	}

	for r, row := range rows {
		var data map[string]string
		_ = json.Unmarshal([]byte(row.DataJSON), &data)

		for col, label := range labels {
			cell, _ := excelize.CoordinatesToCellName(col+1, r+2)
			xl.SetCellStr(sheet, cell, data[label])
			_ = xl.SetCellStyle(sheet, cell, cell, textStyle)
		}
	}

	filename := fmt.Sprintf("%s-export-%s.xlsx", dataset, time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	if err := xl.Write(c.Writer); err != nil {
		c.JSON(500, gin.H{"message": "สร้างไฟล์ export ไม่สำเร็จ"})
	}
}
