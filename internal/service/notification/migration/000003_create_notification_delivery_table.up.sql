-- Create notification_delivery table (delivery history)
CREATE TABLE notification_delivery (
    -- Primary Key
    id BIGSERIAL PRIMARY KEY,
    
    -- Foreign Key
    target_id BIGINT NOT NULL,                     -- ID của notification_target
    
    -- Delivery Status
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, processing, delivered, failed
    attempt_count INTEGER NOT NULL DEFAULT 0,       -- Số lần đã thử gửi (tăng mỗi lần worker xử lý)
    retry_count INTEGER NOT NULL DEFAULT 0,         -- Số lần đã retry (tăng mỗi lần relay)
    last_error TEXT,                                -- Lỗi cuối cùng nếu có
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP,                         -- Thời điểm gửi thành công
    failed_at TIMESTAMP                             -- Thời điểm gửi thất bại (lần cuối)
);

-- Constraints
-- Constraint: status chỉ được là các giá trị hợp lệ
ALTER TABLE notification_delivery 
    ADD CONSTRAINT chk_delivery_status 
    CHECK (status IN ('pending', 'processing', 'delivered', 'failed'));

-- Constraint: attempt_count không được âm
ALTER TABLE notification_delivery 
    ADD CONSTRAINT chk_delivery_attempt_count_non_negative 
    CHECK (attempt_count >= 0);

-- Constraint: retry_count không được âm
ALTER TABLE notification_delivery 
    ADD CONSTRAINT chk_delivery_retry_count_non_negative 
    CHECK (retry_count >= 0);

-- Unique constraint: Mỗi target chỉ có 1 delivery record (1:1 relationship)
-- Update record này mỗi lần attempt thay vì tạo mới
ALTER TABLE notification_delivery 
    ADD CONSTRAINT uq_notification_delivery_target_id 
    UNIQUE (target_id);

-- Indexes
-- Index cho status (để query pending/processing deliveries)
-- Partial index cho performance
CREATE INDEX idx_notification_delivery_status ON notification_delivery(status) WHERE status IN ('pending', 'processing');

