-- 000005 包月核销记录
-- 包月订单按日核销，(order_id, service_date) 唯一，防止同一天被重复核销扣两次。
-- 不预占未来 30 天库存：核销当天才占用时段，避免把可售时段长期锁死。

CREATE TABLE monthly_redemptions (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id    BIGINT UNSIGNED NOT NULL,
    service_date DATE        NOT NULL COMMENT '核销的服务日期',
    operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '核销人',
    remark      VARCHAR(200) NOT NULL DEFAULT '',
    created_at  DATETIME(3) NOT NULL,
    updated_at  DATETIME(3) NOT NULL,
    deleted_at  DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_monthly_order_date (order_id, service_date),
    KEY idx_monthly_order_id (order_id),
    KEY idx_monthly_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='包月核销记录';
