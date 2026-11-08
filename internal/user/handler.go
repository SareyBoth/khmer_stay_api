package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/http/response"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service} }
func (h *Handler) Register(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	u, err := h.service.Create(c, req)
	if err != nil {
		response.Error(c, http.StatusConflict, err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"user": u})
}
func (h *Handler) List(c *gin.Context) {
	users, err := h.service.List(c)
	if err != nil {
		response.Error(c, 500, "could not list users")
		return
	}
	response.JSON(c, 200, gin.H{"users": users})
}
func (h *Handler) UpdateStatus(c *gin.Context) {
	var req StatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	u, err := h.service.UpdateStatus(c, c.Param("id"), req.Status)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	response.JSON(c, 200, gin.H{"user": u})
}
