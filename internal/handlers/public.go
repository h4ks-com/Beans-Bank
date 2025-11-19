package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/bean-bank/internal/services"
)

type PublicHandler struct {
	walletService  *services.WalletService
	harvestService *services.HarvestService
}

func NewPublicHandler(walletService *services.WalletService, harvestService *services.HarvestService) *PublicHandler {
	return &PublicHandler{
		walletService:  walletService,
		harvestService: harvestService,
	}
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

type HarvestListItem struct {
	ID             uint   `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	BeanAmount     int    `json:"bean_amount"`
	AssignedUserID *uint  `json:"assigned_user_id,omitempty"`
	AssignedUser   string `json:"assigned_user,omitempty"`
	Completed      bool   `json:"completed"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type HarvestListResponse struct {
	Harvests   []HarvestListItem `json:"harvests"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	TotalPages int               `json:"total_pages"`
}

// GetHarvests godoc
// @Summary Get harvests
// @Description Get list of harvests with optional search and pagination
// @Tags public
// @Accept json
// @Produce json
// @Param search query string false "Search query for title and description"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Items per page (default 20)"
// @Success 200 {object} HarvestListResponse
// @Failure 500 {object} ErrorResponse
// @Router /harvests [get]
func (h *PublicHandler) GetHarvests(c *gin.Context) {
	search := c.DefaultQuery("search", "")
	page := 1
	limit := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	harvests, total, err := h.harvestService.SearchHarvests(search, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	items := make([]HarvestListItem, len(harvests))
	for i, harvest := range harvests {
		item := HarvestListItem{
			ID:          harvest.ID,
			Title:       harvest.Title,
			Description: harvest.Description,
			BeanAmount:  harvest.BeanAmount,
			Completed:   harvest.Completed,
			CreatedAt:   harvest.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   harvest.UpdatedAt.Format("2006-01-02 15:04:05"),
		}

		if harvest.AssignedUserID != nil {
			item.AssignedUserID = harvest.AssignedUserID
			if harvest.AssignedUser != nil {
				item.AssignedUser = harvest.AssignedUser.Username
			}
		}

		items[i] = item
	}

	totalPages := int(total) / limit
	if int(total)%limit != 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, HarvestListResponse{
		Harvests:   items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	})
}
