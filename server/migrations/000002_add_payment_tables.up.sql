-- 000002 支付与后台支撑表
-- 需求文档九张表未覆盖，但支付幂等与后台登录必须落表

CREATE TABLE payment_transactions (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_no       VARCHAR(32) NOT NULL,
    transaction_id VARCHAR(64) NOT NULL COMMENT '微信支付交易号，幂等键',
    out_trade_no   VARCHAR(64) NOT NULL DEFAULT '',
    amount         BIGINT      NOT NULL COMMENT '实付金额，单位分',
    status         VARCHAR(16) NOT NULL COMMENT 'success/closed/refunded',
    raw_callback   JSON        NULL COMMENT '原始回调报文，排查用',
    paid_at        DATETIME(3) NULL,
    created_at     DATETIME(3) NOT NULL,
    updated_at     DATETIME(3) NOT NULL,
    deleted_at     DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_payment_transactions_transaction_id (transaction_id),
    KEY idx_payment_transactions_order_no (order_no),
    KEY idx_payment_transactions_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='支付流水，transaction_id 唯一保证幂等';

CREATE TABLE refunds (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id      BIGINT UNSIGNED NOT NULL,
    out_refund_no VARCHAR(64) NOT NULL COMMENT '商户退款单号，幂等键',
    refund_id     VARCHAR(64) NOT NULL DEFAULT '' COMMENT '微信退款单号',
    amount        BIGINT      NOT NULL COMMENT '退款金额，单位分',
    reason        VARCHAR(200) NOT NULL DEFAULT '',
    status        VARCHAR(16) NOT NULL COMMENT 'pending/success/failed',
    operator_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
    refunded_at   DATETIME(3) NULL,
    created_at    DATETIME(3) NOT NULL,
    updated_at    DATETIME(3) NOT NULL,
    deleted_at    DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_refunds_out_refund_no (out_refund_no),
    KEY idx_refunds_order_id (order_id),
    KEY idx_refunds_status (status),
    KEY idx_refunds_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='退款单';

CREATE TABLE admin_users (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username      VARCHAR(32)  NOT NULL,
    password_hash VARCHAR(128) NOT NULL COMMENT 'bcrypt',
    role          VARCHAR(16)  NOT NULL DEFAULT 'operator' COMMENT 'admin/operator',
    status        TINYINT      NOT NULL DEFAULT 1,
    created_at    DATETIME(3)  NOT NULL,
    updated_at    DATETIME(3)  NOT NULL,
    deleted_at    DATETIME(3)  NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_admin_users_username (username),
    KEY idx_admin_users_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='后台账号';
