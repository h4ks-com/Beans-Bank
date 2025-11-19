package services

import (
	"testing"

	"github.com/h4ks-com/bean-bank/internal/database"
	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"github.com/stretchr/testify/assert"
)

func setupHarvestTestDB(t *testing.T) (*repository.HarvestRepository, *repository.UserRepository, *repository.TransactionRepository, *HarvestService) {
	db, err := database.Connect(":memory:")
	assert.NoError(t, err)

	err = database.Migrate(db)
	assert.NoError(t, err)

	harvestRepo := repository.NewHarvestRepository(db)
	userRepo := repository.NewUserRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	harvestService := NewHarvestService(harvestRepo, userRepo, transactionRepo, db)

	return harvestRepo, userRepo, transactionRepo, harvestService
}

func TestHarvestService_CreateHarvest(t *testing.T) {
	_, _, _, harvestService := setupHarvestTestDB(t)

	harvest, err := harvestService.CreateHarvest("Test Harvest", "This is a test harvest description", 50)
	assert.NoError(t, err)
	assert.NotNil(t, harvest)
	assert.Equal(t, "Test Harvest", harvest.Title)
	assert.Equal(t, "This is a test harvest description", harvest.Description)
	assert.Equal(t, 50, harvest.BeanAmount)
	assert.False(t, harvest.Completed)
	assert.Nil(t, harvest.AssignedUserID)
}

func TestHarvestService_UpdateHarvest(t *testing.T) {
	_, _, _, harvestService := setupHarvestTestDB(t)

	harvest, err := harvestService.CreateHarvest("Original", "Original description", 100)
	assert.NoError(t, err)

	updated, err := harvestService.UpdateHarvest(harvest.ID, "Updated", "Updated description", 150)
	assert.NoError(t, err)
	assert.Equal(t, "Updated", updated.Title)
	assert.Equal(t, "Updated description", updated.Description)
	assert.Equal(t, 150, updated.BeanAmount)
}

func TestHarvestService_AssignUser(t *testing.T) {
	_, userRepo, _, harvestService := setupHarvestTestDB(t)

	user := &models.User{Username: "testuser", BeanAmount: 0}
	err := userRepo.Create(user)
	assert.NoError(t, err)

	harvest, err := harvestService.CreateHarvest("Test", "Description", 50)
	assert.NoError(t, err)

	assigned, err := harvestService.AssignUser(harvest.ID, user.ID)
	assert.NoError(t, err)
	assert.NotNil(t, assigned.AssignedUserID)
	assert.Equal(t, user.ID, *assigned.AssignedUserID)
	assert.NotNil(t, assigned.AssignedUser)
	assert.Equal(t, "testuser", assigned.AssignedUser.Username)
}

func TestHarvestService_CompleteHarvest(t *testing.T) {
	_, userRepo, transactionRepo, harvestService := setupHarvestTestDB(t)

	user := &models.User{Username: "testuser", BeanAmount: 100}
	err := userRepo.Create(user)
	assert.NoError(t, err)

	harvest, err := harvestService.CreateHarvest("Complete Test", "Test completion", 50)
	assert.NoError(t, err)

	_, err = harvestService.AssignUser(harvest.ID, user.ID)
	assert.NoError(t, err)

	completed, err := harvestService.CompleteHarvest(harvest.ID)
	assert.NoError(t, err)
	assert.True(t, completed.Completed)

	userAfter, err := userRepo.FindByUsername("testuser")
	assert.NoError(t, err)
	assert.Equal(t, 150, userAfter.BeanAmount)

	transactions, err := transactionRepo.FindByUsername("testuser")
	assert.NoError(t, err)
	assert.Len(t, transactions, 1)
	assert.Equal(t, 50, transactions[0].Amount)
	assert.Equal(t, "Harvest completed: Complete Test", transactions[0].Note)
	assert.Equal(t, "system", transactions[0].FromUser.Username)
}

func TestHarvestService_CompleteUnassignedHarvest(t *testing.T) {
	_, _, _, harvestService := setupHarvestTestDB(t)

	harvest, err := harvestService.CreateHarvest("Unassigned", "No user assigned", 50)
	assert.NoError(t, err)

	_, err = harvestService.CompleteHarvest(harvest.ID)
	assert.Equal(t, ErrNoAssignedUser, err)
}

func TestHarvestService_CompleteAlreadyCompleted(t *testing.T) {
	_, userRepo, _, harvestService := setupHarvestTestDB(t)

	user := &models.User{Username: "testuser", BeanAmount: 100}
	err := userRepo.Create(user)
	assert.NoError(t, err)

	harvest, err := harvestService.CreateHarvest("Double Complete", "Test", 50)
	assert.NoError(t, err)

	_, err = harvestService.AssignUser(harvest.ID, user.ID)
	assert.NoError(t, err)

	_, err = harvestService.CompleteHarvest(harvest.ID)
	assert.NoError(t, err)

	_, err = harvestService.CompleteHarvest(harvest.ID)
	assert.Equal(t, ErrHarvestAlreadyCompleted, err)
}

func TestHarvestService_GetHarvest(t *testing.T) {
	_, _, _, harvestService := setupHarvestTestDB(t)

	created, err := harvestService.CreateHarvest("Get Test", "Description", 75)
	assert.NoError(t, err)

	retrieved, err := harvestService.GetHarvest(created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, "Get Test", retrieved.Title)
}

func TestHarvestService_GetAllHarvests(t *testing.T) {
	_, _, _, harvestService := setupHarvestTestDB(t)

	_, err := harvestService.CreateHarvest("Harvest 1", "Desc 1", 10)
	assert.NoError(t, err)
	_, err = harvestService.CreateHarvest("Harvest 2", "Desc 2", 20)
	assert.NoError(t, err)
	_, err = harvestService.CreateHarvest("Harvest 3", "Desc 3", 30)
	assert.NoError(t, err)

	harvests, err := harvestService.GetAllHarvests()
	assert.NoError(t, err)
	assert.Len(t, harvests, 3)
}

func TestHarvestService_SearchHarvests(t *testing.T) {
	_, _, _, harvestService := setupHarvestTestDB(t)

	_, err := harvestService.CreateHarvest("Find Me", "Description with keyword", 10)
	assert.NoError(t, err)
	_, err = harvestService.CreateHarvest("Other Task", "keyword in description", 20)
	assert.NoError(t, err)
	_, err = harvestService.CreateHarvest("Unrelated", "Nothing here", 30)
	assert.NoError(t, err)

	harvests, total, err := harvestService.SearchHarvests("keyword", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, harvests, 2)
	assert.Equal(t, int64(2), total)

	harvests, total, err = harvestService.SearchHarvests("Find", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, harvests, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "Find Me", harvests[0].Title)
}

func TestHarvestService_SearchHarvestsPagination(t *testing.T) {
	_, _, _, harvestService := setupHarvestTestDB(t)

	for i := 1; i <= 25; i++ {
		_, err := harvestService.CreateHarvest("Harvest", "Description", 10)
		assert.NoError(t, err)
	}

	harvests, total, err := harvestService.SearchHarvests("", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, harvests, 10)
	assert.Equal(t, int64(25), total)

	harvests, total, err = harvestService.SearchHarvests("", 2, 10)
	assert.NoError(t, err)
	assert.Len(t, harvests, 10)
	assert.Equal(t, int64(25), total)

	harvests, total, err = harvestService.SearchHarvests("", 3, 10)
	assert.NoError(t, err)
	assert.Len(t, harvests, 5)
	assert.Equal(t, int64(25), total)
}

func TestHarvestService_DeleteHarvest(t *testing.T) {
	harvestRepo, _, _, harvestService := setupHarvestTestDB(t)

	harvest, err := harvestService.CreateHarvest("Delete Me", "Will be deleted", 50)
	assert.NoError(t, err)

	err = harvestService.DeleteHarvest(harvest.ID)
	assert.NoError(t, err)

	_, err = harvestRepo.FindByID(harvest.ID)
	assert.Error(t, err)
}
