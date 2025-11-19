package models

import "gorm.io/gorm"

type Harvest struct {
	gorm.Model
	Title          string `gorm:"not null" json:"title"`
	Description    string `gorm:"type:text" json:"description"`
	BeanAmount     int    `gorm:"not null" json:"bean_amount"`
	AssignedUserID *uint  `gorm:"index" json:"assigned_user_id,omitempty"`
	AssignedUser   *User  `gorm:"foreignKey:AssignedUserID" json:"assigned_user,omitempty"`
	Completed      bool   `gorm:"default:false;index" json:"completed"`
}
