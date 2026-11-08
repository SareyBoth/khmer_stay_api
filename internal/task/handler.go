package task

import (
	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/auth"
	"github.com/khmerstay/gin-api/internal/http/middleware"
	"github.com/khmerstay/gin-api/internal/http/response"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service} }
func current(c *gin.Context) auth.Claims   { claims, _ := middleware.Current(c); return claims }
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, 400, "valid title is required")
		return
	}
	t, err := h.service.Create(c, current(c).UserID, req)
	if err != nil {
		response.Error(c, 500, "could not create task")
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"task": t})
}
func (h *Handler) List(c *gin.Context) {
	rows, err := h.service.List(c, current(c).UserID)
	if err != nil {
		response.Error(c, 500, "could not list tasks")
		return
	}
	response.JSON(c, 200, gin.H{"tasks": rows})
}
func (h *Handler) Update(c *gin.Context) {
	var req UpdateRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, 400, "invalid task update")
		return
	}
	t, err := h.service.Update(c, current(c).UserID, c.Param("id"), req)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	response.JSON(c, 200, gin.H{"task": t})
}
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c, current(c).UserID, c.Param("id")); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
