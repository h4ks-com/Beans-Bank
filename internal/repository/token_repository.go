package repository

import (
	"time"

	"github.com/h4ks-com/bean-bank/internal/models"
	"gorm.io/gorm"
)

type TokenRepository struct {
	db *gorm.DB
}

func NewTokenRepository(db *gorm.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

func (r *TokenRepository) Create(token *models.APIToken) error {
	return r.db.Create(token).Error
}

func (r *TokenRepository) FindByToken(tokenStr string) (*models.APIToken, error) {
	var token models.APIToken
	err := r.db.Where("token = ? AND expires_at > ?", tokenStr, time.Now()).
		Preload("User").
		First(&token).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *TokenRepository) FindByUserID(userID uint) ([]models.APIToken, error) {
	var tokens []models.APIToken
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&tokens).Error
	return tokens, err
}

func (r *TokenRepository) Delete(id uint, userID uint) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.APIToken{}).Error
}

func (r *TokenRepository) DeleteExpired() error {
	return r.db.Where("expires_at < ?", time.Now()).Delete(&models.APIToken{}).Error
}
