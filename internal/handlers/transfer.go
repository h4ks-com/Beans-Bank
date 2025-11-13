package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/bean-bank/internal/middleware"
	"github.com/h4ks-com/bean-bank/internal/services"
)

type TransferHandler struct {
	transferService *services.TransferService
}

func NewTransferHandler(transferService *services.TransferService) *TransferHandler {
	return &TransferHandler{transferService: transferService}
}

type TransferRequest struct {
	ToUser string `json:"to_user" binding:"required"`
	Amount int    `json:"amount" binding:"required,gt=0"`
	Force  bool   `json:"force"`
}

type TransferResponse struct {
	Message string `json:"message"`
	Amount  int    `json:"amount"`
	ToUser  string `json:"to_user"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Transfer godoc
// @Summary Transfer beans
// @Description Transfer beans from authenticated user to recipient
// @Tags transfer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body TransferRequest true "Transfer details"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /transfer [post]
func (h *TransferHandler) Transfer(c *gin.Context) {
	username := middleware.GetUsername(c)

	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	err := h.transferService.Transfer(username, req.ToUser, req.Amount, req.Force)
	if err != nil {
		switch err {
		case services.ErrInsufficientBalance:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "insufficient balance"})
		case services.ErrRecipientNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "recipient not found, use force=true to create wallet"})
		case services.ErrInvalidAmount:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "amount must be positive"})
		case services.ErrSelfTransfer:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "cannot transfer to yourself"})
		case services.ErrUserNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "sender not found"})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, TransferResponse{
		Message: "transfer successful",
		Amount:  req.Amount,
		ToUser:  req.ToUser,
	})
}
