package mailer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendFlowPayload(t *testing.T) {
	var got flowPayload
	var secret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret = r.Header.Get("x-iconfirm-secret")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	cfg := Config{Provider: ProviderFlow, FlowURL: srv.URL, FlowSecret: "s3cret", FlowInline: "strip", To: []string{"boss@kobelco.com"}}
	msg := Message{
		To:      []string{"boss@kobelco.com"},
		CC:      []string{"a@kobelco.com", "b@kobelco.com"},
		Subject: "แจ้งเตือนรายสัปดาห์",
		HTML:    `<div><img src="cid:iconfirm-logo" width="190"><p>สวัสดีครับ</p></div>`,
		Attachments: []Attachment{
			{FileName: "logo.png", Data: []byte{1, 2, 3}, ContentType: "image/png", Inline: true, ContentID: "iconfirm-logo"},
			{FileName: "report.xlsx", Data: []byte("hello"), ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		},
	}
	if err := Send(context.Background(), cfg, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if secret != "s3cret" {
		t.Fatalf("secret header = %q", secret)
	}
	if got.To != "boss@kobelco.com" || got.CC != "a@kobelco.com;b@kobelco.com" {
		t.Fatalf("recipients: to=%q cc=%q", got.To, got.CC)
	}
	if strings.Contains(got.HTML, "cid:") {
		t.Fatalf("cid image not stripped: %s", got.HTML)
	}
	if !strings.Contains(got.HTML, "สวัสดีครับ") {
		t.Fatalf("body lost: %s", got.HTML)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].Name != "report.xlsx" || got.Attachments[0].ContentBytes != "aGVsbG8=" {
		t.Fatalf("attachments = %+v", got.Attachments)
	}

	// โหมด datauri ต้องฝังรูปแทนการตัดทิ้ง
	cfg.FlowInline = "datauri"
	if err := Send(context.Background(), cfg, msg); err != nil {
		t.Fatalf("Send datauri: %v", err)
	}
	if !strings.Contains(got.HTML, "data:image/png;base64,AQID") {
		t.Fatalf("datauri missing: %s", got.HTML)
	}
}

func TestFlowValidate(t *testing.T) {
	c := Config{Provider: ProviderFlow, To: []string{"x@y.com"}}
	if err := c.Validate(); err == nil {
		t.Fatal("ต้อง error เมื่อไม่ตั้ง MAIL_FLOW_URL")
	}
	c.FlowURL = "http://insecure"
	if err := c.Validate(); err == nil {
		t.Fatal("ต้อง error เมื่อไม่ใช่ https")
	}
	c.FlowURL = "https://prod-00.southeastasia.logic.azure.com/workflows/xxx"
	if err := c.Validate(); err != nil {
		t.Fatalf("ควรผ่าน: %v", err)
	}
}
