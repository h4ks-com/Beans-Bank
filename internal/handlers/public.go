package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/beapin/internal/services"
)

type PublicHandler struct {
	walletService *services.WalletService
}

func NewPublicHandler(walletService *services.WalletService) *PublicHandler {
	return &PublicHandler{walletService: walletService}
}

type TotalBeansResponse struct {
	TotalBeans int64 `json:"total_beans"`
}

// GetTotalBeans godoc
// @Summary Get total beans
// @Description Get the total number of beans in the system
// @Tags public
// @Accept json
// @Produce json
// @Success 200 {object} TotalBeansResponse
// @Failure 500 {object} ErrorResponse
// @Router /total [get]
func (h *PublicHandler) GetTotalBeans(c *gin.Context) {
	total, err := h.walletService.GetTotalBeans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TotalBeansResponse{
		TotalBeans: total,
	})
}
