package repository

import (
	"errors"

	"github.com/h4ks-com/bean-bank/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByUsernameForUpdate(tx *gorm.DB, username string) (*models.User, error) {
	var user models.User
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("username = ?", username).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepository) UpdateInTx(tx *gorm.DB, user *models.User) error {
	return tx.Save(user).Error
}

func (r *UserRepository) FindAll() ([]models.User, error) {
	var users []models.User
	err := r.db.Find(&users).Error
	return users, err
}

func (r *UserRepository) GetTotalBeans() (int64, error) {
	var total int64
	err := r.db.Model(&models.User{}).Select("COALESCE(SUM(bean_amount), 0)").Scan(&total).Error
	return total, err
}

func (r *UserRepository) GetTopWallets(limit int) ([]models.User, error) {
	var users []models.User
	err := r.db.Order("bean_amount DESC").Limit(limit).Find(&users).Error
	return users, err
}
