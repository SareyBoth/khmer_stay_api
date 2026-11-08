-- Migration 008: Production enhancements (Date ranges, refresh tokens, and host payouts)

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS check_in DATE;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS check_out DATE;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS room_id TEXT;

UPDATE bookings
SET check_in = CASE
      WHEN data->>'checkIn' ~ '^\d{4}-\d{2}-\d{2}$' THEN (data->>'checkIn')::date
      ELSE NULL
    END,
    check_out = CASE
      WHEN data->>'checkOut' ~ '^\d{4}-\d{2}-\d{2}$' THEN (data->>'checkOut')::date
      ELSE NULL
    END,
    room_id = COALESCE(data->>'roomId', '')
WHERE check_in IS NULL;

CREATE INDEX IF NOT EXISTS bookings_availability_idx
    ON bookings(property_id, check_in, check_out, status);

CREATE INDEX IF NOT EXISTS bookings_expiry_idx
    ON bookings(created_at)
    WHERE status IN ('Pending', 'New request') AND payment_status = 'Pending';

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS refresh_tokens_lookup_idx
    ON refresh_tokens(token_hash)
    WHERE revoked = FALSE;

CREATE INDEX IF NOT EXISTS refresh_tokens_user_idx
    ON refresh_tokens(user_id);

CREATE TABLE IF NOT EXISTS host_payouts (
    id TEXT PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id),
    booking_id TEXT REFERENCES bookings(id),
    gross_amount NUMERIC(14,2) NOT NULL,
    commission_rate NUMERIC(5,2) NOT NULL,
    commission_amount NUMERIC(14,2) NOT NULL,
    net_amount NUMERIC(14,2) NOT NULL,
    currency CHAR(3) NOT NULL CHECK (currency IN ('USD', 'KHR')),
    bakong_account TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('Pending', 'Processing', 'Paid', 'Declined')),
    reference TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS host_payouts_owner_idx
    ON host_payouts(owner_id, created_at DESC);

CREATE INDEX IF NOT EXISTS host_payouts_status_idx
    ON host_payouts(status);
