package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/beapin/internal/middleware"
	"github.com/h4ks-com/beapin/internal/services"
)

type WalletHandler struct {
	walletService *services.WalletService
}

func NewWalletHandler(walletService *services.WalletService) *WalletHandler {
	return &WalletHandler{walletService: walletService}
}

type WalletResponse struct {
	Username   string `json:"username"`
	BeanAmount int    `json:"bean_amount"`
}

type TransactionHistoryResponse struct {
	ID         uint   `json:"id"`
	FromUser   string `json:"from_user"`
	ToUser     string `json:"to_user"`
	Amount     int    `json:"amount"`
	Timestamp  string `json:"timestamp"`
}

// GetWallet godoc
// @Summary Get wallet balance
// @Description Get the authenticated user's wallet balance
// @Tags wallet
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} WalletResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /wallet [get]
func (h *WalletHandler) GetWallet(c *gin.Context) {
	username := middleware.GetUsername(c)

	user, err := h.walletService.GetOrCreateWallet(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, WalletResponse{
		Username:   user.Username,
		BeanAmount: user.BeanAmount,
	})
}

// GetTransactions godoc
// @Summary Get transaction history
// @Description Get the authenticated user's transaction history
// @Tags wallet
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} TransactionHistoryResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /transactions [get]
func (h *WalletHandler) GetTransactions(c *gin.Context) {
	username := middleware.GetUsername(c)

	transactions, err := h.walletService.GetTransactionHistory(username)
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
