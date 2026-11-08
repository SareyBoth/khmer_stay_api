package payment

import (
	"context"
	"errors"
	"math"
	"time"
)

type Config struct {
	KHQR            KHQRConfig
	TokenConfigured bool
	WebhookSecret   string
}
type Service struct {
	repo   *Repository
	bakong BakongClient
	config Config
	now    func() time.Time
	OnPaid func(b Booking)
}

func NewService(repo *Repository, bakong BakongClient, config Config) *Service {
	return &Service{repo: repo, bakong: bakong, config: config, now: time.Now}
}
func (s *Service) authorized(b Booking, userID, role string) bool {
	return role == "admin" || b.OwnerID == userID || b.TravelerID == userID
}
func (s *Service) Get(ctx context.Context, bookingID, userID, role string) (Booking, Payment, error) {
	b, err := s.repo.Booking(ctx, bookingID)
	if err != nil {
		return Booking{}, Payment{}, err
	}
	if !s.authorized(b, userID, role) {
		return Booking{}, Payment{}, errors.New("permission denied")
	}
	p, err := s.repo.Payment(ctx, bookingID)
	if isMissing(err) {
		p = Payment{BookingID: bookingID}
		err = nil
	} else if err != nil {
		return Booking{}, Payment{}, err
	}
	if b.PaymentStatus != "Paid" && s.config.KHQR.AccountID != "" && (p.QRPayload == "" || p.ExpiresAt.Before(s.now())) {
		khqr := s.config.KHQR
		khqr.Currency = b.Currency
		qr, genErr := GenerateKHQR(khqr, b.Amount, b.ID, s.now())
		if genErr != nil {
			return Booking{}, Payment{}, genErr
		}
		p, err = s.repo.SaveQR(ctx, b.ID, qr)
	}
	return b, p, err
}
func (s *Service) Check(ctx context.Context, bookingID, userID, role string) (Booking, Payment, error) {
	b, p, err := s.Get(ctx, bookingID, userID, role)
	if err != nil {
		return b, p, err
	}
	if b.PaymentStatus == "Paid" {
		return b, p, nil
	}
	if !s.config.TokenConfigured {
		return b, p, errors.New("BAKONG_API_TOKEN is not configured")
	}
	if p.QRMD5 == "" {
		return b, p, errors.New("KHQR is not configured")
	}
	if p.ExpiresAt.Before(s.now()) {
		return b, p, errors.New("KHQR expired; request a new payment QR")
	}
	transaction, err := s.bakong.CheckByMD5(ctx, p.QRMD5)
	_ = s.repo.MarkChecked(ctx, bookingID)
	if errors.Is(err, ErrTransactionNotFound) {
		p.CheckedAt = pointer(s.now())
		return b, p, nil
	}
	if err != nil {
		return b, p, err
	}
	if transaction.Hash == "" || transaction.ToAccountID != s.config.KHQR.AccountID || transaction.Currency != b.Currency || math.Abs(transaction.Amount-b.Amount) > 0.009 {
		return b, p, errors.New("Bakong transaction does not match this booking")
	}
	if err = s.repo.MarkPaid(ctx, bookingID, transaction.Hash); err != nil {
		return b, p, err
	}
	b.PaymentStatus = "Paid"
	if b.Status == "Pending" || b.Status == "New request" {
		b.Status = "Confirmed"
	}
	p.BakongHash = transaction.Hash
	p.CheckedAt = pointer(s.now())
	if s.OnPaid != nil {
		s.OnPaid(b)
	}
	return b, p, nil
}

func (s *Service) ProcessWebhook(ctx context.Context, hash, bookingID string, amount float64, currency string) (Booking, error) {
	b, err := s.repo.Booking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if b.PaymentStatus == "Paid" {
		return b, nil
	}
	if b.Currency != currency || math.Abs(b.Amount-amount) > 0.009 {
		return Booking{}, errors.New("webhook transaction currency or amount mismatch")
	}
	if err = s.repo.MarkPaid(ctx, bookingID, hash); err != nil {
		return Booking{}, err
	}
	b.PaymentStatus = "Paid"
	if b.Status == "Pending" || b.Status == "New request" {
		b.Status = "Confirmed"
	}
	if s.OnPaid != nil {
		s.OnPaid(b)
	}
	return b, nil
}
func pointer(value time.Time) *time.Time { return &value }
