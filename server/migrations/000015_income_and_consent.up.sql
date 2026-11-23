-- 遛宠流程图 P2：
-- 1) service_records.owner_consented：服务中拍照/录像需「经主人同意」，线上留痕；
-- 2) provider_income：订单完成后服务方自动入账，图中「自动入账，通知客户」的账本部分。

ALTER TABLE service_records
    ADD COLUMN owner_consented TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 表示主人已同意拍摄' AFTER duration_sec;

CREATE TABLE IF NOT EXISTS provider_income (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    provider_id BIGINT UNSIGNED NOT NULL COMMENT '服务方用户 ID',
    order_id   BIGINT UNSIGNED NOT NULL COMMENT '订单 ID',
    order_no   VARCHAR(32)     NOT NULL COMMENT '订单号（冗余，便于直接查询）',
    amount     BIGINT          NOT NULL DEFAULT 0 COMMENT '入账金额（分），= 订单已付金额',
    created_at DATETIME        NOT NULL,
    updated_at DATETIME        NOT NULL,
    deleted_at DATETIME        NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_provider_income_order (order_id),
    KEY idx_provider_income_provider (provider_id, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='服务方入账流水';
