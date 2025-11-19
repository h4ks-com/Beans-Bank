package repository

import (
	"github.com/h4ks-com/bean-bank/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HarvestRepository struct {
	db *gorm.DB
}

func NewHarvestRepository(db *gorm.DB) *HarvestRepository {
	return &HarvestRepository{db: db}
}

func (r *HarvestRepository) Create(harvest *models.Harvest) error {
	return r.db.Create(harvest).Error
}

func (r *HarvestRepository) FindByID(id uint) (*models.Harvest, error) {
	var harvest models.Harvest
	err := r.db.Preload("AssignedUser").First(&harvest, id).Error
	if err != nil {
		return nil, err
	}
	return &harvest, nil
}

func (r *HarvestRepository) FindByIDForUpdate(tx *gorm.DB, id uint) (*models.Harvest, error) {
	var harvest models.Harvest
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&harvest, id).Error
	if err != nil {
		return nil, err
	}
	return &harvest, nil
}

func (r *HarvestRepository) Update(harvest *models.Harvest) error {
	return r.db.Save(harvest).Error
}

func (r *HarvestRepository) UpdateInTx(tx *gorm.DB, harvest *models.Harvest) error {
	return tx.Save(harvest).Error
}

func (r *HarvestRepository) Delete(id uint) error {
	return r.db.Delete(&models.Harvest{}, id).Error
}

func (r *HarvestRepository) Search(query string, page, limit int) ([]models.Harvest, error) {
	var harvests []models.Harvest
	offset := (page - 1) * limit

	db := r.db.Preload("AssignedUser")

	if query != "" {
		searchPattern := "%" + query + "%"
		db = db.Where("title LIKE ? OR description LIKE ?", searchPattern, searchPattern)
	}

	err := db.Order("updated_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&harvests).Error

	return harvests, err
}

func (r *HarvestRepository) CountSearch(query string) (int64, error) {
	var count int64
	db := r.db.Model(&models.Harvest{})

	if query != "" {
		searchPattern := "%" + query + "%"
		db = db.Where("title LIKE ? OR description LIKE ?", searchPattern, searchPattern)
	}

	err := db.Count(&count).Error
	return count, err
}

func (r *HarvestRepository) FindAll() ([]models.Harvest, error) {
	var harvests []models.Harvest
	err := r.db.Preload("AssignedUser").Order("updated_at DESC").Find(&harvests).Error
	return harvests, err
}
