package models

import (
	"strings"
	"unicode"
)

// ชนิดการเชื่อมต่อของ IT Controller (ตรงกับ "IT device" ใน Daily Plan)
//
//	SATELLITE_IRIDIUM → IT(Satellite, iridium)
//	MOBILE_4G_HIGH_H  → IT(Mobile4G, high speed-H)
//	MOBILE_4G_NORMAL  → IT(Mobile4G, normal speed)
//	MOBILE_4G3_HIGH   → IT(Mobile4G-3, high speed)
//	MOBILE_4G_HIGH    → IT(Mobile4G, high speed)
const (
	ConnMobile4GNormal = "MOBILE_4G_NORMAL"
	ConnMobile4GHigh   = "MOBILE_4G_HIGH"
	ConnMobile4GHighH  = "MOBILE_4G_HIGH_H"
	ConnMobile4G3High  = "MOBILE_4G3_HIGH"
	ConnSatelliteIrid  = "SATELLITE_IRIDIUM"
)

func compactUpper(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ClassifyConnectivity(partName, model string) string {
	s := strings.ToUpper(partName + " " + model)
	compact := compactUpper(s)
	has := func(sub string) bool { return strings.Contains(s, sub) }
	hasC := func(sub string) bool { return strings.Contains(compact, sub) }

	switch {
	case has("IRIDIUM") || has("SATELLITE") || has("SAT"):
		return ConnSatelliteIrid
	// Mobile4G-3 (high speed)
	case has("4G-3") || has("4G 3") || hasC("MOBILE4G3") || hasC("4G3HIGH") || hasC("4G3HS"):
		return ConnMobile4G3High
	// high speed-H
	case hasC("HIGHSPEEDH") || has("HIGH-H") || has("HS-H"):
		return ConnMobile4GHighH
	case has("HIGH") || has("HS") || has("HIGHSPEED"):
		return ConnMobile4GHigh
	case has("4G") || has("MOBILE") || has("LTE") || has("NORMAL"):
		return ConnMobile4GNormal
	default:
		return ""
	}
}

func NormalizeConnectivity(raw string) string {
	up := strings.ToUpper(strings.TrimSpace(raw))
	switch up {
	case ConnMobile4GNormal, ConnMobile4GHigh, ConnMobile4GHighH, ConnMobile4G3High, ConnSatelliteIrid:
		return up
	}
	return ClassifyConnectivity(raw, "")
}
