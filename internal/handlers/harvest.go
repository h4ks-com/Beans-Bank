package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/bean-bank/internal/models"
	"github.com/h4ks-com/bean-bank/internal/services"
)

type HarvestHandler struct {
	harvestService *services.HarvestService
}

func NewHarvestHandler(harvestService *services.HarvestService) *HarvestHandler {
	return &HarvestHandler{harvestService: harvestService}
}

type CreateHarvestRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	BeanAmount  int    `json:"bean_amount" binding:"required,min=1"`
}

type UpdateHarvestRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	BeanAmount  int    `json:"bean_amount" binding:"required,min=1"`
}

type AssignUserRequest struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
}

type HarvestResponse struct {
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

// @Summary Create a harvest task
// @Description Create a new harvest task that can be assigned to users for bean rewards
// @Tags harvests
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateHarvestRequest true "Harvest creation request"
// @Success 201 {object} HarvestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/harvests [post]
func (h *HarvestHandler) CreateHarvest(c *gin.Context) {
	var req CreateHarvestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	harvest, err := h.harvestService.CreateHarvest(req.Title, req.Description, req.BeanAmount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toHarvestResponse(harvest))
}

// @Summary Update a harvest task
// @Description Update the details of an existing harvest task
// @Tags harvests
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Harvest ID"
// @Param request body UpdateHarvestRequest true "Harvest update request"
// @Success 200 {object} HarvestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/harvests/{id} [put]
func (h *HarvestHandler) UpdateHarvest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid harvest ID"})
		return
	}

	var req UpdateHarvestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	harvest, err := h.harvestService.UpdateHarvest(uint(id), req.Title, req.Description, req.BeanAmount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, toHarvestResponse(harvest))
}

// @Summary Assign user to harvest
// @Description Assign a user to a harvest task by username or user ID
// @Tags harvests
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Harvest ID"
// @Param request body AssignUserRequest true "User assignment request"
// @Success 200 {object} HarvestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/harvests/{id}/assign [post]
func (h *HarvestHandler) AssignUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid harvest ID"})
		return
	}

	var req AssignUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	var harvest *models.Harvest
	if req.Username != "" {
		harvest, err = h.harvestService.AssignUserByUsername(uint(id), req.Username)
	} else if req.UserID != 0 {
		harvest, err = h.harvestService.AssignUser(uint(id), req.UserID)
	} else {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "either username or user_id is required"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, toHarvestResponse(harvest))
}

// @Summary Complete a harvest task
// @Description Mark a harvest as completed and transfer beans to the assigned user
// @Tags harvests
// @Produce json
// @Security BearerAuth
// @Param id path int true "Harvest ID"
// @Success 200 {object} HarvestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/harvests/{id}/complete [post]
func (h *HarvestHandler) CompleteHarvest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid harvest ID"})
		return
	}

	harvest, err := h.harvestService.CompleteHarvest(uint(id))
	if err != nil {
		if err == services.ErrHarvestNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "harvest not found"})
			return
		}
		if err == services.ErrHarvestAlreadyCompleted {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "harvest already completed"})
			return
		}
		if err == services.ErrNoAssignedUser {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "harvest has no assigned user"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, toHarvestResponse(harvest))
}

// @Summary Delete a harvest task
// @Description Delete a harvest task by ID
// @Tags harvests
// @Security BearerAuth
// @Param id path int true "Harvest ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/harvests/{id} [delete]
func (h *HarvestHandler) DeleteHarvest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid harvest ID"})
		return
	}

	err = h.harvestService.DeleteHarvest(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary List all harvest tasks
// @Description Get a list of all harvest tasks (admin only)
// @Tags harvests
// @Produce json
// @Security BearerAuth
// @Success 200 {array} HarvestResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/harvests [get]
func (h *HarvestHandler) GetAllHarvests(c *gin.Context) {
	harvests, err := h.harvestService.GetAllHarvests()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	responses := make([]HarvestResponse, len(harvests))
	for i, harvest := range harvests {
		responses[i] = *toHarvestResponse(&harvest)
	}

	c.JSON(http.StatusOK, responses)
}

func toHarvestResponse(harvest *models.Harvest) *HarvestResponse {
	resp := &HarvestResponse{
		ID:          harvest.ID,
		Title:       harvest.Title,
		Description: harvest.Description,
		BeanAmount:  harvest.BeanAmount,
		Completed:   harvest.Completed,
		CreatedAt:   harvest.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   harvest.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	if harvest.AssignedUserID != nil {
		resp.AssignedUserID = harvest.AssignedUserID
		if harvest.AssignedUser != nil {
			resp.AssignedUser = harvest.AssignedUser.Username
		}
	}

	return resp
}
