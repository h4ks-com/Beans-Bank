package services

import (
	"testing"
	"time"

	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupGiftLinkTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.User{}, &models.Transaction{}, &models.GiftLink{})
	require.NoError(t, err)

	systemUser := &models.User{Username: "system", BeanAmount: 1000000}
	require.NoError(t, db.Create(systemUser).Error)

	return db
}

func TestGiftLinkService_CreateGiftLink(t *testing.T) {
	db := setupGiftLinkTestDB(t)
	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	giftLinkRepo := repository.NewGiftLinkRepository(db)
	transferService := NewTransferService(userRepo, transactionRepo, db)
	service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

	sender := &models.User{Username: "alice", BeanAmount: 500}
	require.NoError(t, db.Create(sender).Error)

	t.Run("creates gift link successfully", func(t *testing.T) {
		giftLink, err := service.CreateGiftLink("alice", 100, "Happy Birthday!", "24h")
		require.NoError(t, err)
		assert.NotEmpty(t, giftLink.Code)
		assert.Equal(t, sender.ID, giftLink.FromUserID)
		assert.Equal(t, 100, giftLink.Amount)
		assert.Equal(t, "Happy Birthday!", giftLink.Message)
		assert.True(t, giftLink.Active)
		assert.NotNil(t, giftLink.ExpiresAt)
		assert.Nil(t, giftLink.RedeemedAt)

		var updatedSender models.User
		require.NoError(t, db.First(&updatedSender, sender.ID).Error)
		assert.Equal(t, 400, updatedSender.BeanAmount)

		var systemUser models.User
		require.NoError(t, db.Where("username = ?", "system").First(&systemUser).Error)
		assert.Equal(t, 1000100, systemUser.BeanAmount)
	})

	t.Run("creates gift link without expiry", func(t *testing.T) {
		giftLink, err := service.CreateGiftLink("alice", 50, "No expiry", "never")
		require.NoError(t, err)
		assert.Nil(t, giftLink.ExpiresAt)
	})

	t.Run("fails with insufficient balance", func(t *testing.T) {
		_, err := service.CreateGiftLink("alice", 10000, "Too much", "24h")
		assert.ErrorIs(t, err, ErrInsufficientBalanceForGift)
	})

	t.Run("fails with invalid amount", func(t *testing.T) {
		_, err := service.CreateGiftLink("alice", 0, "Zero beans", "24h")
		assert.ErrorIs(t, err, ErrInvalidAmount)

		_, err = service.CreateGiftLink("alice", -10, "Negative beans", "24h")
		assert.ErrorIs(t, err, ErrInvalidAmount)
	})

	t.Run("fails with non-existent user", func(t *testing.T) {
		_, err := service.CreateGiftLink("nonexistent", 100, "Ghost", "24h")
		assert.Error(t, err)
	})
}

func TestGiftLinkService_RedeemGiftLink(t *testing.T) {
	t.Run("redeems gift link successfully", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 500}
		require.NoError(t, db.Create(sender).Error)

		recipient := &models.User{Username: "bob", BeanAmount: 100}
		require.NoError(t, db.Create(recipient).Error)

		giftLink, err := service.CreateGiftLink("alice", 100, "For you", "24h")
		require.NoError(t, err)

		err = service.RedeemGiftLink(giftLink.Code, "bob")
		require.NoError(t, err)

		var updatedRecipient models.User
		require.NoError(t, db.First(&updatedRecipient, recipient.ID).Error)
		assert.Equal(t, 200, updatedRecipient.BeanAmount)

		var updatedGiftLink models.GiftLink
		require.NoError(t, db.First(&updatedGiftLink, giftLink.ID).Error)
		assert.False(t, updatedGiftLink.Active)
		assert.NotNil(t, updatedGiftLink.RedeemedAt)
		assert.Equal(t, recipient.ID, *updatedGiftLink.RedeemedByID)
	})

	t.Run("auto-creates recipient if not exists", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 500}
		require.NoError(t, db.Create(sender).Error)

		giftLink, err := service.CreateGiftLink("alice", 50, "New user", "24h")
		require.NoError(t, err)

		err = service.RedeemGiftLink(giftLink.Code, "charlie")
		require.NoError(t, err)

		var newUser models.User
		require.NoError(t, db.Where("username = ?", "charlie").First(&newUser).Error)
		assert.Equal(t, 50, newUser.BeanAmount)
	})

	t.Run("fails with non-existent gift link", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		err := service.RedeemGiftLink("nonexistent", "bob")
		assert.ErrorIs(t, err, ErrGiftLinkNotFound)
	})

	t.Run("fails when already redeemed", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 500}
		require.NoError(t, db.Create(sender).Error)

		recipient := &models.User{Username: "bob", BeanAmount: 100}
		require.NoError(t, db.Create(recipient).Error)

		giftLink, err := service.CreateGiftLink("alice", 100, "Once only", "24h")
		require.NoError(t, err)

		err = service.RedeemGiftLink(giftLink.Code, "bob")
		require.NoError(t, err)

		err = service.RedeemGiftLink(giftLink.Code, "bob")
		assert.ErrorIs(t, err, ErrGiftLinkRedeemed)
	})

	t.Run("fails when expired", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 500}
		require.NoError(t, db.Create(sender).Error)

		giftLink, err := service.CreateGiftLink("alice", 100, "Expired", "1h")
		require.NoError(t, err)

		pastTime := time.Now().Add(-2 * time.Hour)
		require.NoError(t, db.Model(&giftLink).Update("expires_at", pastTime).Error)

		err = service.RedeemGiftLink(giftLink.Code, "bob")
		assert.ErrorIs(t, err, ErrGiftLinkExpired)
	})

	t.Run("fails when inactive", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 500}
		require.NoError(t, db.Create(sender).Error)

		giftLink, err := service.CreateGiftLink("alice", 100, "Inactive", "24h")
		require.NoError(t, err)

		require.NoError(t, db.Model(&giftLink).Update("active", false).Error)

		err = service.RedeemGiftLink(giftLink.Code, "bob")
		assert.ErrorIs(t, err, ErrGiftLinkInactive)
	})

	t.Run("fails when redeeming own link", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 500}
		require.NoError(t, db.Create(sender).Error)

		giftLink, err := service.CreateGiftLink("alice", 100, "Self gift", "24h")
		require.NoError(t, err)

		err = service.RedeemGiftLink(giftLink.Code, "alice")
		assert.ErrorIs(t, err, ErrCannotRedeemOwnLink)
	})
}

func TestGiftLinkService_ListGiftLinks(t *testing.T) {
	t.Run("lists user's gift links", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 1000}
		require.NoError(t, db.Create(sender).Error)

		_, err := service.CreateGiftLink("alice", 100, "Gift 1", "24h")
		require.NoError(t, err)
		_, err = service.CreateGiftLink("alice", 200, "Gift 2", "never")
		require.NoError(t, err)

		links, err := service.ListGiftLinks("alice")
		require.NoError(t, err)
		assert.Len(t, links, 2)
	})

	t.Run("only lists active gift links", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 1000}
		require.NoError(t, db.Create(sender).Error)

		giftLink, err := service.CreateGiftLink("alice", 50, "Will be inactive", "24h")
		require.NoError(t, err)

		require.NoError(t, db.Model(&giftLink).Update("active", false).Error)

		links, err := service.ListGiftLinks("alice")
		require.NoError(t, err)

		for _, link := range links {
			assert.True(t, link.Active)
		}
	})

	t.Run("returns empty list for user with no gifts", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		otherUser := &models.User{Username: "bob", BeanAmount: 100}
		require.NoError(t, db.Create(otherUser).Error)

		links, err := service.ListGiftLinks("bob")
		require.NoError(t, err)
		assert.Empty(t, links)
	})

	t.Run("fails with non-existent user", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		_, err := service.ListGiftLinks("nonexistent")
		assert.Error(t, err)
	})
}

func TestGiftLinkService_DeleteGiftLink(t *testing.T) {
	t.Run("deletes unredeemed gift link and refunds beans", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 1000}
		require.NoError(t, db.Create(sender).Error)

		giftLink, err := service.CreateGiftLink("alice", 100, "Delete me", "24h")
		require.NoError(t, err)

		var beforeBalance models.User
		require.NoError(t, db.First(&beforeBalance, sender.ID).Error)

		err = service.DeleteGiftLink(giftLink.ID, "alice")
		require.NoError(t, err)

		var updatedGiftLink models.GiftLink
		require.NoError(t, db.First(&updatedGiftLink, giftLink.ID).Error)
		assert.False(t, updatedGiftLink.Active)

		var afterBalance models.User
		require.NoError(t, db.First(&afterBalance, sender.ID).Error)
		assert.Equal(t, beforeBalance.BeanAmount+100, afterBalance.BeanAmount)
	})

	t.Run("deletes redeemed gift link without refund", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 1000}
		require.NoError(t, db.Create(sender).Error)

		recipient := &models.User{Username: "bob", BeanAmount: 0}
		require.NoError(t, db.Create(recipient).Error)

		giftLink, err := service.CreateGiftLink("alice", 100, "Already redeemed", "24h")
		require.NoError(t, err)

		err = service.RedeemGiftLink(giftLink.Code, "bob")
		require.NoError(t, err)

		var beforeBalance models.User
		require.NoError(t, db.First(&beforeBalance, sender.ID).Error)

		err = service.DeleteGiftLink(giftLink.ID, "alice")
		require.NoError(t, err)

		var afterBalance models.User
		require.NoError(t, db.First(&afterBalance, sender.ID).Error)
		assert.Equal(t, beforeBalance.BeanAmount, afterBalance.BeanAmount)
	})

	t.Run("fails when deleting another user's gift link", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 1000}
		require.NoError(t, db.Create(sender).Error)

		otherUser := &models.User{Username: "charlie", BeanAmount: 100}
		require.NoError(t, db.Create(otherUser).Error)

		giftLink, err := service.CreateGiftLink("alice", 50, "Not yours", "24h")
		require.NoError(t, err)

		err = service.DeleteGiftLink(giftLink.ID, "charlie")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot delete gift link you don't own")
	})

	t.Run("fails with non-existent gift link", func(t *testing.T) {
		db := setupGiftLinkTestDB(t)
		userRepo := repository.NewUserRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		giftLinkRepo := repository.NewGiftLinkRepository(db)
		transferService := NewTransferService(userRepo, transactionRepo, db)
		service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

		sender := &models.User{Username: "alice", BeanAmount: 1000}
		require.NoError(t, db.Create(sender).Error)

		err := service.DeleteGiftLink(99999, "alice")
		assert.ErrorIs(t, err, ErrGiftLinkNotFound)
	})
}
func TestGiftLinkService_GetGiftLinkByCode(t *testing.T) {
	db := setupGiftLinkTestDB(t)
	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	giftLinkRepo := repository.NewGiftLinkRepository(db)
	transferService := NewTransferService(userRepo, transactionRepo, db)
	service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

	sender := &models.User{Username: "alice", BeanAmount: 500}
	require.NoError(t, db.Create(sender).Error)

	t.Run("retrieves gift link by code", func(t *testing.T) {
		giftLink, err := service.CreateGiftLink("alice", 100, "Find me", "24h")
		require.NoError(t, err)

		found, err := service.GetGiftLinkByCode(giftLink.Code)
		require.NoError(t, err)
		assert.Equal(t, giftLink.Code, found.Code)
		assert.Equal(t, 100, found.Amount)
		assert.Equal(t, "Find me", found.Message)
	})

	t.Run("returns nil for non-existent code", func(t *testing.T) {
		found, err := service.GetGiftLinkByCode("nonexistent")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestGiftLinkService_ExpiryParsing(t *testing.T) {
	db := setupGiftLinkTestDB(t)
	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	giftLinkRepo := repository.NewGiftLinkRepository(db)
	transferService := NewTransferService(userRepo, transactionRepo, db)
	service := NewGiftLinkService(giftLinkRepo, userRepo, transferService, db)

	sender := &models.User{Username: "alice", BeanAmount: 1000}
	require.NoError(t, db.Create(sender).Error)

	testCases := []struct {
		name           string
		expiresIn      string
		expectNil      bool
		expectedOffset time.Duration
	}{
		{"1 hour", "1h", false, 1 * time.Hour},
		{"24 hours", "24h", false, 24 * time.Hour},
		{"7 days", "7d", false, 7 * 24 * time.Hour},
		{"30 days", "30d", false, 30 * 24 * time.Hour},
		{"never", "never", true, 0},
		{"empty string", "", true, 0},
		{"invalid", "invalid", true, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			giftLink, err := service.CreateGiftLink("alice", 10, "Expiry test", tc.expiresIn)
			require.NoError(t, err)

			if tc.expectNil {
				assert.Nil(t, giftLink.ExpiresAt)
			} else {
				require.NotNil(t, giftLink.ExpiresAt)
				diff := giftLink.ExpiresAt.Sub(time.Now())
				assert.InDelta(t, tc.expectedOffset.Seconds(), diff.Seconds(), 5)
			}
		})
	}
}
