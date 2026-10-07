package models

import "testing"

func TestClassifyConnectivity(t *testing.T) {
	cases := []struct {
		name     string
		partName string
		model    string
		want     string
	}{
		{"iridium satellite", "Q4000 IRIDIUM IT CONTROLLER", "JRN-260K", ConnSatelliteIrid},
		{"satellite keyword", "Satellite Terminal", "", ConnSatelliteIrid},
		{"sat abbrev", "SAT UNIT", "", ConnSatelliteIrid},
		{"4g high speed", "IT Controller 4G HIGH", "", ConnMobile4GHigh},
		{"hs abbrev counts as high", "MODULE HS", "", ConnMobile4GHigh},
		{"4g normal", "Mobile 4G Normal", "", ConnMobile4GNormal},
		{"lte maps to normal", "LTE MODULE", "", ConnMobile4GNormal},
		{"plain mobile", "MOBILE UNIT", "", ConnMobile4GNormal},
		{"unknown returns empty", "Random Widget", "ABC-1", ""},
		{"empty input", "", "", ""},
		{"iridium wins over 4g", "4G IRIDIUM", "", ConnSatelliteIrid},
		{"high wins over normal", "4G HIGH NORMAL", "", ConnMobile4GHigh},
		{"daily plan: satellite", "IT(Satellite, iridium)", "", ConnSatelliteIrid},
		{"daily plan: high speed-H", "IT(Mobile4G, high speed-H)", "", ConnMobile4GHighH},
		{"daily plan: normal speed", "IT(Mobile4G, normal speed)", "", ConnMobile4GNormal},
		{"daily plan: 4G-3 high speed", "IT(Mobile4G-3, high speed)", "", ConnMobile4G3High},
		{"daily plan: high speed", "IT(Mobile4G, high speed)", "", ConnMobile4GHigh},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyConnectivity(tc.partName, tc.model); got != tc.want {
				t.Errorf("ClassifyConnectivity(%q,%q) = %q, want %q", tc.partName, tc.model, got, tc.want)
			}
		})
	}
}

func TestNormalizeConnectivity(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"Satellite", ConnSatelliteIrid},
		{"iridium", ConnSatelliteIrid},
		{"4G High", ConnMobile4GHigh},
		{"4g normal", ConnMobile4GNormal},
		{"  mobile  ", ConnMobile4GNormal},
		{ConnMobile4GNormal, ConnMobile4GNormal},
		{ConnMobile4GHigh, ConnMobile4GHigh},
		{ConnSatelliteIrid, ConnSatelliteIrid},
		{ConnMobile4GHighH, ConnMobile4GHighH},
		{ConnMobile4G3High, ConnMobile4G3High},
		{"IT(Mobile4G, high speed-H)", ConnMobile4GHighH},
		{"IT(Mobile4G-3, high speed)", ConnMobile4G3High},
		{"garbage", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeConnectivity(tc.raw); got != tc.want {
			t.Errorf("NormalizeConnectivity(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestStatusConstantsStable(t *testing.T) {
	pairs := map[string]string{
		MFGStatusMatched:    "MATCHED",
		MFGStatusNotMatched: "NOT_MATCHED",
		MFGStatusDuplicate:  "DUPLICATE",
	}
	for got, want := range pairs {
		if got != want {
			t.Errorf("MFG status constant = %q, want %q", got, want)
		}
	}
}
