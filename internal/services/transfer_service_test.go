package services

import (
	"testing"

	"github.com/h4ks-com/beapin/internal/database"
	"github.com/h4ks-com/beapin/internal/models"
	"github.com/h4ks-com/beapin/internal/repository"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) (*repository.UserRepository, *repository.TransactionRepository, *TransferService) {
	db, err := database.Connect(":memory:")
	assert.NoError(t, err)

	err = database.Migrate(db)
	assert.NoError(t, err)

	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	transferService := NewTransferService(userRepo, transactionRepo, db)

	return userRepo, transactionRepo, transferService
}

func TestTransferService_SuccessfulTransfer(t *testing.T) {
	userRepo, _, transferService := setupTestDB(t)

	alice := &models.User{Username: "alice", BeanAmount: 100}
	bob := &models.User{Username: "bob", BeanAmount: 50}
	err := userRepo.Create(alice)
	assert.NoError(t, err)
	err = userRepo.Create(bob)
	assert.NoError(t, err)

	err = transferService.Transfer("alice", "bob", 30, false)
	assert.NoError(t, err)

	aliceAfter, _ := userRepo.FindByUsername("alice")
	bobAfter, _ := userRepo.FindByUsername("bob")

	assert.Equal(t, 70, aliceAfter.BeanAmount)
	assert.Equal(t, 80, bobAfter.BeanAmount)
}

func TestTransferService_InsufficientBalance(t *testing.T) {
	userRepo, _, transferService := setupTestDB(t)

	alice := &models.User{Username: "alice", BeanAmount: 10}
	bob := &models.User{Username: "bob", BeanAmount: 50}
	userRepo.Create(alice)
	userRepo.Create(bob)

	err := transferService.Transfer("alice", "bob", 20, false)
	assert.Equal(t, ErrInsufficientBalance, err)

	aliceAfter, _ := userRepo.FindByUsername("alice")
	bobAfter, _ := userRepo.FindByUsername("bob")

	assert.Equal(t, 10, aliceAfter.BeanAmount)
	assert.Equal(t, 50, bobAfter.BeanAmount)
}

func TestTransferService_RecipientNotFound(t *testing.T) {
	userRepo, _, transferService := setupTestDB(t)

	alice := &models.User{Username: "alice", BeanAmount: 100}
	userRepo.Create(alice)

	err := transferService.Transfer("alice", "nonexistent", 10, false)
	assert.Equal(t, ErrRecipientNotFound, err)
}

func TestTransferService_ForceCreateRecipient(t *testing.T) {
	userRepo, _, transferService := setupTestDB(t)

	alice := &models.User{Username: "alice", BeanAmount: 100}
	userRepo.Create(alice)

	err := transferService.Transfer("alice", "newuser", 10, true)
	assert.NoError(t, err)

	newUser, _ := userRepo.FindByUsername("newuser")
	assert.NotNil(t, newUser)
	assert.Equal(t, 10, newUser.BeanAmount)

	aliceAfter, _ := userRepo.FindByUsername("alice")
	assert.Equal(t, 90, aliceAfter.BeanAmount)
}

func TestTransferService_InvalidAmount(t *testing.T) {
	userRepo, _, transferService := setupTestDB(t)

	alice := &models.User{Username: "alice", BeanAmount: 100}
	bob := &models.User{Username: "bob", BeanAmount: 50}
	userRepo.Create(alice)
	userRepo.Create(bob)

	err := transferService.Transfer("alice", "bob", 0, false)
	assert.Equal(t, ErrInvalidAmount, err)

	err = transferService.Transfer("alice", "bob", -10, false)
	assert.Equal(t, ErrInvalidAmount, err)
}

func TestTransferService_SelfTransfer(t *testing.T) {
	userRepo, _, transferService := setupTestDB(t)

	alice := &models.User{Username: "alice", BeanAmount: 100}
	userRepo.Create(alice)

	err := transferService.Transfer("alice", "alice", 10, false)
	assert.Equal(t, ErrSelfTransfer, err)

	aliceAfter, _ := userRepo.FindByUsername("alice")
	assert.Equal(t, 100, aliceAfter.BeanAmount)
}
