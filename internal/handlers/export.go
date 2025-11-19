package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/bean-bank/internal/middleware"
	"github.com/h4ks-com/bean-bank/internal/services"
)

type ExportHandler struct {
	exportService *services.ExportService
}

func NewExportHandler(exportService *services.ExportService) *ExportHandler {
	return &ExportHandler{exportService: exportService}
}

type VerifyExportResponse struct {
	Valid bool `json:"valid"`
}

// ExportTransactions godoc
// @Summary Export transaction history
// @Description Export user's complete transaction history with cryptographic signature
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} services.TransactionExport
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /transactions/export [get]
func (h *ExportHandler) ExportTransactions(c *gin.Context) {
	username := middleware.GetUsername(c)
	if username == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	export, err := h.exportService.ExportTransactions(username)
	if err != nil {
		if err == services.ErrUserNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, export)
}

// VerifyExport godoc
// @Summary Verify transaction export signature
// @Description Verify the cryptographic signature of an exported transaction history
// @Tags transactions
// @Accept json
// @Produce json
// @Param request body services.TransactionExport true "Export data with signature"
// @Success 200 {object} VerifyExportResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /transactions/verify [post]
func (h *ExportHandler) VerifyExport(c *gin.Context) {
	var exportData services.TransactionExport
	if err := c.ShouldBindJSON(&exportData); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	valid, err := h.exportService.VerifyExportData(&exportData)
	if err != nil {
		if err == services.ErrInvalidExport {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid export data"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, VerifyExportResponse{Valid: valid})
}
