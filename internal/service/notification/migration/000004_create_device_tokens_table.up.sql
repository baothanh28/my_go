-- Create device_tokens table
CREATE TABLE IF NOT EXISTS device_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    push_token VARCHAR(500) NOT NULL,
    type VARCHAR(50) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT unique_user_device UNIQUE (user_id, device_id)
);

-- Constraints
-- Constraint: type chỉ được là 'expo', 'fcm', 'apns', hoặc 'native'
ALTER TABLE device_tokens 
    ADD CONSTRAINT chk_token_type 
    CHECK (type IN ('expo', 'fcm', 'apns', 'native'));

-- Constraint: platform chỉ được là 'ios', 'android', hoặc 'web'
ALTER TABLE device_tokens 
    ADD CONSTRAINT chk_platform 
    CHECK (platform IN ('ios', 'android', 'web'));

-- Indexes
CREATE INDEX idx_device_tokens_user_id ON device_tokens(user_id);
CREATE INDEX idx_device_tokens_device_id ON device_tokens(device_id);
CREATE INDEX idx_device_tokens_type ON device_tokens(type);
CREATE INDEX idx_device_tokens_deleted_at ON device_tokens(deleted_at);

