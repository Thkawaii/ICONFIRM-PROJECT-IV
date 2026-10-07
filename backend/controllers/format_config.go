package controllers

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"iconfirm/config"
	"iconfirm/models"

	"github.com/gin-gonic/gin"
)

func NormalizeCodeValue(s string) string {
	s = unwrapExcelText(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

const (
	CodeKindMachine = "machine"
	CodeKindSN      = "sn"
	CodeKindPN      = "pn"
	CodeKindCW      = "cw"
)

var codeKindAliases = map[string]string{
	"machine":       CodeKindMachine,
	"machineno":     CodeKindMachine,
	"machinenumber": CodeKindMachine,
	"mc":            CodeKindMachine,
	"mcno":          CodeKindMachine,

	"sn":             CodeKindSN,
	"serial":         CodeKindSN,
	"serialno":       CodeKindSN,
	"serialnumber":   CodeKindSN,
	"itc":            CodeKindSN,
	"itcontroller":   CodeKindSN,
	"itcontrollerno": CodeKindSN,
	"sm":             CodeKindSN,
	"swingmotor":     CodeKindSN,
	"swingmotorno":   CodeKindSN,
	"ph":             CodeKindSN,
	"pumpassy":       CodeKindSN,
	"pumpassyhyd":    CodeKindSN,
	"pumpassyhydno":  CodeKindSN,
	"mp":             CodeKindSN,
	"motorpropel":    CodeKindSN,
	"motorpropelno":  CodeKindSN,
	"cv":             CodeKindSN,
	"controlvalve":   CodeKindSN,
	"controlvalveno": CodeKindSN,

	"pn":         CodeKindPN,
	"part":       CodeKindPN,
	"partno":     CodeKindPN,
	"partnumber": CodeKindPN,

	"cw":              CodeKindCW,
	"cwno":            CodeKindCW,
	"cwpartno":        CodeKindCW,
	"counterweight":   CodeKindCW,
	"counterweightno": CodeKindCW,
}

func NormalizeCodeKind(raw string) string {
	key := strings.ToLower(NormalizeCodeValue(raw))
	if key == "" {
		return ""
	}
	return codeKindAliases[key]
}

func CodeKindLabel(kind string) string {
	switch NormalizeCodeKind(kind) {
	case CodeKindMachine:
		return "Machine No."
	case CodeKindSN:
		return "S/N"
	case CodeKindPN:
		return "P/N"
	case CodeKindCW:
		return "CW No."
	default:
		return ""
	}
}

func componentTypeOfKind(kind string) string {
	if NormalizeCodeKind(kind) == CodeKindCW {
		return "counter_weight"
	}
	return ""
}

func loadColumnAliases(scope string) map[string][]string {
	var rows []models.ColumnAlias
	config.DB.Where(`"table" = ?`, scope).Find(&rows)

	out := map[string][]string{}
	for _, r := range rows {
		src := normalizeHeader(r.Source)
		tgt := strings.TrimSpace(r.Target)
		if src == "" || tgt == "" {
			continue
		}
		out[tgt] = append(out[tgt], src)
	}
	return out
}

func loadColumnAliasReverse(scope string) map[string]string {
	var rows []models.ColumnAlias
	config.DB.Where(`"table" = ?`, scope).Find(&rows)

	out := map[string]string{}
	for _, r := range rows {
		src := normalizeHeader(r.Source)
		tgt := normalizeHeader(r.Target)
		if src == "" || tgt == "" {
			continue
		}
		out[src] = tgt
	}
	return out
}

func loadColumnAliasReverseMerged(scopes ...string) map[string]string {
	out := map[string]string{}
	for _, sc := range scopes {
		if strings.TrimSpace(sc) == "" {
			continue
		}
		for k, v := range loadColumnAliasReverse(sc) {
			out[k] = v
		}
	}
	return out
}

func aliasHeaderKey(reverse map[string]string, normHeader string) string {
	if reverse == nil {
		return normHeader
	}
	if t, ok := reverse[normHeader]; ok && t != "" {
		return t
	}
	return normHeader
}

func findDuplicateKnownColumns(headers []string, isKnown func(string) bool, headerRow []string) (map[int]bool, []string) {
	skip := map[int]bool{}
	seen := map[string]bool{}
	warned := map[string]bool{}
	var problems []string

	for col, key := range headers {
		if key == "" || !isKnown(key) {
			continue
		}
		if seen[key] {
			skip[col] = true
			if !warned[key] {
				label := key
				if col < len(headerRow) {
					if l := strings.TrimSpace(headerRow[col]); l != "" {
						label = l
					}
				}
				problems = append(problems, "คอลัมน์ซ้ำ '"+label+"' (ถือเป็นช่องเดียวกัน) — ใช้คอลัมน์แรก คอลัมน์ที่ซ้ำถูกข้าม")
				warned[key] = true
			}
			continue
		}
		seen[key] = true
	}
	return skip, problems
}

type registryIndex struct {
	machine map[string]bool
	sn      map[string]bool
	pn      map[string]bool

	cw map[string]bool
}

func buildRegistryIndex() *registryIndex {
	idx := &registryIndex{
		machine: map[string]bool{},
		sn:      map[string]bool{},
		pn:      map[string]bool{},
		cw:      map[string]bool{},
	}
	add := func(m map[string]bool, raw string) {
		if n := NormalizeCodeValue(raw); n != "" {
			m[n] = true
		}
	}

	// ไฟล์ที่อัปโหลดได้จริงตอนนี้ — Planning WH / Planning MFG / Master Data
	// (Planning / Daily Plan / WH1 / WH2 / Engine ไม่เปิดให้อัปโหลดแล้ว)
	for _, row := range loadUploadRows(models.DatasetEngineITAlloc) {
		add(idx.machine, pickField(row, "Machine S/N"))
		add(idx.sn, pickField(row, "IT Serial no."))
		add(idx.sn, pickField(row, "Engine Serial no."))
		add(idx.pn, pickField(row, "IT Part no."))
		add(idx.pn, pickField(row, "Engine Part no."))
	}
	for _, row := range loadUploadRows(models.DatasetSpecSheet) {
		add(idx.machine, pickField(row, "Machine"))
	}
	for _, row := range loadUploadRows(models.DatasetCWCVITS) {
		add(idx.cw, pickField(row, "CW Part No"))
		for _, k := range []string{"CV Part No", "SM Part No", "Motor Propel Part No", "Pump Hydrolics Part No", "Engine Part No"} {
			add(idx.pn, pickField(row, k))
		}
	}

	// ทะเบียนเก่า (Planning / Engine ที่ยังมีข้อมูลค้างในระบบ) ยังอ่านต่อเพื่อไม่ให้รหัสเดิมหาย
	for mc, plan := range loadMachinePlans() {
		add(idx.machine, mc)
		for _, spec := range componentSpecs {
			v := PlannedNoOf(plan, spec.Code)
			if v == "" {
				continue
			}
			if spec.Code == ComponentCW {
				add(idx.cw, v)
				continue
			}
			add(idx.sn, v)
		}
	}

	for _, row := range loadUploadRows(models.DatasetEngine) {
		for _, v := range []string{
			pickField(row, "ENGINE", "Engine"),
			pickField(row, "History", "Engine History"),
		} {
			add(idx.sn, v)
			add(idx.pn, v)
		}
	}

	var mfgRows []models.MFGAssembly
	config.DB.Select("machine_no").Find(&mfgRows)
	for _, r := range mfgRows {
		add(idx.machine, r.MachineNo)
	}

	return idx
}

func (idx *registryIndex) hasOld(kind, oldValue string) bool {
	norm := NormalizeCodeValue(oldValue)
	if norm == "" {
		return false
	}
	switch NormalizeCodeKind(kind) {
	case CodeKindMachine:
		if idx.machine[norm] {
			return true
		}
	case CodeKindSN:
		if idx.sn[norm] {
			return true
		}
	case CodeKindPN:
		if idx.pn[norm] {
			return true
		}
	case CodeKindCW:
		if idx.cw[norm] {
			return true
		}
	}

	return idx.machine[norm] || idx.sn[norm] || idx.pn[norm] || idx.cw[norm]
}

func oldValueExistsInRegistry(kind, oldValue string) bool {
	return buildRegistryIndex().hasOld(kind, oldValue)
}

func lookupCodeAlias(componentType, rawCode string) *models.CodeAlias {
	norm := NormalizeCodeValue(rawCode)
	if norm == "" {
		return nil
	}

	q := config.DB.Order("id asc")
	if strings.TrimSpace(componentType) != "" {
		q = q.Where("component_type = ? OR component_type = ''", componentType)
	}

	var rows []models.CodeAlias
	q.Find(&rows)
	for i := range rows {
		if NormalizeCodeValue(rows[i].FromCode) == norm {
			return &rows[i]
		}
	}
	return nil
}

func lookupCodeAliasKind(componentType, kind, rawCode string) *models.CodeAlias {
	norm := NormalizeCodeValue(rawCode)
	if norm == "" {
		return nil
	}

	q := config.DB.Order("id asc")
	if strings.TrimSpace(kind) != "" {
		q = q.Where("kind = ?", kind)
	}
	if strings.TrimSpace(componentType) != "" {
		q = q.Where("component_type = ? OR component_type = ''", componentType)
	}

	var rows []models.CodeAlias
	q.Find(&rows)
	for i := range rows {
		if NormalizeCodeValue(rows[i].FromCode) == norm {
			return &rows[i]
		}
	}
	return nil
}

func findCodeAliasByFromCode(norm string) *models.CodeAlias {
	if norm == "" {
		return nil
	}
	var rows []models.CodeAlias
	config.DB.Order("id asc").Find(&rows)
	for i := range rows {
		if NormalizeCodeValue(rows[i].FromCode) == norm {
			return &rows[i]
		}
	}
	return nil
}

func findCodeAliasByToOld(norm string) *models.CodeAlias {
	if norm == "" {
		return nil
	}
	var rows []models.CodeAlias
	config.DB.Order("id asc").Find(&rows)
	var found *models.CodeAlias
	for i := range rows {
		if NormalizeCodeValue(rows[i].ToOld) == norm {
			found = &rows[i]
		}
	}
	return found
}

func CurrentCodeOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	old := resolveByKind("", raw)
	if a := findCodeAliasByToOld(NormalizeCodeValue(old)); a != nil {
		if v := strings.TrimSpace(a.FromCode); v != "" {
			return v
		}
	}
	return raw
}

func RetiredCodeReplacement(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	norm := NormalizeCodeValue(raw)
	if findCodeAliasByFromCode(norm) != nil {
		return "", false
	}
	a := findCodeAliasByToOld(norm)
	if a == nil {
		return "", false
	}
	replacement := strings.TrimSpace(a.FromCode)
	if replacement == "" || NormalizeCodeValue(replacement) == norm {
		return "", false
	}
	return replacement, true
}

func retiredScanMessage(codes ...string) (string, bool) {
	var msgs []string
	seen := map[string]bool{}
	for _, raw := range codes {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[NormalizeCodeValue(raw)] {
			continue
		}
		seen[NormalizeCodeValue(raw)] = true
		if repl, ok := RetiredCodeReplacement(raw); ok {
			msgs = append(msgs, raw+" เปลี่ยนเป็น "+repl+" แล้ว")
		}
	}
	if len(msgs) == 0 {
		return "", false
	}
	return "รหัสนี้ถูกเปลี่ยนรูปแบบใน Change Format Part แล้ว — " +
		strings.Join(msgs, ", ") + " กรุณาสแกนรหัสรูปแบบใหม่", true
}

func CodeVariants(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return dedupeCodes(CurrentCodeOf(raw), resolveByKind("", raw), raw)
}

func resolveByKind(kind, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}

	if kind != "" {
		if a := lookupCodeAliasKind("", kind, raw); a != nil {
			if v := strings.TrimSpace(a.ToOld); v != "" {
				return v
			}
		}
	}

	if a := findCodeAliasByFromCode(NormalizeCodeValue(raw)); a != nil {
		if v := strings.TrimSpace(a.ToOld); v != "" {
			return v
		}
	}

	return raw
}

func ResolveComponentSerial(component, raw string) string {
	if strings.ToUpper(strings.TrimSpace(component)) == ComponentCW {
		return resolveByKind(CodeKindCW, raw)
	}
	return resolveByKind(CodeKindSN, raw)
}

func ResolvePartNo(raw string) string {
	return resolveByKind(CodeKindPN, raw)
}

func ResolveMachineNo(raw string) string {
	return resolveByKind(CodeKindMachine, raw)
}

func ResolveScannedCode(raw string) string {
	return resolveByKind("", raw)
}

func SameCode(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return a == b
	}
	if strings.EqualFold(a, b) {
		return true
	}
	na, nb := NormalizeCodeValue(a), NormalizeCodeValue(b)
	return na != "" && na == nb
}

func GetColumnAliases(c *gin.Context) {
	var rows []models.ColumnAlias
	q := config.DB.Order(`"table" asc`).Order("id asc")
	if s := strings.TrimSpace(c.Query("scope")); s != "" {
		q = q.Where(`"table" = ?`, s)
	}
	q.Find(&rows)
	c.JSON(200, rows)
}

func CreateColumnAlias(c *gin.Context) {
	var in models.ColumnAlias
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	in.Scope = strings.ToLower(strings.TrimSpace(in.Scope))
	in.Source = strings.TrimSpace(in.Source)
	in.Target = strings.TrimSpace(in.Target)
	in.Kind = strings.ToLower(strings.TrimSpace(in.Kind))
	if in.Kind == "" {
		in.Kind = "rename"
	}
	if in.Scope == "" || in.Source == "" || in.Target == "" {
		c.JSON(400, gin.H{"message": "ต้องระบุ table, new (หัวคอลัมน์ในไฟล์) และ old (คอลัมน์มาตรฐาน)"})
		return
	}

	userID, userName := lookupUserName(c)
	in.UserID = userID
	in.UploadDate = time.Now()

	if err := config.DB.Create(&in).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	CreateAuditLog("FORMAT_CONFIG", in.ID, "column_alias_add", in.Scope+":"+in.Source+"→"+in.Target, userID, userName)
	c.JSON(201, in)
}

func DeleteColumnAlias(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}
	if err := config.DB.Delete(&models.ColumnAlias{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	userID, userName := lookupUserName(c)
	CreateAuditLog("FORMAT_CONFIG", uint(id), "column_alias_delete", "", userID, userName)
	c.JSON(200, gin.H{"deleted": true})
}

func GetCodeAliases(c *gin.Context) {
	var rows []models.CodeAlias
	q := config.DB.Order("id asc")
	if ct := strings.TrimSpace(c.Query("component_type")); ct != "" {
		q = q.Where("component_type = ?", ct)
	}
	if k := strings.TrimSpace(c.Query("kind")); k != "" {
		q = q.Where("kind = ?", k)
	}
	q.Find(&rows)
	c.JSON(200, rows)
}

func CreateCodeAlias(c *gin.Context) {
	var in models.CodeAlias
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	in.FromCode = strings.TrimSpace(in.FromCode)
	in.ToOld = strings.TrimSpace(in.ToOld)
	in.ComponentType = strings.TrimSpace(in.ComponentType)

	rawKind := strings.TrimSpace(in.Kind)
	in.Kind = NormalizeCodeKind(rawKind)
	if rawKind != "" && in.Kind == "" {
		c.JSON(400, gin.H{"message": "ชนิดรหัส \"" + rawKind + "\" ไม่ถูกต้อง — รองรับ machine / sn / pn / cw"})
		return
	}
	if in.ComponentType == "" {
		in.ComponentType = componentTypeOfKind(in.Kind)
	}

	if in.FromCode == "" || in.ToOld == "" {
		c.JSON(400, gin.H{"message": "ต้องระบุ New (ค่าใหม่) และ Old (ค่าเดิม) ให้ครบ"})
		return
	}

	if !oldValueExistsInRegistry(in.Kind, in.ToOld) {
		msg := "ไม่พบ Old (ค่าเดิม) \"" + in.ToOld + "\" ในระบบ — ต้องมีค่าเดิมอยู่ในทะเบียนก่อนจึงจะเพิ่มได้"
		if label := CodeKindLabel(in.Kind); label != "" {
			msg += " (ชนิด " + label + ")"
		}
		c.JSON(400, gin.H{"message": msg})
		return
	}

	userID, userName := lookupUserName(c)
	in.UserID = userID
	in.UploadDate = time.Now()

	if err := config.DB.Create(&in).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	CreateAuditLog("FORMAT_CONFIG", in.ID, "code_alias_add", in.FromCode+"→"+in.ToOld, userID, userName)
	c.JSON(201, in)
}

func DeleteCodeAlias(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"message": "id ไม่ถูกต้อง"})
		return
	}
	if err := config.DB.Delete(&models.CodeAlias{}, id).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	userID, userName := lookupUserName(c)
	CreateAuditLog("FORMAT_CONFIG", uint(id), "code_alias_delete", "", userID, userName)
	c.JSON(200, gin.H{"deleted": true})
}

var codeAliasFileColumns = map[string]func(*models.CodeAlias, string){
	"fromcode":      func(a *models.CodeAlias, v string) { a.FromCode = v },
	"from":          func(a *models.CodeAlias, v string) { a.FromCode = v },
	"new":           func(a *models.CodeAlias, v string) { a.FromCode = v },
	"oldcode":       func(a *models.CodeAlias, v string) { a.FromCode = v },
	"newcode":       func(a *models.CodeAlias, v string) { a.FromCode = v },
	"scancode":      func(a *models.CodeAlias, v string) { a.FromCode = v },
	"toserialno":    func(a *models.CodeAlias, v string) { a.ToOld = v },
	"serialno":      func(a *models.CodeAlias, v string) { a.ToOld = v },
	"sn":            func(a *models.CodeAlias, v string) { a.ToOld = v },
	"old":           func(a *models.CodeAlias, v string) { a.ToOld = v },
	"componenttype": func(a *models.CodeAlias, v string) { a.ComponentType = v },
	"type":          func(a *models.CodeAlias, v string) { a.ComponentType = v },
	"kind":          func(a *models.CodeAlias, v string) { a.Kind = v },
	"note":          func(a *models.CodeAlias, v string) { a.Note = v },
}

var (
	hdrChangeFormatNew = normalizeHeader("New (ค่าใหม่)")
	hdrChangeFormatOld = normalizeHeader("Old (ค่าเดิม)")
)

func codeAliasSetterFor(header string) func(*models.CodeAlias, string) {
	switch header {
	case hdrChangeFormatNew:
		return func(a *models.CodeAlias, v string) { a.FromCode = v }
	case hdrChangeFormatOld:
		return func(a *models.CodeAlias, v string) { a.ToOld = v }
	}
	if s, ok := codeAliasFileColumns[header]; ok {
		return s
	}
	return nil
}

func findCodeAliasHeader(rows [][]string) (int, []string) {
	limit := 30
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		headers := make([]string, len(rows[i]))
		hasFrom, hasTo := false, false
		for j, cell := range rows[i] {
			key := normalizeHeader(cell)
			headers[j] = key
			switch {
			case key == hdrChangeFormatNew,
				key == "fromcode", key == "from", key == "new",
				key == "oldcode", key == "newcode", key == "scancode":
				hasFrom = true
			case key == hdrChangeFormatOld,
				key == "toserialno", key == "serialno", key == "sn", key == "old":
				hasTo = true
			}
		}
		if hasFrom && hasTo {
			return i, headers
		}
	}
	return -1, nil
}

// codeAliasSheetScore: ไฟล์หลายชีต — เลือกชีต Change Format Part ให้อัตโนมัติ
func codeAliasSheetScore(name string, rows [][]string) int {
	idx, _ := findCodeAliasHeader(rows)
	if idx < 0 {
		return -1
	}
	score := 1
	if sheetNameHas(name, "changeformat", "format", "codealias") {
		score += sheetNameMatchBonus
	}
	return score
}

func UploadCodeAliases(c *gin.Context) {
	rows, fileName, err := readBestSheetFromForm(c, codeAliasSheetScore)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	if len(rows) < 2 {
		c.JSON(400, gin.H{"message": "ไฟล์ไม่มีข้อมูล หรืออ่านไม่ได้"})
		return
	}

	headerIdx, headers := findCodeAliasHeader(rows)
	if headerIdx < 0 {
		c.JSON(400, gin.H{"message": "หาหัวตารางไม่เจอ — ไฟล์ต้องมีคอลัมน์ New (ค่าใหม่) และ Old (ค่าเดิม) อย่างน้อย"})
		return
	}

	defaultComponentType := strings.TrimSpace(c.PostForm("component_type"))

	userID, userName := lookupUserName(c)
	now := time.Now()

	registry := buildRegistryIndex()

	var imported, updated, skipped int
	var problems []string

	for i := headerIdx + 1; i < len(rows); i++ {
		a := models.CodeAlias{}
		for col, header := range headers {
			if col >= len(rows[i]) {
				break
			}
			if setter := codeAliasSetterFor(header); setter != nil {
				setter(&a, unwrapExcelText(rows[i][col]))
			}
		}

		a.FromCode = strings.TrimSpace(a.FromCode)
		a.ToOld = strings.TrimSpace(a.ToOld)
		if a.FromCode == "" || a.ToOld == "" {
			skipped++
			continue
		}
		rawKind := strings.TrimSpace(a.Kind)
		a.Kind = NormalizeCodeKind(rawKind)
		if rawKind != "" && a.Kind == "" {
			skipped++
			problems = append(problems, a.FromCode+": ชนิดรหัส \""+rawKind+"\" ไม่ถูกต้อง — ข้ามแถวนี้")
			continue
		}

		a.ComponentType = strings.TrimSpace(a.ComponentType)
		if a.ComponentType == "" {
			a.ComponentType = componentTypeOfKind(a.Kind)
		}
		if a.ComponentType == "" {
			a.ComponentType = defaultComponentType
		}

		a.UserID = userID
		a.UploadDate = now

		if !registry.hasOld(a.Kind, a.ToOld) {
			msg := a.FromCode + ": ไม่พบ Old (ค่าเดิม) \"" + a.ToOld + "\" ในระบบ"
			if label := CodeKindLabel(a.Kind); label != "" {
				msg += " (ชนิด " + label + ")"
			}
			skipped++
			problems = append(problems, msg+" — ข้ามแถวนี้")
			continue
		}

		if old := findCodeAliasByFromCode(NormalizeCodeValue(a.FromCode)); old != nil {
			if err := config.DB.Model(&models.CodeAlias{}).Where("id = ?", old.ID).
				Updates(map[string]interface{}{
					"component_type": a.ComponentType,
					"kind":           a.Kind,
					"new":            a.FromCode,
					"old":            a.ToOld,
					"note":           a.Note,
					"upload_date":    now,
					"user_id":        userID,
				}).Error; err != nil {
				problems = append(problems, a.FromCode+": อัปเดตไม่สำเร็จ ("+err.Error()+")")
				continue
			}
			updated++
			continue
		}

		if err := config.DB.Create(&a).Error; err != nil {
			problems = append(problems, a.FromCode+": เพิ่มไม่สำเร็จ ("+err.Error()+")")
			continue
		}
		imported++
	}

	CreateAuditLog("FORMAT_CONFIG", 0, "code_alias_upload", fileName, userID, userName)
	c.JSON(201, gin.H{
		"imported": imported,
		"updated":  updated,
		"skipped":  skipped,
		"problems": capProblems(problems),
		"file":     fileName,
	})
}
