package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username     string        `gorm:"uniqueIndex;not null" json:"username"`
	Email        string        `gorm:"" json:"email,omitempty"`
	BeanAmount   int           `gorm:"not null" json:"bean_amount"`
	Transactions []Transaction `gorm:"foreignKey:FromUserID" json:"-"`
	APITokens    []APIToken    `gorm:"foreignKey:UserID" json:"-"`
}
