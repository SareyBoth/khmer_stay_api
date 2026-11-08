ALTER TABLE conversations
    ALTER COLUMN owner_id DROP NOT NULL,
    ALTER COLUMN property_id DROP NOT NULL;

ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'property',
    ADD COLUMN IF NOT EXISTS subject VARCHAR(160) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'open',
    ADD COLUMN IF NOT EXISTS assigned_admin_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE conversations
SET subject = COALESCE(
        NULLIF(subject, ''),
        (SELECT COALESCE(p.data->>'name', 'Property conversation') FROM properties p WHERE p.id = conversations.property_id),
        'Property conversation'
    ),
    updated_at = GREATEST(
        created_at,
        COALESCE((SELECT MAX(m.created_at) FROM messages m WHERE m.conversation_id = conversations.id), created_at)
    );

ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_kind_check;
ALTER TABLE conversations ADD CONSTRAINT conversations_kind_check
    CHECK (kind IN ('property', 'support'));

ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_status_check;
ALTER TABLE conversations ADD CONSTRAINT conversations_status_check
    CHECK (status IN ('open', 'resolved', 'closed'));

ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_context_check;
ALTER TABLE conversations ADD CONSTRAINT conversations_context_check CHECK (
    (kind = 'property' AND owner_id IS NOT NULL AND property_id IS NOT NULL)
    OR
    (kind = 'support' AND owner_id IS NULL AND property_id IS NULL)
);

CREATE UNIQUE INDEX IF NOT EXISTS conversations_property_participants_unique
    ON conversations(traveler_id, owner_id, property_id)
    WHERE kind = 'property';

CREATE UNIQUE INDEX IF NOT EXISTS conversations_open_support_unique
    ON conversations(traveler_id)
    WHERE kind = 'support' AND status = 'open';

CREATE INDEX IF NOT EXISTS conversations_traveler_updated_idx
    ON conversations(traveler_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS conversations_owner_updated_idx
    ON conversations(owner_id, updated_at DESC) WHERE owner_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS conversations_support_updated_idx
    ON conversations(updated_at DESC) WHERE kind = 'support';

CREATE TABLE IF NOT EXISTS conversation_reads (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX IF NOT EXISTS conversation_reads_user_idx
    ON conversation_reads(user_id, last_read_at DESC);
