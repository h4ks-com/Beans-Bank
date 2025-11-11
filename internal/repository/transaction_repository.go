package repository

import (
	"github.com/h4ks-com/beapin/internal/models"
	"gorm.io/gorm"
)

type TransactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(tx *gorm.DB, transaction *models.Transaction) error {
	return tx.Create(transaction).Error
}

func (r *TransactionRepository) FindByUsername(username string) ([]models.Transaction, error) {
	var transactions []models.Transaction
	err := r.db.
		Joins("JOIN users as from_user ON from_user.id = transactions.from_user_id").
		Joins("JOIN users as to_user ON to_user.id = transactions.to_user_id").
		Where("from_user.username = ? OR to_user.username = ?", username, username).
		Preload("FromUser").
		Preload("ToUser").
		Order("transactions.created_at DESC").
		Find(&transactions).Error
	return transactions, err
}

func (r *TransactionRepository) FindAll() ([]models.Transaction, error) {
	var transactions []models.Transaction
	err := r.db.
		Preload("FromUser").
		Preload("ToUser").
		Order("created_at DESC").
		Find(&transactions).Error
	return transactions, err
}
