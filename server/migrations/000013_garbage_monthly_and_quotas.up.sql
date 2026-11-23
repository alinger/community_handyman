-- 代倒垃圾改为「包月为主 + 注册送 2 次免费」。
--
-- 1) user_quotas：按服务类型记录免费次数（当前只有 garbage 用到）。
--    用独立表而不是在 users 上加字段，是因为以后遛宠/陪诊也可能发体验券，
--    加字段会变成 users.new_service_free_count 这类越来越烂的列。
-- 2) orders 增加包月服务期与第二个取件时段：
--    - monthly_start / monthly_end：一张包月订单对应 30 天固定服务期，
--      写死在订单上，核销与对账都不必再去推导；
--    - extra_slot_code：用户额外指定的第二个每日取件时段，空串表示只按默认时段；
--    - free_quota_used：标记这一单是免费次数抵扣的，财务统计能区分真实收入。
-- 3) garbage 月卡价格 200 元 → 30 元 / 30 天，并把时长与默认时段写进 SKU 规则，
--    后台改价不用发版。

CREATE TABLE IF NOT EXISTS user_quotas (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id      BIGINT UNSIGNED NOT NULL COMMENT '所属用户',
    service_type VARCHAR(16)     NOT NULL COMMENT 'garbage / dogwalk / accompany',
    total        INT             NOT NULL DEFAULT 0 COMMENT '累计发放次数',
    used         INT             NOT NULL DEFAULT 0 COMMENT '已使用次数',
    created_at   DATETIME        NOT NULL,
    updated_at   DATETIME        NOT NULL,
    deleted_at   DATETIME        NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_quotas_user_service (user_id, service_type),
    KEY idx_user_quotas_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='用户免费次数额度';

ALTER TABLE orders
    ADD COLUMN extra_slot_code VARCHAR(16) NOT NULL DEFAULT '' COMMENT '包月第二个每日取件时段，空串表示无' AFTER slot_code,
    ADD COLUMN monthly_start DATE NULL COMMENT '包月服务期开始日' AFTER extra_slot_code,
    ADD COLUMN monthly_end DATE NULL COMMENT '包月服务期结束日（含当天）' AFTER monthly_start,
    ADD COLUMN free_quota_used TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 表示本单用免费次数抵扣' AFTER paid_amount;

-- 存量包月订单回填服务期：以服务日期为起点，30 天
UPDATE orders SET monthly_start = service_date, monthly_end = DATE_ADD(service_date, INTERVAL 29 DAY)
WHERE plan = 'monthly' AND service_type = 'garbage';

UPDATE service_skus
SET base_price = 3000,
    rules = JSON_SET(
        COALESCE(rules, JSON_OBJECT()),
        '$.monthly_price', 3000,
        '$.duration_days', 30,
        '$.default_slot', '22:00-22:30',
        '$.extra_slot_options', JSON_ARRAY('22:30-23:00', '07:00-07:30', '19:00-19:30')
    )
WHERE service_type = 'garbage' AND plan = 'monthly';

UPDATE service_skus
SET rules = JSON_SET(COALESCE(rules, JSON_OBJECT()), '$.monthly_price', 3000, '$.duration_days', 30)
WHERE service_type = 'garbage' AND plan = 'single';
