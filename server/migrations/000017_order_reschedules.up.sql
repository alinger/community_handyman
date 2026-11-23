-- 改期申请与记录。
-- 住户可以直接改（status=applied，留一条记录便于纠纷时追溯）；
-- 服务方只能发起申请（status=pending），需住户确认后才生效。
-- from_* 是申请发起时的原时间，to_* 是目标时间：订单表会被覆盖，
-- 没有这张快照就再也说不清"原本约的是几点"。

CREATE TABLE IF NOT EXISTS order_reschedules (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id     BIGINT UNSIGNED NOT NULL COMMENT '订单 ID',
    order_no     VARCHAR(32)     NOT NULL COMMENT '订单号（冗余，便于直接查询）',
    from_date    DATE            NOT NULL COMMENT '原服务日期',
    from_slot    VARCHAR(16)     NOT NULL COMMENT '原时段',
    to_date      DATE            NOT NULL COMMENT '目标服务日期',
    to_slot      VARCHAR(16)     NOT NULL COMMENT '目标时段',
    reason       VARCHAR(200)    NOT NULL DEFAULT '' COMMENT '改期原因',
    initiated_by VARCHAR(16)     NOT NULL COMMENT '发起方：user 住户 / provider 服务方',
    initiated_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '发起人用户 ID',
    status       VARCHAR(16)     NOT NULL COMMENT 'pending 待确认 / applied 已生效 / rejected 已拒绝 / canceled 已作废',
    handled_at   DATETIME        NULL COMMENT '住户处理时间',
    created_at   DATETIME        NOT NULL,
    updated_at   DATETIME        NOT NULL,
    deleted_at   DATETIME        NULL,
    PRIMARY KEY (id),
    KEY idx_order_reschedules_order (order_id, status),
    KEY idx_order_reschedules_order_no (order_no)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='订单改期申请与记录';
