package repository

import (
	"github.com/h4ks-com/bean-bank/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GiftLinkRepository struct {
	db *gorm.DB
}

func NewGiftLinkRepository(db *gorm.DB) *GiftLinkRepository {
	return &GiftLinkRepository{db: db}
}

func (r *GiftLinkRepository) Create(giftLink *models.GiftLink) error {
	return r.db.Create(giftLink).Error
}

func (r *GiftLinkRepository) FindByCode(code string) (*models.GiftLink, error) {
	var giftLink models.GiftLink
	err := r.db.Preload("FromUser").Preload("RedeemedBy").
		Where("code = ?", code).
		First(&giftLink).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &giftLink, nil
}

func (r *GiftLinkRepository) FindByCodeForUpdate(tx *gorm.DB, code string) (*models.GiftLink, error) {
	var giftLink models.GiftLink
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("FromUser").
		Where("code = ?", code).
		First(&giftLink).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &giftLink, nil
}

func (r *GiftLinkRepository) ListByFromUserID(userID uint) ([]models.GiftLink, error) {
	var giftLinks []models.GiftLink
	err := r.db.Preload("FromUser").Preload("RedeemedBy").
		Where("from_user_id = ? AND active = ?", userID, true).
		Order("created_at DESC").
		Find(&giftLinks).Error

	if err != nil {
		return nil, err
	}
	return giftLinks, nil
}

func (r *GiftLinkRepository) FindByID(id uint) (*models.GiftLink, error) {
	var giftLink models.GiftLink
	err := r.db.Preload("FromUser").Preload("RedeemedBy").
		Where("id = ?", id).
		First(&giftLink).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &giftLink, nil
}

func (r *GiftLinkRepository) Update(giftLink *models.GiftLink) error {
	return r.db.Save(giftLink).Error
}

func (r *GiftLinkRepository) UpdateInTx(tx *gorm.DB, giftLink *models.GiftLink) error {
	return tx.Save(giftLink).Error
}

func (r *GiftLinkRepository) Delete(id uint) error {
	return r.db.Delete(&models.GiftLink{}, id).Error
}
