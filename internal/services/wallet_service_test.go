package services

import (
	"testing"

	"github.com/h4ks-com/beapin/internal/database"
	"github.com/h4ks-com/beapin/internal/models"
	"github.com/h4ks-com/beapin/internal/repository"
	"github.com/stretchr/testify/assert"
)

func setupWalletTestDB(t *testing.T) (*repository.UserRepository, *WalletService) {
	db, err := database.Connect(":memory:")
	assert.NoError(t, err)

	err = database.Migrate(db)
	assert.NoError(t, err)

	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	walletService := NewWalletService(userRepo, transactionRepo)

	return userRepo, walletService
}

func TestWalletService_GetOrCreateWallet_NewUser(t *testing.T) {
	_, walletService := setupWalletTestDB(t)

	user, err := walletService.GetOrCreateWallet("alice")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, 1, user.BeanAmount)
}

func TestWalletService_GetOrCreateWallet_ExistingUser(t *testing.T) {
	userRepo, walletService := setupWalletTestDB(t)

	existingUser := &models.User{Username: "bob", BeanAmount: 50}
	userRepo.Create(existingUser)

	user, err := walletService.GetOrCreateWallet("bob")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "bob", user.Username)
	assert.Equal(t, 50, user.BeanAmount)
}

func TestWalletService_GetBalance(t *testing.T) {
	userRepo, walletService := setupWalletTestDB(t)

	user := &models.User{Username: "alice", BeanAmount: 100}
	userRepo.Create(user)

	balance, err := walletService.GetBalance("alice")
	assert.NoError(t, err)
	assert.Equal(t, 100, balance)
}

func TestWalletService_GetBalance_UserNotFound(t *testing.T) {
	_, walletService := setupWalletTestDB(t)

	_, err := walletService.GetBalance("nonexistent")
	assert.Equal(t, ErrUserNotFound, err)
}

func TestWalletService_UpdateBalance(t *testing.T) {
	userRepo, walletService := setupWalletTestDB(t)

	user := &models.User{Username: "alice", BeanAmount: 100}
	userRepo.Create(user)

	err := walletService.UpdateBalance("alice", 200)
	assert.NoError(t, err)

	balance, _ := walletService.GetBalance("alice")
	assert.Equal(t, 200, balance)
}

func TestWalletService_GetTotalBeans(t *testing.T) {
	userRepo, walletService := setupWalletTestDB(t)

	userRepo.Create(&models.User{Username: "alice", BeanAmount: 100})
	userRepo.Create(&models.User{Username: "bob", BeanAmount: 50})
	userRepo.Create(&models.User{Username: "charlie", BeanAmount: 75})

	total, err := walletService.GetTotalBeans()
	assert.NoError(t, err)
	assert.Equal(t, int64(225), total)
}
