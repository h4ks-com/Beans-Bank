package services

import (
	"errors"

	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrRecipientNotFound   = errors.New("recipient not found")
	ErrInvalidAmount       = errors.New("invalid amount")
	ErrSelfTransfer        = errors.New("cannot transfer to yourself")
)

type TransferService struct {
	userRepo        *repository.UserRepository
	transactionRepo *repository.TransactionRepository
	db              *gorm.DB
}

func NewTransferService(userRepo *repository.UserRepository, transactionRepo *repository.TransactionRepository, db *gorm.DB) *TransferService {
	return &TransferService{
		userRepo:        userRepo,
		transactionRepo: transactionRepo,
		db:              db,
	}
}

func (s *TransferService) Transfer(fromUsername, toUsername string, amount int, force bool) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	if fromUsername == toUsername {
		return ErrSelfTransfer
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		fromUser, err := s.userRepo.FindByUsernameForUpdate(tx, fromUsername)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}

		if fromUser.BeanAmount < amount {
			return ErrInsufficientBalance
		}

		toUser, err := s.userRepo.FindByUsernameForUpdate(tx, toUsername)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if toUser == nil {
			if !force {
				return ErrRecipientNotFound
			}
			toUser = &models.User{
				Username:   toUsername,
				BeanAmount: 0,
			}
			if err := tx.Create(toUser).Error; err != nil {
				return err
			}
		}

		fromUser.BeanAmount -= amount
		toUser.BeanAmount += amount

		if err := s.userRepo.UpdateInTx(tx, fromUser); err != nil {
			return err
		}

		if err := s.userRepo.UpdateInTx(tx, toUser); err != nil {
			return err
		}

		transaction := &models.Transaction{
			FromUserID: fromUser.ID,
			ToUserID:   toUser.ID,
			Amount:     amount,
		}

		return s.transactionRepo.Create(tx, transaction)
	})
}
