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

type LeaderboardEntry struct {
	Rank       int    `json:"rank"`
	Username   string `json:"username"`
	BeanAmount int    `json:"bean_amount"`
}

type LeaderboardResponse struct {
	Entries []LeaderboardEntry `json:"entries"`
	Total   int                `json:"total"`
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

// GetLeaderboard godoc
// @Summary Get top bean wallets
// @Description Get the top 50 wallets by bean amount
// @Tags public
// @Accept json
// @Produce json
// @Success 200 {object} LeaderboardResponse
// @Failure 500 {object} ErrorResponse
// @Router /leaderboard [get]
func (h *PublicHandler) GetLeaderboard(c *gin.Context) {
	users, err := h.walletService.GetTopWallets(50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	entries := make([]LeaderboardEntry, len(users))
	for i, user := range users {
		entries[i] = LeaderboardEntry{
			Rank:       i + 1,
			Username:   user.Username,
			BeanAmount: user.BeanAmount,
		}
	}

	c.JSON(http.StatusOK, LeaderboardResponse{
		Entries: entries,
		Total:   len(entries),
	})
}
