package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Send(ctx context.Context, cfg Config, msg Message) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	if msg.FromEmail == "" {
		msg.FromEmail = cfg.FromEmail
	}
	if msg.FromName == "" {
		msg.FromName = cfg.FromName
	}
	if len(msg.To) == 0 {
		msg.To = cfg.To
	}
	if len(msg.CC) == 0 {
		msg.CC = cfg.CC
	}
	if len(msg.BCC) == 0 {
		msg.BCC = cfg.BCC
	}

	switch cfg.Provider {
	case ProviderOutlook:
		return sendOutlook(cfg, msg)

	case ProviderFlow:
		return sendFlow(ctx, cfg, msg)

	case ProviderFile:
		return writeToFile(cfg, msg)

	case ProviderLog:
		log.Printf("[weekly-alert] (MAIL_PROVIDER=log) ไม่ได้ส่งจริง — ถึง: %s | เรื่อง: %s | ขนาด HTML %d ไบต์",
			strings.Join(msg.AllRecipients(), ", "), msg.Subject, len(msg.HTML))
		return nil

	case ProviderGraph:
		return sendGraph(ctx, cfg, msg)

	default:
		return sendSMTP(cfg, msg)
	}
}

func writeToFile(cfg Config, msg Message) error {
	dir := strings.TrimSpace(cfg.OutDir)
	if dir == "" {
		dir = "outbox"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("สร้างโฟลเดอร์ %s ไม่สำเร็จ: %w", dir, err)
	}

	stamp := time.Now().Format("20060102-150405")
	emlPath := filepath.Join(dir, "weekly-alert-"+stamp+".eml")
	htmlPath := filepath.Join(dir, "weekly-alert-"+stamp+".html")

	if err := os.WriteFile(emlPath, msg.Build(), 0o644); err != nil {
		return fmt.Errorf("เขียนไฟล์ %s ไม่สำเร็จ: %w", emlPath, err)
	}
	if err := os.WriteFile(htmlPath, []byte(msg.HTML), 0o644); err != nil {
		return fmt.Errorf("เขียนไฟล์ %s ไม่สำเร็จ: %w", htmlPath, err)
	}

	abs, err := filepath.Abs(emlPath)
	if err != nil {
		abs = emlPath
	}
	log.Printf("[weekly-alert] (MAIL_PROVIDER=file) ไม่ได้ส่งออกนอกเครื่อง — เขียนจดหมายไว้ที่ %s", abs)
	log.Printf("[weekly-alert]     ถ้าจะให้ส่งเข้ากล่องจดหมายจริง ตั้ง MAIL_PROVIDER=smtp หรือ graph")
	return nil
}

type loginAuth struct {
	username string
	password string
	host     string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		return "", nil, errors.New("ต้องเชื่อมต่อแบบเข้ารหัส (TLS) ก่อนจึงจะยืนยันตัวตนได้")
	}
	if server.Name != a.host {
		return "", nil, errors.New("ชื่อเซิร์ฟเวอร์ไม่ตรงกับที่ตั้งไว้")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(fromServer))) {
	case "username:":
		return []byte(a.username), nil
	case "password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("เซิร์ฟเวอร์ถามข้อมูลที่ไม่รู้จัก: %s", string(fromServer))
	}
}

func sendSMTP(cfg Config, msg Message) error {
	addr := net.JoinHostPort(cfg.SMTPHost, fmt.Sprintf("%d", cfg.SMTPPort))
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	var (
		conn net.Conn
		err  error
	)

	tlsConfig := &tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}

	if cfg.SMTPPort == 465 {
		dialer := &net.Dialer{Timeout: timeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = net.DialTimeout("tcp", addr, timeout)
	}
	if err != nil {
		return fmt.Errorf("เชื่อมต่อเซิร์ฟเวอร์เมล %s ไม่สำเร็จ: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("เริ่มต้นการคุยกับเซิร์ฟเวอร์เมลไม่สำเร็จ: %w", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Hello(clientHostname()); err != nil {
		return fmt.Errorf("ส่งคำสั่ง EHLO ไม่สำเร็จ: %w", err)
	}

	if cfg.SMTPPort != 465 && cfg.SMTPStartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("เปิดการเข้ารหัส STARTTLS ไม่สำเร็จ: %w", err)
			}
		} else if cfg.SMTPUsername != "" {
			// มีการล็อกอินแต่เข้ารหัสไม่ได้ = รหัสผ่านจะวิ่งแบบเปลือย ต้องหยุด
			return errors.New("เซิร์ฟเวอร์เมลไม่รองรับ STARTTLS แต่ตั้ง SMTP_USERNAME ไว้ — " +
				"จะส่งรหัสผ่านแบบไม่เข้ารหัส จึงหยุดไว้ก่อน (ตรวจค่า SMTP_HOST/SMTP_PORT)")
		} else {
			// relay ภายในที่ไม่ต้องล็อกอินมักไม่เปิด STARTTLS
			// ไม่มีรหัสผ่านให้รั่ว และวิ่งอยู่ในวงภายใน จึงส่งต่อได้
			log.Printf("[mail] เซิร์ฟเวอร์ %s ไม่รองรับ STARTTLS — ส่งแบบไม่เข้ารหัส (relay ภายใน ไม่มีการล็อกอิน)", cfg.SMTPHost)
		}
	}

	if cfg.SMTPUsername != "" {
		auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			if err2 := client.Auth(&loginAuth{cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost}); err2 != nil {
				return fmt.Errorf("เข้าสู่ระบบเมลไม่สำเร็จ (ตรวจ SMTP_USERNAME/SMTP_PASSWORD และการเปิด SMTP AUTH ของกล่องจดหมาย): %w", err)
			}
		}
	}

	if err := client.Mail(cfg.FromEmail); err != nil {
		return fmt.Errorf("เซิร์ฟเวอร์ไม่รับผู้ส่ง %s: %w", cfg.FromEmail, err)
	}
	for _, rcpt := range msg.AllRecipients() {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("เซิร์ฟเวอร์ไม่รับผู้รับ %s: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("เริ่มส่งเนื้อจดหมายไม่สำเร็จ: %w", err)
	}
	if _, err := w.Write(msg.Build()); err != nil {
		_ = w.Close()
		return fmt.Errorf("ส่งเนื้อจดหมายไม่สำเร็จ: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("ปิดเนื้อจดหมายไม่สำเร็จ: %w", err)
	}

	return client.Quit()
}

func clientHostname() string {
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		return h
	}
	return "localhost"
}

type graphRecipient struct {
	EmailAddress struct {
		Address string `json:"address"`
	} `json:"emailAddress"`
}

func graphRecipients(list []string) []graphRecipient {
	out := make([]graphRecipient, 0, len(list))
	for _, addr := range list {
		var r graphRecipient
		r.EmailAddress.Address = addr
		out = append(out, r)
	}
	return out
}

func sendGraph(ctx context.Context, cfg Config, msg Message) error {
	token, err := graphToken(ctx, cfg)
	if err != nil {
		return err
	}

	type graphAttachment struct {
		ODataType    string `json:"@odata.type"`
		Name         string `json:"name"`
		ContentType  string `json:"contentType"`
		ContentBytes string `json:"contentBytes"`
		IsInline     bool   `json:"isInline,omitempty"`
		ContentID    string `json:"contentId,omitempty"`
	}

	attachments := make([]graphAttachment, 0, len(msg.Attachments))
	for _, att := range msg.Attachments {
		attachments = append(attachments, graphAttachment{
			ODataType:    "#microsoft.graph.fileAttachment",
			Name:         att.FileName,
			ContentType:  att.ContentType,
			ContentBytes: base64.StdEncoding.EncodeToString(att.Data),
			IsInline:     att.Inline,
			ContentID:    att.ContentID,
		})
	}

	payload := map[string]any{
		"message": map[string]any{
			"subject": msg.Subject,
			"body": map[string]any{
				"contentType": "HTML",
				"content":     msg.HTML,
			},
			"toRecipients":  graphRecipients(msg.To),
			"ccRecipients":  graphRecipients(msg.CC),
			"bccRecipients": graphRecipients(msg.BCC),
			"attachments":   attachments,
		},
		"saveToSentItems": true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("สร้างข้อมูลส่ง Graph ไม่สำเร็จ: %w", err)
	}

	endpoint := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/sendMail", url.PathEscape(cfg.GraphSender))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: cfg.Timeout}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("เรียก Microsoft Graph ไม่สำเร็จ: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("Microsoft Graph ตอบกลับ %d: %s", res.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}

func graphToken(ctx context.Context, cfg Config) (string, error) {
	form := url.Values{}
	form.Set("client_id", cfg.GraphClientID)
	form.Set("client_secret", cfg.GraphClientSecret)
	form.Set("scope", "https://graph.microsoft.com/.default")
	form.Set("grant_type", "client_credentials")

	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(cfg.GraphTenantID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: cfg.Timeout}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ขอ token จาก Microsoft ไม่สำเร็จ: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("ขอ token ไม่สำเร็จ (%d): %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("อ่าน token ไม่สำเร็จ: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("Microsoft ไม่ได้ส่ง access_token กลับมา")
	}
	return parsed.AccessToken, nil
}
