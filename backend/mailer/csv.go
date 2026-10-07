package mailer

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Attachment struct {
	FileName    string
	ContentType string
	Data        []byte

	Inline    bool
	ContentID string
}

func (a Attachment) DataURI() string {
	return "data:" + a.ContentType + ";base64," + base64.StdEncoding.EncodeToString(a.Data)
}

func LoadInlineImage(path, contentID string) (Attachment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Attachment{}, err
	}

	contentType := "image/png"
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".gif":
		contentType = "image/gif"
	}

	return Attachment{
		FileName:    filepath.Base(path),
		ContentType: contentType,
		Data:        data,
		Inline:      true,
		ContentID:   contentID,
	}, nil
}

func BuildCSV(r WeeklyReport) Attachment {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)

	_ = w.Write([]string{
		"ประเภท",
		"เลขที่ใบอนุญาต",
		"เลขที่ใบอนุญาตนำเข้า (อ้างอิง)",
		"วันหมดอายุใบนำเข้า",
		"Invoice",
		"ใบขนสินค้า",
		"ยี่ห้อ",
		"รุ่น",
		"จำนวนเครื่อง",
		"ยืนยันแล้ว",
		"วันที่ออก/นำออก",
		"วันหมดอายุ",
		"วันคงเหลือ",
		"สถานะอายุใบอนุญาต",
		"กำหนดยื่น กสทช.",
		"วันคงเหลือถึงกำหนดยื่น",
		"สถานะการยื่น",
	})

	num := func(n int) string { return fmt.Sprintf("%d", n) }

	for _, row := range r.Import {
		_ = w.Write([]string{
			"ใบอนุญาตนำเข้า",
			row.LicenseNo,
			"", "",
			row.InvoiceNo,
			row.DeclarationNo,
			row.Brand,
			row.Model,
			num(row.Machines),
			num(row.Confirmed),
			ThaiDate(row.IssueDate, r.BuddhistEra),
			ThaiDate(row.ExpiryDate, r.BuddhistEra),
			DaysCountLabel(row.DaysLeft),
			StatusLabel(row.Status),
			"", "", "",
		})
	}

	for _, row := range r.Export {
		_ = w.Write([]string{
			"ใบอนุญาตนำออก",
			row.ExportLicenseNo,
			row.ImportLicenseNo,
			ThaiDate(row.ImportExpiryDate, r.BuddhistEra),
			"", "", "", "",
			num(row.Machines),
			"",
			ThaiDate(row.IssueDate, r.BuddhistEra),
			ThaiDate(row.ExpiryDate, r.BuddhistEra),
			DaysCountLabel(row.DaysLeft),
			StatusLabel(row.Status),
			ThaiDate(row.LeadDate, r.BuddhistEra),
			DaysCountLabel(row.LeadDaysLeft),
			LeadLabel(row.LeadStatus, row.LeadDaysLeft),
		})
	}

	w.Flush()

	return Attachment{
		FileName:    fmt.Sprintf("license-weekly-alert-%s.csv", r.FileDateKey()),
		ContentType: "text/csv; charset=utf-8",
		Data:        buf.Bytes(),
	}
}
