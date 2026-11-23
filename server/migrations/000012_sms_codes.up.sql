-- 短信验证码：阿里云短信下发与校验的落库记录。
-- 记录既是验证凭据，也是发送频率审计（同号 60 秒一次、每日上限）。
CREATE TABLE IF NOT EXISTS sms_codes (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    phone      VARCHAR(20)  NOT NULL COMMENT '接收手机号',
    scene      VARCHAR(16)  NOT NULL DEFAULT 'login' COMMENT 'login 登录/注册，reset 重置密码',
    code       CHAR(6)      NOT NULL COMMENT '6 位数字验证码',
    status     TINYINT      NOT NULL DEFAULT 0 COMMENT '0 待下发 1 已下发 2 下发失败',
    attempts   TINYINT      NOT NULL DEFAULT 0 COMMENT '校验失败次数，达到上限即作废',
    used       TINYINT      NOT NULL DEFAULT 0 COMMENT '1 表示已使用，不可重复校验',
    expires_at DATETIME     NOT NULL COMMENT '过期时间',
    ip         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '请求来源 IP，便于风控排查',
    created_at DATETIME     NOT NULL,
    updated_at DATETIME     NOT NULL,
    deleted_at DATETIME     NULL,
    PRIMARY KEY (id),
    KEY idx_sms_phone_scene (phone, scene, created_at),
    KEY idx_sms_created_at (created_at),
    KEY idx_sms_deleted_at (deleted_at)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='短信验证码记录';
