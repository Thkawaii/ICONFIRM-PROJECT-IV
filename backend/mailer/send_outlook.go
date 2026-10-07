package mailer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type outlookManifest struct {
	To          []string            `json:"to"`
	CC          []string            `json:"cc"`
	BCC         []string            `json:"bcc"`
	Subject     string              `json:"subject"`
	BodyFile    string              `json:"bodyFile"`
	Attachments []outlookAttachment `json:"attachments"`
	DisplayOnly bool                `json:"displayOnly"`
}

type outlookAttachment struct {
	Path        string `json:"path"`
	ContentID   string `json:"contentId"`
	ContentType string `json:"contentType"`
}

const outlookScript = `
$ErrorActionPreference = 'Stop'
$manifestPath = $args[0]

$m = Get-Content -Raw -LiteralPath $manifestPath -Encoding UTF8 | ConvertFrom-Json

$outlook = New-Object -ComObject Outlook.Application
$mail = $outlook.CreateItem(0)

$mail.To = ($m.to -join '; ')
if ($m.cc)  { $mail.CC  = ($m.cc  -join '; ') }
if ($m.bcc) { $mail.BCC = ($m.bcc -join '; ') }
$mail.Subject = $m.subject
$mail.HTMLBody = Get-Content -Raw -LiteralPath $m.bodyFile -Encoding UTF8

foreach ($a in $m.attachments) {
    $att = $mail.Attachments.Add($a.path)
    if ($a.contentId) {
        # ตั้ง Content-ID ให้รูปที่ฝังในเนื้อจดหมาย ไม่งั้น cid: ในเนื้อ HTML จะหารูปไม่เจอ
        $att.PropertyAccessor.SetProperty('http://schemas.microsoft.com/mapi/proptag/0x3712001F', $a.contentId)
        $att.PropertyAccessor.SetProperty('http://schemas.microsoft.com/mapi/proptag/0x370E001F', $a.contentType)
    }
}

if ($m.displayOnly) {
    $mail.Display()
    Write-Output 'DISPLAYED'
} else {
    $mail.Send()
    Write-Output 'SENT'
}
`

func sendOutlook(cfg Config, msg Message) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf(
			"MAIL_PROVIDER=outlook ใช้ได้เฉพาะเมื่อ backend รันบน Windows ที่ติดตั้ง Outlook ไว้ "+
				"(ตอนนี้รันบน %s) — ถ้ารัน backend บนเซิร์ฟเวอร์ Linux ให้ใช้ smtp หรือ graph แทน", runtime.GOOS)
	}

	shell, err := powershellPath()
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "iconfirm-mail-")
	if err != nil {
		return fmt.Errorf("สร้างโฟลเดอร์ชั่วคราวไม่สำเร็จ: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	bodyPath := filepath.Join(dir, "body.html")
	if err := os.WriteFile(bodyPath, []byte(msg.HTML), 0o600); err != nil {
		return fmt.Errorf("เขียนเนื้อจดหมายลงไฟล์ชั่วคราวไม่สำเร็จ: %w", err)
	}

	manifest := outlookManifest{
		To:          msg.To,
		CC:          msg.CC,
		BCC:         msg.BCC,
		Subject:     msg.Subject,
		BodyFile:    bodyPath,
		DisplayOnly: cfg.OutlookDisplayOnly,
	}
	if len(manifest.To) == 0 {
		manifest.To = cfg.To
	}

	for i, att := range msg.Attachments {
		name := att.FileName
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("attachment-%d", i+1)
		}
		path := filepath.Join(dir, filepath.Base(name))
		if err := os.WriteFile(path, att.Data, 0o600); err != nil {
			return fmt.Errorf("เขียนไฟล์แนบ %s ไม่สำเร็จ: %w", name, err)
		}

		item := outlookAttachment{Path: path, ContentType: att.ContentType}
		if att.Inline {
			item.ContentID = att.ContentID
		}
		manifest.Attachments = append(manifest.Attachments, item)
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	raw, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("เตรียมข้อมูลจดหมายไม่สำเร็จ: %w", err)
	}
	if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
		return fmt.Errorf("เขียนไฟล์ข้อมูลจดหมายไม่สำเร็จ: %w", err)
	}

	scriptPath := filepath.Join(dir, "send.ps1")
	if err := os.WriteFile(scriptPath, []byte(outlookScript), 0o600); err != nil {
		return fmt.Errorf("เขียนสคริปต์ส่งเมลไม่สำเร็จ: %w", err)
	}

	cmd := exec.Command(shell,
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", scriptPath, manifestPath)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("สั่ง Outlook ส่งจดหมายไม่สำเร็จ: %w\n%s",
			err, strings.TrimSpace(errOut.String()))
	}

	if cfg.OutlookDisplayOnly {
		log.Printf("[weekly-alert] (MAIL_PROVIDER=outlook) เปิดหน้าต่างร่างจดหมายใน Outlook ให้แล้ว — ตรวจแล้วกด Send เอง")
		return nil
	}

	log.Printf("[weekly-alert] (MAIL_PROVIDER=outlook) สั่ง Outlook ส่งแล้ว — ถึง: %s",
		strings.Join(msg.AllRecipients(), ", "))
	log.Printf("[weekly-alert]     ถ้ายังไม่เข้ากล่องจดหมาย ให้ดูที่โฟลเดอร์ Outbox ของ Outlook (จะออกตอนซิงก์รอบถัดไป)")
	return nil
}

func powershellPath() (string, error) {
	for _, name := range []string{"powershell.exe", "pwsh.exe", "powershell", "pwsh"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("หา PowerShell บนเครื่องไม่เจอ — MAIL_PROVIDER=outlook ต้องใช้ PowerShell สั่งงาน Outlook")
}
