package services

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrGiftLinkNotFound          = errors.New("gift link not found")
	ErrGiftLinkExpired           = errors.New("gift link has expired")
	ErrGiftLinkRedeemed          = errors.New("gift link has already been redeemed")
	ErrGiftLinkInactive          = errors.New("gift link is not active")
	ErrCannotRedeemOwnLink       = errors.New("cannot redeem your own gift link")
	ErrInsufficientBalanceForGift = errors.New("insufficient balance to create gift link")
)

type GiftLinkService struct {
	giftLinkRepo    *repository.GiftLinkRepository
	userRepo        *repository.UserRepository
	transferService *TransferService
	db              *gorm.DB
}

func NewGiftLinkService(
	giftLinkRepo *repository.GiftLinkRepository,
	userRepo *repository.UserRepository,
	transferService *TransferService,
	db *gorm.DB,
) *GiftLinkService {
	return &GiftLinkService{
		giftLinkRepo:    giftLinkRepo,
		userRepo:        userRepo,
		transferService: transferService,
		db:              db,
	}
}

func (s *GiftLinkService) generateUniqueCode() (string, error) {
	for i := 0; i < 10; i++ {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}

		code := base64.URLEncoding.EncodeToString(bytes)

		existing, err := s.giftLinkRepo.FindByCode(code)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return code, nil
		}
	}
	return "", errors.New("failed to generate unique code after 10 attempts")
}

func (s *GiftLinkService) parseExpiry(expiresIn string) *time.Time {
	if expiresIn == "" || expiresIn == "never" {
		return nil
	}

	var duration time.Duration
	switch expiresIn {
	case "1h":
		duration = 1 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	default:
		return nil
	}

	expiry := time.Now().Add(duration)
	return &expiry
}

func (s *GiftLinkService) CreateGiftLink(fromUsername string, amount int, message string, expiresIn string) (*models.GiftLink, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	fromUser, err := s.userRepo.FindByUsername(fromUsername)
	if err != nil {
		return nil, err
	}
	if fromUser == nil {
		return nil, ErrUserNotFound
	}

	if fromUser.BeanAmount < amount {
		return nil, ErrInsufficientBalanceForGift
	}

	code, err := s.generateUniqueCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate gift code: %w", err)
	}

	expiry := s.parseExpiry(expiresIn)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.transferService.TransferInTx(tx, fromUsername, "system", amount, true); err != nil {
			return fmt.Errorf("failed to escrow beans: %w", err)
		}

		giftLink := &models.GiftLink{
			Code:       code,
			FromUserID: fromUser.ID,
			Amount:     amount,
			Message:    message,
			ExpiresAt:  expiry,
			Active:     true,
		}

		if err := tx.Create(giftLink).Error; err != nil {
			return fmt.Errorf("failed to create gift link: %w", err)
		}

		if err := tx.Preload("FromUser").First(giftLink, giftLink.ID).Error; err != nil {
			return fmt.Errorf("failed to reload gift link: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	reloadedGift, err := s.giftLinkRepo.FindByCode(code)
	if err != nil {
		return nil, err
	}

	return reloadedGift, nil
}

func (s *GiftLinkService) RedeemGiftLink(code string, redeemUsername string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		giftLink, err := s.giftLinkRepo.FindByCodeForUpdate(tx, code)
		if err != nil {
			return err
		}
		if giftLink == nil {
			return ErrGiftLinkNotFound
		}

		if giftLink.RedeemedAt != nil {
			return ErrGiftLinkRedeemed
		}

		if !giftLink.Active {
			return ErrGiftLinkInactive
		}

		if giftLink.ExpiresAt != nil && time.Now().After(*giftLink.ExpiresAt) {
			return ErrGiftLinkExpired
		}

		if giftLink.FromUser.Username == redeemUsername {
			return ErrCannotRedeemOwnLink
		}

		if err := s.transferService.TransferInTx(tx, "system", redeemUsername, giftLink.Amount, true); err != nil {
			return fmt.Errorf("failed to transfer beans: %w", err)
		}

		redeemUser, err := s.userRepo.FindByUsernameForUpdate(tx, redeemUsername)
		if err != nil {
			return err
		}
		if redeemUser == nil {
			return ErrUserNotFound
		}

		now := time.Now()
		giftLink.RedeemedAt = &now
		giftLink.RedeemedByID = &redeemUser.ID
		giftLink.Active = false

		if err := s.giftLinkRepo.UpdateInTx(tx, giftLink); err != nil {
			return fmt.Errorf("failed to update gift link: %w", err)
		}

		return nil
	})
}

func (s *GiftLinkService) ListGiftLinks(username string) ([]models.GiftLink, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return s.giftLinkRepo.ListByFromUserID(user.ID)
}

func (s *GiftLinkService) DeleteGiftLink(id uint, username string) error {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	giftLink, err := s.giftLinkRepo.FindByID(id)
	if err != nil {
		return err
	}
	if giftLink == nil {
		return ErrGiftLinkNotFound
	}

	if giftLink.FromUserID != user.ID {
		return errors.New("cannot delete gift link you don't own")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if giftLink.RedeemedAt == nil && giftLink.Active {
			if err := s.transferService.TransferInTx(tx, "system", username, giftLink.Amount, true); err != nil {
				return fmt.Errorf("failed to refund beans: %w", err)
			}
		}

		giftLink.Active = false
		if err := s.giftLinkRepo.UpdateInTx(tx, giftLink); err != nil {
			return fmt.Errorf("failed to deactivate gift link: %w", err)
		}

		return nil
	})
}

func (s *GiftLinkService) GetGiftLinkByCode(code string) (*models.GiftLink, error) {
	return s.giftLinkRepo.FindByCode(code)
}
