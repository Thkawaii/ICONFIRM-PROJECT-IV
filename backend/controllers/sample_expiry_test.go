package controllers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

var sampleExpiryRefDate = time.Date(2026, 9, 4, 0, 0, 0, 0, time.Local)

func alertContext(userID uint, username, query string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/?"+query, nil)
	c.Set("user_id", userID)
	c.Set("username", username)
	return c, rec
}

func alertStatuses(t *testing.T, rec *httptest.ResponseRecorder, key string) map[string]string {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(out["items"], &rows); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	got := map[string]string{}
	for _, r := range rows {
		name, _ := r[key].(string)
		status, _ := r["Status"].(string)
		got[name] = status
	}
	return got
}

type leadInfo struct {
	status string
	urgent bool
}

func alertLead(t *testing.T, rec *httptest.ResponseRecorder) map[string]leadInfo {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(out["items"], &rows); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	got := map[string]leadInfo{}
	for _, r := range rows {
		name, _ := r["ExportLicenseNo"].(string)
		status, _ := r["LeadStatus"].(string)
		urgent, _ := r["LeadUrgent"].(bool)
		got[name] = leadInfo{status, urgent}
	}
	return got
}

