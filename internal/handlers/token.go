package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/bean-bank/internal/middleware"
	"github.com/h4ks-com/bean-bank/internal/services"
)

type TokenHandler struct {
	tokenService *services.TokenService
}

func NewTokenHandler(tokenService *services.TokenService) *TokenHandler {
	return &TokenHandler{tokenService: tokenService}
}

type CreateTokenRequest struct {
	ExpiresIn string `json:"expires_in" binding:"required"`
}

type CreateTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type TokenListResponse struct {
	ID        uint   `json:"id"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

// CreateToken godoc
// @Summary Create API token
// @Description Create a new API token with specified expiration
// @Tags tokens
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateTokenRequest true "Token expiration (e.g., 24h, 7d, 30d)"
// @Success 201 {object} CreateTokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tokens [post]
func (h *TokenHandler) CreateToken(c *gin.Context) {
	username := middleware.GetUsername(c)

	var req CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	duration, err := time.ParseDuration(req.ExpiresIn)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid expires_in format, use duration like 24h, 7d, 30d"})
		return
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

// ListTokens godoc
// @Summary List API tokens
// @Description List all API tokens for authenticated user
// @Tags tokens
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} TokenListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tokens [get]
func (h *TokenHandler) ListTokens(c *gin.Context) {
	username := middleware.GetUsername(c)

	tokens, err := h.tokenService.ListUserTokens(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]TokenListResponse, len(tokens))
	for i, token := range tokens {
		response[i] = TokenListResponse{
			ID:        token.ID,
			ExpiresAt: token.ExpiresAt.Format(time.RFC3339),
			CreatedAt: token.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, response)
}

// DeleteToken godoc
// @Summary Delete API token
// @Description Delete an API token by ID
// @Tags tokens
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Token ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tokens/{id} [delete]
func (h *TokenHandler) DeleteToken(c *gin.Context) {
	username := middleware.GetUsername(c)

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
