package controllers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func loadUploadRows(dataset string) []map[string]string {
	var rows []models.UploadDataRow
	config.DB.Where("dataset = ?", dataset).Order("id asc").Find(&rows)
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		m := map[string]string{}
		if err := json.Unmarshal([]byte(r.DataJSON), &m); err == nil {
			out = append(out, m)
		}
	}
	return out
}

func pickField(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(m[k]); v != "" {
			return v
		}
	}
	return ""
}

func machineFromRow(m map[string]string) string {
	if v := pickField(m,
		"Machine No", "Machine", "machine no", "machine",
		"[+] Machine No", "[+] Machine", "[+] machine no", "[+] machine",
	); v != "" {
		return v
	}

	for k, v := range m {
		if strings.TrimSpace(v) == "" {
			continue
		}
		switch normalizeHeader(strings.TrimPrefix(k, extraColumnPrefix)) {
		case "machineno", "machine", "machinenumber", "mcno", "mcnumber", "machineid":
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func joinKeyVariants(raw string) []string {
	s := strings.ToUpper(strings.TrimSpace(unwrapExcelText(raw)))
	if s == "" {
		return nil
	}

	s = strings.TrimSuffix(s, ".0")

	compact := strings.NewReplacer(" ", "", "-", "", "_", "", "/", "", ".", "").Replace(s)
	if compact == "" {
		return nil
	}

	out := []string{compact}

	if d := strings.TrimLeft(digitsOnly(compact), "0"); len(d) >= 4 {
		out = append(out, "#"+d)
	}

	return dedupStrings(out)
}

func orderKeysFromRow(m map[string]string) []string {

	priority := []string{"orderno", "workorder", "order", "kcmorder"}

	byField := map[string][]string{}
	for k, v := range m {
		if strings.TrimSpace(v) == "" {
			continue
		}
		nk := normalizeHeader(k)
		for _, p := range priority {
			if nk == p {
				byField[p] = append(byField[p], v)
			}
		}
	}

	var keys []string
	for _, p := range priority {
		vals := byField[p]
		sort.Strings(vals)
		for _, v := range vals {
			keys = append(keys, joinKeyVariants(v)...)
		}
	}
	return dedupStrings(keys)
}

type machineIndexStore struct {
	mu   sync.Mutex
	db   *gorm.DB
	sig  string
	data map[string]map[string]string
}

var machineIndexCache machineIndexStore

func InvalidateMachineIndex() {
	machineIndexCache.mu.Lock()
	machineIndexCache.db = nil
	machineIndexCache.sig = ""
	machineIndexCache.data = nil
	machineIndexCache.mu.Unlock()
}

func machineIndexSignature() string {
	var udCount, udMaxID, mdCount, ilCount, elCount, mfgCount int64

	config.DB.Model(&models.UploadDataRow{}).Count(&udCount)
	if row := config.DB.Model(&models.UploadDataRow{}).
		Select("COALESCE(MAX(id), 0)").Row(); row != nil {
		_ = row.Scan(&udMaxID)
	}
	config.DB.Model(&models.LicenseItem{}).Count(&ilCount)
	config.DB.Model(&models.LicenseItem{}).
		Where("export_license_no IS NOT NULL AND export_license_no <> ''").
		Count(&elCount)
	config.DB.Model(&models.MFGAssembly{}).Count(&mfgCount)

	return fmt.Sprintf("%d|%d|%d|%d|%d|%d",
		udCount, udMaxID, mdCount, ilCount, elCount, mfgCount)
}

func machineIndex() map[string]map[string]string {
	if config.DB == nil {
		return map[string]map[string]string{}
	}

	sig := machineIndexSignature()

	machineIndexCache.mu.Lock()
	if machineIndexCache.data != nil &&
		machineIndexCache.db == config.DB &&
		machineIndexCache.sig == sig {
		cached := machineIndexCache.data
		machineIndexCache.mu.Unlock()
		return cached
	}
	machineIndexCache.mu.Unlock()

	built := buildMachineIndex()

	machineIndexCache.mu.Lock()
	machineIndexCache.db = config.DB
	machineIndexCache.sig = sig
	machineIndexCache.data = built
	machineIndexCache.mu.Unlock()

	return built
}

type machineParts struct {
	no        string
	name      string
	partsNo   string
	partsName string
	orderNo   string
	warehouse string
	quantity  string
	location  string
}

func setIfEmpty(dst *string, v string) {
	if *dst == "" {
		*dst = strings.TrimSpace(v)
	}
}

type machineRefInfo struct {
	itcNo   string
	country string
}

func buildMachineIndex() map[string]map[string]string {

	planning := loadUploadRows(models.DatasetPlanning)
	// Daily Plan (Specification sheet) ใช้เป็นข้อมูลแผนของเครื่องด้วย (Model / Spec code / ประเทศ ...)
	planning = append(planning, loadUploadRows(models.DatasetDailyPlan)...)
	wh1 := loadUploadRows(models.DatasetWH1)
	wh2 := loadUploadRows(models.DatasetWH2)
	engine := loadUploadRows(models.DatasetEngine)

	planningByMachine := map[string]map[string]string{}
	orderToMachine := map[string]string{}

	for _, p := range planning {
		mc := machineFromRow(p)
		if mc == "" {
			continue
		}
		cur, ok := planningByMachine[mc]
		if !ok {
			cur = map[string]string{}
			planningByMachine[mc] = cur
		}
		for k, v := range p {
			if strings.TrimSpace(v) == "" {
				continue
			}
			cur[k] = v
		}

		for _, raw := range []string{p["KCM Order"], p["Work order"], p["Order No"]} {
			for _, k := range joinKeyVariants(raw) {
				if _, ok := orderToMachine[k]; !ok {
					orderToMachine[k] = mc
				}
			}
		}
	}

	for _, p := range planning {
		mc := machineFromRow(p)
		if mc == "" {
			continue
		}
		for _, k := range joinKeyVariants(p["LOT NO."]) {
			if _, ok := orderToMachine[k]; !ok {
				orderToMachine[k] = mc
			}
		}
	}

	machineOf := func(row map[string]string) string {
		if mc := machineFromRow(row); mc != "" {
			return mc
		}
		for _, k := range orderKeysFromRow(row) {
			if mc, ok := orderToMachine[k]; ok {
				return mc
			}
		}
		return ""
	}

	wh1ByMachine := map[string]*machineParts{}
	for _, w := range wh1 {
		mc := machineOf(w)
		if mc == "" {
			continue
		}
		cur, ok := wh1ByMachine[mc]
		if !ok {
			cur = &machineParts{}
			wh1ByMachine[mc] = cur
		}
		setIfEmpty(&cur.no, pickField(w, "Assembly Parts Number", "Assembly Parts No", "Assembly_Parts_Number"))
		setIfEmpty(&cur.name, pickField(w, "Assembly Parts Name", "Assembly_Parts_Name"))
		setIfEmpty(&cur.partsNo, pickField(w, "Parts No", "Part No"))
		setIfEmpty(&cur.partsName, pickField(w, "Name", "Parts Name"))
		setIfEmpty(&cur.orderNo, pickField(w, "Order No", "Work order"))
		setIfEmpty(&cur.warehouse, pickField(w, "Warehouse", "Forwarding Warehouse"))
	}

	wh2ByMachine := map[string]*machineParts{}
	for _, w := range wh2 {
		mc := machineOf(w)
		if mc == "" {
			continue
		}
		cur, ok := wh2ByMachine[mc]
		if !ok {
			cur = &machineParts{}
			wh2ByMachine[mc] = cur
		}
		setIfEmpty(&cur.partsNo, pickField(w, "Parts No", "Part No"))
		setIfEmpty(&cur.partsName, pickField(w, "PARTS NAME", "Parts Name"))
		setIfEmpty(&cur.orderNo, pickField(w, "ORDER No.", "Order"))
		setIfEmpty(&cur.quantity, pickField(w, "Quantity"))
		setIfEmpty(&cur.location, pickField(w, "LOCATION"))
	}

	type engineInfo struct{ engine, history string }
	engineByMachine := map[string]engineInfo{}
	for _, e := range engine {
		mc := strings.TrimSpace(pickField(e, "Machine No", "Machine"))
		if mc == "" {
			continue
		}
		cur := engineByMachine[mc]
		setIfEmpty(&cur.engine, pickField(e, "ENGINE", "Engine"))
		setIfEmpty(&cur.history, pickField(e, "History", "Engine History"))
		engineByMachine[mc] = cur
	}

	// ไม่มีทะเบียนกลางให้เทียบแล้ว

	licCountryByITC := map[string]string{}
	var licItems []models.LicenseItem
	config.DB.Find(&licItems)
	for _, it := range licItems {
		key := strings.TrimSpace(it.MachineNo)
		if key == "" {
			continue
		}
		if _, ok := licCountryByITC[key]; !ok {
			licCountryByITC[key] = strings.TrimSpace(it.ExportCountry)
		}
	}

	// ฝั่งนำออกอ่านจากทะเบียนเครื่องชุดเดียวกับฝั่งนำเข้า จึงไม่ต้องทำดัชนีซ้ำอีกชุด
	expByMachine := map[string]machineRefInfo{}

	mfgByMachine := map[string]machineRefInfo{}
	var mfgRows []models.MFGAssembly
	config.DB.Order("id asc").Find(&mfgRows)
	for _, m := range mfgRows {
		key := strings.TrimSpace(m.MachineNo)
		if key == "" {
			continue
		}
		mfgByMachine[key] = machineRefInfo{
			itcNo:   strings.TrimSpace(m.ITControllerNo),
			country: strings.TrimSpace(m.Country),
		}
	}

	seen := map[string]bool{}
	ordered := make([]string, 0, len(planningByMachine)+len(engineByMachine))
	addMachine := func(mc string) {
		mc = strings.TrimSpace(mc)
		if mc == "" || seen[mc] {
			return
		}
		seen[mc] = true
		ordered = append(ordered, mc)
	}
	for _, e := range engine {
		addMachine(pickField(e, "Machine No", "Machine"))
	}
	for _, p := range planning {
		addMachine(machineFromRow(p))
	}

	out := make(map[string]map[string]string, len(ordered))

	for _, mc := range ordered {
		p := planningByMachine[mc]

		rec := make(map[string]string, len(p)+24)
		for k, v := range p {
			rec[k] = v
		}

		specCode := strings.TrimSpace(p["Product Spec 1"])
		specDetail := strings.TrimSpace(p["Product Spec 2"])
		itDevice := strings.TrimSpace(p["IT device"])
		country := strings.TrimSpace(p["Country Name"])

		planITC := strings.TrimSpace(pickField(p, "IT Controller No", "IT Controller"))
		planSwing := strings.TrimSpace(p["Swing Motor No"])
		planPump := strings.TrimSpace(p["Pump Assy HYD No"])
		planPropel := strings.TrimSpace(p["Motor Propel No"])
		planValve := strings.TrimSpace(p["Control Valve No"])
		planCW := strings.TrimSpace(pickField(p, "CW No", extraColumnPrefix+"CW No"))

		itcNo := planITC

		guessedITC := ""
		if looks12Digit(planITC) {
			guessedITC = planITC
		}
		deriveCountry := country

		if mfg, ok := mfgByMachine[mc]; ok {
			if guessedITC == "" {
				guessedITC = mfg.itcNo
			}
			if deriveCountry == "" {
				deriveCountry = mfg.country
			}
		}
		if exp, ok := expByMachine[mc]; ok {
			if guessedITC == "" {
				guessedITC = exp.itcNo
			}
			if deriveCountry == "" {
				deriveCountry = exp.country
			}
		}

		licCountry := licCountryByITC[itcNo]
		if licCountry == "" && guessedITC != "" {
			licCountry = licCountryByITC[guessedITC]
		}
		if licCountry != "" {
			country = licCountry
		} else if country == "" && deriveCountry != "" {
			country = deriveCountry
		}

		partsNo, partsName := "", ""
		if w := wh1ByMachine[mc]; w != nil {
			partsNo, partsName = w.no, w.name

			if w.orderNo != "" {
				rec["WH1 Order No"] = w.orderNo
			}
			if w.partsNo != "" {
				rec["WH1 Parts No"] = w.partsNo
			}
			if w.partsName != "" {
				rec["WH1 Parts Name"] = w.partsName
			}
			if w.warehouse != "" && strings.TrimSpace(rec["Warehouse"]) == "" {
				rec["Warehouse"] = w.warehouse
			}
		}
		if partsNo == "" && partsName == "" && p != nil {
			partsNo = pickField(p,
				"Assembly Parts Number", "Assembly_Parts_Number",
				extraColumnPrefix+"Assembly Parts Number", extraColumnPrefix+"Assembly_Parts_Number")
			partsName = pickField(p,
				"Assembly Parts Name", "Assembly_Parts_Name",
				extraColumnPrefix+"Assembly Parts Name", extraColumnPrefix+"Assembly_Parts_Name")
		}

		if w := wh2ByMachine[mc]; w != nil {
			if w.orderNo != "" {
				rec["WH2 Order No"] = w.orderNo
			}
			if w.partsNo != "" {
				rec["WH2 Parts No"] = w.partsNo
			}
			if w.partsName != "" {
				rec["WH2 Parts Name"] = w.partsName
			}
			if w.quantity != "" {
				rec["WH2 Quantity"] = w.quantity
			}
			if w.location != "" {
				rec["WH2 Location"] = w.location
			}
			if partsNo == "" && partsName == "" {
				partsNo, partsName = w.partsNo, w.partsName
			}
		}

		if e, ok := engineByMachine[mc]; ok {
			if e.engine != "" {
				rec["ENGINE"] = e.engine
			}
			if e.history != "" {
				rec["History"] = e.history
			}
		}

		rec["Machine No"] = mc
		if strings.TrimSpace(rec["Machine"]) == "" {
			rec["Machine"] = mc
		}
		rec["Spec Code"] = specCode
		rec["Specification Detail"] = specDetail
		rec["Country Name"] = country
		rec["IT device"] = itDevice
		rec["IT Controller No"] = itcNo
		rec["Swing Motor No"] = planSwing
		rec["Pump Assy HYD No"] = planPump
		rec["Motor Propel No"] = planPropel
		rec["Control Valve No"] = planValve
		rec["CW No"] = planCW
		rec["Assembly_Parts_Number"] = partsNo
		rec["Assembly_Parts_Name"] = partsName

		out[mc] = rec
	}

	return out
}

type MachinePlanRow struct {
	MachineNo      string `json:"machineNo"`
	ITControllerNo string `json:"itControllerNo"`

	Model       string `json:"model"`
	PartsNumber string `json:"partsNumber"`
	SpecCode    string `json:"specCode"`
	SpecDetail  string `json:"specDetail"`
	Country     string `json:"country"`
	ITDevice    string `json:"itDevice"`

	KCMOrder string `json:"kcmOrder"`
	LotNo    string `json:"lotNo"`

	SwingMotorNo   string `json:"swingMotorNo"`
	PumpAssyHydNo  string `json:"pumpAssyHydNo"`
	MotorPropelNo  string `json:"motorPropelNo"`
	ControlValveNo string `json:"controlValveNo"`
	CWNo           string `json:"cwNo"`

	Engine        string `json:"engine"`
	EngineHistory string `json:"engineHistory"`

	WH1PartsNo   string `json:"wh1PartsNo"`
	WH1PartsName string `json:"wh1PartsName"`
	WH1OrderNo   string `json:"wh1OrderNo"`
	Warehouse    string `json:"warehouse"`

	WH2PartsNo   string `json:"wh2PartsNo"`
	WH2PartsName string `json:"wh2PartsName"`
	WH2OrderNo   string `json:"wh2OrderNo"`
	WH2Quantity  string `json:"wh2Quantity"`
	WH2Location  string `json:"wh2Location"`

	ITControllerPartNo string `json:"itControllerPartNo"`
	ITControllerSN     string `json:"itControllerSN"`
	IMEI               string `json:"imei"`
}

func machinePlanRowOf(machineNo string, rec map[string]string) MachinePlanRow {
	get := func(keys ...string) string { return planValue(rec, keys...) }

	return MachinePlanRow{
		MachineNo:      machineNo,
		ITControllerNo: get("IT Controller No"),

		Model:       get("Assembly_Parts_Name", "Model", "Base machine spec."),
		PartsNumber: get("Assembly_Parts_Number"),
		SpecCode:    get("Spec Code"),
		SpecDetail:  get("Specification Detail"),
		Country:     get("Country Name"),
		ITDevice:    get("IT device"),

		KCMOrder: get("KCM Order", "Order No"),
		LotNo:    get("LOT NO."),

		SwingMotorNo:   get("Swing Motor No"),
		PumpAssyHydNo:  get("Pump Assy HYD No"),
		MotorPropelNo:  get("Motor Propel No"),
		ControlValveNo: get("Control Valve No"),
		CWNo:           get("CW No"),

		Engine:        get("ENGINE"),
		EngineHistory: get("History"),

		WH1PartsNo:   get("WH1 Parts No"),
		WH1PartsName: get("WH1 Parts Name"),
		WH1OrderNo:   get("WH1 Order No"),
		Warehouse:    get("Warehouse"),

		WH2PartsNo:   get("WH2 Parts No"),
		WH2PartsName: get("WH2 Parts Name"),
		WH2OrderNo:   get("WH2 Order No"),
		WH2Quantity:  get("WH2 Quantity"),
		WH2Location:  get("WH2 Location"),

		ITControllerPartNo: get("IT Controller Part No"),
		ITControllerSN:     get("IT Controller S/N"),
		IMEI:               get("IMEI"),
	}
}

func GetMachinePlans(c *gin.Context) {
	idx := machineIndex()

	machines := make([]string, 0, len(idx))
	for mc := range idx {
		machines = append(machines, mc)
	}
	sort.Strings(machines)

	rows := make([]MachinePlanRow, 0, len(machines))
	for _, mc := range machines {
		rows = append(rows, machinePlanRowOf(mc, idx[mc]))
	}

	c.JSON(200, gin.H{
		"rows":  rows,
		"total": len(rows),
	})
}
