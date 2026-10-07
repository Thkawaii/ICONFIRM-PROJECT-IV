package controllers

import (
	"strings"

	"iconfirm/config"
	"iconfirm/models"
)

type codeFormatIndex struct {
	oldByNew map[string]string
	newByOld map[string]string

	currentCache  map[string]string
	variantsCache map[string][]string
}

func loadCodeFormatIndex() *codeFormatIndex {
	x := &codeFormatIndex{
		oldByNew:      map[string]string{},
		newByOld:      map[string]string{},
		currentCache:  map[string]string{},
		variantsCache: map[string][]string{},
	}

	var rows []models.CodeAlias
	config.DB.Order("id asc").Find(&rows)
	for _, r := range rows {
		from := strings.TrimSpace(r.FromCode)
		to := strings.TrimSpace(r.ToOld)

		if nf := NormalizeCodeValue(from); nf != "" {
			if _, ok := x.oldByNew[nf]; !ok {
				x.oldByNew[nf] = to
			}
		}
		if nt := NormalizeCodeValue(to); nt != "" {
			x.newByOld[nt] = from
		}
	}
	return x
}

func (x *codeFormatIndex) old(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if v, ok := x.oldByNew[NormalizeCodeValue(raw)]; ok && v != "" {
		return v
	}
	return raw
}

func (x *codeFormatIndex) current(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if hit, ok := x.currentCache[raw]; ok {
		return hit
	}
	out := raw
	if v, ok := x.newByOld[NormalizeCodeValue(x.old(raw))]; ok && v != "" {
		out = v
	}
	x.currentCache[raw] = out
	return out
}

func (x *codeFormatIndex) variants(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if hit, ok := x.variantsCache[raw]; ok {
		return hit
	}
	out := dedupeCodes(x.current(raw), x.old(raw), raw)
	x.variantsCache[raw] = out
	return out
}

func (x *codeFormatIndex) scanKeys(raw string) []string {
	vs := x.variants(raw)
	out := make([]string, 0, len(vs))
	seen := map[string]bool{}
	for _, v := range vs {
		k := qaScanKey(v)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func (x *codeFormatIndex) formerCodes(raw string) []string {
	cur := x.current(raw)
	var out []string
	for _, v := range x.variants(raw) {
		if !SameCode(v, cur) {
			out = append(out, v)
		}
	}
	return out
}
