package platform

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khmerstay/gin-api/internal/events"
)

// StartBookingReaper runs a background worker that automatically cancels and expires
// bookings that have remained unpaid past their validity timeout.
func StartBookingReaper(ctx context.Context, db *pgxpool.Pool, timeout time.Duration, interval time.Duration, broker *events.Broker) {
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	if interval <= 0 {
		interval = 1 * time.Minute
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reapExpiredBookings(ctx, db, timeout, broker)
			}
		}
	}()
}

func reapExpiredBookings(ctx context.Context, db *pgxpool.Pool, timeout time.Duration, broker *events.Broker) {
	query := `
		SELECT id, traveler_id, owner_id 
		FROM bookings 
		WHERE status IN ('Pending', 'New request') 
		  AND payment_status = 'Pending' 
		  AND created_at < NOW() - $1::interval`

	timeoutStr := timeout.String()
	rows, err := db.Query(ctx, query, timeoutStr)
	if err != nil {
		return
	}
	defer rows.Close()

	type expiredBooking struct {
		id         string
		travelerID string
		ownerID    string
	}
	var expired []expiredBooking
	for rows.Next() {
		var b expiredBooking
		if err := rows.Scan(&b.id, &b.travelerID, &b.ownerID); err == nil {
			expired = append(expired, b)
		}
	}
	rows.Close()

	for _, b := range expired {
		tx, err := db.Begin(ctx)
		if err != nil {
			continue
		}

		tag, err := tx.Exec(ctx, `
			UPDATE bookings 
			SET status='Cancelled',
			    payment_status='Expired',
			    data=jsonb_set(jsonb_set(data, '{status}', '"Cancelled"'::jsonb), '{payment}', '"Expired"'::jsonb),
			    updated_at=NOW() 
			WHERE id=$1 AND payment_status='Pending'`, b.id)
		if err != nil || tag.RowsAffected() == 0 {
			_ = tx.Rollback(ctx)
			continue
		}

		_, _ = tx.Exec(ctx, `UPDATE payments SET expires_at=NOW() WHERE booking_id=$1`, b.id)
		if err = tx.Commit(ctx); err != nil {
			continue
		}

		log.Printf("booking reaper: expired unpaid booking %s (freed room inventory)", b.id)

		if broker != nil {
			event := events.Event{
				Type: "booking_updated",
				Data: map[string]any{
					"bookingId": b.id,
					"status":    "Cancelled",
					"payment":   "Expired",
					"reason":    "Payment window expired",
				},
			}
			broker.Publish(b.travelerID, event)
			broker.Publish(b.ownerID, event)
		}
	}
}
