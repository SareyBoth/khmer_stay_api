CREATE TABLE IF NOT EXISTS properties (
    id TEXT PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id),
    status TEXT NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS properties_owner_idx ON properties(owner_id, status);

CREATE TABLE IF NOT EXISTS bookings (
    id TEXT PRIMARY KEY,
    traveler_id UUID NOT NULL REFERENCES users(id),
    owner_id UUID NOT NULL REFERENCES users(id),
    property_id TEXT NOT NULL REFERENCES properties(id),
    status TEXT NOT NULL,
    payment_status TEXT NOT NULL,
    amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    currency CHAR(3) NOT NULL CHECK (currency IN ('USD','KHR')),
    data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS bookings_owner_idx ON bookings(owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS bookings_traveler_idx ON bookings(traveler_id, created_at DESC);

CREATE TABLE IF NOT EXISTS payments (
    booking_id TEXT PRIMARY KEY REFERENCES bookings(id) ON DELETE CASCADE,
    qr_payload TEXT NOT NULL DEFAULT '',
    qr_md5 TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL DEFAULT 'epoch',
    checked_at TIMESTAMPTZ,
    bakong_hash TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS payments_bakong_hash_unique ON payments(bakong_hash) WHERE bakong_hash <> '';

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    category TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS notifications_user_idx ON notifications(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY,
    traveler_id UUID NOT NULL REFERENCES users(id),
    owner_id UUID NOT NULL REFERENCES users(id),
    property_id TEXT NOT NULL REFERENCES properties(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id),
    body VARCHAR(2000) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS messages_conversation_idx ON messages(conversation_id, created_at);

CREATE TABLE IF NOT EXISTS blocked_dates (
    owner_id UUID NOT NULL REFERENCES users(id),
    property_id TEXT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    PRIMARY KEY(property_id, date)
);
CREATE TABLE IF NOT EXISTS promotions (id TEXT PRIMARY KEY, data JSONB NOT NULL);
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value JSONB NOT NULL);
CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY,
    property_id TEXT NOT NULL REFERENCES properties(id),
    traveler_id UUID NOT NULL REFERENCES users(id),
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment VARCHAR(2000) NOT NULL,
    response VARCHAR(2000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
