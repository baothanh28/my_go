-- Create notification table (metadata only)
CREATE TABLE notification (
    -- Primary Key
    id BIGSERIAL PRIMARY KEY,
    
    -- Core Metadata
    type VARCHAR(100) NOT NULL,                    -- Loại notification (e.g., "order_created", "message_received")
    target_type VARCHAR(20) NOT NULL DEFAULT 'user', -- 'user', 'group', 'alluser' (chỉ để phân loại, không lưu target cụ thể)
    priority INTEGER NOT NULL DEFAULT 0,            -- Độ ưu tiên (0 = normal, 1 = high, 2 = urgent)
    trace_id VARCHAR(255),                         -- Trace ID cho distributed tracing
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP                            -- Soft delete (optional)
);

-- Constraints
-- Constraint: target_type chỉ được là 'user', 'group', hoặc 'alluser'
ALTER TABLE notification 
    ADD CONSTRAINT chk_target_type 
    CHECK (target_type IN ('user', 'group', 'alluser'));

-- Constraint: priority trong khoảng hợp lệ
ALTER TABLE notification 
    ADD CONSTRAINT chk_priority_range 
    CHECK (priority >= 0 AND priority <= 2);

