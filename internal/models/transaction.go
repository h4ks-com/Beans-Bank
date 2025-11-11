package models

import (
	"time"

	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model
	FromUserID uint      `gorm:"not null;index" json:"from_user_id"`
	FromUser   User      `gorm:"foreignKey:FromUserID" json:"from_user,omitempty"`
	ToUserID   uint      `gorm:"not null;index" json:"to_user_id"`
	ToUser     User      `gorm:"foreignKey:ToUserID" json:"to_user,omitempty"`
	Amount     int       `gorm:"not null" json:"amount"`
	Timestamp  time.Time `gorm:"autoCreateTime" json:"timestamp"`
}
