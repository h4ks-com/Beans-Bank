package main

import (
	"fmt"
	"os"

	"github.com/h4ks-com/bean-bank/internal/database"
	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"github.com/h4ks-com/bean-bank/internal/services"
)

func main() {
	db, err := database.Connect(":memory:")
	if err != nil {
		panic(err)
	}

	if err := database.Migrate(db); err != nil {
		panic(err)
	}

	userRepo := repository.NewUserRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	exportService := services.NewExportService(userRepo, txRepo, "test-signing-key-32-characters!!")

	alice := &models.User{Username: "alice", Email: "alice@example.com", BeanAmount: 1234}
	if err := userRepo.Create(alice); err != nil {
		panic(err)
	}

	bob := &models.User{Username: "bob", Email: "bob@example.com", BeanAmount: 50}
	if err := userRepo.Create(bob); err != nil {
		panic(err)
	}

	tx := &models.Transaction{FromUserID: alice.ID, ToUserID: bob.ID, Amount: 200, Note: "Dummy payment"}
	if err := txRepo.Create(db, tx); err != nil {
		panic(err)
	}

	pdfBytes, err := exportService.GenerateStatementPDF("alice")
	if err != nil {
		panic(err)
	}

	out := "sample_statement.pdf"
	f, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if _, err := f.Write(pdfBytes); err != nil {
		panic(err)
	}

	fmt.Printf("Wrote %s (%d bytes)\n", out, len(pdfBytes))
}
