package payment

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/http/middleware"
	"github.com/khmerstay/gin-api/internal/http/response"
	"github.com/skip2/go-qrcode"
)

type Handler struct {
	service    *Service
	configured bool
	automatic  bool
}

func NewHandler(service *Service, configured, automatic bool) *Handler {
	return &Handler{service, configured, automatic}
}
func (h *Handler) status(c *gin.Context, check bool) {
	u, _ := middleware.Current(c)
	var b Booking
	var p Payment
	var err error
	if check {
		b, p, err = h.service.Check(c, c.Param("id"), u.UserID, u.Role)
	} else {
		b, p, err = h.service.Get(c, c.Param("id"), u.UserID, u.Role)
	}
	if err != nil {
		code := http.StatusBadRequest
		if err.Error() == "permission denied" {
			code = 403
		}
		response.Error(c, code, err.Error())
		return
	}
	response.JSON(c, 200, gin.H{"bookingId": b.ID, "status": b.PaymentStatus, "amount": b.Amount, "currency": b.Currency, "qrPayload": p.QRPayload, "qrMd5": p.QRMD5, "qrImageUrl": "/bookings/" + b.ID + "/payment/qr.png", "expiresAt": p.ExpiresAt, "checkedAt": p.CheckedAt, "bakongHash": p.BakongHash, "configured": h.configured, "automaticVerification": h.automatic})
}
func (h *Handler) Get(c *gin.Context)   { h.status(c, false) }
func (h *Handler) Check(c *gin.Context) { h.status(c, true) }
func (h *Handler) QR(c *gin.Context) {
	u, _ := middleware.Current(c)
	_, p, err := h.service.Get(c, c.Param("id"), u.UserID, u.Role)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if p.QRPayload == "" {
		response.Error(c, 409, "KHQR is not configured")
		return
	}
	png, err := qrcode.Encode(p.QRPayload, qrcode.Medium, 360)
	if err != nil {
		response.Error(c, 500, "could not render QR")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(200, "image/png", png)
}

func (h *Handler) Webhook(c *gin.Context) {
	var req struct {
		Hash        string  `json:"hash" binding:"required"`
		BookingID   string  `json:"bookingId"`
		BillNumber  string  `json:"billNumber"`
		Amount      float64 `json:"amount" binding:"required"`
		Currency    string  `json:"currency" binding:"required"`
		SecretToken string  `json:"secretToken"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, http.StatusBadRequest, "hash, bookingId/billNumber, amount, and currency are required")
		return
	}
	bookingID := req.BookingID
	if bookingID == "" {
		bookingID = req.BillNumber
	}
	if bookingID == "" {
		response.Error(c, http.StatusBadRequest, "bookingId or billNumber is required")
		return
	}
	if h.service.config.WebhookSecret != "" {
		headerSecret := c.GetHeader("X-Bakong-Webhook-Secret")
		if req.SecretToken != h.service.config.WebhookSecret && headerSecret != h.service.config.WebhookSecret {
			response.Error(c, http.StatusUnauthorized, "invalid webhook secret")
			return
		}
	}
	b, err := h.service.ProcessWebhook(c, req.Hash, bookingID, req.Amount, req.Currency)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"status": "ok", "bookingId": b.ID, "paymentStatus": b.PaymentStatus})
}
