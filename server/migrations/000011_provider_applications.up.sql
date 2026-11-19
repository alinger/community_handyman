-- 服务方资质申请与审核
-- 一个用户只有一条记录：驳回后重新提交是更新同一行回 pending，
-- 保证「当前认证状态」唯一可查，不需要在列表里挑最新一条。
CREATE TABLE provider_applications (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id           BIGINT UNSIGNED NOT NULL COMMENT '申请人',
    real_name         VARCHAR(32)  NOT NULL COMMENT '真实姓名',
    phone             VARCHAR(20)  NOT NULL COMMENT '联系手机号',
    service_type      VARCHAR(16)  NOT NULL COMMENT '申请承接的服务类型 garbage/dogwalk/accompany/all',
    experience        VARCHAR(500) NOT NULL DEFAULT '' COMMENT '经验说明',
    id_card_no        VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '身份证号（仅审核展示，脱敏输出）',
    id_card_front_key VARCHAR(255) NOT NULL DEFAULT '' COMMENT '身份证正面 object key',
    id_card_back_key  VARCHAR(255) NOT NULL DEFAULT '' COMMENT '身份证反面 object key',
    health_cert_key   VARCHAR(255) NOT NULL DEFAULT '' COMMENT '健康证 object key',
    status            VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected',
    reviewer_id       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '审核人（后台账号 id）',
    review_note       VARCHAR(200) NOT NULL DEFAULT '' COMMENT '审核意见/驳回原因',
    reviewed_at       DATETIME(3)  NULL COMMENT '审核时间',
    created_at        DATETIME(3)  NOT NULL,
    updated_at        DATETIME(3)  NOT NULL,
    deleted_at        DATETIME(3)  NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_provider_applications_user (user_id),
    KEY idx_provider_applications_status (status),
    KEY idx_provider_applications_deleted_at (deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='服务方资质申请';
