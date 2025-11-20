package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type GiftLink struct {
	gorm.Model
	Code         string     `gorm:"uniqueIndex;not null;size:64" json:"code"`
	FromUserID   uint       `gorm:"not null;index" json:"from_user_id"`
	FromUser     User       `gorm:"foreignKey:FromUserID" json:"-"`
	Amount       int        `gorm:"not null" json:"amount"`
	Message      string     `gorm:"type:text" json:"message"`
	ExpiresAt    *time.Time `gorm:"index" json:"expires_at"`
	RedeemedAt   *time.Time `json:"redeemed_at"`
	RedeemedByID *uint      `gorm:"index" json:"redeemed_by_id"`
	RedeemedBy   *User      `gorm:"foreignKey:RedeemedByID" json:"-"`
	Active       bool       `gorm:"default:true;index" json:"active"`
}

func (g GiftLink) MarshalJSON() ([]byte, error) {
	type Alias GiftLink
	var redeemedByUsername *string
	if g.RedeemedBy != nil {
		redeemedByUsername = &g.RedeemedBy.Username
	}

	return json.Marshal(&struct {
		ID                 uint       `json:"id"`
		CreatedAt          time.Time  `json:"created_at"`
		UpdatedAt          time.Time  `json:"updated_at"`
		Code               string     `json:"code"`
		FromUserID         uint       `json:"from_user_id"`
		FromUsername       string     `json:"from_username"`
		Amount             int        `json:"amount"`
		Message            string     `json:"message"`
		ExpiresAt          *time.Time `json:"expires_at"`
		RedeemedAt         *time.Time `json:"redeemed_at"`
		RedeemedByID       *uint      `json:"redeemed_by_id"`
		RedeemedBy         *string    `json:"redeemed_by,omitempty"`
		RedeemedByUsername *string    `json:"redeemed_by_username,omitempty"`
		Active             bool       `json:"active"`
	}{
		ID:                 g.ID,
		CreatedAt:          g.CreatedAt,
		UpdatedAt:          g.UpdatedAt,
		Code:               g.Code,
		FromUserID:         g.FromUserID,
		FromUsername:       g.FromUser.Username,
		Amount:             g.Amount,
		Message:            g.Message,
		ExpiresAt:          g.ExpiresAt,
		RedeemedAt:         g.RedeemedAt,
		RedeemedByID:       g.RedeemedByID,
		RedeemedBy:         redeemedByUsername,
		RedeemedByUsername: redeemedByUsername,
		Active:             g.Active,
	})
}
