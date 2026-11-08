package payment

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Booking struct {
	ID, TravelerID, OwnerID, Status, PaymentStatus, Currency string
	Amount                                                   float64
}
type Payment struct {
	BookingID, QRPayload, QRMD5, BakongHash string
	ExpiresAt                               time.Time
	CheckedAt                               *time.Time
}
type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db} }
func (r *Repository) Booking(ctx context.Context, id string) (Booking, error) {
	var b Booking
	err := r.db.QueryRow(ctx, `SELECT id,traveler_id,owner_id,status,payment_status,amount,currency FROM bookings WHERE id=$1`, id).Scan(&b.ID, &b.TravelerID, &b.OwnerID, &b.Status, &b.PaymentStatus, &b.Amount, &b.Currency)
	return b, err
}
func (r *Repository) Payment(ctx context.Context, bookingID string) (Payment, error) {
	var p Payment
	err := r.db.QueryRow(ctx, `SELECT booking_id,qr_payload,qr_md5,expires_at,checked_at,bakong_hash FROM payments WHERE booking_id=$1`, bookingID).Scan(&p.BookingID, &p.QRPayload, &p.QRMD5, &p.ExpiresAt, &p.CheckedAt, &p.BakongHash)
	return p, err
}
func (r *Repository) SaveQR(ctx context.Context, bookingID string, qr QRData) (Payment, error) {
	_, err := r.db.Exec(ctx, `INSERT INTO payments(booking_id,qr_payload,qr_md5,expires_at)VALUES($1,$2,$3,$4)ON CONFLICT(booking_id)DO UPDATE SET qr_payload=EXCLUDED.qr_payload,qr_md5=EXCLUDED.qr_md5,expires_at=EXCLUDED.expires_at,checked_at=NULL,bakong_hash='' WHERE payments.bakong_hash=''`, bookingID, qr.Payload, qr.MD5, qr.ExpiresAt)
	if err != nil {
		return Payment{}, err
	}
	return r.Payment(ctx, bookingID)
}
func (r *Repository) MarkChecked(ctx context.Context, bookingID string) error {
	_, err := r.db.Exec(ctx, `UPDATE payments SET checked_at=NOW() WHERE booking_id=$1`, bookingID)
	return err
}
func (r *Repository) MarkPaid(ctx context.Context, bookingID, hash string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status string
	if err = tx.QueryRow(ctx, `SELECT payment_status FROM bookings WHERE id=$1 FOR UPDATE`, bookingID).Scan(&status); err != nil {
		return err
	}
	if status == "Paid" {
		return tx.Commit(ctx)
	}
	var used bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE bakong_hash=$1 AND booking_id<>$2)`, hash, bookingID).Scan(&used); err != nil {
		return err
	}
	if used {
		return errors.New("Bakong transaction already used for another booking")
	}
	if _, err = tx.Exec(ctx, `UPDATE payments SET bakong_hash=$2,checked_at=NOW() WHERE booking_id=$1`, bookingID, hash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE bookings SET payment_status='Paid',status=CASE WHEN status IN ('Pending','New request') THEN 'Confirmed' ELSE status END,data=jsonb_set(jsonb_set(data,'{payment}','"Paid"'::jsonb),'{status}',to_jsonb(CASE WHEN status IN ('Pending','New request') THEN 'Confirmed' ELSE status END)),updated_at=NOW() WHERE id=$1`, bookingID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func isMissing(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && err.Error() == pgx.ErrNoRows.Error())
}
