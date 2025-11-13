package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/bean-bank/internal/repository"
	"github.com/h4ks-com/bean-bank/internal/services"
)

type AdminHandler struct {
	userRepo        *repository.UserRepository
	transactionRepo *repository.TransactionRepository
	walletService   *services.WalletService
}

func NewAdminHandler(userRepo *repository.UserRepository, transactionRepo *repository.TransactionRepository, walletService *services.WalletService) *AdminHandler {
	return &AdminHandler{
		userRepo:        userRepo,
		transactionRepo: transactionRepo,
		walletService:   walletService,
	}
}

type UserListResponse struct {
	Username   string `json:"username"`
	BeanAmount int    `json:"bean_amount"`
	CreatedAt  string `json:"created_at"`
}

type UpdateWalletRequest struct {
	BeanAmount int `json:"bean_amount" binding:"required,gte=0"`
}

// ListUsers godoc
// @Summary List all users (Admin)
// @Description Get a list of all users and their wallets
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} UserListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.userRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]UserListResponse, len(users))
	for i, user := range users {
		response[i] = UserListResponse{
			Username:   user.Username,
			BeanAmount: user.BeanAmount,
			CreatedAt:  user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, response)
}

// ListAllTransactions godoc
// @Summary List all transactions (Admin)
// @Description Get a list of all transactions in the system
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} TransactionHistoryResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/transactions [get]
func (h *AdminHandler) ListAllTransactions(c *gin.Context) {
	transactions, err := h.transactionRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]TransactionHistoryResponse, len(transactions))
	for i, tx := range transactions {
		response[i] = TransactionHistoryResponse{
			ID:        tx.ID,
			FromUser:  tx.FromUser.Username,
			ToUser:    tx.ToUser.Username,
			Amount:    tx.Amount,
			Timestamp: tx.Timestamp.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, response)
}

// UpdateWallet godoc
// @Summary Update wallet balance (Admin)
// @Description Update a user's wallet balance directly
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Param request body UpdateWalletRequest true "New balance"
// @Success 200 {object} WalletResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/wallet/{username} [put]
func (h *AdminHandler) UpdateWallet(c *gin.Context) {
	username := c.Param("username")

	var req UpdateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	err := h.walletService.UpdateBalance(username, req.BeanAmount)
	if err != nil {
		if err == services.ErrUserNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, WalletResponse{
		Username:   username,
		BeanAmount: req.BeanAmount,
	})
}
