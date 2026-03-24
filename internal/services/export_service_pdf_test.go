package services

import (
	"strings"
	"testing"

	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestExportService_GenerateStatementPDF(t *testing.T) {
	db, userRepo, transactionRepo, exportService := setupExportTestDB(t)

	alice := &models.User{Username: "alice", Email: "alice@example.com", BeanAmount: 100}
	bob := &models.User{Username: "bob", Email: "bob@example.com", BeanAmount: 50}
	err := userRepo.Create(alice)
	assert.NoError(t, err)
	err = userRepo.Create(bob)
	assert.NoError(t, err)

	tx := &models.Transaction{FromUserID: alice.ID, ToUserID: bob.ID, Amount: 30, Note: "Test PDF"}
	err = transactionRepo.Create(db, tx)
	assert.NoError(t, err)

	pdfBytes, err := exportService.GenerateStatementPDF("alice")
	assert.NoError(t, err)
	assert.NotEmpty(t, pdfBytes)
	// basic check that it looks like a PDF file
	assert.True(t, strings.HasPrefix(string(pdfBytes), "%PDF"))
}
