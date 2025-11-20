package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/bean-bank/internal/middleware"
	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/services"
)

type GiftLinkHandler struct {
	giftLinkService *services.GiftLinkService
}

func NewGiftLinkHandler(giftLinkService *services.GiftLinkService) *GiftLinkHandler {
	return &GiftLinkHandler{giftLinkService: giftLinkService}
}

type CreateGiftLinkRequest struct {
	Amount    int    `json:"amount" binding:"required,gt=0"`
	Message   string `json:"message"`
	ExpiresIn string `json:"expires_in"`
}

type GiftLinkResponse struct {
	ID           uint   `json:"id"`
	Code         string `json:"code"`
	Amount       int    `json:"amount"`
	Message      string `json:"message"`
	ExpiresAt    *int64 `json:"expires_at,omitempty"`
	RedeemedAt   *int64 `json:"redeemed_at,omitempty"`
	RedeemedBy   string `json:"redeemed_by,omitempty"`
	FromUsername string `json:"from_username"`
	Active       bool   `json:"active"`
	CreatedAt    int64  `json:"created_at"`
}

// CreateGiftLink godoc
// @Summary Create a gift link
// @Description Create a shareable gift link that escrows beans until redeemed
// @Tags giftlinks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateGiftLinkRequest true "Gift link creation request"
// @Success 200 {object} GiftLinkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /giftlinks [post]
func (h *GiftLinkHandler) CreateGiftLink(c *gin.Context) {
	username := middleware.GetUsername(c)
	if username == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req CreateGiftLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	giftLink, err := h.giftLinkService.CreateGiftLink(username, req.Amount, req.Message, req.ExpiresIn)
	if err != nil {
		switch err {
		case services.ErrInvalidAmount:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid amount"})
		case services.ErrUserNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
		case services.ErrInsufficientBalanceForGift:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "insufficient balance"})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return
	}

	response := mapGiftLinkToResponse(giftLink)
	c.JSON(http.StatusOK, response)
}

// ListGiftLinks godoc
// @Summary List user's gift links
// @Description Get all active gift links created by the authenticated user
// @Tags giftlinks
// @Produce json
// @Security BearerAuth
// @Success 200 {array} GiftLinkResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /giftlinks [get]
func (h *GiftLinkHandler) ListGiftLinks(c *gin.Context) {
	username := middleware.GetUsername(c)
	if username == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	giftLinks, err := h.giftLinkService.ListGiftLinks(username)
	if err != nil {
		if err == services.ErrUserNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	response := make([]GiftLinkResponse, len(giftLinks))
	for i, gl := range giftLinks {
		response[i] = mapGiftLinkToResponse(&gl)
	}

	c.JSON(http.StatusOK, response)
}

// DeleteGiftLink godoc
// @Summary Delete a gift link
// @Description Delete a gift link and refund beans if not redeemed
// @Tags giftlinks
// @Produce json
// @Security BearerAuth
// @Param id path int true "Gift Link ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /giftlinks/{id} [delete]
func (h *GiftLinkHandler) DeleteGiftLink(c *gin.Context) {
	username := middleware.GetUsername(c)
	if username == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid gift link id"})
		return
	}

	err = h.giftLinkService.DeleteGiftLink(uint(id), username)
	if err != nil {
		switch err {
		case services.ErrGiftLinkNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "gift link not found"})
		case services.ErrUserNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "gift link deleted successfully"})
}

// GetGiftLinkInfo godoc
// @Summary Get gift link information
// @Description Get details about a gift link by code (public endpoint)
// @Tags giftlinks
// @Produce json
// @Param code path string true "Gift Link Code"
// @Success 200 {object} GiftLinkResponse
// @Failure 404 {object} ErrorResponse
// @Router /gift/{code} [get]
func (h *GiftLinkHandler) GetGiftLinkInfo(c *gin.Context) {
	code := c.Param("code")

	giftLink, err := h.giftLinkService.GetGiftLinkByCode(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if giftLink == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "gift link not found"})
		return
	}

	response := mapGiftLinkToResponse(giftLink)
	c.JSON(http.StatusOK, response)
}

type RedeemGiftLinkRequest struct {
	Code string `json:"code" binding:"required"`
}

// RedeemGiftLink godoc
// @Summary Redeem a gift link
// @Description Redeem a gift link and transfer beans to authenticated user
// @Tags giftlinks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body RedeemGiftLinkRequest true "Redeem request"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /gift/redeem [post]
func (h *GiftLinkHandler) RedeemGiftLink(c *gin.Context) {
	username := middleware.GetUsername(c)
	if username == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req RedeemGiftLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	err := h.giftLinkService.RedeemGiftLink(req.Code, username)
	if err != nil {
		switch err {
		case services.ErrGiftLinkNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "gift link not found"})
		case services.ErrGiftLinkExpired:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "gift link has expired"})
		case services.ErrGiftLinkRedeemed:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "gift link already redeemed"})
		case services.ErrGiftLinkInactive:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "gift link is inactive"})
		case services.ErrCannotRedeemOwnLink:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "cannot redeem your own gift link"})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "gift link redeemed successfully"})
}

func mapGiftLinkToResponse(gl *models.GiftLink) GiftLinkResponse {
	response := GiftLinkResponse{
		ID:           gl.ID,
		Code:         gl.Code,
		Amount:       gl.Amount,
		Message:      gl.Message,
		FromUsername: gl.FromUser.Username,
		Active:       gl.Active,
		CreatedAt:    gl.CreatedAt.Unix(),
	}

	if gl.ExpiresAt != nil {
		expiresAt := gl.ExpiresAt.Unix()
		response.ExpiresAt = &expiresAt
	}

	if gl.RedeemedAt != nil {
		redeemedAt := gl.RedeemedAt.Unix()
		response.RedeemedAt = &redeemedAt
	}

	if gl.RedeemedBy != nil {
		response.RedeemedBy = gl.RedeemedBy.Username
	}

	return response
}
