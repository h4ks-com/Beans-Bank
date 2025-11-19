package services

import (
	"errors"
	"fmt"

	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrHarvestNotFound        = errors.New("harvest not found")
	ErrHarvestAlreadyCompleted = errors.New("harvest already completed")
	ErrNoAssignedUser         = errors.New("harvest has no assigned user")
)

type HarvestService struct {
	harvestRepo     *repository.HarvestRepository
	userRepo        *repository.UserRepository
	transactionRepo *repository.TransactionRepository
	db              *gorm.DB
}

func NewHarvestService(
	harvestRepo *repository.HarvestRepository,
	userRepo *repository.UserRepository,
	transactionRepo *repository.TransactionRepository,
	db *gorm.DB,
) *HarvestService {
	return &HarvestService{
		harvestRepo:     harvestRepo,
		userRepo:        userRepo,
		transactionRepo: transactionRepo,
		db:              db,
	}
}

func (s *HarvestService) CreateHarvest(title, description string, beanAmount int) (*models.Harvest, error) {
	harvest := &models.Harvest{
		Title:       title,
		Description: description,
		BeanAmount:  beanAmount,
		Completed:   false,
	}

	err := s.harvestRepo.Create(harvest)
	if err != nil {
		return nil, err
	}

	return harvest, nil
}

func (s *HarvestService) UpdateHarvest(id uint, title, description string, beanAmount int) (*models.Harvest, error) {
	harvest, err := s.harvestRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	harvest.Title = title
	harvest.Description = description
	harvest.BeanAmount = beanAmount

	err = s.harvestRepo.Update(harvest)
	if err != nil {
		return nil, err
	}

	return harvest, nil
}

func (s *HarvestService) AssignUser(harvestID, userID uint) (*models.Harvest, error) {
	harvest, err := s.harvestRepo.FindByID(harvestID)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	harvest.AssignedUserID = &userID

	err = s.harvestRepo.Update(harvest)
	if err != nil {
		return nil, err
	}

	return s.harvestRepo.FindByID(harvestID)
}

func (s *HarvestService) AssignUserByUsername(harvestID uint, username string) (*models.Harvest, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return s.AssignUser(harvestID, user.ID)
}

func (s *HarvestService) CompleteHarvest(harvestID uint) (*models.Harvest, error) {
	var harvest *models.Harvest

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		harvest, err = s.harvestRepo.FindByIDForUpdate(tx, harvestID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrHarvestNotFound
			}
			return err
		}

		if harvest.Completed {
			return ErrHarvestAlreadyCompleted
		}

		if harvest.AssignedUserID == nil {
			return ErrNoAssignedUser
		}

		var assignedUser models.User
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&assignedUser, harvest.AssignedUserID).Error
		if err != nil {
			return err
		}

		assignedUser.BeanAmount += harvest.BeanAmount

		err = tx.Save(&assignedUser).Error
		if err != nil {
			return err
		}

		var systemUser models.User
		err = tx.Where("username = ?", "system").First(&systemUser).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				systemUser = models.User{
					Username:   "system",
					BeanAmount: 0,
				}
				err = tx.Create(&systemUser).Error
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		transaction := &models.Transaction{
			FromUserID: systemUser.ID,
			ToUserID:   assignedUser.ID,
			Amount:     harvest.BeanAmount,
			Note:       fmt.Sprintf("Harvest completed: %s", harvest.Title),
		}

		err = s.transactionRepo.Create(tx, transaction)
		if err != nil {
			return err
		}

		harvest.Completed = true
		err = s.harvestRepo.UpdateInTx(tx, harvest)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.harvestRepo.FindByID(harvestID)
}

func (s *HarvestService) GetHarvest(id uint) (*models.Harvest, error) {
	return s.harvestRepo.FindByID(id)
}

func (s *HarvestService) GetAllHarvests() ([]models.Harvest, error) {
	return s.harvestRepo.FindAll()
}

func (s *HarvestService) SearchHarvests(query string, page, limit int) ([]models.Harvest, int64, error) {
	harvests, err := s.harvestRepo.Search(query, page, limit)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.harvestRepo.CountSearch(query)
	if err != nil {
		return nil, 0, err
	}

	return harvests, count, nil
}

func (s *HarvestService) DeleteHarvest(id uint) error {
	return s.harvestRepo.Delete(id)
}
