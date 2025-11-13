package services

import (
	"errors"

	"github.com/h4ks-com/beapin/internal/models"
	"github.com/h4ks-com/beapin/internal/repository"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type WalletService struct {
	userRepo        *repository.UserRepository
	transactionRepo *repository.TransactionRepository
}

func NewWalletService(userRepo *repository.UserRepository, transactionRepo *repository.TransactionRepository) *WalletService {
	return &WalletService{
		userRepo:        userRepo,
		transactionRepo: transactionRepo,
	}
}

func (s *WalletService) GetOrCreateWallet(username string) (*models.User, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	if user == nil {
		user = &models.User{
			Username:   username,
			BeanAmount: 1,
		}
		err = s.userRepo.Create(user)
		if err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (s *WalletService) GetBalance(username string) (int, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, ErrUserNotFound
	}
	return user.BeanAmount, nil
}

func (s *WalletService) GetTransactionHistory(username string) ([]models.Transaction, error) {
	return s.transactionRepo.FindByUsername(username)
}

func (s *WalletService) GetTotalBeans() (int64, error) {
	return s.userRepo.GetTotalBeans()
}

func (s *WalletService) UpdateBalance(username string, newAmount int) error {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	user.BeanAmount = newAmount
	return s.userRepo.Update(user)
}

func (s *WalletService) GetTopWallets(limit int) ([]models.User, error) {
	return s.userRepo.GetTopWallets(limit)
}
