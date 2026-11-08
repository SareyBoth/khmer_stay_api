package platform

import (
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/auth"
	"github.com/khmerstay/gin-api/internal/events"
	"github.com/khmerstay/gin-api/internal/http/middleware"
	"github.com/khmerstay/gin-api/internal/http/response"
	"github.com/khmerstay/gin-api/internal/storage"
)

type Handler struct {
	repo       *Repository
	uploadsDir string
	storage    storage.Uploader
	broker     *events.Broker
}

func NewHandler(repo *Repository, uploadsDir string, uploader storage.Uploader, broker *events.Broker) *Handler {
	if uploader == nil {
		uploader = storage.NewLocalStorage(uploadsDir, "/uploads")
	}
	return &Handler{repo: repo, uploadsDir: uploadsDir, storage: uploader, broker: broker}
}
func claims(c *gin.Context) auth.Claims { value, _ := middleware.Current(c); return value }
func fail(c *gin.Context, err error) {
	if err == ErrNotFound {
		response.Error(c, 404, err.Error())
	} else {
		response.Error(c, 500, "database operation failed")
	}
}
func bindMap(c *gin.Context) (map[string]any, bool) {
	var input map[string]any
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, 400, "invalid JSON body")
		return nil, false
	}
	return input, true
}
func canModify(record map[string]any, user auth.Claims) bool {
	return user.Role == "admin" || record["ownerId"] == user.UserID
}
func canSeeBooking(record map[string]any, user auth.Claims) bool {
	return user.Role == "admin" || record["ownerId"] == user.UserID || record["travelerId"] == user.UserID
}

func (h *Handler) ListProperties(c *gin.Context) {
	u := claims(c)
	minPrice, _ := strconv.ParseFloat(c.Query("minPrice"), 64)
	maxPrice, _ := strconv.ParseFloat(c.Query("maxPrice"), 64)
	guests, _ := strconv.Atoi(c.Query("guests"))
	filter := PropertyFilter{
		Role:         u.Role,
		UserID:       u.UserID,
		ProvinceCode: strings.TrimSpace(c.Query("provinceCode")),
		CityID:       strings.TrimSpace(c.Query("cityId")),
		CheckIn:      strings.TrimSpace(c.Query("checkIn")),
		CheckOut:     strings.TrimSpace(c.Query("checkOut")),
		Guests:       guests,
		MinPrice:     minPrice,
		MaxPrice:     maxPrice,
		Category:     strings.TrimSpace(c.Query("category")),
		Amenities:    strings.TrimSpace(c.Query("amenities")),
		Query:        strings.TrimSpace(c.Query("q")),
	}
	rows, err := h.repo.SearchProperties(c, filter)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"properties": rows})
}
func (h *Handler) GetProperty(c *gin.Context) {
	record, err := h.repo.GetProperty(c, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	u := claims(c)
	if u.Role == "traveler" {
		_ = h.repo.RecordRecommendationEvent(c, u.UserID, c.Param("id"), "view", nil)
	}
	response.JSON(c, 200, gin.H{"property": record})
}
func (h *Handler) CreateProperty(c *gin.Context) {
	input, ok := bindMap(c)
	if !ok {
		return
	}
	u := claims(c)
	record, err := h.repo.CreateProperty(c, u.UserID, u.Name, input)
	if err != nil {
		if err == ErrInvalidLocation {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"property": record})
}
func (h *Handler) UpdateProperty(c *gin.Context) {
	current, err := h.repo.GetProperty(c, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	if !canModify(current, claims(c)) {
		response.Error(c, 403, "permission denied")
		return
	}
	input, ok := bindMap(c)
	if !ok {
		return
	}
	delete(input, "id")
	delete(input, "ownerId")
	record, err := h.repo.UpdateProperty(c, c.Param("id"), input)
	if err != nil {
		if err == ErrInvalidLocation {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"property": record})
}
func (h *Handler) PropertyStatus(c *gin.Context) {
	current, err := h.repo.GetProperty(c, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	if !canModify(current, claims(c)) {
		response.Error(c, 403, "permission denied")
		return
	}
	var input struct {
		Status string `json:"status" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, 400, "status is required")
		return
	}
	allowed := map[string]bool{"Draft": true, "Published": true, "Paused": true, "Review pending": true, "Rejected": true}
	if !allowed[input.Status] {
		response.Error(c, 400, "invalid property status")
		return
	}
	record, err := h.repo.SetPropertyStatus(c, c.Param("id"), input.Status)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"property": record})
}

func (h *Handler) ListBookings(c *gin.Context) {
	u := claims(c)
	rows, err := h.repo.ListBookings(c, u.Role, u.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"bookings": rows})
}

func (h *Handler) Favorites(c *gin.Context) {
	rows, err := h.repo.Favorites(c, claims(c).UserID)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"propertyIds": rows})
}

func (h *Handler) AddFavorite(c *gin.Context) {
	u := claims(c)
	if err := h.repo.AddFavorite(c, u.UserID, c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	_ = h.repo.RecordRecommendationEvent(c, u.UserID, c.Param("id"), "favorite", nil)
	response.JSON(c, http.StatusOK, gin.H{"propertyId": c.Param("id"), "favorite": true})
}

func (h *Handler) RemoveFavorite(c *gin.Context) {
	u := claims(c)
	if err := h.repo.RemoveFavorite(c, u.UserID, c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	_ = h.repo.RecordRecommendationEvent(c, u.UserID, c.Param("id"), "unfavorite", nil)
	response.JSON(c, http.StatusOK, gin.H{"propertyId": c.Param("id"), "favorite": false})
}
func (h *Handler) GetBooking(c *gin.Context) {
	record, err := h.repo.GetBooking(c, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	if !canSeeBooking(record, claims(c)) {
		response.Error(c, 403, "permission denied")
		return
	}
	response.JSON(c, 200, gin.H{"booking": record})
}
func (h *Handler) CreateBooking(c *gin.Context) {
	input, ok := bindMap(c)
	if !ok {
		return
	}
	u := claims(c)
	record, err := h.repo.CreateBooking(c, u.UserID, u.Name, u.Email, input)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if propertyID, _ := record["propertyId"].(string); propertyID != "" {
		_ = h.repo.RecordRecommendationEvent(c, u.UserID, propertyID, "booking_started", nil)
	}
	if h.broker != nil {
		if ownerID, ok := record["ownerId"].(string); ok && ownerID != "" {
			h.broker.Publish(ownerID, events.Event{Type: "new_booking_request", Data: record})
		}
	}
	response.JSON(c, 201, gin.H{"booking": record})
}
func (h *Handler) BookingStatus(c *gin.Context) {
	record, err := h.repo.GetBooking(c, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	u := claims(c)
	if u.Role != "admin" && record["ownerId"] != u.UserID {
		response.Error(c, 403, "permission denied")
		return
	}
	var input struct {
		Status string `json:"status" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, 400, "status is required")
		return
	}
	allowed := map[string]bool{"Pending": true, "New request": true, "Confirmed": true, "Checked in": true, "Completed": true, "Cancelled": true, "Declined": true, "No-show": true}
	if !allowed[input.Status] {
		response.Error(c, 400, "invalid booking status")
		return
	}
	currentStatus, _ := record["status"].(string)
	paymentStatus, _ := record["payment"].(string)
	if !bookingTransitionAllowed(currentStatus, input.Status, paymentStatus, u.Role) {
		response.Error(c, http.StatusConflict, "booking status transition is not allowed")
		return
	}
	record, err = h.repo.SetBookingStatus(c, c.Param("id"), input.Status)
	if err != nil {
		fail(c, err)
		return
	}
	travelerID, _ := record["travelerId"].(string)
	propertyID, _ := record["propertyId"].(string)
	if travelerID != "" && propertyID != "" {
		eventType := ""
		if input.Status == "Completed" {
			eventType = "booking_completed"
		} else if input.Status == "Cancelled" || input.Status == "Declined" || input.Status == "No-show" {
			eventType = "booking_cancelled"
		}
		if eventType != "" {
			_ = h.repo.RecordRecommendationEvent(c, travelerID, propertyID, eventType, nil)
		}
	}
	if h.broker != nil {
		if travelerID != "" {
			h.broker.Publish(travelerID, events.Event{Type: "booking_updated", Data: record})
		}
		if ownerID, ok := record["ownerId"].(string); ok && ownerID != "" {
			h.broker.Publish(ownerID, events.Event{Type: "booking_updated", Data: record})
		}
	}
	response.JSON(c, 200, gin.H{"booking": record})
}

func bookingTransitionAllowed(current, next, paymentStatus, role string) bool {
	if current == next {
		return true
	}
	switch next {
	case "Confirmed":
		return (current == "Pending" || current == "New request") && paymentStatus == "Paid"
	case "Declined":
		return current == "Pending" || current == "New request"
	case "Checked in":
		return current == "Confirmed"
	case "Completed":
		return current == "Checked in"
	case "No-show":
		return current == "Confirmed"
	case "Cancelled":
		return role == "admin" && current != "Completed"
	default:
		return false
	}
}

func (h *Handler) Notifications(c *gin.Context) {
	rows, err := h.repo.Notifications(c, claims(c).UserID)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"notifications": rows})
}
func (h *Handler) ReadNotification(c *gin.Context) {
	if err := h.repo.ReadNotification(c, claims(c).UserID, c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"read": true})
}
func (h *Handler) Conversations(c *gin.Context) {
	u := claims(c)
	rows, err := h.repo.Conversations(c, u.Role, u.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"conversations": rows})
}
func (h *Handler) CreateConversation(c *gin.Context) {
	var input struct {
		Type       string `json:"type" binding:"required,oneof=property support"`
		PropertyID string `json:"propertyId"`
		Subject    string `json:"subject" binding:"max=160"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, http.StatusBadRequest, "type must be property or support")
		return
	}
	u := claims(c)
	conversation, err := h.repo.CreateConversation(c, u.UserID, input.Type, input.PropertyID, input.Subject)
	if err == ErrNotFound {
		response.Error(c, http.StatusNotFound, "property not found")
		return
	}
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"conversation": conversation})
}
func (h *Handler) Messages(c *gin.Context) {
	u := claims(c)
	rows, err := h.repo.Messages(c, c.Param("id"), u.UserID, u.Role)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"messages": rows})
}
func (h *Handler) ReadConversation(c *gin.Context) {
	u := claims(c)
	if err := h.repo.MarkConversationRead(c, c.Param("id"), u.UserID, u.Role); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) ConversationStatus(c *gin.Context) {
	var input struct {
		Status string `json:"status" binding:"required,oneof=open resolved closed"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, http.StatusBadRequest, "status must be open, resolved, or closed")
		return
	}
	u := claims(c)
	conversation, err := h.repo.SetConversationStatus(c, c.Param("id"), u.UserID, input.Status)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"conversation": conversation})
}
func (h *Handler) SendMessage(c *gin.Context) {
	var input struct {
		Body string `json:"body"`
		Text string `json:"text"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, 400, "message body is required")
		return
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		body = strings.TrimSpace(input.Text)
	}
	if body == "" || len(body) > 2000 {
		response.Error(c, 400, "message must be 1-2000 characters")
		return
	}
	u := claims(c)
	message, err := h.repo.SendMessage(c, c.Param("id"), u.UserID, u.Role, body)
	if err != nil {
		fail(c, err)
		return
	}
	if h.broker != nil {
		recipients, recipientErr := h.repo.ConversationRecipients(c, c.Param("id"), u.UserID)
		if recipientErr != nil {
			log.Printf("message recipients: %v", recipientErr)
		} else {
			realtimeMessage := make(map[string]any, len(message))
			for key, value := range message {
				realtimeMessage[key] = value
			}
			realtimeMessage["fromMe"] = false
			for _, recipientID := range recipients {
				h.broker.Publish(recipientID, events.Event{Type: "new_message", Data: realtimeMessage})
			}
		}
	}
	response.JSON(c, 201, gin.H{"message": message})
}
func (h *Handler) Promotions(c *gin.Context) {
	rows, err := h.repo.Promotions(c)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"promotions": rows})
}
func (h *Handler) CreatePromotion(c *gin.Context) {
	input, ok := bindMap(c)
	if !ok {
		return
	}
	record, err := h.repo.CreatePromotion(c, input)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 201, gin.H{"promotion": record})
}
func (h *Handler) BlockedDates(c *gin.Context) {
	u := claims(c)
	rows, err := h.repo.BlockedDates(c, u.UserID, u.Role)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"blockedDates": rows})
}
func (h *Handler) ToggleBlockedDate(c *gin.Context) {
	var input struct {
		PropertyID string `json:"propertyId" binding:"required"`
		Date       string `json:"date" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, 400, "propertyId and date are required")
		return
	}
	current, err := h.repo.GetProperty(c, input.PropertyID)
	if err != nil {
		fail(c, err)
		return
	}
	u := claims(c)
	if !canModify(current, u) {
		response.Error(c, 403, "permission denied")
		return
	}
	blocked, err := h.repo.ToggleBlockedDate(c, u.UserID, input.PropertyID, input.Date)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"propertyId": input.PropertyID, "date": input.Date, "blocked": blocked})
}
func (h *Handler) Settings(c *gin.Context) {
	rows, err := h.repo.Settings(c)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"settings": rows})
}
func (h *Handler) UpdateSettings(c *gin.Context) {
	input, ok := bindMap(c)
	if !ok {
		return
	}
	rows, err := h.repo.UpdateSettings(c, input)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"settings": rows})
}
func (h *Handler) Reviews(c *gin.Context) {
	rows, err := h.repo.Reviews(c, c.Query("propertyId"))
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"reviews": rows})
}
func (h *Handler) CreateReview(c *gin.Context) {
	var input struct {
		PropertyID string `json:"propertyId" binding:"required"`
		Rating     int    `json:"rating" binding:"required,min=1,max=5"`
		Comment    string `json:"comment" binding:"required,min=2,max=2000"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, http.StatusBadRequest, "propertyId, rating from 1 to 5, and comment are required")
		return
	}
	review, err := h.repo.CreateReview(c, claims(c).UserID, input.PropertyID, input.Rating, strings.TrimSpace(input.Comment))
	if err == ErrReviewNotAllowed {
		response.Error(c, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"review": review, "id": review["id"]})
}
func (h *Handler) RespondReview(c *gin.Context) {
	var input struct {
		Response string `json:"response" binding:"required,max=2000"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, 400, "response is required")
		return
	}
	if err := h.repo.RespondReview(c, c.Param("id"), input.Response); err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, gin.H{"id": c.Param("id"), "response": input.Response})
}
func (h *Handler) Dashboard(c *gin.Context) {
	u := claims(c)
	data, err := h.repo.Dashboard(c, u.Role, u.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 200, data)
}

func (h *Handler) Provinces(c *gin.Context) {
	rows, err := h.repo.Provinces(c)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"provinces": rows})
}

func (h *Handler) Destinations(c *gin.Context) {
	rows, err := h.repo.Destinations(c, strings.TrimSpace(c.Query("provinceCode")))
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"destinations": rows})
}

func (h *Handler) Recommendations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))
	rows, err := h.repo.Recommendations(c, claims(c).UserID, strings.TrimSpace(c.Query("provinceCode")), strings.TrimSpace(c.Query("cityId")), limit)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"properties": rows})
}

func (h *Handler) RecommendationEvent(c *gin.Context) {
	var input struct {
		PropertyID string         `json:"propertyId" binding:"required"`
		EventType  string         `json:"eventType" binding:"required"`
		Metadata   map[string]any `json:"metadata"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, http.StatusBadRequest, "propertyId and eventType are required")
		return
	}
	if _, ok := recommendationWeights[input.EventType]; !ok {
		response.Error(c, http.StatusBadRequest, "unsupported recommendation event")
		return
	}
	if err := h.repo.RecordRecommendationEvent(c, claims(c).UserID, input.PropertyID, input.EventType, input.Metadata); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, 400, "file is required")
		return
	}
	if file.Size > 5<<20 {
		response.Error(c, 400, "file must be 5 MB or smaller")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	if !allowed[ext] {
		response.Error(c, 400, "only PNG, JPG and WebP are allowed")
		return
	}
	src, err := file.Open()
	if err != nil {
		response.Error(c, 500, "could not open uploaded file")
		return
	}
	defer src.Close()

	url, err := h.storage.Upload(c, src, file.Filename, file.Size, file.Header.Get("Content-Type"))
	if err != nil {
		response.Error(c, 500, "could not save upload: "+err.Error())
		return
	}
	response.JSON(c, 201, gin.H{"url": url})
}
