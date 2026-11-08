package platform

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/khmerstay/gin-api/internal/http/response"
)

var (
	ErrPayoutNotEligible = errors.New("booking must be completed and paid to request payout")
	ErrPayoutExists      = errors.New("a payout has already been requested for this booking")
)

type Payout struct {
	ID               string     `json:"id"`
	OwnerID          string     `json:"ownerId"`
	BookingID        string     `json:"bookingId"`
	GrossAmount      float64    `json:"grossAmount"`
	CommissionRate   float64    `json:"commissionRate"`
	CommissionAmount float64    `json:"commissionAmount"`
	NetAmount        float64    `json:"netAmount"`
	Currency         string     `json:"currency"`
	BakongAccount    string     `json:"bakongAccount"`
	Status           string     `json:"status"` // Pending, Processing, Paid, Declined
	Reference        string     `json:"reference"`
	CreatedAt        time.Time  `json:"createdAt"`
	PaidAt           *time.Time `json:"paidAt,omitempty"`
	PropertyName     string     `json:"propertyName,omitempty"`
	GuestName        string     `json:"guestName,omitempty"`
}

func (r *Repository) RequestPayout(ctx context.Context, ownerID, bookingID, bakongAccount string) (Payout, error) {
	bakongAccount = strings.TrimSpace(bakongAccount)
	if bakongAccount == "" {
		return Payout{}, errors.New("Bakong account is required")
	}

	// 1. Check if booking exists, belongs to this owner, and is Completed + Paid
	var bookingStatus, paymentStatus, currency, propertyTitle, guestName string
	var grossAmount float64
	err := r.db.QueryRow(ctx, `
		SELECT status, payment_status, amount, currency, 
		       COALESCE(data->>'property', ''), COALESCE(data->>'guest', '')
		FROM bookings 
		WHERE id=$1 AND owner_id=$2`, bookingID, ownerID).
		Scan(&bookingStatus, &paymentStatus, &grossAmount, &currency, &propertyTitle, &guestName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payout{}, ErrNotFound
		}
		return Payout{}, err
	}

	if bookingStatus != "Completed" || paymentStatus != "Paid" {
		return Payout{}, ErrPayoutNotEligible
	}

	// 2. Check if a payout already exists for this booking
	var exists bool
	if err = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM host_payouts WHERE booking_id=$1)`, bookingID).Scan(&exists); err != nil {
		return Payout{}, err
	}
	if exists {
		return Payout{}, ErrPayoutExists
	}

	// 3. Determine commission rate from settings (default 15%)
	commissionRate := 15.0
	var rawCommission []byte
	if err = r.db.QueryRow(ctx, `SELECT value FROM settings WHERE key='commission'`).Scan(&rawCommission); err == nil {
		var n float64
		if val, ok := number(n); ok && val > 0 && val < 100 {
			commissionRate = val
		}
	}

	commissionAmount := math.Round(grossAmount*(commissionRate/100.0)*100) / 100
	netAmount := grossAmount - commissionAmount
	if currency == "KHR" {
		commissionAmount = math.Round(commissionAmount/100) * 100
		netAmount = grossAmount - commissionAmount
	}

	payoutID := "PO-" + time.Now().Format("060102") + "-" + uuid.NewString()[:6]
	now := time.Now().UTC()

	_, err = r.db.Exec(ctx, `
		INSERT INTO host_payouts (
			id, owner_id, booking_id, gross_amount, commission_rate, 
			commission_amount, net_amount, currency, bakong_account, status, reference, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'Pending', '', $10)`,
		payoutID, ownerID, bookingID, grossAmount, commissionRate,
		commissionAmount, netAmount, currency, bakongAccount, now)
	if err != nil {
		return Payout{}, err
	}

	return Payout{
		ID:               payoutID,
		OwnerID:          ownerID,
		BookingID:        bookingID,
		GrossAmount:      grossAmount,
		CommissionRate:   commissionRate,
		CommissionAmount: commissionAmount,
		NetAmount:        netAmount,
		Currency:         currency,
		BakongAccount:    bakongAccount,
		Status:           "Pending",
		Reference:        "",
		CreatedAt:        now,
		PropertyName:     propertyTitle,
		GuestName:        guestName,
	}, nil
}

func (r *Repository) ListPayouts(ctx context.Context, role, userID, status string) ([]Payout, error) {
	query := `
		SELECT p.id, p.owner_id, p.booking_id, p.gross_amount, p.commission_rate, 
		       p.commission_amount, p.net_amount, p.currency, p.bakong_account, 
		       p.status, p.reference, p.created_at, p.paid_at,
		       COALESCE(b.data->>'property', ''), COALESCE(b.data->>'guest', '')
		FROM host_payouts p
		LEFT JOIN bookings b ON b.id = p.booking_id`

	args := []any{}
	conditions := []string{}

	if role == "owner" {
		args = append(args, userID)
		conditions = append(conditions, fmt.Sprintf("p.owner_id=$%d", len(args)))
	}
	if status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("p.status=$%d", len(args)))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY p.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Payout{}
	for rows.Next() {
		var p Payout
		if err = rows.Scan(
			&p.ID, &p.OwnerID, &p.BookingID, &p.GrossAmount, &p.CommissionRate,
			&p.CommissionAmount, &p.NetAmount, &p.Currency, &p.BakongAccount,
			&p.Status, &p.Reference, &p.CreatedAt, &p.PaidAt,
			&p.PropertyName, &p.GuestName,
		); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *Repository) SettlePayout(ctx context.Context, payoutID, reference, status string) (Payout, error) {
	if status != "Paid" && status != "Declined" && status != "Processing" {
		status = "Paid"
	}
	var paidAt *time.Time
	if status == "Paid" {
		now := time.Now().UTC()
		paidAt = &now
	}

	tag, err := r.db.Exec(ctx, `
		UPDATE host_payouts 
		SET status=$2, reference=$3, paid_at=$4 
		WHERE id=$1`, payoutID, status, reference, paidAt)
	if err != nil {
		return Payout{}, err
	}
	if tag.RowsAffected() == 0 {
		return Payout{}, ErrNotFound
	}

	// Fetch updated
	var p Payout
	err = r.db.QueryRow(ctx, `
		SELECT p.id, p.owner_id, p.booking_id, p.gross_amount, p.commission_rate, 
		       p.commission_amount, p.net_amount, p.currency, p.bakong_account, 
		       p.status, p.reference, p.created_at, p.paid_at,
		       COALESCE(b.data->>'property', ''), COALESCE(b.data->>'guest', '')
		FROM host_payouts p
		LEFT JOIN bookings b ON b.id = p.booking_id
		WHERE p.id=$1`, payoutID).
		Scan(
			&p.ID, &p.OwnerID, &p.BookingID, &p.GrossAmount, &p.CommissionRate,
			&p.CommissionAmount, &p.NetAmount, &p.Currency, &p.BakongAccount,
			&p.Status, &p.Reference, &p.CreatedAt, &p.PaidAt,
			&p.PropertyName, &p.GuestName,
		)
	return p, err
}

// Handlers for platform

func (h *Handler) RequestPayout(c *gin.Context) {
	var input struct {
		BookingID     string `json:"bookingId" binding:"required"`
		BakongAccount string `json:"bakongAccount" binding:"required"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, http.StatusBadRequest, "bookingId and bakongAccount are required")
		return
	}

	u := claims(c)
	payout, err := h.repo.RequestPayout(c, u.UserID, input.BookingID, input.BakongAccount)
	if err != nil {
		if err == ErrPayoutNotEligible || err == ErrPayoutExists {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"payout": payout})
}

func (h *Handler) ListPayouts(c *gin.Context) {
	u := claims(c)
	payouts, err := h.repo.ListPayouts(c, u.Role, u.UserID, strings.TrimSpace(c.Query("status")))
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"payouts": payouts})
}

func (h *Handler) SettlePayout(c *gin.Context) {
	var input struct {
		Reference string `json:"reference" binding:"required"`
		Status    string `json:"status"`
	}
	if c.ShouldBindJSON(&input) != nil {
		response.Error(c, http.StatusBadRequest, "reference is required")
		return
	}

	payout, err := h.repo.SettlePayout(c, c.Param("id"), input.Reference, input.Status)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"payout": payout})
}
