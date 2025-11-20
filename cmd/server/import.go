package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/h4ks-com/bean-bank/internal/config"
	"github.com/h4ks-com/bean-bank/internal/database"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"github.com/h4ks-com/bean-bank/internal/services"
	"github.com/spf13/cobra"
)

type WalletImport struct {
	Nick  string `json:"nick"`
	Beans int    `json:"beans"`
}

var (
	importFile       string
	skipZero         bool
	skipInvalid      bool
	strictMode       bool
	usernameRegex    = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import wallets from JSON file",
	Long: `Import wallet balances from a JSON file.

Expected JSON format:
[
  {"nick": "username", "beans": 123},
  {"nick": "valware", "beans": 1894}
]

By default, the import will skip zero balance wallets and invalid usernames.
Use --strict to fail on any validation error instead.`,
	Example: `  beapin import -f wallets.json
  beapin import --file wallets.json --no-skip-zero
  beapin import -f wallets.json --strict`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runImport(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	importCmd.Flags().StringVarP(&importFile, "file", "f", "", "JSON file to import (required)")
	importCmd.Flags().BoolVar(&skipZero, "skip-zero", true, "Skip wallets with zero balance")
	importCmd.Flags().BoolVar(&skipInvalid, "skip-invalid", true, "Skip invalid usernames")
	importCmd.Flags().BoolVar(&strictMode, "strict", false, "Fail on any validation error")
	importCmd.MarkFlagRequired("file")
}

func runImport() error {
	if importFile == "" {
		return fmt.Errorf("file path is required")
	}

	data, err := os.ReadFile(importFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var wallets []WalletImport
	if err := json.Unmarshal(data, &wallets); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	walletService := services.NewWalletService(userRepo, transactionRepo)

	log.Printf("Starting import of %d wallets from %s", len(wallets), importFile)

	imported := 0
	skipped := 0
	failed := 0

	for _, w := range wallets {
		if err := validateAndImportWallet(w, walletService); err != nil {
			if strictMode {
				return fmt.Errorf("import failed for %s: %w", w.Nick, err)
			}
			log.Printf("Skipped %s: %v", w.Nick, err)
			skipped++
			continue
		}
		imported++
	}

	log.Printf("\nImport complete:")
	log.Printf("  ✅ Imported: %d", imported)
	log.Printf("  ⏭️  Skipped: %d", skipped)
	if failed > 0 {
		log.Printf("  ❌ Failed: %d", failed)
	}

	return nil
}

func validateAndImportWallet(w WalletImport, walletService *services.WalletService) error {
	if skipZero && w.Beans == 0 {
		return fmt.Errorf("zero balance")
	}

	if w.Beans < 0 {
		return fmt.Errorf("negative balance not allowed")
	}

	if skipInvalid && !usernameRegex.MatchString(w.Nick) {
		return fmt.Errorf("invalid username format")
	}

	if w.Nick == "" {
		return fmt.Errorf("empty username")
	}

	user, err := walletService.GetOrCreateWallet(w.Nick)
	if err != nil {
		return fmt.Errorf("failed to get or create wallet: %w", err)
	}

	if user.BeanAmount > 0 && user.BeanAmount != w.Beans {
		log.Printf("User %s already exists with balance %d, updating to %d", w.Nick, user.BeanAmount, w.Beans)
	}

	if err := walletService.UpdateBalance(w.Nick, w.Beans); err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	log.Printf("✅ Imported %s with 🫘%d", w.Nick, w.Beans)
	return nil
}
