package models

import "time"

const (
	MailRecipientTo  = "TO"
	MailRecipientCC  = "CC"
	MailRecipientBCC = "BCC"
)

type MailRecipient struct {
	ID uint `gorm:"primaryKey" json:"id"`

	Email string `gorm:"size:190;not null;uniqueIndex:idx_mail_recipient_email_kind" json:"email"`

	Kind string `gorm:"size:8;not null;default:TO;uniqueIndex:idx_mail_recipient_email_kind" json:"kind"`

	Name string `gorm:"size:190" json:"name"`

	Active bool `gorm:"not null;default:true" json:"active"`

	Note string `gorm:"size:190" json:"note"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
