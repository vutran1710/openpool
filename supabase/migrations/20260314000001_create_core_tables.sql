CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     VARCHAR(8) NOT NULL UNIQUE,
    storage_key   VARCHAR(64) NOT NULL UNIQUE,
    auth_provider VARCHAR(16) NOT NULL CHECK (auth_provider IN ('github', 'google')),
    provider_id   VARCHAR(128) NOT NULL,
    display_name  VARCHAR(128) NOT NULL,
    email         VARCHAR(256),
    avatar_url    TEXT,
    status        VARCHAR(16) NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (auth_provider, provider_id)
);

CREATE INDEX idx_users_public_id ON users (public_id);
CREATE INDEX idx_users_status ON users (status) WHERE status = 'active';

CREATE TABLE profiles_index (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name  VARCHAR(128) NOT NULL,
    bio           VARCHAR(500),
    city          VARCHAR(128),
    interests     TEXT[] DEFAULT '{}',
    looking_for   VARCHAR(32) CHECK (looking_for IN ('dating', 'friends', 'networking', 'any')),
    discoverable  BOOLEAN NOT NULL DEFAULT true,
    avatar_url    TEXT,
    public_id     VARCHAR(8) NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_profiles_discoverable ON profiles_index (discoverable) WHERE discoverable = true;
CREATE INDEX idx_profiles_city ON profiles_index (city) WHERE discoverable = true;
CREATE INDEX idx_profiles_interests ON profiles_index USING GIN (interests) WHERE discoverable = true;

CREATE TABLE likes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    liker_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    liked_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (liker_id, liked_id),
    CHECK (liker_id <> liked_id)
);

CREATE INDEX idx_likes_liked_id ON likes (liked_id);
CREATE INDEX idx_likes_pair_check ON likes (liked_id, liker_id);

CREATE TABLE matches (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_a     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (user_a < user_b),
    UNIQUE (user_a, user_b)
);

CREATE INDEX idx_matches_user_a ON matches (user_a);
CREATE INDEX idx_matches_user_b ON matches (user_b);

CREATE TABLE conversations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id        UUID NOT NULL UNIQUE REFERENCES matches(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_message_at TIMESTAMPTZ
);

CREATE TABLE messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body            TEXT NOT NULL CHECK (char_length(body) > 0 AND char_length(body) <= 4000),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_conversation ON messages (conversation_id, created_at DESC);

CREATE TABLE commitments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id    UUID NOT NULL UNIQUE REFERENCES matches(id) ON DELETE CASCADE,
    proposer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    accepter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      VARCHAR(16) NOT NULL DEFAULT 'proposed'
                CHECK (status IN ('proposed', 'accepted', 'declined', 'dissolved')),
    proposed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    CHECK (proposer_id <> accepter_id)
);

CREATE INDEX idx_commitments_status ON commitments (status) WHERE status IN ('proposed', 'accepted');
