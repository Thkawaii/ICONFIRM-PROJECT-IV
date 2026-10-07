package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// ส่งอีเมลผ่าน Power Automate (Flow) — ไม่ใช้ SMTP relay
//
// backend ยิง HTTP POST (JSON) ไปที่ URL ของ trigger "When an HTTP request is received"
// แล้ว Flow เป็นคนสั่ง Office 365 Outlook ส่งอีเมลให้ ทำให้เมลออกจากกล่องจดหมายจริง
// โดยเครื่องเซิร์ฟเวอร์ไม่ต้องมีสิทธิ์ relay และไม่ต้องเก็บรหัสผ่านอีเมลไว้เลย

type flowAttachment struct {
	// ชื่อคีย์ต้องเป็น Name / ContentBytes ตัวใหญ่ตามนี้
	// เพราะ action "Send an email (V2)" ของ Power Automate รับ array รูปแบบนี้ตรง ๆ
	Name         string `json:"Name"`
	ContentBytes string `json:"ContentBytes"`
	ContentType  string `json:"ContentType,omitempty"`
}

type flowPayload struct {
	To          string           `json:"to"`
	CC          string           `json:"cc"`
	BCC         string           `json:"bcc"`
	Subject     string           `json:"subject"`
	HTML        string           `json:"html"`
	Attachments []flowAttachment `json:"attachments"`
}

// รูปที่ฝังในเนื้อจดหมายแบบ cid: (เช่นโลโก้) — Outlook ฝั่ง Power Automate อ้าง cid ไม่ได้
var cidImageRe = regexp.MustCompile(`(?is)<img[^>]+src=["']cid:[^"']*["'][^>]*>`)

// Power Automate + Outlook รับไฟล์แนบรวมได้ราว 25–30 MB (นับหลังแปลง base64)
const flowSizeWarnBytes = 20 << 20

func flowBody(cfg Config, msg Message) string {
	html := msg.HTML
	inline := msg.InlineAttachments()
	if len(inline) == 0 {
		return html
	}

	if cfg.FlowInline == "datauri" {
		// ฝังรูปเป็น data URI — เปิดดูสวยใน Outlook Web และมือถือ
		// แต่ Outlook เดสก์ท็อป (เอนจิน Word) ไม่รองรับ จะขึ้นกรอบรูปแตก
		for _, att := range inline {
			ct := strings.TrimSpace(att.ContentType)
			if ct == "" {
				ct = "application/octet-stream"
			}
			html = strings.ReplaceAll(html,
				"cid:"+att.ContentID,
				"data:"+ct+";base64,"+base64.StdEncoding.EncodeToString(att.Data))
		}
		return html
	}

	// ค่าเริ่มต้น: ตัดแท็กรูปที่อ้าง cid ทิ้ง ส่วนหัวจดหมายเหลือชื่อองค์กรเป็นข้อความ
	// ปลอดภัยกว่าเพราะไม่มีรูปแตกในทุกโปรแกรมอ่านเมล
	return cidImageRe.ReplaceAllString(html, "")
}

func sendFlow(ctx context.Context, cfg Config, msg Message) error {
	endpoint := strings.TrimSpace(cfg.FlowURL)
	if endpoint == "" {
		return fmt.Errorf("ยังไม่ได้ตั้ง MAIL_FLOW_URL — เอา URL จาก trigger ของ Power Automate มาใส่ในไฟล์ .env")
	}

	payload := flowPayload{
		To:          strings.Join(msg.To, ";"),
		CC:          strings.Join(msg.CC, ";"),
		BCC:         strings.Join(msg.BCC, ";"),
		Subject:     msg.Subject,
		HTML:        flowBody(cfg, msg),
		Attachments: []flowAttachment{},
	}
	if payload.To == "" {
		payload.To = strings.Join(cfg.To, ";")
	}

	for i, att := range msg.Attachments {
		if att.Inline && cfg.FlowInline != "attach" {
			continue
		}
		name := strings.TrimSpace(att.FileName)
		if name == "" {
			name = fmt.Sprintf("attachment-%d", i+1)
		}
		payload.Attachments = append(payload.Attachments, flowAttachment{
			Name:         name,
			ContentBytes: base64.StdEncoding.EncodeToString(att.Data),
			ContentType:  att.ContentType,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("เตรียมข้อมูลส่ง Power Automate ไม่สำเร็จ: %w", err)
	}
	if len(body) > flowSizeWarnBytes {
		log.Printf("[weekly-alert] ⚠️  ข้อมูลที่ส่งให้ Flow ใหญ่ %.1f MB — ถ้า Flow ตีกลับ ให้ลด WEEKLY_ALERT_MAX_ROWS หรือปิด WEEKLY_ALERT_ATTACH_CSV",
			float64(len(body))/(1<<20))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("สร้างคำขอไปยัง Power Automate ไม่สำเร็จ: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if s := strings.TrimSpace(cfg.FlowSecret); s != "" {
		req.Header.Set("x-iconfirm-secret", s)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("เรียก Power Automate ไม่สำเร็จ (ตรวจว่าเซิร์ฟเวอร์ออกเน็ตไปหา *.environment.api.powerplatform.com หรือ *.logic.azure.com ได้): %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	detail, _ := io.ReadAll(io.LimitReader(res.Body, 2048))

	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return fmt.Errorf("Power Automate ปฏิเสธคำขอ (%d) — URL อาจหมดอายุหรือ Flow ถูกปิดอยู่ ให้เข้าไปคัดลอก URL ใหม่: %s",
			res.StatusCode, strings.TrimSpace(string(detail)))
	case res.StatusCode == http.StatusNotFound:
		return fmt.Errorf("Power Automate ตอบ 404 — MAIL_FLOW_URL ไม่ถูกต้อง หรือ Flow ถูกลบไปแล้ว")
	case res.StatusCode >= 300:
		return fmt.Errorf("Power Automate ตอบกลับ %d: %s", res.StatusCode, strings.TrimSpace(string(detail)))
	}

	log.Printf("[weekly-alert] (MAIL_PROVIDER=flow) ส่งให้ Power Automate แล้ว (%d) — ถึง: %s | ไฟล์แนบ %d ไฟล์",
		res.StatusCode, strings.Join(msg.AllRecipients(), ", "), len(payload.Attachments))
	log.Printf("[weekly-alert]     ถ้าเมลไม่เข้ากล่องจดหมาย ให้ดู Run history ของ Flow ที่ make.powerautomate.com")
	return nil
}
