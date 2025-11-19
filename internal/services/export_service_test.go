package services

import (
	"encoding/json"
	"testing"

	"github.com/h4ks-com/bean-bank/internal/database"
	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupExportTestDB(t *testing.T) (*gorm.DB, *repository.UserRepository, *repository.TransactionRepository, *ExportService) {
	db, err := database.Connect(":memory:")
	assert.NoError(t, err)

	err = database.Migrate(db)
	assert.NoError(t, err)

	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	exportService := NewExportService(userRepo, transactionRepo, "test-signing-key-32-characters!!")

	return db, userRepo, transactionRepo, exportService
}

func TestExportService_ExportTransactions(t *testing.T) {
	db, userRepo, transactionRepo, exportService := setupExportTestDB(t)

	alice := &models.User{Username: "alice", Email: "alice@example.com", BeanAmount: 100}
	bob := &models.User{Username: "bob", Email: "bob@example.com", BeanAmount: 50}
	err := userRepo.Create(alice)
	assert.NoError(t, err)
	err = userRepo.Create(bob)
	assert.NoError(t, err)

	tx := &models.Transaction{
		FromUserID: alice.ID,
		ToUserID:   bob.ID,
		Amount:     30,
		Note:       "Test transaction",
	}
	err = transactionRepo.Create(db, tx)
	assert.NoError(t, err)

	export, err := exportService.ExportTransactions("alice")
	assert.NoError(t, err)
	assert.NotNil(t, export)
	assert.Equal(t, alice.ID, export.UserID)
	assert.Equal(t, "alice", export.Username)
	assert.Equal(t, "alice@example.com", export.Email)
	assert.Equal(t, 100, export.TotalBeans)
	assert.Len(t, export.Transactions, 1)
	assert.Equal(t, "alice", export.Transactions[0].FromUser)
	assert.Equal(t, "bob", export.Transactions[0].ToUser)
	assert.Equal(t, 30, export.Transactions[0].Amount)
	assert.Equal(t, "Test transaction", export.Transactions[0].Note)
	assert.NotEmpty(t, export.Signature)
}

func TestExportService_ExportUserNotFound(t *testing.T) {
	_, _, _, exportService := setupExportTestDB(t)

	_, err := exportService.ExportTransactions("nonexistent")
	assert.Equal(t, ErrUserNotFound, err)
}

func TestExportService_VerifyExport(t *testing.T) {
	db, userRepo, transactionRepo, exportService := setupExportTestDB(t)

	alice := &models.User{Username: "alice", Email: "alice@example.com", BeanAmount: 100}
	bob := &models.User{Username: "bob", Email: "bob@example.com", BeanAmount: 50}
	err := userRepo.Create(alice)
	assert.NoError(t, err)
	err = userRepo.Create(bob)
	assert.NoError(t, err)

	tx := &models.Transaction{
		FromUserID: alice.ID,
		ToUserID:   bob.ID,
		Amount:     30,
		Note:       "Test",
	}
	err = transactionRepo.Create(db, tx)
	assert.NoError(t, err)

	export, err := exportService.ExportTransactions("alice")
	assert.NoError(t, err)

	exportJSON, err := json.Marshal(export)
	assert.NoError(t, err)

	valid, err := exportService.VerifyExport(exportJSON, export.Signature)
	assert.NoError(t, err)
	assert.True(t, valid)
}

func TestExportService_VerifyExportInvalidSignature(t *testing.T) {
	db, userRepo, transactionRepo, exportService := setupExportTestDB(t)

	alice := &models.User{Username: "alice", Email: "alice@example.com", BeanAmount: 100}
	bob := &models.User{Username: "bob", Email: "bob@example.com", BeanAmount: 50}
	err := userRepo.Create(alice)
	assert.NoError(t, err)
	err = userRepo.Create(bob)
	assert.NoError(t, err)

	tx := &models.Transaction{
		FromUserID: alice.ID,
		ToUserID:   bob.ID,
		Amount:     30,
	}
	err = transactionRepo.Create(db, tx)
	assert.NoError(t, err)

	export, err := exportService.ExportTransactions("alice")
	assert.NoError(t, err)

	exportJSON, err := json.Marshal(export)
	assert.NoError(t, err)

	valid, err := exportService.VerifyExport(exportJSON, "invalid-signature-12345")
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestExportService_VerifyExportTamperedData(t *testing.T) {
	db, userRepo, transactionRepo, exportService := setupExportTestDB(t)

	alice := &models.User{Username: "alice", Email: "alice@example.com", BeanAmount: 100}
	bob := &models.User{Username: "bob", Email: "bob@example.com", BeanAmount: 50}
	err := userRepo.Create(alice)
	assert.NoError(t, err)
	err = userRepo.Create(bob)
	assert.NoError(t, err)

	tx := &models.Transaction{
		FromUserID: alice.ID,
		ToUserID:   bob.ID,
		Amount:     30,
	}
	err = transactionRepo.Create(db, tx)
	assert.NoError(t, err)

	export, err := exportService.ExportTransactions("alice")
	assert.NoError(t, err)
	originalSignature := export.Signature

	export.TotalBeans = 999999

	tamperedJSON, err := json.Marshal(export)
	assert.NoError(t, err)

	valid, err := exportService.VerifyExport(tamperedJSON, originalSignature)
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestExportService_VerifyExportInvalidJSON(t *testing.T) {
	_, _, _, exportService := setupExportTestDB(t)

	invalidJSON := []byte("{invalid json")
	_, err := exportService.VerifyExport(invalidJSON, "some-signature")
	assert.Equal(t, ErrInvalidExport, err)
}

func TestExportService_VerifyExportWithDifferentKey(t *testing.T) {
	_, userRepo, transactionRepo, _ := setupExportTestDB(t)

	alice := &models.User{Username: "alice", Email: "alice@example.com", BeanAmount: 100}
	err := userRepo.Create(alice)
	assert.NoError(t, err)

	exportService1 := NewExportService(userRepo, transactionRepo, "key1-32-characters-long-here!!")
	exportService2 := NewExportService(userRepo, transactionRepo, "key2-different-32-characters!!")

	export, err := exportService1.ExportTransactions("alice")
	assert.NoError(t, err)

	exportJSON, err := json.Marshal(export)
	assert.NoError(t, err)

	valid, err := exportService2.VerifyExport(exportJSON, export.Signature)
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestExportService_ExportMultipleTransactions(t *testing.T) {
	db, userRepo, transactionRepo, exportService := setupExportTestDB(t)

	alice := &models.User{Username: "alice", Email: "alice@example.com", BeanAmount: 100}
	bob := &models.User{Username: "bob", Email: "bob@example.com", BeanAmount: 50}
	charlie := &models.User{Username: "charlie", Email: "charlie@example.com", BeanAmount: 75}

	err := userRepo.Create(alice)
	assert.NoError(t, err)
	err = userRepo.Create(bob)
	assert.NoError(t, err)
	err = userRepo.Create(charlie)
	assert.NoError(t, err)

	tx1 := &models.Transaction{FromUserID: alice.ID, ToUserID: bob.ID, Amount: 10}
	tx2 := &models.Transaction{FromUserID: alice.ID, ToUserID: charlie.ID, Amount: 20}
	tx3 := &models.Transaction{FromUserID: bob.ID, ToUserID: alice.ID, Amount: 5}

	err = transactionRepo.Create(db, tx1)
	assert.NoError(t, err)
	err = transactionRepo.Create(db, tx2)
	assert.NoError(t, err)
	err = transactionRepo.Create(db, tx3)
	assert.NoError(t, err)

	export, err := exportService.ExportTransactions("alice")
	assert.NoError(t, err)
	assert.Len(t, export.Transactions, 3)
}
