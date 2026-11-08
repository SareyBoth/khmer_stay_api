ALTER TABLE otp_codes DROP CONSTRAINT IF EXISTS otp_codes_purpose_check;
ALTER TABLE otp_codes
    ADD CONSTRAINT otp_codes_purpose_check
    CHECK (purpose IN ('reset', 'login', 'verify_email'));

CREATE TABLE IF NOT EXISTS favorites (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id TEXT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, property_id)
);

CREATE INDEX IF NOT EXISTS favorites_user_created_idx
    ON favorites(user_id, created_at DESC);
