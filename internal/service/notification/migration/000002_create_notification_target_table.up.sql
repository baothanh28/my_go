-- Create notification_target table (target & payload)
CREATE TABLE notification_target (
    -- Primary Key
    id BIGSERIAL PRIMARY KEY,
    
    -- Foreign Key
    notification_id BIGINT NOT NULL,                -- ID của notification gốc
    
    -- Target Information
    user_id VARCHAR(255) NOT NULL,                 -- User cần gửi notification
    
    -- Payload
    payload JSONB NOT NULL DEFAULT '{}',           -- Payload data của notification cho user này
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP                            -- Soft delete (optional)
);

-- Constraints
-- Constraint: user_id không được rỗng
ALTER TABLE notification_target 
    ADD CONSTRAINT chk_user_id_not_empty 
    CHECK (user_id != '');

-- Indexes
-- Index cho notification_id (foreign key - cần cho JOIN performance)
CREATE INDEX idx_notification_target_notification_id ON notification_target(notification_id);

-- Index cho user_id (để query notifications của một user)
CREATE INDEX idx_notification_target_user_id ON notification_target(user_id);

