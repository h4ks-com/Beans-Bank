package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/beapin/internal/auth"
	"github.com/h4ks-com/beapin/internal/services"
)

type BrowserHandler struct {
	walletService   *services.WalletService
	transferService *services.TransferService
	tokenService    *services.TokenService
	logtoHandler    *auth.LogtoHandler
}

func NewBrowserHandler(
	walletService *services.WalletService,
	transferService *services.TransferService,
	tokenService *services.TokenService,
	logtoHandler *auth.LogtoHandler,
) *BrowserHandler {
	return &BrowserHandler{
		walletService:   walletService,
		transferService: transferService,
		tokenService:    tokenService,
		logtoHandler:    logtoHandler,
	}
}

func (h *BrowserHandler) GetWallet(c *gin.Context) {
	username, ok := h.logtoHandler.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "not authenticated"})
		return
	}

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

func (h *BrowserHandler) GetTransactions(c *gin.Context) {
	username, ok := h.logtoHandler.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "not authenticated"})
		return
	}

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

func (h *BrowserHandler) Transfer(c *gin.Context) {
	username, ok := h.logtoHandler.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "not authenticated"})
		return
	}

	var req struct {
		ToUser string `json:"to_user" binding:"required"`
		Amount int    `json:"amount" binding:"required"`
		Force  bool   `json:"force"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "amount must be positive"})
		return
	}

	if err := h.transferService.Transfer(username, req.ToUser, req.Amount, req.Force); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "transfer successful",
		"to_user": req.ToUser,
		"amount":  req.Amount,
	})
}

func (h *BrowserHandler) CreateToken(c *gin.Context) {
	username, ok := h.logtoHandler.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "not authenticated"})
		return
	}

	var req CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	var duration time.Duration
	var err error

	if req.ExpiresIn == "never" {
		duration = 87600 * time.Hour
	} else {
		duration, err = time.ParseDuration(req.ExpiresIn)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid expires_in format"})
			return
		}
	}

	if duration <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "expires_in must be positive"})
		return
	}

	token, err := h.tokenService.GenerateToken(username, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, CreateTokenResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(duration).Format(time.RFC3339),
	})
}

func (h *BrowserHandler) ListTokens(c *gin.Context) {
	username, ok := h.logtoHandler.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "not authenticated"})
		return
	}

	tokens, err := h.tokenService.ListUserTokens(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := struct {
		Tokens []TokenListResponse `json:"tokens"`
	}{
		Tokens: make([]TokenListResponse, len(tokens)),
	}

	for i, token := range tokens {
		response.Tokens[i] = TokenListResponse{
			ID:        token.ID,
			ExpiresAt: token.ExpiresAt.Format(time.RFC3339),
			CreatedAt: token.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *BrowserHandler) DeleteToken(c *gin.Context) {
	username, ok := h.logtoHandler.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "not authenticated"})
		return
	}

	var idParam struct {
		ID uint `uri:"id" binding:"required"`
	}

	if err := c.ShouldBindUri(&idParam); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid token ID"})
		return
	}

	err := h.tokenService.DeleteToken(idParam.ID, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "token deleted successfully"})
}
